package distill

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// RunClaude は claude -p を実行し、stdout を返す。
// prompt は LLM への指示、stdin は標準入力に渡すテキスト（会話 transcript）。
func RunClaude(ctx context.Context, prompt string, stdin string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt, "--no-input")
	cmd.Stdin = strings.NewReader(stdin)

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
