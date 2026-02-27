package distill

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupWeeklyTestReports はテスト用の日次レポートを配置する。
// 2026-W09 (2/23〜3/1) のうち 5日分のレポートを作成する。
func setupWeeklyTestReports(t *testing.T) (dailyDir, outputDir, promptPath string) {
	t.Helper()
	base := t.TempDir()
	dailyDir = filepath.Join(base, "daily")
	outputDir = filepath.Join(base, "weekly")
	promptDir := filepath.Join(base, "prompts")
	os.MkdirAll(dailyDir, 0o755)
	os.MkdirAll(promptDir, 0o755)

	// 5日分のレポートを配置（2/25, 2/26 はスキップ）
	reports := map[string]string{
		"2026-02-23.md": "# トピック1\n\n月曜の活動内容",
		"2026-02-24.md": "# トピック2\n\n火曜の活動内容",
		// 2/25, 2/26 はなし
		"2026-02-27.md": "# トピック3\n\n金曜の活動内容",
		"2026-02-28.md": "# トピック4\n\n土曜の活動内容",
		"2026-03-01.md": "# トピック5\n\n日曜の活動内容",
	}
	for name, content := range reports {
		os.WriteFile(filepath.Join(dailyDir, name), []byte(content+"\n"), 0o644)
	}

	promptPath = filepath.Join(promptDir, "weekly.md")
	os.WriteFile(promptPath, []byte("Summarize {{.Period}} reports."), 0o644)

	return dailyDir, outputDir, promptPath
}

func TestRunWeeklyWith_Success(t *testing.T) {
	// Given: 5日分の日次レポートとモック LLM
	dailyDir, outputDir, promptPath := setupWeeklyTestReports(t)

	distiller := mockDistiller("# Weekly Report\n\nA productive week.", nil)

	params := WeeklyParams{
		Year:       2026,
		Week:       9,
		DailyDir:   dailyDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunWeeklyWith を呼ぶ
	err := RunWeeklyWith(context.Background(), params, distiller)

	// Then: エラーなしでレポートが生成される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reportPath := filepath.Join(outputDir, "2026-W09.md")
	content, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}
	if !strings.Contains(string(content), "Weekly Report") {
		t.Errorf("expected report content, got:\n%s", string(content))
	}
}

func TestRunWeeklyWith_NoReports(t *testing.T) {
	// Given: 日次レポートが存在しない
	base := t.TempDir()
	dailyDir := filepath.Join(base, "daily")
	outputDir := filepath.Join(base, "weekly")
	os.MkdirAll(dailyDir, 0o755)

	promptPath := filepath.Join(base, "prompt.md")
	os.WriteFile(promptPath, []byte("{{.Period}}"), 0o644)

	distiller := mockDistiller("should not be called", nil)

	params := WeeklyParams{
		Year:       2026,
		Week:       9,
		DailyDir:   dailyDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunWeeklyWith を呼ぶ
	err := RunWeeklyWith(context.Background(), params, distiller)

	// Then: エラーなしでスキップ
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reportPath := filepath.Join(outputDir, "2026-W09.md")
	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Error("expected no report to be generated")
	}
}

func TestRunWeeklyWith_LLMError(t *testing.T) {
	// Given: LLM がエラーを返す
	dailyDir, outputDir, promptPath := setupWeeklyTestReports(t)

	distiller := mockDistiller("", errors.New("API error"))

	params := WeeklyParams{
		Year:       2026,
		Week:       9,
		DailyDir:   dailyDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunWeeklyWith を呼ぶ
	err := RunWeeklyWith(context.Background(), params, distiller)

	// Then: LLM エラーが伝播する
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "LLM distillation failed") {
		t.Errorf("expected LLM error, got: %v", err)
	}
}

func TestRunWeeklyWith_PromptReceivesPeriod(t *testing.T) {
	// Given: prompt と stdin を記録する distiller
	dailyDir, outputDir, promptPath := setupWeeklyTestReports(t)

	var capturedPrompt, capturedStdin string
	distiller := func(_ context.Context, prompt string, stdin string) (string, error) {
		capturedPrompt = prompt
		capturedStdin = stdin
		return "ok", nil
	}

	params := WeeklyParams{
		Year:       2026,
		Week:       9,
		DailyDir:   dailyDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunWeeklyWith を呼ぶ
	err := RunWeeklyWith(context.Background(), params, distiller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Then: prompt に期間、stdin に日次レポートが含まれる
	if !strings.Contains(capturedPrompt, "2026年2月23日〜3月1日") {
		t.Errorf("expected period in prompt, got:\n%s", capturedPrompt)
	}
	if !strings.Contains(capturedStdin, "月曜の活動内容") {
		t.Errorf("expected daily content in stdin, got:\n%s", capturedStdin)
	}
}

func TestCollectDailyReports_Format(t *testing.T) {
	// Given: 2日分の日次レポート
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "2026-02-23.md"), []byte("Report A\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "2026-02-24.md"), []byte("Report B\n"), 0o644)

	start := isoWeekStart(2026, 9)  // 2/23
	end := isoWeekEnd(2026, 9)      // 3/1

	// When: collectDailyReports を呼ぶ
	combined, count, err := collectDailyReports(dir, start, end)

	// Then: 2件のレポートが見出し付きで結合される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 reports, got %d", count)
	}
	if !strings.Contains(combined, "# 2026年2月23日（月）") {
		t.Errorf("expected Monday header, got:\n%s", combined)
	}
	if !strings.Contains(combined, "# 2026年2月24日（火）") {
		t.Errorf("expected Tuesday header, got:\n%s", combined)
	}
	if !strings.Contains(combined, "Report A") || !strings.Contains(combined, "Report B") {
		t.Errorf("expected report contents, got:\n%s", combined)
	}
}

func TestFormatPeriodRange_SameMonth(t *testing.T) {
	// Given: 同月内の範囲
	start := isoWeekStart(2026, 5) // 2026-01-26
	end := isoWeekEnd(2026, 5)     // 2026-02-01

	// When: formatPeriodRange を呼ぶ
	got := formatPeriodRange(start, end)

	// Then: 月をまたぐので "1月26日〜2月1日" 形式
	if !strings.Contains(got, "1月26日") || !strings.Contains(got, "2月1日") {
		t.Errorf("unexpected period: %s", got)
	}
}

func TestFormatPeriodRange_CrossMonth(t *testing.T) {
	// Given: 月をまたぐ範囲（W09: 2/23〜3/1）
	start := isoWeekStart(2026, 9)
	end := isoWeekEnd(2026, 9)

	// When: formatPeriodRange を呼ぶ
	got := formatPeriodRange(start, end)

	// Then: "2026年2月23日〜3月1日"
	expected := "2026年2月23日〜3月1日"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
