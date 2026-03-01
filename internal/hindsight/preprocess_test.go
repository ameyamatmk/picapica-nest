package hindsight

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

func TestCollectLogs_4AMBoundary(t *testing.T) {
	// Given: 深夜〜早朝のログ（4時境界をまたぐ）
	logsDir := t.TempDir()
	date := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)

	chDir := filepath.Join(logsDir, "discord_chat-001")
	os.MkdirAll(chDir, 0o755)

	// 2/27 のファイル: 03:00 JST（=2/26 18:00 UTC）は4時前なので除外される
	// 2/27 のファイル: 10:00 JST（=2/27 01:00 UTC）は4時以降なので含まれる
	os.WriteFile(filepath.Join(chDir, "2026-02-27.jsonl"), []byte(
		`{"ts":"2026-02-26T18:00:00Z","dir":"in","sender":"user","content":"before 4am excluded"}
{"ts":"2026-02-27T01:00:00Z","dir":"in","sender":"user","content":"after 4am included"}
`), 0o644)

	// 2/28 のファイル: 02:00 JST（=2/27 17:00 UTC）は翌日4時前なので含まれる
	// 2/28 のファイル: 05:00 JST（=2/27 20:00 UTC）は翌日4時以降なので除外
	os.WriteFile(filepath.Join(chDir, "2026-02-28.jsonl"), []byte(
		`{"ts":"2026-02-27T17:00:00Z","dir":"in","sender":"user","content":"late night included"}
{"ts":"2026-02-27T20:00:00Z","dir":"in","sender":"user","content":"next day excluded"}
`), 0o644)

	// When: 2/27 のログを収集
	entries, err := CollectLogs(logsDir, date)

	// Then: 4AM 境界内の2エントリのみ収集される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Content != "after 4am included" {
		t.Errorf("entry[0]: expected 'after 4am included', got %q", entries[0].Content)
	}
	if entries[1].Content != "late night included" {
		t.Errorf("entry[1]: expected 'late night included', got %q", entries[1].Content)
	}
}

func TestCollectLogs_4AMBoundary_ExactBoundary(t *testing.T) {
	// Given: ちょうど4時のエントリ
	logsDir := t.TempDir()
	date := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)

	chDir := filepath.Join(logsDir, "discord_chat-001")
	os.MkdirAll(chDir, 0o755)

	// 2/27 04:00 JST = 2/26 19:00 UTC → 含まれる（start <= ts）
	// 2/28 04:00 JST = 2/27 19:00 UTC → 除外される（ts < end）
	os.WriteFile(filepath.Join(chDir, "2026-02-27.jsonl"), []byte(
		`{"ts":"2026-02-26T19:00:00Z","dir":"in","sender":"user","content":"exactly 4am start"}
`), 0o644)
	os.WriteFile(filepath.Join(chDir, "2026-02-28.jsonl"), []byte(
		`{"ts":"2026-02-27T19:00:00Z","dir":"in","sender":"user","content":"exactly 4am end"}
`), 0o644)

	// When: 2/27 のログを収集
	entries, err := CollectLogs(logsDir, date)

	// Then: 開始時刻は含まれ、終了時刻は除外される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (start inclusive, end exclusive), got %d", len(entries))
	}
	if entries[0].Content != "exactly 4am start" {
		t.Errorf("expected start boundary entry, got %q", entries[0].Content)
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

func TestGroupByChannel(t *testing.T) {
	// Given: 異なるチャンネルのエントリ
	entries := []logging.LogEntry{
		{Timestamp: "2026-02-27T01:00:00Z", Direction: "in", ChatID: "111", Content: "a"},
		{Timestamp: "2026-02-27T01:00:01Z", Direction: "in", ChatID: "222", Content: "b"},
		{Timestamp: "2026-02-27T01:00:02Z", Direction: "in", ChatID: "111", Content: "c"},
		{Timestamp: "2026-02-27T01:00:03Z", Direction: "in", Content: "d"}, // ChatID 空
	}

	// When
	grouped := GroupByChannel(entries)

	// Then
	if len(grouped) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(grouped))
	}
	if len(grouped["111"]) != 2 {
		t.Errorf("expected 2 entries in '111', got %d", len(grouped["111"]))
	}
	if len(grouped["222"]) != 1 {
		t.Errorf("expected 1 entry in '222', got %d", len(grouped["222"]))
	}
	if len(grouped[""]) != 1 {
		t.Errorf("expected 1 entry in '', got %d", len(grouped[""]))
	}
}

func TestFormatTranscriptByChannel_MultipleChannels(t *testing.T) {
	// Given: 複数チャンネルのエントリ
	entries := []logging.LogEntry{
		{Timestamp: "2026-02-27T01:00:00Z", Direction: "in", ChatID: "111", Sender: "user1", Content: "hello"},
		{Timestamp: "2026-02-27T01:00:01Z", Direction: "out", ChatID: "111", Content: "hi!"},
		{Timestamp: "2026-02-27T02:00:00Z", Direction: "in", ChatID: "222", Sender: "user2", Content: "work log"},
	}

	labelFn := func(id string) string {
		switch id {
		case "111":
			return "#general"
		case "222":
			return "#work-log"
		}
		return id
	}

	// When
	result := FormatTranscriptByChannel(entries, labelFn)

	// Then: セクションが分かれている
	if !strings.Contains(result, "## #general") {
		t.Errorf("expected #general section, got:\n%s", result)
	}
	if !strings.Contains(result, "## #work-log") {
		t.Errorf("expected #work-log section, got:\n%s", result)
	}
	if !strings.Contains(result, "hello") {
		t.Error("expected 'hello' in output")
	}
	if !strings.Contains(result, "work log") {
		t.Error("expected 'work log' in output")
	}
}

func TestFormatTranscriptByChannel_SingleChannel(t *testing.T) {
	// Given: 全エントリが同一チャンネル
	entries := []logging.LogEntry{
		{Timestamp: "2026-02-27T01:00:00Z", Direction: "in", ChatID: "111", Sender: "user", Content: "hello"},
		{Timestamp: "2026-02-27T01:00:01Z", Direction: "out", ChatID: "111", Content: "hi!"},
	}

	// When: FormatTranscriptByChannel を呼ぶ
	result := FormatTranscriptByChannel(entries, nil)

	// Then: セクション見出しなしのフラット出力（FormatTranscript と同じ）
	if strings.Contains(result, "##") {
		t.Errorf("expected flat output for single channel, got:\n%s", result)
	}
	if !strings.Contains(result, "hello") {
		t.Error("expected content to be present")
	}
}

func TestFormatTranscriptByChannel_NilLabelFn(t *testing.T) {
	// Given: 複数チャンネル、labelFn は nil
	entries := []logging.LogEntry{
		{Timestamp: "2026-02-27T01:00:00Z", Direction: "in", ChatID: "111", Sender: "user", Content: "a"},
		{Timestamp: "2026-02-27T01:00:01Z", Direction: "in", ChatID: "222", Sender: "user", Content: "b"},
	}

	// When: labelFn = nil
	result := FormatTranscriptByChannel(entries, nil)

	// Then: ChatID がそのまま見出しに使われる
	if !strings.Contains(result, "## 111") {
		t.Errorf("expected '## 111' section, got:\n%s", result)
	}
	if !strings.Contains(result, "## 222") {
		t.Errorf("expected '## 222' section, got:\n%s", result)
	}
}
