// Package logging は会話ログの記録を提供する。
// PicoClaw の MessageBus を流れるメッセージをインターセプトし、
// JSONL 形式でファイルに記録する。
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
)

// LogEntry は JSONL ファイルに書き込む1行分のデータ。
type LogEntry struct {
	Timestamp string `json:"ts"`
	Direction string `json:"dir"`
	Sender    string `json:"sender,omitempty"`
	Content   string `json:"content"`
}

// ConversationLogger は MessageBus を流れるメッセージを JSONL ファイルに記録する。
//
// 2つの MessageBus の間にブリッジとして配置し、メッセージを透過的に転送しつつ
// ログを記録する（Dual MessageBus + Bridge パターン）。
//
//	Channel --[PublishInbound]--> channelBus
//	  ConversationLogger: channelBus.ConsumeInbound -> log -> agentBus.PublishInbound
//	AgentLoop --[ConsumeInbound]--> agentBus
//	AgentLoop --[PublishOutbound]--> agentBus
//	  ConversationLogger: agentBus.SubscribeOutbound -> log -> channelBus.PublishOutbound
//	ChannelManager --[SubscribeOutbound]--> channelBus
type ConversationLogger struct {
	basePath   string
	channelBus *bus.MessageBus
	agentBus   *bus.MessageBus

	// mu はファイル書き込みの排他制御用
	mu sync.Mutex
}

// NewConversationLogger は新しい ConversationLogger を作成する。
//
// basePath はログディレクトリのルートパス。
// channelBus は Channel 側の MessageBus（Channel が PublishInbound する先）。
// agentBus は AgentLoop 側の MessageBus（AgentLoop が ConsumeInbound する先）。
func NewConversationLogger(basePath string, channelBus, agentBus *bus.MessageBus) *ConversationLogger {
	return &ConversationLogger{
		basePath:   basePath,
		channelBus: channelBus,
		agentBus:   agentBus,
	}
}

// Run はブリッジ処理を開始する。
// inbound と outbound の2つの goroutine を起動し、ctx がキャンセルされるまで動作する。
func (cl *ConversationLogger) Run(ctx context.Context) {
	go cl.bridgeInbound(ctx)
	go cl.bridgeOutbound(ctx)
}

// bridgeInbound は channelBus の inbound メッセージを agentBus に転送しつつログ記録する。
func (cl *ConversationLogger) bridgeInbound(ctx context.Context) {
	for {
		msg, ok := cl.channelBus.ConsumeInbound(ctx)
		if !ok {
			return
		}

		// 内部チャンネルのメッセージはログに記録しない
		if !isLoggableChannel(msg.Channel) {
			cl.agentBus.PublishInbound(ctx, msg)
			continue
		}

		cl.logInbound(msg)
		cl.agentBus.PublishInbound(ctx, msg)
	}
}

// bridgeOutbound は agentBus の outbound メッセージを channelBus に転送しつつログ記録する。
func (cl *ConversationLogger) bridgeOutbound(ctx context.Context) {
	for {
		msg, ok := cl.agentBus.SubscribeOutbound(ctx)
		if !ok {
			return
		}

		// 内部チャンネルのメッセージはログに記録しない
		if !isLoggableChannel(msg.Channel) {
			cl.channelBus.PublishOutbound(ctx, msg)
			continue
		}

		cl.logOutbound(msg)
		cl.channelBus.PublishOutbound(ctx, msg)
	}
}

// logInbound は inbound メッセージをファイルに記録する。
func (cl *ConversationLogger) logInbound(msg bus.InboundMessage) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Direction: "in",
		Sender:    msg.SenderID,
		Content:   msg.Content,
	}
	cl.writeEntry(msg.Channel, msg.ChatID, entry)
}

// logOutbound は outbound メッセージをファイルに記録する。
func (cl *ConversationLogger) logOutbound(msg bus.OutboundMessage) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Direction: "out",
		Content:   msg.Content,
	}
	cl.writeEntry(msg.Channel, msg.ChatID, entry)
}

// writeEntry は LogEntry を JSONL ファイルに追記する。
// ファイルパス: {basePath}/{channel}_{chatID}/{date}.jsonl
func (cl *ConversationLogger) writeEntry(channel, chatID string, entry LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		slog.Error("failed to marshal log entry", "component", "conversation-logger", "error", err)
		return
	}

	// ディレクトリ名に使えない文字をサニタイズ
	safeChatID := sanitizePath(chatID)
	dirName := fmt.Sprintf("%s_%s", channel, safeChatID)
	dirPath := filepath.Join(cl.basePath, dirName)

	date := time.Now().UTC().Format("2006-01-02")
	filePath := filepath.Join(dirPath, date+".jsonl")

	cl.mu.Lock()
	defer cl.mu.Unlock()

	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		slog.Error("failed to create log directory", "component", "conversation-logger", "path", dirPath, "error", err)
		return
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Error("failed to open log file", "component", "conversation-logger", "path", filePath, "error", err)
		return
	}
	defer f.Close()

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		slog.Error("failed to write to log file", "component", "conversation-logger", "path", filePath, "error", err)
	}
}

// isLoggableChannel は指定チャンネルのメッセージをログに記録すべきかを返す。
// system, cli, subagent などの内部チャンネルはログに記録しない。
func isLoggableChannel(channel string) bool {
	switch channel {
	case "system", "cli", "subagent":
		return false
	default:
		return true
	}
}

// sanitizePath はファイルパスに使えない文字を安全な文字に置換する。
func sanitizePath(s string) string {
	result := make([]byte, 0, len(s))
	for i := range len(s) {
		c := s[i]
		switch c {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			result = append(result, '_')
		default:
			result = append(result, c)
		}
	}
	return string(result)
}
