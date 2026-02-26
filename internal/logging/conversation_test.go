package logging

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
)

func TestConversationLogger_InboundMessage(t *testing.T) {
	// Given: 一時ディレクトリと MessageBus のペア
	tmpDir := t.TempDir()
	channelBus := bus.NewMessageBus()
	agentBus := bus.NewMessageBus()
	logger := NewConversationLogger(tmpDir, channelBus, agentBus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Run(ctx)

	// When: channelBus に inbound メッセージを publish する
	channelBus.PublishInbound(bus.InboundMessage{
		Channel:  "discord",
		SenderID: "user#1234",
		ChatID:   "chat-001",
		Content:  "Hello, world!",
	})

	// Then: agentBus で inbound メッセージを受信できる
	msg, ok := agentBus.ConsumeInbound(ctx)
	if !ok {
		t.Fatal("expected to consume inbound message from agentBus")
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %q", msg.Content)
	}
	if msg.SenderID != "user#1234" {
		t.Errorf("expected sender 'user#1234', got %q", msg.SenderID)
	}

	// Then: ログファイルが作成されている
	date := time.Now().UTC().Format("2006-01-02")
	logFile := filepath.Join(tmpDir, "discord_chat-001", date+".jsonl")

	// ファイル書き込みを待つ
	var content []byte
	for range 50 {
		var err error
		content, err = os.ReadFile(logFile)
		if err == nil && len(content) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if len(content) == 0 {
		t.Fatalf("expected log file to contain data, got empty or file not found: %s", logFile)
	}

	var entry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(content))), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Direction != "in" {
		t.Errorf("expected direction 'in', got %q", entry.Direction)
	}
	if entry.Sender != "user#1234" {
		t.Errorf("expected sender 'user#1234', got %q", entry.Sender)
	}
	if entry.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %q", entry.Content)
	}
}

func TestConversationLogger_OutboundMessage(t *testing.T) {
	// Given: 一時ディレクトリと MessageBus のペア
	tmpDir := t.TempDir()
	channelBus := bus.NewMessageBus()
	agentBus := bus.NewMessageBus()
	logger := NewConversationLogger(tmpDir, channelBus, agentBus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Run(ctx)

	// When: agentBus に outbound メッセージを publish する
	agentBus.PublishOutbound(bus.OutboundMessage{
		Channel: "discord",
		ChatID:  "chat-001",
		Content: "Hi there!",
	})

	// Then: channelBus で outbound メッセージを受信できる
	msg, ok := channelBus.SubscribeOutbound(ctx)
	if !ok {
		t.Fatal("expected to subscribe outbound message from channelBus")
	}
	if msg.Content != "Hi there!" {
		t.Errorf("expected content 'Hi there!', got %q", msg.Content)
	}

	// Then: ログファイルが作成されている
	date := time.Now().UTC().Format("2006-01-02")
	logFile := filepath.Join(tmpDir, "discord_chat-001", date+".jsonl")

	var content []byte
	for range 50 {
		var err error
		content, err = os.ReadFile(logFile)
		if err == nil && len(content) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if len(content) == 0 {
		t.Fatalf("expected log file to contain data, got empty or file not found: %s", logFile)
	}

	var entry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(content))), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Direction != "out" {
		t.Errorf("expected direction 'out', got %q", entry.Direction)
	}
	if entry.Sender != "" {
		t.Errorf("expected empty sender for outbound, got %q", entry.Sender)
	}
	if entry.Content != "Hi there!" {
		t.Errorf("expected content 'Hi there!', got %q", entry.Content)
	}
}

func TestConversationLogger_InternalChannelNotLogged(t *testing.T) {
	// Given: 一時ディレクトリと MessageBus のペア
	tmpDir := t.TempDir()
	channelBus := bus.NewMessageBus()
	agentBus := bus.NewMessageBus()
	logger := NewConversationLogger(tmpDir, channelBus, agentBus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Run(ctx)

	// When: system チャンネルのメッセージを送信する
	channelBus.PublishInbound(bus.InboundMessage{
		Channel:  "system",
		SenderID: "cron",
		ChatID:   "heartbeat",
		Content:  "internal message",
	})

	// Then: agentBus で受信できる（転送はされる）
	msg, ok := agentBus.ConsumeInbound(ctx)
	if !ok {
		t.Fatal("expected to consume inbound message from agentBus")
	}
	if msg.Content != "internal message" {
		t.Errorf("expected content 'internal message', got %q", msg.Content)
	}

	// Then: ログファイルは作成されない
	time.Sleep(50 * time.Millisecond)
	entries, _ := os.ReadDir(tmpDir)
	if len(entries) > 0 {
		t.Errorf("expected no log directories for internal channel, got %d entries", len(entries))
	}
}

func TestConversationLogger_MultipleMessages(t *testing.T) {
	// Given: 一時ディレクトリと MessageBus のペア
	tmpDir := t.TempDir()
	channelBus := bus.NewMessageBus()
	agentBus := bus.NewMessageBus()
	logger := NewConversationLogger(tmpDir, channelBus, agentBus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Run(ctx)

	// When: 複数メッセージを送信する
	channelBus.PublishInbound(bus.InboundMessage{
		Channel:  "discord",
		SenderID: "user#1234",
		ChatID:   "chat-001",
		Content:  "first message",
	})
	agentBus.ConsumeInbound(ctx)

	agentBus.PublishOutbound(bus.OutboundMessage{
		Channel: "discord",
		ChatID:  "chat-001",
		Content: "first response",
	})
	channelBus.SubscribeOutbound(ctx)

	channelBus.PublishInbound(bus.InboundMessage{
		Channel:  "discord",
		SenderID: "user#1234",
		ChatID:   "chat-001",
		Content:  "second message",
	})
	agentBus.ConsumeInbound(ctx)

	// Then: ログファイルに3行記録されている
	date := time.Now().UTC().Format("2006-01-02")
	logFile := filepath.Join(tmpDir, "discord_chat-001", date+".jsonl")

	var content []byte
	for range 50 {
		var err error
		content, err = os.ReadFile(logFile)
		if err == nil && len(content) > 0 {
			lines := strings.Split(strings.TrimSpace(string(content)), "\n")
			if len(lines) >= 3 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 log lines, got %d: %s", len(lines), string(content))
	}

	// 1行目: inbound
	var entry1 LogEntry
	json.Unmarshal([]byte(lines[0]), &entry1)
	if entry1.Direction != "in" || entry1.Content != "first message" {
		t.Errorf("unexpected first entry: %+v", entry1)
	}

	// 2行目: outbound
	var entry2 LogEntry
	json.Unmarshal([]byte(lines[1]), &entry2)
	if entry2.Direction != "out" || entry2.Content != "first response" {
		t.Errorf("unexpected second entry: %+v", entry2)
	}

	// 3行目: inbound
	var entry3 LogEntry
	json.Unmarshal([]byte(lines[2]), &entry3)
	if entry3.Direction != "in" || entry3.Content != "second message" {
		t.Errorf("unexpected third entry: %+v", entry3)
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with/slash", "with_slash"},
		{"with:colon", "with_colon"},
		{"with*star", "with_star"},
		{"normal-chat-id", "normal-chat-id"},
		{`a\b"c<d>e|f`, "a_b_c_d_e_f"},
	}

	for _, tt := range tests {
		result := sanitizePath(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsLoggableChannel(t *testing.T) {
	tests := []struct {
		channel  string
		expected bool
	}{
		{"discord", true},
		{"telegram", true},
		{"slack", true},
		{"system", false},
		{"cli", false},
		{"subagent", false},
	}

	for _, tt := range tests {
		result := isLoggableChannel(tt.channel)
		if result != tt.expected {
			t.Errorf("isLoggableChannel(%q) = %v, want %v", tt.channel, result, tt.expected)
		}
	}
}
