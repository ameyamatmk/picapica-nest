package distill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ameyamatmk/picapica-nest/internal/logging"
)

func TestCollectLogs_MultipleChannels(t *testing.T) {
	// Given: 2つのチャンネルに同日のログがある
	logsDir := t.TempDir()
	date := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)

	ch1Dir := filepath.Join(logsDir, "discord_chat-001")
	ch2Dir := filepath.Join(logsDir, "discord_chat-002")
	os.MkdirAll(ch1Dir, 0o755)
	os.MkdirAll(ch2Dir, 0o755)

	os.WriteFile(filepath.Join(ch1Dir, "2026-02-27.jsonl"), []byte(
		`{"ts":"2026-02-27T01:00:00Z","dir":"in","sender":"user#1","content":"hello"}
{"ts":"2026-02-27T01:00:05Z","dir":"out","content":"hi there!"}
`), 0o644)

	os.WriteFile(filepath.Join(ch2Dir, "2026-02-27.jsonl"), []byte(
		`{"ts":"2026-02-27T02:00:00Z","dir":"in","sender":"user#2","content":"hey"}
`), 0o644)

	// When: CollectLogs を呼ぶ
	entries, err := CollectLogs(logsDir, date)

	// Then: 全チャンネルのエントリが収集される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestCollectLogs_EmptyDirectory(t *testing.T) {
	// Given: 空のログディレクトリ
	logsDir := t.TempDir()
	date := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)

	// When: CollectLogs を呼ぶ
	entries, err := CollectLogs(logsDir, date)

	// Then: 空のスライスが返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestCollectLogs_NonExistentDirectory(t *testing.T) {
	// Given: 存在しないディレクトリ
	date := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)

	// When: CollectLogs を呼ぶ
	entries, err := CollectLogs("/nonexistent/path", date)

	// Then: エラーなしで空が返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestCollectLogs_InvalidJSONSkipped(t *testing.T) {
	// Given: 不正な JSON 行を含むログファイル
	logsDir := t.TempDir()
	date := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)

	chDir := filepath.Join(logsDir, "discord_chat-001")
	os.MkdirAll(chDir, 0o755)

	os.WriteFile(filepath.Join(chDir, "2026-02-27.jsonl"), []byte(
		`{"ts":"2026-02-27T01:00:00Z","dir":"in","sender":"user#1","content":"valid"}
this is not json
{"ts":"2026-02-27T01:00:10Z","dir":"out","content":"also valid"}
`), 0o644)

	// When: CollectLogs を呼ぶ
	entries, err := CollectLogs(logsDir, date)

	// Then: 有効なエントリのみ収集され、不正行はスキップ
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (skipping invalid), got %d", len(entries))
	}
}

func TestCollectLogs_NoMatchingDate(t *testing.T) {
	// Given: 異なる日付のログファイルのみ
	logsDir := t.TempDir()
	date := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)

	chDir := filepath.Join(logsDir, "discord_chat-001")
	os.MkdirAll(chDir, 0o755)

	os.WriteFile(filepath.Join(chDir, "2026-02-26.jsonl"), []byte(
		`{"ts":"2026-02-26T01:00:00Z","dir":"in","sender":"user#1","content":"yesterday"}
`), 0o644)

	// When: 2/27 のログを収集
	entries, err := CollectLogs(logsDir, date)

	// Then: 空のスライスが返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestFormatTranscript_InboundAndOutbound(t *testing.T) {
	// Given: in と out のエントリ
	entries := []logging.LogEntry{
		{Timestamp: "2026-02-27T01:23:00Z", Direction: "in", Sender: "user#1234", Content: "今日の会議どうだった？"},
		{Timestamp: "2026-02-27T01:23:05Z", Direction: "out", Content: "会議の件だね！"},
	}

	// When: FormatTranscript を呼ぶ
	result := FormatTranscript(entries)

	// Then: チャット形式の Markdown が返る
	if !strings.Contains(result, "**user#1234** (10:23)") {
		t.Errorf("expected sender name with JST time, got:\n%s", result)
	}
	if !strings.Contains(result, "**picapica** (10:23)") {
		t.Errorf("expected assistant name with JST time, got:\n%s", result)
	}
	if !strings.Contains(result, "今日の会議どうだった？") {
		t.Error("expected content to be preserved")
	}
}

func TestFormatTranscript_EmptyEntries(t *testing.T) {
	// Given: 空のエントリリスト
	// When: FormatTranscript を呼ぶ
	result := FormatTranscript(nil)

	// Then: 空文字列が返る
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatTranscript_TimeConversion(t *testing.T) {
	// Given: UTC 15:00 = JST 00:00 (翌日)
	entries := []logging.LogEntry{
		{Timestamp: "2026-02-27T15:00:00Z", Direction: "in", Sender: "user", Content: "midnight JST"},
	}

	// When: FormatTranscript を呼ぶ
	result := FormatTranscript(entries)

	// Then: JST に変換されて 00:00 と表示される
	if !strings.Contains(result, "(00:00)") {
		t.Errorf("expected JST 00:00, got:\n%s", result)
	}
}

func TestFormatTranscript_InvalidTimestamp(t *testing.T) {
	// Given: パースできないタイムスタンプ
	entries := []logging.LogEntry{
		{Timestamp: "not-a-timestamp", Direction: "in", Sender: "user", Content: "test"},
	}

	// When: FormatTranscript を呼ぶ
	result := FormatTranscript(entries)

	// Then: 元のタイムスタンプがそのまま表示される
	if !strings.Contains(result, "(not-a-timestamp)") {
		t.Errorf("expected raw timestamp fallback, got:\n%s", result)
	}
}
