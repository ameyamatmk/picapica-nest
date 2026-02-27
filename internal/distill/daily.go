package distill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DailyParams は日次蒸留のパラメータ。
type DailyParams struct {
	Date       time.Time
	LogsDir    string // logs/ ディレクトリのパス
	OutputDir  string // memory/daily/ のパス
	PromptPath string // プロンプトテンプレートのパス
}

// Distiller は LLM 呼び出しの関数型（テスト用に差し替え可能）。
type Distiller func(ctx context.Context, prompt string, stdin string) (string, error)

// RunDaily は日次蒸留を実行する。
// LLM 呼び出しには RunClaude を使用する。
func RunDaily(ctx context.Context, params DailyParams) error {
	return RunDailyWith(ctx, params, RunClaude)
}

// RunDailyWith はテスト可能な日次蒸留の内部実装。
// distill 引数で LLM 呼び出しを差し替え可能。
func RunDailyWith(ctx context.Context, params DailyParams, distill Distiller) error {
	dateStr := params.Date.Format("2006-01-02")

	// 1. ログ収集
	entries, err := CollectLogs(params.LogsDir, params.Date)
	if err != nil {
		return fmt.Errorf("failed to collect logs: %w", err)
	}
	if len(entries) == 0 {
		fmt.Printf("No logs found for %s, skipping.\n", dateStr)
		return nil
	}
	fmt.Printf("Collected %d log entries for %s\n", len(entries), dateStr)

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

	// 4. LLM 蒸留
	fmt.Printf("Running LLM distillation...\n")
	result, err := distill(ctx, prompt, transcript)
	if err != nil {
		return fmt.Errorf("LLM distillation failed: %w", err)
	}

	// 5. 結果保存
	if err := writeReport(params.OutputDir, params.Date, result); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	outputPath := filepath.Join(params.OutputDir, dateStr+".md")
	fmt.Printf("Daily report saved to %s\n", outputPath)
	return nil
}

// writeReport は蒸留結果をファイルに保存する。
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
