package claudecode

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// StreamEvent は CLI の stream-json から抽出したイベント。
type StreamEvent struct {
	Type     string // "tool_use" | "result"
	ToolName string // tool_use 時のツール名
	Content  string // result 時の最終テキスト
}

// ProgressFunc はストリーミング中にイベントを受け取るコールバック。
type ProgressFunc func(event StreamEvent)

// WithProgress はストリーミングモードを有効化する Option。
// fn が非 nil の場合、CLI を --output-format stream-json で実行し、
// tool_use イベントを検知するたびに fn を呼び出す。
func WithProgress(fn ProgressFunc) Option {
	return func(c *runConfig) {
		c.progress = fn
	}
}

// stream-json パース用の内部型

type streamLine struct {
	Type    string     `json:"type"`
	Subtype string     `json:"subtype,omitempty"`
	Message *streamMsg `json:"message,omitempty"`
	Result  string     `json:"result,omitempty"`
}

type streamMsg struct {
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// runStream は --output-format stream-json で CLI を実行し、
// イベントをパースして progress コールバックに通知する。
// 最終結果テキストを返す。
func runStream(ctx context.Context, cfg *runConfig) (string, error) {
	model := cfg.model
	if model == "" {
		model = DefaultModel
	}

	args := []string{"-p", cfg.prompt, "--model", model, "--output-format", "stream-json"}
	if len(cfg.allowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(cfg.allowedTools, ","))
	}
	if cfg.appendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", cfg.appendSystemPrompt)
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	if cfg.stdin != "" {
		cmd.Stdin = strings.NewReader(cfg.stdin)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// stderr はエラー時に参照する
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start claude: %w", err)
	}

	var resultText string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		result, ok := parseLine(line)
		if !ok {
			continue
		}

		switch result.Type {
		case "tool_use":
			if cfg.progress != nil {
				cfg.progress(result)
			}
		case "result":
			resultText = result.Content
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("stream scanner error", "error", err)
	}

	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("claude exited with code %d: %s", exitErr.ExitCode(), stderrBuf.String())
		}
		return "", fmt.Errorf("claude execution failed: %w", err)
	}

	return strings.TrimSpace(resultText), nil
}

// parseLine は stream-json の1行をパースし、StreamEvent を返す。
// パース不能な行は ok=false を返す。
func parseLine(line []byte) (StreamEvent, bool) {
	var sl streamLine
	if err := json.Unmarshal(line, &sl); err != nil {
		slog.Warn("failed to parse stream-json line", "error", err, "line", string(line))
		return StreamEvent{}, false
	}

	switch sl.Type {
	case "assistant":
		if sl.Message == nil {
			return StreamEvent{}, false
		}
		for _, block := range sl.Message.Content {
			if block.Type == "tool_use" && block.Name != "" {
				return StreamEvent{
					Type:     "tool_use",
					ToolName: block.Name,
				}, true
			}
		}
		return StreamEvent{}, false

	case "result":
		return StreamEvent{
			Type:    "result",
			Content: sl.Result,
		}, true

	default:
		return StreamEvent{}, false
	}
}
