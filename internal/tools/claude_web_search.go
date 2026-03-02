package tools

import (
	"context"
	"fmt"

	"github.com/ameyamatmk/picapica-nest/internal/claudecode"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// ClaudeWebSearchTool は Claude Code CLI を使って Web 検索を行うツール。
// PicoClaw 組み込みの web_search（Brave/Tavily 等）とは別名で共存する。
type ClaudeWebSearchTool struct{}

// NewClaudeWebSearchTool は ClaudeWebSearchTool を作成する。
func NewClaudeWebSearchTool() *ClaudeWebSearchTool {
	return &ClaudeWebSearchTool{}
}

func (t *ClaudeWebSearchTool) Name() string { return "claude_web_search" }

func (t *ClaudeWebSearchTool) Description() string {
	return "Web を検索して最新情報を取得し、結果を要約します。検索クエリを指定してください。"
}

func (t *ClaudeWebSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "検索クエリ",
			},
		},
		"required": []string{"query"},
	}
}

func (t *ClaudeWebSearchTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	query, _ := args["query"].(string)
	if query == "" {
		return tools.ErrorResult("query は必須です")
	}

	prompt := fmt.Sprintf("次のクエリについて Web 検索し、結果を日本語で簡潔にまとめてください: %s", query)
	result, err := claudecode.Run(ctx, prompt, "")
	if err != nil {
		return tools.ErrorResult("Web 検索に失敗しました: " + err.Error())
	}

	return tools.NewToolResult(result)
}
