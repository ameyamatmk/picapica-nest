package hindsight

import (
	"context"

	"github.com/ameyamatmk/picapica-nest/internal/claudecode"
)

// RunClaude は claude -p を実行し、stdout を返す。
// prompt は LLM への指示、stdin は標準入力に渡すテキスト（会話 transcript）。
func RunClaude(ctx context.Context, prompt string, stdin string) (string, error) {
	return claudecode.Run(ctx, prompt, stdin)
}
