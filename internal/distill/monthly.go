package distill

import (
	"context"
	"fmt"
	"path/filepath"
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

// isoWeek は ISO 週の年と週番号のペア。
type isoWeek struct {
	Year int
	Week int
}

// RunMonthly は月次蒸留を実行する。
func RunMonthly(ctx context.Context, params MonthlyParams) error {
	return RunMonthlyWith(ctx, params, RunClaude)
}

// RunMonthlyWith はテスト可能な月次蒸留の内部実装。
func RunMonthlyWith(ctx context.Context, params MonthlyParams, distill Distiller) error {
	monthLabel := fmt.Sprintf("%d-%02d", params.Year, params.Month)
	month := time.Month(params.Month)

	// 1. 当月に属する ISO 週を列挙
	weeks := weeksInMonth(params.Year, params.Month)

	// 2. 週次レポートを収集・結合（主）
	weeklyCombined, weeklyCount, err := collectWeeklyReports(params.WeeklyDir, weeks)
	if err != nil {
		return fmt.Errorf("failed to collect weekly reports: %w", err)
	}

	// 3. 当月の日次レポートを収集・結合（参照）
	firstDay := time.Date(params.Year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)
	dailyCombined, dailyCount, err := collectDailyReports(params.DailyDir, firstDay, lastDay)
	if err != nil {
		return fmt.Errorf("failed to collect daily reports: %w", err)
	}

	// 4. 両方0件ならスキップ
	if weeklyCount == 0 && dailyCount == 0 {
		fmt.Printf("No reports found for %s, skipping.\n", monthLabel)
		return nil
	}
	fmt.Printf("Collected %d weekly + %d daily reports for %s\n", weeklyCount, dailyCount, monthLabel)

	// 5. stdin 組み立て（週次 + 区切り + 日次）
	stdin := buildMonthlyStdin(weeklyCombined, weeklyCount, dailyCombined, dailyCount)

	// 6. プロンプト読み込み
	period := fmt.Sprintf("%d年%d月", params.Year, params.Month)
	promptData := PromptData{Period: period}
	prompt, err := LoadPrompt(params.PromptPath, promptData)
	if err != nil {
		return fmt.Errorf("failed to load prompt: %w", err)
	}

	// 7. LLM 蒸留
	fmt.Printf("Running LLM distillation...\n")
	result, err := distill(ctx, prompt, stdin)
	if err != nil {
		return fmt.Errorf("LLM distillation failed: %w", err)
	}

	// 8. 結果保存
	fileName := monthLabel + ".md"
	if err := writeReportFile(params.OutputDir, fileName, result); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	outputPath := filepath.Join(params.OutputDir, fileName)
	fmt.Printf("Monthly report saved to %s\n", outputPath)
	return nil
}

// weeksInMonth は指定月に属する ISO 週を返す。
// 「土曜日（週の開始日）が当月内にある週」を対象とする。
func weeksInMonth(year, month int) []isoWeek {
	m := time.Month(month)
	firstDay := time.Date(year, m, 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	seen := make(map[isoWeek]bool)
	var weeks []isoWeek

	for d := firstDay; !d.After(lastDay); d = d.AddDate(0, 0, 1) {
		// 土曜日（週の開始日）のみチェック
		if d.Weekday() != time.Saturday {
			continue
		}
		// この土曜日を含む週の ISO 週番号を取得
		// 土曜日+2日 = 月曜日 の ISOWeek を使う
		monday := d.AddDate(0, 0, 2)
		isoY, isoW := monday.ISOWeek()
		w := isoWeek{Year: isoY, Week: isoW}
		if !seen[w] {
			seen[w] = true
			weeks = append(weeks, w)
		}
	}

	return weeks
}

// collectWeeklyReports は週次レポートを読み込み、見出し付き Markdown に結合する。
func collectWeeklyReports(weeklyDir string, weeks []isoWeek) (string, int, error) {
	var sb strings.Builder
	count := 0

	for _, w := range weeks {
		fileName := weekFileName(w.Year, w.Week)
		path := filepath.Join(weeklyDir, fileName)

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

		// 見出し: "## 第9週 (2/21〜2/27)"
		start := weekStartSat(w.Year, w.Week)
		end := weekEndFri(w.Year, w.Week)
		header := fmt.Sprintf("## 第%d週 (%d/%d〜%d/%d)",
			w.Week, start.Month(), start.Day(), end.Month(), end.Day())
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
