package tools

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/sipeed/picoclaw/pkg/tools"
)

// NewClaudeTools は Claude Code CLI 委譲ツールの一覧を返す。
// 動的エージェント作成時にも同じツールセットを登録するために使用する。
// workspacePath が指定されている場合、SOUL.md を読み込んで
// --append-system-prompt に渡すことで口調を統一する。
func NewClaudeTools(tempDir string, workspacePath string) []tools.Tool {
	soulPrompt := loadSOUL(workspacePath)

	return []tools.Tool{
		NewClaudeAnalyzeImageTool(tempDir, soulPrompt),
		NewClaudeWebSearchTool(soulPrompt),
		NewClaudeWebFetchTool(soulPrompt),
	}
}

// loadSOUL は workspacePath/SOUL.md を読み込んで内容を返す。
// 読み込めない場合は空文字を返す。
func loadSOUL(workspacePath string) string {
	if workspacePath == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(workspacePath, "SOUL.md"))
	if err != nil {
		slog.Debug("SOUL.md not found, skipping append-system-prompt", "error", err)
		return ""
	}
	text := strings.TrimSpace(string(data))
	if text != "" {
		slog.Info("SOUL.md loaded for claude CLI system prompt", "length", len(text))
	}
	return text
}
