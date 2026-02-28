package console

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ameyamatmk/picapica-nest/internal/pricing"
)

// usageRecord は usage.jsonl の1行分。
// provider.UsageRecord と同じ JSON タグだが依存を切るために独立定義。
type usageRecord struct {
	Timestamp        string `json:"ts"`
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	LatencyMs        int64  `json:"latency_ms"`
	Error            string `json:"error,omitempty"`
}

// DailyUsage は日別の集計結果。
type DailyUsage struct {
	Date             string
	CallCount        int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	ErrorCount       int
	AvgLatencyMs     int64
	CostUSD          float64
	CostJPY          *float64
}

// usagePeriod は期間選択の選択肢。
type usagePeriod struct {
	Label  string
	Value  string
	Active bool
}

// usageData は Usage 画面のテンプレートデータ。
type usageData struct {
	pageData
	Days         []DailyUsage
	Error        string
	Periods      []usagePeriod
	TotalCostUSD float64
	TotalCostJPY *float64
}

// modelTokens はモデル別のトークン数。
type modelTokens struct {
	promptTokens     int
	completionTokens int
}

// dailyAccumulator は日別集計の中間データ。
type dailyAccumulator struct {
	date             string
	callCount        int
	promptTokens     int
	completionTokens int
	totalTokens      int
	errorCount       int
	totalLatencyMs   int64
	byModel          map[string]*modelTokens
}

func (a *dailyAccumulator) add(rec usageRecord) {
	a.callCount++
	a.promptTokens += rec.PromptTokens
	a.completionTokens += rec.CompletionTokens
	a.totalTokens += rec.TotalTokens
	a.totalLatencyMs += rec.LatencyMs
	if rec.Error != "" {
		a.errorCount++
	}
	if rec.Model != "" {
		if a.byModel == nil {
			a.byModel = make(map[string]*modelTokens)
		}
		mt, ok := a.byModel[rec.Model]
		if !ok {
			mt = &modelTokens{}
			a.byModel[rec.Model] = mt
		}
		mt.promptTokens += rec.PromptTokens
		mt.completionTokens += rec.CompletionTokens
	}
}

func (a *dailyAccumulator) toDailyUsage(pricer *pricing.Pricer) DailyUsage {
	avg := int64(0)
	if a.callCount > 0 {
		avg = a.totalLatencyMs / int64(a.callCount)
	}
	du := DailyUsage{
		Date:             a.date,
		CallCount:        a.callCount,
		PromptTokens:     a.promptTokens,
		CompletionTokens: a.completionTokens,
		TotalTokens:      a.totalTokens,
		ErrorCount:       a.errorCount,
		AvgLatencyMs:     avg,
	}
	if pricer != nil {
		for model, mt := range a.byModel {
			usd, jpy := pricer.CalcDailyCost(model, mt.promptTokens, mt.completionTokens)
			du.CostUSD += usd
			if jpy != nil {
				if du.CostJPY == nil {
					v := 0.0
					du.CostJPY = &v
				}
				*du.CostJPY += *jpy
			}
		}
	}
	return du
}

// periodOptions は期間選択の選択肢を定義する。
var periodOptions = []struct {
	label string
	value string
	days  int // 0 は全期間
}{
	{"7日", "7d", 7},
	{"30日", "30d", 30},
	{"90日", "90d", 90},
	{"全期間", "all", 0},
}

// handleUsage は Usage 画面をフルページで返す。
func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	days, err := loadUsage(s.workspacePath, s.pricer)

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	// 期間フィルタリング
	filtered := filterByPeriod(days, period)

	// 期間合計コスト算出
	var totalUSD float64
	var totalJPY *float64
	for _, d := range filtered {
		totalUSD += d.CostUSD
		if d.CostJPY != nil {
			if totalJPY == nil {
				v := 0.0
				totalJPY = &v
			}
			*totalJPY += *d.CostJPY
		}
	}

	// 期間選択タブ用データ
	periods := make([]usagePeriod, len(periodOptions))
	for i, opt := range periodOptions {
		periods[i] = usagePeriod{
			Label:  opt.label,
			Value:  opt.value,
			Active: opt.value == period,
		}
	}

	data := usageData{
		pageData: pageData{
			Title:  "Usage",
			Active: "usage",
		},
		Days:         filtered,
		Periods:      periods,
		TotalCostUSD: totalUSD,
		TotalCostJPY: totalJPY,
	}
	if err != nil {
		data.Error = err.Error()
		slog.Error("failed to load usage", "component", "console", "error", err)
	}

	if err := s.usageTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("failed to render usage page", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// filterByPeriod は指定期間のデータだけを返す。
func filterByPeriod(days []DailyUsage, period string) []DailyUsage {
	var numDays int
	for _, opt := range periodOptions {
		if opt.value == period {
			numDays = opt.days
			break
		}
	}
	if numDays == 0 {
		return days
	}

	cutoff := time.Now().AddDate(0, 0, -numDays).Format("2006-01-02")
	var filtered []DailyUsage
	for _, d := range days {
		if d.Date >= cutoff {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// loadUsage は usage.jsonl を読み込み、日別に集計して降順で返す。
// pricer が nil の場合、コストは 0 になる。
func loadUsage(workspacePath string, pricer *pricing.Pricer) ([]DailyUsage, error) {
	usagePath := filepath.Join(workspacePath, "usage.jsonl")
	f, err := os.Open(usagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	dayMap := make(map[string]*dailyAccumulator)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec usageRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}

		date := extractDate(rec.Timestamp)
		if date == "" {
			continue
		}

		acc, ok := dayMap[date]
		if !ok {
			acc = &dailyAccumulator{date: date}
			dayMap[date] = acc
		}
		acc.add(rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	days := make([]DailyUsage, 0, len(dayMap))
	for _, acc := range dayMap {
		days = append(days, acc.toDailyUsage(pricer))
	}
	slices.SortFunc(days, func(a, b DailyUsage) int {
		return strings.Compare(b.Date, a.Date)
	})
	return days, nil
}

// extractDate はタイムスタンプ文字列から日付部分（YYYY-MM-DD）を抽出する。
func extractDate(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		// RFC3339 でなくても先頭10文字が日付ならそのまま使う
		if len(ts) >= 10 {
			return ts[:10]
		}
		return ""
	}
	return t.Format("2006-01-02")
}
