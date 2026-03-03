//go:build smoke

package tools

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
)

// setupSmokeBus はスモークテスト用の MessageBus を作成し、
// outbound メッセージを収集するゴルーチンを起動する。
// 返される collected には受信した進捗メッセージが格納される。
func setupSmokeBus(ctx context.Context, t *testing.T) (*bus.MessageBus, *[]string) {
	t.Helper()
	mb := bus.NewMessageBus()
	var mu sync.Mutex
	var collected []string

	go func() {
		for {
			msg, ok := mb.SubscribeOutbound(ctx)
			if !ok {
				return
			}
			mu.Lock()
			collected = append(collected, msg.Content)
			mu.Unlock()
			t.Logf("[progress] %s", msg.Content)
		}
	}()

	return mb, &collected
}

func TestSmoke_AnalyzeImage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mb, collected := setupSmokeBus(ctx, t)

	tool := NewClaudeAnalyzeImageTool(os.TempDir(), "", mb)
	tool.SetContext("test", "smoke-analyze")
	result := tool.Execute(ctx, map[string]any{
		"image_url": "https://upload.wikimedia.org/wikipedia/commons/thumb/4/47/PNG_transparency_demonstration_1.png/280px-PNG_transparency_demonstration_1.png",
		"question":  "この画像には何が写っていますか？",
	})

	t.Logf("IsError: %v", result.IsError)
	t.Logf("Result: %s", result.ForLLM)
	t.Logf("Progress messages: %v", *collected)

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

	mb, collected := setupSmokeBus(ctx, t)

	tool := NewClaudeWebSearchTool("", mb)
	tool.SetContext("test", "smoke-search")
	result := tool.Execute(ctx, map[string]any{
		"query": "Go 1.25 release date",
	})

	t.Logf("IsError: %v", result.IsError)
	t.Logf("Result: %s", result.ForLLM)
	t.Logf("Progress messages: %v", *collected)

	if result.IsError {
		t.Errorf("expected success, got error: %s", result.ForLLM)
	}
	if result.ForLLM == "" {
		t.Error("expected non-empty result")
	}
	if len(*collected) == 0 {
		t.Error("expected at least one progress message")
	}
}

func TestSmoke_WebFetch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mb, collected := setupSmokeBus(ctx, t)

	tool := NewClaudeWebFetchTool("", mb)
	tool.SetContext("test", "smoke-fetch")
	result := tool.Execute(ctx, map[string]any{
		"url":      "https://go.dev/blog/",
		"question": "最新の記事のタイトルは？",
	})

	t.Logf("IsError: %v", result.IsError)
	t.Logf("Result: %s", result.ForLLM)
	t.Logf("Progress messages: %v", *collected)

	if result.IsError {
		t.Errorf("expected success, got error: %s", result.ForLLM)
	}
	if result.ForLLM == "" {
		t.Error("expected non-empty result")
	}
	if len(*collected) == 0 {
		t.Error("expected at least one progress message")
	}
}
