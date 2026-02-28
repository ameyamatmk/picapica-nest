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
}

// usageData は Usage 画面のテンプレートデータ。
type usageData struct {
	pageData
	Days  []DailyUsage
	Error string
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
}

func (a *dailyAccumulator) toDailyUsage() DailyUsage {
	avg := int64(0)
	if a.callCount > 0 {
		avg = a.totalLatencyMs / int64(a.callCount)
	}
	return DailyUsage{
		Date:             a.date,
		CallCount:        a.callCount,
		PromptTokens:     a.promptTokens,
		CompletionTokens: a.completionTokens,
		TotalTokens:      a.totalTokens,
		ErrorCount:       a.errorCount,
		AvgLatencyMs:     avg,
	}
}

// handleUsage は Usage 画面をフルページで返す。
func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	days, err := loadUsage(s.workspacePath)

	data := usageData{
		pageData: pageData{
			Title:  "Usage",
			Active: "usage",
		},
		Days: days,
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

// loadUsage は usage.jsonl を読み込み、日別に集計して降順で返す。
func loadUsage(workspacePath string) ([]DailyUsage, error) {
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
		days = append(days, acc.toDailyUsage())
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
