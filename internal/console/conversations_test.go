package console

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleConversations_ReturnsHTML(t *testing.T) {
	// Given: 会話ログが存在するワークスペース
	dir := t.TempDir()
	channelDir := filepath.Join(dir, "logs", "discord_test-channel")
	if err := os.MkdirAll(channelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logData := `{"ts":"2026-02-27T01:15:00Z","dir":"in","sender":"ameyama","content":"おはよう"}
{"ts":"2026-02-27T01:15:05Z","dir":"out","content":"おはよう！"}
`
	if err := os.WriteFile(filepath.Join(channelDir, "2026-02-27.jsonl"), []byte(logData), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(dir, nil, nil)

	// When: GET /conversations にリクエスト
	req := httptest.NewRequest("GET", "/conversations", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でHTMLが返り、会話ログの内容を含む
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "会話ログ") {
		t.Error("expected page to contain '会話ログ'")
	}
	if !strings.Contains(body, "おはよう") {
		t.Error("expected page to contain message content")
	}
	if !strings.Contains(body, "test-channel") {
		t.Error("expected page to contain channel label")
	}
}

func TestHandleConversations_EmptyLogs(t *testing.T) {
	// Given: ログディレクトリが存在しないワークスペース
	dir := t.TempDir()

	s := NewServer(dir, nil, nil)

	// When: GET /conversations にリクエスト
	req := httptest.NewRequest("GET", "/conversations", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 が返る（エラーにならない）
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "会話ログ") {
		t.Error("expected page to contain '会話ログ'")
	}
}

func TestHandleConversationMessages_WithData(t *testing.T) {
	// Given: 会話ログが存在するワークスペース
	dir := t.TempDir()
	channelDir := filepath.Join(dir, "logs", "discord_general")
	if err := os.MkdirAll(channelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logData := `{"ts":"2026-02-27T10:30:00Z","dir":"in","sender":"user1","content":"こんにちは"}
{"ts":"2026-02-27T10:30:05Z","dir":"out","content":"こんにちは！何かお手伝いできることはある？"}
`
	if err := os.WriteFile(filepath.Join(channelDir, "2026-02-27.jsonl"), []byte(logData), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(dir, nil, nil)

	// When: GET /conversations/messages にリクエスト
	req := httptest.NewRequest("GET", "/conversations/messages?channel=discord_general&date=2026-02-27.jsonl", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でフラグメントが返り、メッセージを含む
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	// layout を含まない（フラグメント）
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("fragment should not contain DOCTYPE")
	}

	// メッセージ内容を含む
	if !strings.Contains(body, "こんにちは") {
		t.Error("expected fragment to contain message content")
	}
	if !strings.Contains(body, "10:30") {
		t.Error("expected fragment to contain message time")
	}
	if !strings.Contains(body, "ameyama") {
		t.Error("expected fragment to contain sender name")
	}

	// チャットバブルの CSS クラスを含む
	if !strings.Contains(body, "chat-in") {
		t.Error("expected fragment to contain 'chat-in' class")
	}
	if !strings.Contains(body, "chat-out") {
		t.Error("expected fragment to contain 'chat-out' class")
	}
}

func TestListConversationChannels(t *testing.T) {
	// Given: logs/ に複数のチャンネルと app/ がある
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")
	for _, name := range []string{"app", "discord_general", "discord_test-channel"} {
		if err := os.MkdirAll(filepath.Join(logsDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// When: listConversationChannels を呼ぶ（resolver なし）
	channels, err := listConversationChannels(dir, nil)

	// Then: app/ は除外され、チャンネルが昇順で返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
	if channels[0].DirName != "discord_general" {
		t.Errorf("expected first channel 'discord_general', got %q", channels[0].DirName)
	}
	if channels[0].Label != "general" {
		t.Errorf("expected label 'general', got %q", channels[0].Label)
	}
	if channels[1].DirName != "discord_test-channel" {
		t.Errorf("expected second channel 'discord_test-channel', got %q", channels[1].DirName)
	}

	// app/ が含まれていないことを確認
	for _, ch := range channels {
		if ch.DirName == "app" {
			t.Error("expected 'app' to be excluded from channels")
		}
	}
}

func TestListConversationChannels_Empty(t *testing.T) {
	// Given: logs/ ディレクトリが存在しない
	dir := filepath.Join(t.TempDir(), "nonexistent")

	// When
	channels, err := listConversationChannels(dir, nil)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if channels != nil {
		t.Errorf("expected nil, got %v", channels)
	}
}

func TestLoadConversationMessages(t *testing.T) {
	// Given: JSONL ファイルが存在する
	dir := t.TempDir()
	channelDir := filepath.Join(dir, "logs", "discord_test")
	if err := os.MkdirAll(channelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logData := `{"ts":"2026-02-27T01:15:00Z","dir":"in","sender":"ameyama","content":"おはよう、今日は何する？"}
{"ts":"2026-02-27T01:15:05Z","dir":"out","content":"おはよう！今日はいい天気だね。"}
{"ts":"2026-02-27T02:00:00Z","dir":"in","sender":"ameyama","content":"散歩しよう"}
`
	if err := os.WriteFile(filepath.Join(channelDir, "2026-02-27.jsonl"), []byte(logData), 0o644); err != nil {
		t.Fatal(err)
	}

	// When: loadConversationMessages を呼ぶ
	messages, err := loadConversationMessages(dir, "discord_test", "2026-02-27.jsonl")

	// Then: 3件のメッセージが返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}

	// 1件目（逆順なので最新のメッセージが先頭）: inbound メッセージ
	if messages[0].Direction != "in" {
		t.Errorf("expected direction 'in', got %q", messages[0].Direction)
	}
	if messages[0].Sender != "ameyama" {
		t.Errorf("expected sender 'ameyama', got %q", messages[0].Sender)
	}
	if messages[0].Time != "02:00" {
		t.Errorf("expected time '02:00', got %q", messages[0].Time)
	}
	if messages[0].Content != "散歩しよう" {
		t.Errorf("expected content '散歩しよう', got %q", messages[0].Content)
	}

	// 最後のメッセージ（最古）: inbound メッセージ
	last := messages[len(messages)-1]
	if last.Time != "01:15" {
		t.Errorf("expected time '01:15', got %q", last.Time)
	}
	if last.Content != "おはよう、今日は何する？" {
		t.Errorf("expected content 'おはよう、今日は何する？', got %q", last.Content)
	}
}

func TestLoadConversationMessages_PathTraversal(t *testing.T) {
	// Given: ワークスペース
	dir := t.TempDir()

	// When: パストラバーサル攻撃のチャンネル名
	messages, err := loadConversationMessages(dir, "../../../etc", "passwd")

	// Then: メッセージなし、エラーなし（安全に処理される）
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestLoadConversationMessages_Empty(t *testing.T) {
	// Given: ファイルが存在しない
	dir := t.TempDir()

	// When
	messages, err := loadConversationMessages(dir, "discord_test", "2026-01-01.jsonl")

	// Then: nil、エラーなし
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if messages != nil {
		t.Errorf("expected nil, got %v", messages)
	}
}

func TestFormatChannelLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"discord_test-channel", "test-channel"},
		{"discord_general", "general"},
		{"simple", "simple"},
		{"a_b_c", "b_c"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatChannelLabel(tt.input)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFormatMessageTime(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2026-02-27T01:15:00Z", "01:15"},
		{"2026-02-27T23:59:59Z", "23:59"},
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatMessageTime(tt.input)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestSenderByDirection(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"in", "ameyama"},
		{"out", "Haruka"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := senderByDirection(tt.input)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestListConversationDates(t *testing.T) {
	// Given: チャンネルディレクトリにJSONLファイルがある
	dir := t.TempDir()
	channelDir := filepath.Join(dir, "logs", "discord_test")
	if err := os.MkdirAll(channelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"2026-02-25.jsonl", "2026-02-27.jsonl", "2026-02-26.jsonl"} {
		if err := os.WriteFile(filepath.Join(channelDir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 非JSONLファイルも配置
	os.WriteFile(filepath.Join(channelDir, "notes.txt"), []byte("note"), 0o644)

	// When: listConversationDates を呼ぶ
	dates, err := listConversationDates(dir, "discord_test")

	// Then: 降順で .jsonl ファイルのみ返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dates) != 3 {
		t.Fatalf("expected 3 dates, got %d", len(dates))
	}
	if dates[0].FileName != "2026-02-27.jsonl" {
		t.Errorf("expected first '2026-02-27.jsonl', got %q", dates[0].FileName)
	}
	if dates[0].Label != "2026-02-27" {
		t.Errorf("expected label '2026-02-27', got %q", dates[0].Label)
	}
	if dates[2].FileName != "2026-02-25.jsonl" {
		t.Errorf("expected last '2026-02-25.jsonl', got %q", dates[2].FileName)
	}
}
