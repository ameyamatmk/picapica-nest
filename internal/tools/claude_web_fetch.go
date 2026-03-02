package tools

import (
	"context"
	"fmt"

	"github.com/ameyamatmk/picapica-nest/internal/claudecode"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// ClaudeWebFetchTool は指定 URL のページ内容を Claude Code CLI で取得・要約するツール。
// Name() は "web_fetch" を返し、PicoClaw 組み込みの web_fetch を上書きする。
type ClaudeWebFetchTool struct{}

// NewClaudeWebFetchTool は ClaudeWebFetchTool を作成する。
func NewClaudeWebFetchTool() *ClaudeWebFetchTool {
	return &ClaudeWebFetchTool{}
}

func (t *ClaudeWebFetchTool) Name() string { return "web_fetch" }

func (t *ClaudeWebFetchTool) Description() string {
	return "指定した URL のページ内容を取得し、要約します。Web ページの内容を読みたいときに使います。"
}

func (t *ClaudeWebFetchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "取得するページの URL",
			},
			"question": map[string]any{
				"type":        "string",
				"description": "ページ内容について知りたいこと（省略時は全体の要約）",
			},
		},
		"required": []string{"url"},
	}
}

func (t *ClaudeWebFetchTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	url, _ := args["url"].(string)
	if url == "" {
		return tools.ErrorResult("url は必須です")
	}

	question, _ := args["question"].(string)

	prompt := buildFetchPrompt(url, question)
	result, err := claudecode.Run(ctx, prompt, "",
		claudecode.WithAllowedTools("WebFetch", "WebSearch"),
	)
	if err != nil {
		return tools.ErrorResult("ページの取得に失敗しました: " + err.Error())
	}

	return tools.NewToolResult(result)
}

// buildFetchPrompt は Claude Code CLI に渡す URL 取得プロンプトを組み立てる。
func buildFetchPrompt(url, question string) string {
	if question == "" {
		return fmt.Sprintf("次の URL のページ内容を取得して、日本語で簡潔に要約してください: %s", url)
	}
	return fmt.Sprintf("次の URL のページ内容を取得して、以下の質問に日本語で回答してください。\nURL: %s\n質問: %s", url, question)
}
