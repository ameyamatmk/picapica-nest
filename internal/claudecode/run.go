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

type runConfig struct {
	allowedTools []string
}

// WithAllowedTools は Claude Code CLI に --allowedTools を指定する。
// 指定されたツールのみが CLI 内で利用可能になる。
func WithAllowedTools(tools ...string) Option {
	return func(c *runConfig) {
		c.allowedTools = append(c.allowedTools, tools...)
	}
}

// Run は claude -p を実行し、stdout を返す。
// prompt は LLM への指示、stdin は標準入力に渡すテキスト。
func Run(ctx context.Context, prompt string, stdin string, opts ...Option) (string, error) {
	cfg := &runConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	args := []string{"-p", prompt}
	if len(cfg.allowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(cfg.allowedTools, ","))
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
