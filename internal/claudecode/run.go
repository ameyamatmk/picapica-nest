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
func Run(ctx context.Context, prompt string, stdin string, opts ...Option) (string, error) {
	cfg := &runConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	model := cfg.model
	if model == "" {
		model = DefaultModel
	}

	args := []string{"-p", prompt, "--model", model}
	if len(cfg.allowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(cfg.allowedTools, ","))
	}
	if cfg.appendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", cfg.appendSystemPrompt)
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
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
