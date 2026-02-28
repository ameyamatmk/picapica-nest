package hindsight

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mockSummarizer は固定のレスポンスを返す Summarizer。
func mockSummarizer(response string, err error) Summarizer {
	return func(_ context.Context, _ string, _ string) (string, error) {
		return response, err
	}
}

// setupTestLogs はテスト用のログとプロンプトを配置する。
func setupTestLogs(t *testing.T, date time.Time) (logsDir, outputDir, promptPath string) {
	t.Helper()
	base := t.TempDir()
	logsDir = filepath.Join(base, "logs")
	outputDir = filepath.Join(base, "output")
	promptDir := filepath.Join(base, "prompts")

	chDir := filepath.Join(logsDir, "discord_chat-001")
	os.MkdirAll(chDir, 0o755)
	os.MkdirAll(promptDir, 0o755)

	dateStr := date.Format("2006-01-02")
	os.WriteFile(filepath.Join(chDir, dateStr+".jsonl"), []byte(
		`{"ts":"`+dateStr+`T01:00:00Z","dir":"in","sender":"user#1","content":"hello"}
{"ts":"`+dateStr+`T01:00:05Z","dir":"out","content":"hi!"}
`), 0o644)

	promptPath = filepath.Join(promptDir, "daily.md")
	os.WriteFile(promptPath, []byte("Summarize {{.Date}} logs."), 0o644)

	return logsDir, outputDir, promptPath
}

func TestRunDailyWith_Success(t *testing.T) {
	// Given: ログファイルとモック LLM
	date := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)
	logsDir, outputDir, promptPath := setupTestLogs(t, date)

	summarizer := mockSummarizer("# Daily Report\n\nToday was productive.", nil)

	params := DailyParams{
		Date:       date,
		LogsDir:    logsDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunDailyWith を呼ぶ
	err := RunDailyWith(context.Background(), params, summarizer)

	// Then: エラーなしでレポートが生成される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reportPath := filepath.Join(outputDir, "2026-02-27.md")
	content, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}
	if !strings.Contains(string(content), "Daily Report") {
		t.Errorf("expected report content, got:\n%s", string(content))
	}
}

func TestRunDailyWith_NoLogs(t *testing.T) {
	// Given: ログが存在しない日付
	date := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	base := t.TempDir()
	logsDir := filepath.Join(base, "logs")
	outputDir := filepath.Join(base, "output")
	os.MkdirAll(logsDir, 0o755)

	promptPath := filepath.Join(base, "prompt.md")
	os.WriteFile(promptPath, []byte("{{.Date}}"), 0o644)

	summarizer := mockSummarizer("should not be called", nil)

	params := DailyParams{
		Date:       date,
		LogsDir:    logsDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunDailyWith を呼ぶ
	err := RunDailyWith(context.Background(), params, summarizer)

	// Then: エラーなしでスキップ（レポートは生成されない）
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reportPath := filepath.Join(outputDir, "2026-03-01.md")
	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Error("expected no report to be generated")
	}
}

func TestRunDailyWith_LLMError(t *testing.T) {
	// Given: LLM がエラーを返す
	date := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)
	logsDir, outputDir, promptPath := setupTestLogs(t, date)

	summarizer := mockSummarizer("", errors.New("API rate limit exceeded"))

	params := DailyParams{
		Date:       date,
		LogsDir:    logsDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunDailyWith を呼ぶ
	err := RunDailyWith(context.Background(), params, summarizer)

	// Then: LLM エラーが伝播する
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "LLM summarization failed") {
		t.Errorf("expected LLM error, got: %v", err)
	}
}

func TestRunDailyWith_PromptReceivesTranscript(t *testing.T) {
	// Given: LLM に渡される prompt と stdin を記録する summarizer
	date := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)
	logsDir, outputDir, promptPath := setupTestLogs(t, date)

	var capturedPrompt, capturedStdin string
	summarizer := func(_ context.Context, prompt string, stdin string) (string, error) {
		capturedPrompt = prompt
		capturedStdin = stdin
		return "ok", nil
	}

	params := DailyParams{
		Date:       date,
		LogsDir:    logsDir,
		OutputDir:  outputDir,
		PromptPath: promptPath,
	}

	// When: RunDailyWith を呼ぶ
	err := RunDailyWith(context.Background(), params, summarizer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Then: prompt にはテンプレート展開結果、stdin には transcript が渡される
	if !strings.Contains(capturedPrompt, "2026年2月27日") {
		t.Errorf("expected date in prompt, got:\n%s", capturedPrompt)
	}
	if !strings.Contains(capturedStdin, "hello") {
		t.Errorf("expected transcript in stdin, got:\n%s", capturedStdin)
	}
}

func TestFormatDateJP(t *testing.T) {
	date := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	got := formatDateJP(date)
	if got != "2026年2月28日" {
		t.Errorf("expected '2026年2月28日', got %q", got)
	}
}
