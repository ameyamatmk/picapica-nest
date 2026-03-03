//go:build smoke

package tools

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSmoke_AnalyzeImage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tool := NewClaudeAnalyzeImageTool(os.TempDir(), "")
	result := tool.Execute(ctx, map[string]any{
		"image_url": "https://upload.wikimedia.org/wikipedia/commons/thumb/4/47/PNG_transparency_demonstration_1.png/280px-PNG_transparency_demonstration_1.png",
		"question":  "この画像には何が写っていますか？",
	})

	t.Logf("IsError: %v", result.IsError)
	t.Logf("Result: %s", result.ForLLM)

	if result.IsError {
		t.Errorf("expected success, got error: %s", result.ForLLM)
	}
	if result.ForLLM == "" {
		t.Error("expected non-empty result")
	}
}

func TestSmoke_WebSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tool := NewClaudeWebSearchTool("")
	result := tool.Execute(ctx, map[string]any{
		"query": "Go 1.25 release date",
	})

	t.Logf("IsError: %v", result.IsError)
	t.Logf("Result: %s", result.ForLLM)

	if result.IsError {
		t.Errorf("expected success, got error: %s", result.ForLLM)
	}
	if result.ForLLM == "" {
		t.Error("expected non-empty result")
	}
}

func TestSmoke_WebFetch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tool := NewClaudeWebFetchTool("")
	result := tool.Execute(ctx, map[string]any{
		"url":      "https://go.dev/blog/",
		"question": "最新の記事のタイトルは？",
	})

	t.Logf("IsError: %v", result.IsError)
	t.Logf("Result: %s", result.ForLLM)

	if result.IsError {
		t.Errorf("expected success, got error: %s", result.ForLLM)
	}
	if result.ForLLM == "" {
		t.Error("expected non-empty result")
	}
}
