package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/providers"
)

func TestPromptRewriteProvider_AddsCurrentSituation(t *testing.T) {
	// Given: system メッセージを含むリクエスト
	mock := &mockProvider{
		response: &providers.LLMResponse{
			Content:      "OK",
			FinishReason: "stop",
		},
	}

	rp := NewPromptRewriteProvider(mock)

	messages := []providers.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
	}

	// When: Chat() を呼ぶ
	_, err := rp.Chat(context.Background(), messages, nil, "test-model", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Then: inner に渡された system メッセージに Current Situation が追加されている
	if len(mock.lastMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(mock.lastMessages))
	}

	systemContent := mock.lastMessages[0].Content
	if !strings.Contains(systemContent, "## Current Situation") {
		t.Error("expected system message to contain '## Current Situation'")
	}
	if !strings.Contains(systemContent, "現在時刻:") {
		t.Error("expected system message to contain '現在時刻:'")
	}
	if !strings.Contains(systemContent, "曜日:") {
		t.Error("expected system message to contain '曜日:'")
	}
	// 元のプロンプトも含まれていること
	if !strings.HasPrefix(systemContent, "You are a helpful assistant.") {
		t.Error("expected system message to start with original content")
	}
}

func TestPromptRewriteProvider_NoSystemMessage(t *testing.T) {
	// Given: system メッセージを含まないリクエスト
	mock := &mockProvider{
		response: &providers.LLMResponse{
			Content:      "OK",
			FinishReason: "stop",
		},
	}

	rp := NewPromptRewriteProvider(mock)

	messages := []providers.Message{
		{Role: "user", Content: "Hello"},
	}

	// When: Chat() を呼ぶ
	_, err := rp.Chat(context.Background(), messages, nil, "test-model", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Then: メッセージはそのまま通過する
	if len(mock.lastMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.lastMessages))
	}
	if mock.lastMessages[0].Content != "Hello" {
		t.Errorf("expected content 'Hello', got %q", mock.lastMessages[0].Content)
	}
}

func TestPromptRewriteProvider_DoesNotMutateOriginal(t *testing.T) {
	// Given: system メッセージを含むリクエスト
	mock := &mockProvider{
		response: &providers.LLMResponse{
			Content:      "OK",
			FinishReason: "stop",
		},
	}

	rp := NewPromptRewriteProvider(mock)

	original := "You are a helpful assistant."
	messages := []providers.Message{
		{Role: "system", Content: original},
		{Role: "user", Content: "Hello"},
	}

	// When: Chat() を呼ぶ
	_, _ = rp.Chat(context.Background(), messages, nil, "test-model", nil)

	// Then: 元の messages slice は変更されていない
	if messages[0].Content != original {
		t.Errorf("original message was mutated: got %q", messages[0].Content)
	}
}

func TestPromptRewriteProvider_EmptyMessages(t *testing.T) {
	// Given: 空のメッセージリスト
	mock := &mockProvider{
		response: &providers.LLMResponse{
			Content:      "OK",
			FinishReason: "stop",
		},
	}

	rp := NewPromptRewriteProvider(mock)

	// When: Chat() を空の messages で呼ぶ
	_, err := rp.Chat(context.Background(), nil, nil, "test-model", nil)

	// Then: エラーなしで通過する
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromptRewriteProvider_GetDefaultModel(t *testing.T) {
	// Given: defaultModel が設定されたモック
	mock := &mockProvider{defaultModel: "claude-sonnet-4-5-20250514"}
	rp := NewPromptRewriteProvider(mock)

	// Then: inner の GetDefaultModel() がそのまま返る
	if got := rp.GetDefaultModel(); got != "claude-sonnet-4-5-20250514" {
		t.Errorf("expected 'claude-sonnet-4-5-20250514', got %q", got)
	}
}
