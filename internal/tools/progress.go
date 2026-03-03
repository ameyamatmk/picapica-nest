package tools

import (
	"context"
	"log/slog"

	"github.com/ameyamatmk/picapica-nest/internal/claudecode"
	"github.com/sipeed/picoclaw/pkg/bus"
)

// ツール名→進捗メッセージのマッピング
var toolProgressMessages = map[string]string{
	"WebSearch": "Web を検索中...",
	"WebFetch":  "ページを読み込み中...",
	"Read":      "ファイルを読み取り中...",
}

// progressMessage はツール名に対応する進捗メッセージを返す。
func progressMessage(toolName string) string {
	if msg, ok := toolProgressMessages[toolName]; ok {
		return msg
	}
	return toolName + " を実行中..."
}

// newProgressFunc は agentBus に進捗メッセージを送信する ProgressFunc を返す。
// 同じツール名の重複通知を防ぐ。
// mb, channel, chatID のいずれかが空の場合は nil を返す（進捗通知なし）。
func newProgressFunc(mb *bus.MessageBus, channel, chatID string) claudecode.ProgressFunc {
	if mb == nil || channel == "" || chatID == "" {
		return nil
	}

	notified := make(map[string]bool)

	return func(event claudecode.StreamEvent) {
		if event.Type != "tool_use" {
			return
		}
		if notified[event.ToolName] {
			return
		}
		notified[event.ToolName] = true

		msg := progressMessage(event.ToolName)
		if err := mb.PublishOutbound(context.Background(), bus.OutboundMessage{
			Channel: channel,
			ChatID:  chatID,
			Content: msg,
		}); err != nil {
			slog.Warn("failed to send progress", "error", err, "tool", event.ToolName)
		}
	}
}
