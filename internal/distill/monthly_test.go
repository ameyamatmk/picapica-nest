package distill

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupMonthlyTestReports はテスト用の週次・日次レポートを配置する。
func setupMonthlyTestReports(t *testing.T) (weeklyDir, dailyDir, outputDir, promptPath string) {
	t.Helper()
	base := t.TempDir()
	weeklyDir = filepath.Join(base, "weekly")
	dailyDir = filepath.Join(base, "daily")
	outputDir = filepath.Join(base, "monthly")
	promptDir := filepath.Join(base, "prompts")
	os.MkdirAll(weeklyDir, 0o755)
	os.MkdirAll(dailyDir, 0o755)
	os.MkdirAll(promptDir, 0o755)

	// 2026年2月の週次レポート（金曜が2月内のもの）
	// W07: 金2/13, W08: 金2/20, W09: 金2/27 → 全て2月
	// W10: 金3/6 → 3月なので2月には含まれない
	weeklyReports := map[string]string{
		"2026-02-W07.md": "第7週のサマリ",
		"2026-02-W08.md": "第8週のサマリ",
		"2026-02-W09.md": "第9週のサマリ",
	}
	for name, content := range weeklyReports {
		os.WriteFile(filepath.Join(weeklyDir, name), []byte(content+"\n"), 0o644)
	}

	// 日次レポート（一部の日だけ）
	dailyReports := map[string]string{
		"2026-02-02.md": "2/2の出来事",
		"2026-02-10.md": "2/10の出来事",
		"2026-02-20.md": "2/20の出来事",
		"2026-02-28.md": "2/28の出来事",
	}
	for name, content := range dailyReports {
		os.WriteFile(filepath.Join(dailyDir, name), []byte(content+"\n"), 0o644)
	}

	promptPath = filepath.Join(promptDir, "monthly.md")
	os.WriteFile(promptPath, []byte("Monthly report for {{.Period}}."), 0o644)

	return weeklyDir, dailyDir, outputDir, promptPath
}

func TestRunMonthlyWith_Success(t *testing.T) {
	// Given: 週次3件 + 日次4件とモック LLM
	weeklyDir, dailyDir, outputDir, promptPath := setupMonthlyTestReports(t)

	distiller := mockDistiller("# Monthly Report\n\nFebruary summary.", nil)

	params := MonthlyParams{
		Year:       2026,
		Month:      2,
		WeeklyDir:  weeklyDir,
		DailyDir:   dailyDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunMonthlyWith を呼ぶ
	err := RunMonthlyWith(context.Background(), params, distiller)

	// Then: エラーなしでレポートが生成される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reportPath := filepath.Join(outputDir, "2026-02.md")
	content, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}
	if !strings.Contains(string(content), "Monthly Report") {
		t.Errorf("expected report content, got:\n%s", string(content))
	}
}

func TestRunMonthlyWith_WeeklyOnly(t *testing.T) {
	// Given: 週次のみ存在、日次なし
	base := t.TempDir()
	weeklyDir := filepath.Join(base, "weekly")
	dailyDir := filepath.Join(base, "daily")
	outputDir := filepath.Join(base, "monthly")
	os.MkdirAll(weeklyDir, 0o755)
	os.MkdirAll(dailyDir, 0o755)

	os.WriteFile(filepath.Join(weeklyDir, "2026-02-W07.md"), []byte("Week 7 report\n"), 0o644)

	promptPath := filepath.Join(base, "prompt.md")
	os.WriteFile(promptPath, []byte("{{.Period}}"), 0o644)

	var capturedStdin string
	distiller := func(_ context.Context, _ string, stdin string) (string, error) {
		capturedStdin = stdin
		return "ok", nil
	}

	params := MonthlyParams{
		Year:       2026,
		Month:      2,
		WeeklyDir:  weeklyDir,
		DailyDir:   dailyDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunMonthlyWith を呼ぶ
	err := RunMonthlyWith(context.Background(), params, distiller)

	// Then: 週次セクションだけで実行される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedStdin, "# 週次サマリ") {
		t.Errorf("expected weekly section, got:\n%s", capturedStdin)
	}
	if strings.Contains(capturedStdin, "# 日次レポート") {
		t.Errorf("should not contain daily section, got:\n%s", capturedStdin)
	}
}

func TestRunMonthlyWith_DailyFallback(t *testing.T) {
	// Given: 週次0件、日次のみ存在
	base := t.TempDir()
	weeklyDir := filepath.Join(base, "weekly")
	dailyDir := filepath.Join(base, "daily")
	outputDir := filepath.Join(base, "monthly")
	os.MkdirAll(weeklyDir, 0o755)
	os.MkdirAll(dailyDir, 0o755)

	os.WriteFile(filepath.Join(dailyDir, "2026-02-15.md"), []byte("Mid-month report\n"), 0o644)

	promptPath := filepath.Join(base, "prompt.md")
	os.WriteFile(promptPath, []byte("{{.Period}}"), 0o644)

	var capturedStdin string
	distiller := func(_ context.Context, _ string, stdin string) (string, error) {
		capturedStdin = stdin
		return "ok", nil
	}

	params := MonthlyParams{
		Year:       2026,
		Month:      2,
		WeeklyDir:  weeklyDir,
		DailyDir:   dailyDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunMonthlyWith を呼ぶ
	err := RunMonthlyWith(context.Background(), params, distiller)

	// Then: 日次セクションだけで実行される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(capturedStdin, "# 週次サマリ") {
		t.Errorf("should not contain weekly section, got:\n%s", capturedStdin)
	}
	if !strings.Contains(capturedStdin, "# 日次レポート") {
		t.Errorf("expected daily section, got:\n%s", capturedStdin)
	}
	if !strings.Contains(capturedStdin, "Mid-month report") {
		t.Errorf("expected daily content, got:\n%s", capturedStdin)
	}
}

func TestRunMonthlyWith_NoReports(t *testing.T) {
	// Given: 週次も日次も存在しない
	base := t.TempDir()
	weeklyDir := filepath.Join(base, "weekly")
	dailyDir := filepath.Join(base, "daily")
	outputDir := filepath.Join(base, "monthly")
	os.MkdirAll(weeklyDir, 0o755)
	os.MkdirAll(dailyDir, 0o755)

	promptPath := filepath.Join(base, "prompt.md")
	os.WriteFile(promptPath, []byte("{{.Period}}"), 0o644)

	distiller := mockDistiller("should not be called", nil)

	params := MonthlyParams{
		Year:       2026,
		Month:      2,
		WeeklyDir:  weeklyDir,
		DailyDir:   dailyDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunMonthlyWith を呼ぶ
	err := RunMonthlyWith(context.Background(), params, distiller)

	// Then: エラーなしでスキップ
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reportPath := filepath.Join(outputDir, "2026-02.md")
	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Error("expected no report to be generated")
	}
}

func TestRunMonthlyWith_LLMError(t *testing.T) {
	// Given: LLM がエラーを返す
	weeklyDir, dailyDir, outputDir, promptPath := setupMonthlyTestReports(t)

	distiller := mockDistiller("", errors.New("API error"))

	params := MonthlyParams{
		Year:       2026,
		Month:      2,
		WeeklyDir:  weeklyDir,
		DailyDir:   dailyDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunMonthlyWith を呼ぶ
	err := RunMonthlyWith(context.Background(), params, distiller)

	// Then: LLM エラーが伝播する
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "LLM distillation failed") {
		t.Errorf("expected LLM error, got: %v", err)
	}
}

func TestCollectWeeklyReports_Format(t *testing.T) {
	// Given: 2つの週次レポート（W09 は存在しない）
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "2026-02-W07.md"), []byte("Week 7 content\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "2026-02-W08.md"), []byte("Week 8 content\n"), 0o644)

	// When: collectWeeklyReports を呼ぶ（glob ベース）
	combined, count, err := collectWeeklyReports(dir, 2026, 2)

	// Then: 2件が見出し付きで結合される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 reports, got %d", count)
	}
	if !strings.Contains(combined, "## 第7週") {
		t.Errorf("expected week 7 header, got:\n%s", combined)
	}
	if !strings.Contains(combined, "## 第8週") {
		t.Errorf("expected week 8 header, got:\n%s", combined)
	}
	if !strings.Contains(combined, "Week 7 content") {
		t.Errorf("expected week 7 content, got:\n%s", combined)
	}
}

func TestCollectWeeklyReports_IgnoresOtherMonths(t *testing.T) {
	// Given: 2月と3月のレポートが混在
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "2026-02-W09.md"), []byte("Feb week\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "2026-03-W10.md"), []byte("Mar week\n"), 0o644)

	// When: 2月の週次レポートを収集
	combined, count, err := collectWeeklyReports(dir, 2026, 2)

	// Then: 2月分のみ収集される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 report, got %d", count)
	}
	if !strings.Contains(combined, "Feb week") {
		t.Errorf("expected Feb content, got:\n%s", combined)
	}
	if strings.Contains(combined, "Mar week") {
		t.Errorf("should not contain Mar content, got:\n%s", combined)
	}
}

func TestBuildMonthlyStdin_BothSections(t *testing.T) {
	// Given: 週次と日次の両方がある
	stdin := buildMonthlyStdin("weekly content", 2, "daily content", 3)

	// Then: 両セクションが区切り付きで含まれる
	if !strings.Contains(stdin, "# 週次サマリ") {
		t.Error("expected weekly section header")
	}
	if !strings.Contains(stdin, "---") {
		t.Error("expected separator")
	}
	if !strings.Contains(stdin, "# 日次レポート（詳細参照用）") {
		t.Error("expected daily section header")
	}
}

func TestBuildMonthlyStdin_WeeklyOnly(t *testing.T) {
	// Given: 週次のみ
	stdin := buildMonthlyStdin("weekly content", 1, "", 0)

	// Then: 週次セクションのみ
	if !strings.Contains(stdin, "# 週次サマリ") {
		t.Error("expected weekly section")
	}
	if strings.Contains(stdin, "---") {
		t.Error("should not contain separator")
	}
	if strings.Contains(stdin, "# 日次レポート") {
		t.Error("should not contain daily section")
	}
}

func TestBuildMonthlyStdin_DailyOnly(t *testing.T) {
	// Given: 日次のみ
	stdin := buildMonthlyStdin("", 0, "daily content", 5)

	// Then: 日次セクションのみ
	if strings.Contains(stdin, "# 週次サマリ") {
		t.Error("should not contain weekly section")
	}
	if !strings.Contains(stdin, "# 日次レポート") {
		t.Error("expected daily section")
	}
}
