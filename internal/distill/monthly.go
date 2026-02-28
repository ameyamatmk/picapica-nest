package distill

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MonthlyParams は月次蒸留のパラメータ。
type MonthlyParams struct {
	Year       int
	Month      int    // 1-12
	WeeklyDir  string // memory/weekly/ のパス
	DailyDir   string // memory/daily/ のパス
	OutputDir  string // memory/monthly/ のパス
	PromptPath string // プロンプトテンプレートのパス
}

// RunMonthly は月次蒸留を実行する。
func RunMonthly(ctx context.Context, params MonthlyParams) error {
	return RunMonthlyWith(ctx, params, RunClaude)
}

// RunMonthlyWith はテスト可能な月次蒸留の内部実装。
func RunMonthlyWith(ctx context.Context, params MonthlyParams, distill Distiller) error {
	monthLabel := fmt.Sprintf("%d-%02d", params.Year, params.Month)
	month := time.Month(params.Month)

	// 1. 週次レポートを収集・結合（主）
	// ファイル名パターン YYYY-MM-W*.md で当月の作成済みレポートを発見する。
	weeklyCombined, weeklyCount, err := collectWeeklyReports(params.WeeklyDir, params.Year, params.Month)
	if err != nil {
		return fmt.Errorf("failed to collect weekly reports: %w", err)
	}

	// 2. 当月の日次レポートを収集・結合（参照）
	firstDay := time.Date(params.Year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)
	dailyCombined, dailyCount, err := collectDailyReports(params.DailyDir, firstDay, lastDay)
	if err != nil {
		return fmt.Errorf("failed to collect daily reports: %w", err)
	}

	// 3. 両方0件ならスキップ
	if weeklyCount == 0 && dailyCount == 0 {
		slog.Info("no reports found, skipping", "component", "distill", "month", monthLabel)
		return nil
	}
	slog.Info("collected reports", "component", "distill", "weekly_count", weeklyCount, "daily_count", dailyCount, "month", monthLabel)

	// 4. stdin 組み立て（週次 + 区切り + 日次）
	stdin := buildMonthlyStdin(weeklyCombined, weeklyCount, dailyCombined, dailyCount)

	// 5. プロンプト読み込み
	period := fmt.Sprintf("%d年%d月", params.Year, params.Month)
	promptData := PromptData{Period: period}
	prompt, err := LoadPrompt(params.PromptPath, promptData)
	if err != nil {
		return fmt.Errorf("failed to load prompt: %w", err)
	}

	// 6. LLM 蒸留
	slog.Info("running LLM distillation", "component", "distill")
	result, err := distill(ctx, prompt, stdin)
	if err != nil {
		return fmt.Errorf("LLM distillation failed: %w", err)
	}

	// 7. 結果保存
	fileName := monthLabel + ".md"
	if err := writeReportFile(params.OutputDir, fileName, result); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	outputPath := filepath.Join(params.OutputDir, fileName)
	slog.Info("monthly report saved", "component", "distill", "path", outputPath)
	return nil
}

// collectWeeklyReports は指定月の週次レポートをファイル名パターンで収集し、
// 見出し付き Markdown に結合する。
// ファイル名の形式 YYYY-MM-WNN.md から当月の作成済みレポートを発見する。
func collectWeeklyReports(weeklyDir string, year, month int) (string, int, error) {
	pattern := filepath.Join(weeklyDir, fmt.Sprintf("%d-%02d-W*.md", year, month))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", 0, fmt.Errorf("failed to glob weekly reports: %w", err)
	}
	sort.Strings(matches)

	var sb strings.Builder
	count := 0

	for _, path := range matches {
		content, err := readReportFile(path)
		if err != nil {
			return "", 0, err
		}
		if content == "" {
			continue
		}

		// ファイル名から ISO 年と週番号を抽出
		base := filepath.Base(path)
		var isoYear, monthNum, weekNum int
		if _, err := fmt.Sscanf(base, "%d-%d-W%d.md", &isoYear, &monthNum, &weekNum); err != nil {
			continue
		}

		if count > 0 {
			sb.WriteString("\n\n")
		}

		// 見出し: "## 第9週 (2/21〜2/27)"
		start := weekStartSat(isoYear, weekNum)
		end := weekEndFri(isoYear, weekNum)
		header := fmt.Sprintf("## 第%d週 (%d/%d〜%d/%d)",
			weekNum, start.Month(), start.Day(), end.Month(), end.Day())
		sb.WriteString(header)
		sb.WriteString("\n\n")
		sb.WriteString(content)
		count++
	}

	return sb.String(), count, nil
}

// buildMonthlyStdin は週次セクションと日次セクションを組み立てる。
func buildMonthlyStdin(weeklyCombined string, weeklyCount int, dailyCombined string, dailyCount int) string {
	var sb strings.Builder

	if weeklyCount > 0 {
		sb.WriteString("# 週次サマリ\n\n")
		sb.WriteString(weeklyCombined)
	}

	if weeklyCount > 0 && dailyCount > 0 {
		sb.WriteString("\n\n---\n\n")
	}

	if dailyCount > 0 {
		sb.WriteString("# 日次レポート（詳細参照用）\n\n")
		sb.WriteString(dailyCombined)
	}

	return sb.String()
}
