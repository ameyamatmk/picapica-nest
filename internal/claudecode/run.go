package claudecode

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Option は Claude Code CLI 実行時のオプション。
type Option func(*runConfig)

// DefaultModel は Claude Code CLI のデフォルトモデル。
const DefaultModel = "sonnet"

type runConfig struct {
	model              string
	allowedTools       []string
	appendSystemPrompt string
	progress           ProgressFunc
	prompt             string
	stdin              string
}

// WithModel は Claude Code CLI に --model を指定する。
func WithModel(model string) Option {
	return func(c *runConfig) {
		c.model = model
	}
}

// WithAllowedTools は Claude Code CLI に --allowedTools を指定する。
// 指定されたツールのみが CLI 内で利用可能になる。
func WithAllowedTools(tools ...string) Option {
	return func(c *runConfig) {
		c.allowedTools = append(c.allowedTools, tools...)
	}
}

// WithAppendSystemPrompt は Claude Code CLI に --append-system-prompt を指定する。
// システムプロンプトの末尾に追加テキストを付与する。
func WithAppendSystemPrompt(text string) Option {
	return func(c *runConfig) {
		c.appendSystemPrompt = text
	}
}

// Run は claude -p を実行し、stdout を返す。
// prompt は LLM への指示、stdin は標準入力に渡すテキスト。
// WithProgress が指定されている場合は stream-json モードで実行し、
// CLI 内部のツール使用をリアルタイムで通知する。
func Run(ctx context.Context, prompt string, stdin string, opts ...Option) (string, error) {
	cfg := &runConfig{
		prompt: prompt,
		stdin:  stdin,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.progress != nil {
		return runStream(ctx, cfg)
	}
	return runSync(ctx, cfg)
}

// runSync は従来の同期実行（cmd.Output）で CLI を実行する。
func runSync(ctx context.Context, cfg *runConfig) (string, error) {
	model := cfg.model
	if model == "" {
		model = DefaultModel
	}

	args := []string{"-p", cfg.prompt, "--model", model}
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

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("claude exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude execution failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
