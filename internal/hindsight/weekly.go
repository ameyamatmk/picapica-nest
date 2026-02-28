package hindsight

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
)

// 日本語の曜日名
var weekdayJP = [...]string{
	time.Sunday:    "日",
	time.Monday:    "月",
	time.Tuesday:   "火",
	time.Wednesday: "水",
	time.Thursday:  "木",
	time.Friday:    "金",
	time.Saturday:  "土",
}

// WeeklyParams は週次 hindsight のパラメータ。
type WeeklyParams struct {
	Year       int
	Week       int    // ISO 週番号
	DailyDir   string // memory/daily/ のパス
	OutputDir  string // memory/weekly/ のパス
	PromptPath string // プロンプトテンプレートのパス
}

// RunWeekly は週次 hindsight を実行する。
func RunWeekly(ctx context.Context, params WeeklyParams) error {
	return RunWeeklyWith(ctx, params, RunClaude)
}

// RunWeeklyWith はテスト可能な週次 hindsight の内部実装。
func RunWeeklyWith(ctx context.Context, params WeeklyParams, summarize Summarizer) error {
	weekLabel := fmt.Sprintf("%d-W%02d", params.Year, params.Week)

	// 1. 週の日付範囲を計算（土曜〜金曜）
	start := weekStartSat(params.Year, params.Week)
	end := weekEndFri(params.Year, params.Week)

	// 2. 日次レポートを収集・結合
	combined, count, err := collectDailyReports(params.DailyDir, start, end)
	if err != nil {
		return fmt.Errorf("failed to collect daily reports: %w", err)
	}
	if count == 0 {
		slog.Info("no daily reports found, skipping", "component", "hindsight", "week", weekLabel)
		return nil
	}
	slog.Info("collected daily reports", "component", "hindsight", "count", count, "week", weekLabel)

	// 3. プロンプト読み込み
	period := formatPeriodRange(start, end)
	promptData := PromptData{Period: period}
	prompt, err := LoadPrompt(params.PromptPath, promptData)
	if err != nil {
		return fmt.Errorf("failed to load prompt: %w", err)
	}

	// 4. LLM summarization
	slog.Info("running LLM summarization", "component", "hindsight")
	result, err := summarize(ctx, prompt, combined)
	if err != nil {
		return fmt.Errorf("LLM summarization failed: %w", err)
	}

	// 5. 結果保存
	fileName := weekFileName(params.Year, params.Week)
	if err := writeReportFile(params.OutputDir, fileName, result); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	outputPath := filepath.Join(params.OutputDir, fileName)
	slog.Info("weekly report saved", "component", "hindsight", "path", outputPath)
	return nil
}

// collectDailyReports は日付範囲の日次レポートを読み込み、見出し付き Markdown に結合する。
// 戻り値は (結合文字列, レポート数, error)。
func collectDailyReports(dailyDir string, start, end time.Time) (string, int, error) {
	var sb strings.Builder
	count := 0

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		fileName := d.Format("2006-01-02") + ".md"
		path := filepath.Join(dailyDir, fileName)

		content, err := readReportFile(path)
		if err != nil {
			return "", 0, err
		}
		if content == "" {
			continue
		}

		if count > 0 {
			sb.WriteString("\n\n")
		}

		// 見出し: "# 2026年2月23日（月）"
		header := fmt.Sprintf("# %s（%s）", formatDateJP(d), weekdayJP[d.Weekday()])
		sb.WriteString(header)
		sb.WriteString("\n\n")
		sb.WriteString(content)
		count++
	}

	return sb.String(), count, nil
}

// formatPeriodRange は日付範囲を日本語文字列に変換する。
// 例: "2026年2月23日〜3月1日"
func formatPeriodRange(start, end time.Time) string {
	startStr := formatDateJP(start)
	// 同じ年・月なら省略形
	if start.Year() == end.Year() && start.Month() == end.Month() {
		return fmt.Sprintf("%s〜%d日", startStr, end.Day())
	}
	if start.Year() == end.Year() {
		return fmt.Sprintf("%s〜%d月%d日", startStr, end.Month(), end.Day())
	}
	return fmt.Sprintf("%s〜%s", startStr, formatDateJP(end))
}
