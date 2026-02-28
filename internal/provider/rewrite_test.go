package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	rp := NewPromptRewriteProvider(mock, "")

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

	rp := NewPromptRewriteProvider(mock, "")

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

	rp := NewPromptRewriteProvider(mock, "")

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

	rp := NewPromptRewriteProvider(mock, "")

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
	rp := NewPromptRewriteProvider(mock, "")

	// Then: inner の GetDefaultModel() がそのまま返る
	if got := rp.GetDefaultModel(); got != "claude-sonnet-4-5-20250514" {
		t.Errorf("expected 'claude-sonnet-4-5-20250514', got %q", got)
	}
}

// setupRewriteWorkspace はテスト用のワークスペースを作成する。
func setupRewriteWorkspace(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	os.MkdirAll(filepath.Join(base, "memory", "daily"), 0o755)
	os.MkdirAll(filepath.Join(base, "memory", "weekly"), 0o755)
	os.MkdirAll(filepath.Join(base, "memory", "monthly"), 0o755)
	return base
}

// chatAndGetSystem は Chat() を呼び、inner に渡された system メッセージを返す。
func chatAndGetSystem(t *testing.T, rp *PromptRewriteProvider) string {
	t.Helper()
	mock := rp.inner.(*mockProvider)
	messages := []providers.Message{
		{Role: "system", Content: "Base prompt."},
		{Role: "user", Content: "Hello"},
	}
	_, err := rp.Chat(context.Background(), messages, nil, "test-model", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return mock.lastMessages[0].Content
}

func TestPromptRewriteProvider_InjectsRecentDaily(t *testing.T) {
	// Given: memory/daily/ に2日分のレポートを配置
	workspace := setupRewriteWorkspace(t)
	dailyDir := filepath.Join(workspace, "memory", "daily")

	// 今日と昨日の日付でファイルを作成
	now := func() string {
		loc, _ := time.LoadLocation("Asia/Tokyo")
		if loc == nil {
			loc = time.FixedZone("JST", 9*60*60)
		}
		return time.Now().In(loc).Format("2006-01-02")
	}()
	yesterday := func() string {
		loc, _ := time.LoadLocation("Asia/Tokyo")
		if loc == nil {
			loc = time.FixedZone("JST", 9*60*60)
		}
		return time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
	}()

	os.WriteFile(filepath.Join(dailyDir, now+".md"), []byte("今日の出来事です。\n"), 0o644)
	os.WriteFile(filepath.Join(dailyDir, yesterday+".md"), []byte("昨日の出来事です。\n"), 0o644)

	mock := &mockProvider{
		response: &providers.LLMResponse{Content: "OK", FinishReason: "stop"},
	}
	rp := NewPromptRewriteProvider(mock, workspace)

	// When: Chat() を呼ぶ
	systemContent := chatAndGetSystem(t, rp)

	// Then: system prompt に日次レポート内容が含まれる
	if !strings.Contains(systemContent, "## 直近の出来事") {
		t.Error("expected '## 直近の出来事' section")
	}
	if !strings.Contains(systemContent, "今日の出来事です。") {
		t.Error("expected today's daily report content")
	}
	if !strings.Contains(systemContent, "昨日の出来事です。") {
		t.Error("expected yesterday's daily report content")
	}
}

func TestPromptRewriteProvider_InjectsWeeklyMonthlyPaths(t *testing.T) {
	// Given: memory/weekly/ と memory/monthly/ にファイルを配置
	workspace := setupRewriteWorkspace(t)

	os.WriteFile(filepath.Join(workspace, "memory", "weekly", "2026-02-W09.md"),
		[]byte("Weekly report\n"), 0o644)
	os.WriteFile(filepath.Join(workspace, "memory", "monthly", "2026-01.md"),
		[]byte("Monthly report\n"), 0o644)

	mock := &mockProvider{
		response: &providers.LLMResponse{Content: "OK", FinishReason: "stop"},
	}
	rp := NewPromptRewriteProvider(mock, workspace)

	// When: Chat() を呼ぶ
	systemContent := chatAndGetSystem(t, rp)

	// Then: system prompt にファイルパスが含まれる
	if !strings.Contains(systemContent, "## 参照可能なレポート") {
		t.Error("expected '## 参照可能なレポート' section")
	}
	if !strings.Contains(systemContent, "週次:") {
		t.Error("expected weekly path reference")
	}
	if !strings.Contains(systemContent, "2026-02-W09.md") {
		t.Error("expected weekly file path")
	}
	if !strings.Contains(systemContent, "月次:") {
		t.Error("expected monthly path reference")
	}
	if !strings.Contains(systemContent, "2026-01.md") {
		t.Error("expected monthly file path")
	}
}

func TestPromptRewriteProvider_NoMemoryFiles(t *testing.T) {
	// Given: memory ディレクトリが空
	workspace := setupRewriteWorkspace(t)

	mock := &mockProvider{
		response: &providers.LLMResponse{Content: "OK", FinishReason: "stop"},
	}
	rp := NewPromptRewriteProvider(mock, workspace)

	// When: Chat() を呼ぶ
	systemContent := chatAndGetSystem(t, rp)

	// Then: 時刻セクションのみ（蒸留セクションなし）
	if !strings.Contains(systemContent, "## Current Situation") {
		t.Error("expected Current Situation section")
	}
	if strings.Contains(systemContent, "## 直近の出来事") {
		t.Error("should not contain daily section when no files exist")
	}
	if strings.Contains(systemContent, "## 参照可能なレポート") {
		t.Error("should not contain refs section when no files exist")
	}
}

func TestPromptRewriteProvider_PartialFiles(t *testing.T) {
	// Given: 日次1日分のみ存在、週次・月次なし
	workspace := setupRewriteWorkspace(t)
	dailyDir := filepath.Join(workspace, "memory", "daily")

	// 今日の日付でファイルを作成
	loc, _ := time.LoadLocation("Asia/Tokyo")
	if loc == nil {
		loc = time.FixedZone("JST", 9*60*60)
	}
	today := time.Now().In(loc).Format("2006-01-02")
	os.WriteFile(filepath.Join(dailyDir, today+".md"), []byte("今日だけの出来事。\n"), 0o644)

	mock := &mockProvider{
		response: &providers.LLMResponse{Content: "OK", FinishReason: "stop"},
	}
	rp := NewPromptRewriteProvider(mock, workspace)

	// When: Chat() を呼ぶ
	systemContent := chatAndGetSystem(t, rp)

	// Then: 日次セクションのみ表示、参照セクションなし
	if !strings.Contains(systemContent, "## 直近の出来事") {
		t.Error("expected daily section")
	}
	if !strings.Contains(systemContent, "今日だけの出来事。") {
		t.Error("expected today's daily content")
	}
	if strings.Contains(systemContent, "## 参照可能なレポート") {
		t.Error("should not contain refs section when no weekly/monthly files exist")
	}
}
