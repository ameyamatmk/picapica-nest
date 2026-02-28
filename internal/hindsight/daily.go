package hindsight

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// DailyParams は日次 hindsight のパラメータ。
type DailyParams struct {
	Date       time.Time
	LogsDir    string // logs/ ディレクトリのパス
	OutputDir  string // memory/daily/ のパス
	PromptPath string // プロンプトテンプレートのパス
}

// Summarizer は LLM 呼び出しの関数型（テスト用に差し替え可能）。
type Summarizer func(ctx context.Context, prompt string, stdin string) (string, error)

// RunDaily は日次 hindsight を実行する。
// LLM 呼び出しには RunClaude を使用する。
func RunDaily(ctx context.Context, params DailyParams) error {
	return RunDailyWith(ctx, params, RunClaude)
}

// RunDailyWith はテスト可能な日次 hindsight の内部実装。
// summarize 引数で LLM 呼び出しを差し替え可能。
func RunDailyWith(ctx context.Context, params DailyParams, summarize Summarizer) error {
	dateStr := params.Date.Format("2006-01-02")

	// 1. ログ収集
	entries, err := CollectLogs(params.LogsDir, params.Date)
	if err != nil {
		return fmt.Errorf("failed to collect logs: %w", err)
	}
	if len(entries) == 0 {
		slog.Info("no logs found, skipping", "component", "hindsight", "date", dateStr)
		return nil
	}
	slog.Info("collected log entries", "component", "hindsight", "count", len(entries), "date", dateStr)

	// 2. transcript 生成
	transcript := FormatTranscript(entries)

	// 3. プロンプト読み込み
	promptData := PromptData{
		Date: formatDateJP(params.Date),
	}
	prompt, err := LoadPrompt(params.PromptPath, promptData)
	if err != nil {
		return fmt.Errorf("failed to load prompt: %w", err)
	}

	// 4. LLM summarization
	slog.Info("running LLM summarization", "component", "hindsight")
	result, err := summarize(ctx, prompt, transcript)
	if err != nil {
		return fmt.Errorf("LLM summarization failed: %w", err)
	}

	// 5. 結果保存
	if err := writeReport(params.OutputDir, params.Date, result); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	outputPath := filepath.Join(params.OutputDir, dateStr+".md")
	slog.Info("daily report saved", "component", "hindsight", "path", outputPath)
	return nil
}

// writeReport は hindsight 結果をファイルに保存する。
func writeReport(outputDir string, date time.Time, content string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fileName := date.Format("2006-01-02") + ".md"
	filePath := filepath.Join(outputDir, fileName)

	return os.WriteFile(filePath, []byte(content+"\n"), 0o644)
}

// formatDateJP は日付を日本語形式に変換する。
func formatDateJP(date time.Time) string {
	return fmt.Sprintf("%d年%d月%d日", date.Year(), date.Month(), date.Day())
}
