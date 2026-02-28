package console

import (
	"bufio"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// latestReport は蒸留レポートの最新ファイル情報。
type latestReport struct {
	Tab      string // "daily", "weekly", "monthly"
	TabLabel string // "日次", "週次", "月次"
	FileName string
	Label    string // ファイル名から拡張子除去
	Preview  string // 最初の段落（プレーンテキスト、100文字程度で切る）
}

// usageSummary は Usage のサマリデータ。
type usageSummary struct {
	TodayCalls  int
	TodayTokens int
	WeekCalls   int
	WeekTokens  int
}

// dashboardData はダッシュボード画面のテンプレートデータ。
type dashboardData struct {
	pageData
	LatestReports []latestReport // 日次/週次/月次の最新レポート
	UsageSummary  *usageSummary  // Usage サマリ
}

// handleDashboard はダッシュボード画面をフルページで返す。
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data := s.buildDashboardData()

	if err := s.dashboardTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("failed to render dashboard page", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// buildDashboardData はダッシュボードに表示するデータを構築する。
func (s *Server) buildDashboardData() dashboardData {
	data := dashboardData{
		pageData: pageData{
			Title:  "ダッシュボード",
			Active: "dashboard",
		},
	}

	// 蒸留レポートの最新ファイルを取得
	data.LatestReports = s.buildLatestReports()

	// Usage サマリを取得
	summary, err := buildUsageSummary(s.workspacePath)
	if err != nil {
		slog.Error("failed to build usage summary", "component", "console", "error", err)
	}
	data.UsageSummary = summary

	return data
}

// tabInfo はタブ名とラベルの対応。
type tabInfo struct {
	tab   string
	label string
}

// buildLatestReports は日次/週次/月次それぞれの最新レポートを取得する。
func (s *Server) buildLatestReports() []latestReport {
	tabs := []tabInfo{
		{"daily", "日次"},
		{"weekly", "週次"},
		{"monthly", "月次"},
	}

	var reports []latestReport
	for _, t := range tabs {
		report := latestReport{
			Tab:      t.tab,
			TabLabel: t.label,
		}

		files, err := listReports(s.workspacePath, t.tab)
		if err != nil {
			slog.Error("failed to list reports for dashboard",
				"component", "console", "tab", t.tab, "error", err)
			reports = append(reports, report)
			continue
		}

		if len(files) == 0 {
			reports = append(reports, report)
			continue
		}

		// 最新ファイル（降順ソート済みなので先頭）
		latest := files[0]
		report.FileName = latest.Name
		report.Label = latest.Label

		// プレビューを取得
		preview, err := readReportPreview(s.workspacePath, t.tab, latest.Name)
		if err != nil {
			slog.Error("failed to read report preview",
				"component", "console", "tab", t.tab, "file", latest.Name, "error", err)
		}
		report.Preview = preview

		reports = append(reports, report)
	}

	return reports
}

// maxPreviewLen はレポートプレビューの最大文字数。
const maxPreviewLen = 100

// readReportPreview はレポートファイルの最初の段落をプレーンテキストで返す。
// 見出し行（# で始まる行）はスキップし、最初の非空段落を返す。
func readReportPreview(workspacePath, tab, fileName string) (string, error) {
	safeName := filepath.Base(fileName)
	if safeName != fileName || !strings.HasSuffix(safeName, ".md") {
		return "", nil
	}

	path := filepath.Join(reportDir(workspacePath, tab), safeName)
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var paragraph strings.Builder
	inParagraph := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 見出し行はスキップ
		if strings.HasPrefix(line, "#") {
			if inParagraph {
				break // 段落の途中で見出しが出たら終了
			}
			continue
		}

		// 空行は段落の区切り
		if line == "" {
			if inParagraph {
				break // 段落が終わった
			}
			continue
		}

		// 段落のテキストを蓄積
		if inParagraph {
			paragraph.WriteString(" ")
		}
		paragraph.WriteString(line)
		inParagraph = true
	}

	result := paragraph.String()
	if len([]rune(result)) > maxPreviewLen {
		result = string([]rune(result)[:maxPreviewLen]) + "..."
	}

	return result, scanner.Err()
}

// buildUsageSummary は usage.jsonl から当日と直近7日間のサマリを構築する。
func buildUsageSummary(workspacePath string) (*usageSummary, error) {
	days, err := loadUsage(workspacePath, nil)
	if err != nil {
		return nil, err
	}

	summary := &usageSummary{}
	if len(days) == 0 {
		return summary, nil
	}

	today := time.Now().Format("2006-01-02")
	weekAgo := time.Now().AddDate(0, 0, -6).Format("2006-01-02") // 当日含む7日間

	for _, d := range days {
		if d.Date == today {
			summary.TodayCalls = d.CallCount
			summary.TodayTokens = d.TotalTokens
		}
		if d.Date >= weekAgo {
			summary.WeekCalls += d.CallCount
			summary.WeekTokens += d.TotalTokens
		}
	}

	return summary, nil
}

