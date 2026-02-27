package provider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/providers"
)

// mockProvider はテスト用のモック LLMProvider。
type mockProvider struct {
	response     *providers.LLMResponse
	err          error
	defaultModel string
	// Chat() に渡された引数を記録する
	lastMessages []providers.Message
}

func (m *mockProvider) Chat(_ context.Context, messages []providers.Message, _ []providers.ToolDefinition, _ string, _ map[string]interface{}) (*providers.LLMResponse, error) {
	m.lastMessages = messages
	return m.response, m.err
}

func (m *mockProvider) GetDefaultModel() string {
	return m.defaultModel
}

func TestLoggingProvider_RecordsUsage(t *testing.T) {
	// Given: usage 付きのレスポンスを返すモック Provider
	mock := &mockProvider{
		response: &providers.LLMResponse{
			Content:      "Hello!",
			FinishReason: "stop",
			Usage: &providers.UsageInfo{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		},
		defaultModel: "claude-sonnet-4-5-20250514",
	}

	logPath := filepath.Join(t.TempDir(), "usage.jsonl")
	lp := NewLoggingProvider(mock, logPath)

	// When: Chat() を呼ぶ
	resp, err := lp.Chat(context.Background(), nil, nil, "claude-sonnet-4-5-20250514", nil)

	// Then: inner のレスポンスがそのまま返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("expected content 'Hello!', got %q", resp.Content)
	}

	// Then: usage.jsonl にレコードが記録されている
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var record UsageRecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(content))), &record); err != nil {
		t.Fatalf("failed to unmarshal record: %v", err)
	}

	if record.Model != "claude-sonnet-4-5-20250514" {
		t.Errorf("expected model 'claude-sonnet-4-5-20250514', got %q", record.Model)
	}
	if record.PromptTokens != 100 {
		t.Errorf("expected prompt_tokens 100, got %d", record.PromptTokens)
	}
	if record.CompletionTokens != 50 {
		t.Errorf("expected completion_tokens 50, got %d", record.CompletionTokens)
	}
	if record.TotalTokens != 150 {
		t.Errorf("expected total_tokens 150, got %d", record.TotalTokens)
	}
	if record.LatencyMs < 0 {
		t.Errorf("expected non-negative latency, got %d", record.LatencyMs)
	}
	if record.Error != "" {
		t.Errorf("expected empty error, got %q", record.Error)
	}
	if record.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestLoggingProvider_RecordsOnError(t *testing.T) {
	// Given: エラーを返すモック Provider
	mock := &mockProvider{
		response: nil,
		err:      errors.New("API rate limit exceeded"),
	}

	logPath := filepath.Join(t.TempDir(), "usage.jsonl")
	lp := NewLoggingProvider(mock, logPath)

	// When: Chat() を呼ぶ
	resp, err := lp.Chat(context.Background(), nil, nil, "claude-sonnet-4-5-20250514", nil)

	// Then: inner のエラーがそのまま返る
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response, got %+v", resp)
	}

	// Then: エラー情報付きでレコードが記録されている
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var record UsageRecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(content))), &record); err != nil {
		t.Fatalf("failed to unmarshal record: %v", err)
	}

	if record.Error != "API rate limit exceeded" {
		t.Errorf("expected error 'API rate limit exceeded', got %q", record.Error)
	}
	if record.PromptTokens != 0 {
		t.Errorf("expected prompt_tokens 0, got %d", record.PromptTokens)
	}
}

func TestLoggingProvider_NilUsage(t *testing.T) {
	// Given: Usage が nil のレスポンスを返すモック Provider
	mock := &mockProvider{
		response: &providers.LLMResponse{
			Content:      "Hello!",
			FinishReason: "stop",
			Usage:        nil,
		},
	}

	logPath := filepath.Join(t.TempDir(), "usage.jsonl")
	lp := NewLoggingProvider(mock, logPath)

	// When: Chat() を呼ぶ
	_, _ = lp.Chat(context.Background(), nil, nil, "test-model", nil)

	// Then: トークン数が0で記録されている
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var record UsageRecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(content))), &record); err != nil {
		t.Fatalf("failed to unmarshal record: %v", err)
	}

	if record.PromptTokens != 0 || record.CompletionTokens != 0 || record.TotalTokens != 0 {
		t.Errorf("expected all token counts to be 0, got prompt=%d completion=%d total=%d",
			record.PromptTokens, record.CompletionTokens, record.TotalTokens)
	}
}

func TestLoggingProvider_GetDefaultModel(t *testing.T) {
	// Given: defaultModel が設定されたモック
	mock := &mockProvider{defaultModel: "claude-sonnet-4-5-20250514"}
	lp := NewLoggingProvider(mock, filepath.Join(t.TempDir(), "usage.jsonl"))

	// Then: inner の GetDefaultModel() がそのまま返る
	if got := lp.GetDefaultModel(); got != "claude-sonnet-4-5-20250514" {
		t.Errorf("expected 'claude-sonnet-4-5-20250514', got %q", got)
	}
}

func TestLoggingProvider_MultipleRecords(t *testing.T) {
	// Given: 正常なモック Provider
	mock := &mockProvider{
		response: &providers.LLMResponse{
			Content:      "OK",
			FinishReason: "stop",
			Usage: &providers.UsageInfo{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	}

	logPath := filepath.Join(t.TempDir(), "usage.jsonl")
	lp := NewLoggingProvider(mock, logPath)

	// When: Chat() を3回呼ぶ
	for range 3 {
		_, _ = lp.Chat(context.Background(), nil, nil, "test-model", nil)
	}

	// Then: 3行のレコードが記録されている
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
}
