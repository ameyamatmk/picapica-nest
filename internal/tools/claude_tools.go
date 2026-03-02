package tools

import "github.com/sipeed/picoclaw/pkg/tools"

// NewClaudeTools は Claude Code CLI 委譲ツールの一覧を返す。
// 動的エージェント作成時にも同じツールセットを登録するために使用する。
func NewClaudeTools(tempDir string) []tools.Tool {
	return []tools.Tool{
		NewClaudeAnalyzeImageTool(tempDir),
		NewClaudeWebSearchTool(),
		NewClaudeWebFetchTool(),
	}
}
