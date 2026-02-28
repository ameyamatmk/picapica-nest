package console

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// conversationChannel はチャンネルのメタデータ。
type conversationChannel struct {
	DirName string // ディレクトリ名 (例: "discord_test-channel")
	Label   string // 表示用ラベル (例: "discord / test-channel")
}

// conversationDate は会話ログの日付メタデータ。
type conversationDate struct {
	FileName string // ファイル名 (例: "2026-02-27.jsonl")
	Label    string // 表示用 (例: "2026-02-27")
}

// chatMessage は会話ログの1メッセージ。
type chatMessage struct {
	Time      string // 時刻表示 (HH:MM)
	Direction string // "in" or "out"
	Sender    string
	Content   string
}

// conversationsData は会話ログ画面のテンプレートデータ。
type conversationsData struct {
	pageData
	Channels       []conversationChannel
	Dates          []conversationDate
	CurrentChannel string
	CurrentDate    string
	Messages       []chatMessage
}

// logEntry は会話ログ JSONL の1行分。
// logging.LogEntry と同じ構造だが、依存を切るために独立定義。
type logEntry struct {
	Timestamp string `json:"ts"`
	Direction string `json:"dir"`
	Sender    string `json:"sender,omitempty"`
	Content   string `json:"content"`
}

// handleConversations は会話ログ画面をフルページで返す。
func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	data := s.buildConversationsData("", "")

	if err := s.conversationsTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("failed to render conversations page", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleConversationMessages は HTMX fragment としてメッセージ表示を返す。
func (s *Server) handleConversationMessages(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query().Get("channel")
	date := r.URL.Query().Get("date")

	data := s.buildConversationsData(channel, date)

	if err := s.conversationsTmpl.ExecuteTemplate(w, "conversations_content", data); err != nil {
		slog.Error("failed to render conversations content", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// buildConversationsData はテンプレートに渡すデータを構築する。
func (s *Server) buildConversationsData(channel, date string) conversationsData {
	data := conversationsData{
		pageData: pageData{
			Title:  "会話ログ",
			Active: "conversations",
		},
	}

	// チャンネル一覧を取得
	channels, err := listConversationChannels(s.workspacePath)
	if err != nil {
		slog.Error("failed to list conversation channels", "component", "console", "error", err)
	}
	data.Channels = channels

	// チャンネルが未指定 or 不正な場合、最初のチャンネルを選択
	if channel == "" && len(channels) > 0 {
		channel = channels[0].DirName
	}
	// パストラバーサル対策
	channel = filepath.Base(channel)
	data.CurrentChannel = channel

	// 選択中チャンネルの日付一覧を取得
	if channel != "" {
		dates, err := listConversationDates(s.workspacePath, channel)
		if err != nil {
			slog.Error("failed to list conversation dates", "component", "console", "channel", channel, "error", err)
		}
		data.Dates = dates

		// 日付が未指定の場合、最新の日付を選択
		if date == "" && len(dates) > 0 {
			date = dates[0].FileName
		}
	}
	// パストラバーサル対策
	date = filepath.Base(date)
	data.CurrentDate = date

	// メッセージを読み込み
	if channel != "" && date != "" {
		messages, err := loadConversationMessages(s.workspacePath, channel, date)
		if err != nil {
			slog.Error("failed to load conversation messages", "component", "console",
				"channel", channel, "date", date, "error", err)
		}
		data.Messages = messages
	}

	return data
}

// listConversationChannels は logs/ 直下のチャンネルディレクトリ一覧を返す。
// "app" ディレクトリは除外する。
func listConversationChannels(workspacePath string) ([]conversationChannel, error) {
	logsDir := filepath.Join(workspacePath, "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var channels []conversationChannel
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// "app" ディレクトリは会話ログではないので除外
		if name == "app" {
			continue
		}
		channels = append(channels, conversationChannel{
			DirName: name,
			Label:   formatChannelLabel(name),
		})
	}

	// ディレクトリ名で昇順ソート
	slices.SortFunc(channels, func(a, b conversationChannel) int {
		return strings.Compare(a.DirName, b.DirName)
	})

	return channels, nil
}

// formatChannelLabel はディレクトリ名を表示用ラベルに変換する。
// 例: "discord_test-channel" → "discord / test-channel"
func formatChannelLabel(dirName string) string {
	// 最初の "_" で分割
	idx := strings.Index(dirName, "_")
	if idx < 0 {
		return dirName
	}
	return dirName[:idx] + " / " + dirName[idx+1:]
}

// listConversationDates は指定チャンネルの日付一覧を降順で返す。
func listConversationDates(workspacePath, channel string) ([]conversationDate, error) {
	channelDir := filepath.Join(workspacePath, "logs", filepath.Base(channel))
	entries, err := os.ReadDir(channelDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var dates []conversationDate
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		dates = append(dates, conversationDate{
			FileName: e.Name(),
			Label:    strings.TrimSuffix(e.Name(), ".jsonl"),
		})
	}

	// 降順ソート（新しい順）
	slices.SortFunc(dates, func(a, b conversationDate) int {
		return strings.Compare(b.FileName, a.FileName)
	})

	return dates, nil
}

// loadConversationMessages は指定チャンネル・日付の会話ログを読み込む。
func loadConversationMessages(workspacePath, channel, fileName string) ([]chatMessage, error) {
	// パストラバーサル対策
	safeChannel := filepath.Base(channel)
	safeFileName := filepath.Base(fileName)
	if safeChannel != channel || safeFileName != fileName {
		return nil, nil
	}
	if !strings.HasSuffix(safeFileName, ".jsonl") {
		return nil, nil
	}

	filePath := filepath.Join(workspacePath, "logs", safeChannel, safeFileName)
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var messages []chatMessage
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry logEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			slog.Warn("failed to parse log entry", "component", "console", "error", err)
			continue
		}
		messages = append(messages, chatMessage{
			Time:      formatMessageTime(entry.Timestamp),
			Direction: entry.Direction,
			Sender:    entry.Sender,
			Content:   entry.Content,
		})
	}
	if err := scanner.Err(); err != nil {
		return messages, err
	}

	// 新しいメッセージを上に表示するため逆順にする
	slices.Reverse(messages)

	return messages, nil
}

// formatMessageTime はタイムスタンプから時刻部分（HH:MM）を抽出する。
func formatMessageTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		// パースできない場合はそのまま返す
		return ts
	}
	return t.Format("15:04")
}
