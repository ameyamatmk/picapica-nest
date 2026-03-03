package tools

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ameyamatmk/picapica-nest/internal/claudecode"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// ClaudeAnalyzeImageTool は画像の URL を受け取り、Claude Code CLI で分析するツール。
type ClaudeAnalyzeImageTool struct {
	tempDir    string
	soulPrompt string
	bus        *bus.MessageBus
	channel    string
	chatID     string
}

// NewClaudeAnalyzeImageTool は ClaudeAnalyzeImageTool を作成する。
// tempDir は画像ダウンロード用の一時ディレクトリ。
func NewClaudeAnalyzeImageTool(tempDir string, soulPrompt string, mb *bus.MessageBus) *ClaudeAnalyzeImageTool {
	return &ClaudeAnalyzeImageTool{tempDir: tempDir, soulPrompt: soulPrompt, bus: mb}
}

// SetContext は ContextualTool インターフェースの実装。
func (t *ClaudeAnalyzeImageTool) SetContext(channel, chatID string) {
	t.channel = channel
	t.chatID = chatID
}

func (t *ClaudeAnalyzeImageTool) Name() string { return "claude_analyze_image" }

func (t *ClaudeAnalyzeImageTool) Description() string {
	return "画像の内容を分析します。画像の URL を指定してください。"
}

func (t *ClaudeAnalyzeImageTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"image_url": map[string]any{
				"type":        "string",
				"description": "分析する画像の URL",
			},
			"question": map[string]any{
				"type":        "string",
				"description": "画像について知りたいこと（省略時は全体的な説明）",
			},
		},
		"required": []string{"image_url"},
	}
}

func (t *ClaudeAnalyzeImageTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	imageURL, _ := args["image_url"].(string)
	if imageURL == "" {
		return tools.ErrorResult("image_url は必須です")
	}

	question, _ := args["question"].(string)

	slog.Info("executing claude code delegation", "tool", t.Name(), "image_url", imageURL)

	// 画像ダウンロード → 一時ファイル
	tmpFile, err := downloadImage(ctx, imageURL, t.tempDir)
	if err != nil {
		return tools.ErrorResult("画像のダウンロードに失敗しました: " + err.Error())
	}
	defer os.Remove(tmpFile)

	// Claude Code CLI で分析（Read のみ許可）
	prompt := buildVisionPrompt(tmpFile, question)
	opts := []claudecode.Option{
		claudecode.WithAllowedTools("Read"),
	}
	if t.soulPrompt != "" {
		opts = append(opts, claudecode.WithAppendSystemPrompt(t.soulPrompt))
	}
	if progressFn := newProgressFunc(t.bus, t.channel, t.chatID); progressFn != nil {
		opts = append(opts, claudecode.WithProgress(progressFn))
	}
	result, err := claudecode.Run(ctx, prompt, "", opts...)
	if err != nil {
		return tools.ErrorResult("画像の分析に失敗しました: " + err.Error())
	}

	return tools.NewToolResult(result)
}

// downloadImage は URL から画像をダウンロードし、一時ファイルパスを返す。
func downloadImage(ctx context.Context, imageURL, tempDir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("request creation failed: %w", err)
	}
	req.Header.Set("User-Agent", "picapica-nest/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// 拡張子を URL から推測
	ext := filepath.Ext(imageURL)
	if idx := strings.IndexByte(ext, '?'); idx >= 0 {
		ext = ext[:idx]
	}
	if ext == "" {
		ext = ".jpg"
	}

	tmpFile, err := os.CreateTemp(tempDir, "claude_analyze_*"+ext)
	if err != nil {
		return "", fmt.Errorf("temp file creation failed: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("write failed: %w", err)
	}

	return tmpFile.Name(), nil
}

// buildVisionPrompt は Claude Code CLI に渡す画像分析プロンプトを組み立てる。
func buildVisionPrompt(imagePath, question string) string {
	if question == "" {
		return fmt.Sprintf("画像ファイル %s の内容を分析して、何が写っているか日本語で詳しく説明してください。", imagePath)
	}
	return fmt.Sprintf("画像ファイル %s を分析して、次の質問に日本語で回答してください: %s", imagePath, question)
}
