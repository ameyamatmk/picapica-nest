// Package hindsight は振り返りパイプラインの前処理とオーケストレーションを提供する。
package hindsight

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ameyamatmk/picapica-nest/internal/logging"
)

// アシスタント側の表示名
const assistantName = "picapica"

// dayBoundaryHour は日次ログの境界時刻（JST）。
// 午前4時で日をまたぐ（深夜〜4時の活動は前日扱い）。
const dayBoundaryHour = 4

// jst はタイムスタンプ表示用のタイムゾーン。
var jst *time.Location

func init() {
	var err error
	jst, err = time.LoadLocation("Asia/Tokyo")
	if err != nil {
		jst = time.FixedZone("JST", 9*60*60)
	}
}

// CollectLogs は指定日のログを全チャンネルから収集する。
// logsDir 配下のサブディレクトリを走査し、JSONL ファイルを読み込む。
// 日の境界は午前4時 JST（date の 04:00 〜 date+1 の 04:00）。
// 深夜〜午前4時のログは前日分として扱われる。
func CollectLogs(logsDir string, date time.Time) ([]logging.LogEntry, error) {
	// date の 04:00 JST ～ date+1 の 04:00 JST が対象範囲
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), dayBoundaryHour, 0, 0, 0, jst)
	dayEnd := dayStart.AddDate(0, 0, 1)

	// カレンダー日ベースの2つのファイルを読む（4時境界をまたぐため）
	fileNames := []string{
		date.Format("2006-01-02") + ".jsonl",
		date.AddDate(0, 0, 1).Format("2006-01-02") + ".jsonl",
	}

	dirEntries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read logs directory: %w", err)
	}

	var allEntries []logging.LogEntry
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}
		for _, fileName := range fileNames {
			filePath := filepath.Join(logsDir, dirEntry.Name(), fileName)
			logEntries, err := readJSONL(filePath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				slog.Warn("failed to read log file", "component", "hindsight", "path", filePath, "error", err)
				continue
			}
			allEntries = append(allEntries, logEntries...)
		}
	}

	return filterByTimeRange(allEntries, dayStart, dayEnd), nil
}

// filterByTimeRange はエントリをタイムスタンプの範囲でフィルタリングする。
// start <= timestamp < end の範囲に含まれるエントリを返す。
// タイムスタンプがパースできないエントリは安全側で含める。
func filterByTimeRange(entries []logging.LogEntry, start, end time.Time) []logging.LogEntry {
	var filtered []logging.LogEntry
	for _, entry := range entries {
		t, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			// パースできないエントリは含める
			filtered = append(filtered, entry)
			continue
		}
		if !t.Before(start) && t.Before(end) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// readJSONL は JSONL ファイルを読み込み、LogEntry のスライスを返す。
// 不正な JSON 行はスキップする。
func readJSONL(path string) ([]logging.LogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []logging.LogEntry
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry logging.LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			slog.Warn("invalid JSON in log file, skipping", "component", "hindsight", "path", path, "line", lineNum, "error", err)
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return entries, fmt.Errorf("error reading %s: %w", path, err)
	}

	return entries, nil
}

// GroupByChannel は LogEntry をチャンネル（channel_chatID）別にグルーピングする。
// Channel/ChatID が空のエントリは "" キーにまとめる。
func GroupByChannel(entries []logging.LogEntry) map[string][]logging.LogEntry {
	grouped := make(map[string][]logging.LogEntry)
	for _, entry := range entries {
		key := ""
		if entry.ChatID != "" {
			key = entry.ChatID
		}
		grouped[key] = append(grouped[key], entry)
	}
	return grouped
}

// FormatTranscriptByChannel はチャンネル別にセクション分けされたトランスクリプトを生成する。
// labelFn は ChatID → 表示名の変換関数（nil なら ChatID をそのまま使用）。
func FormatTranscriptByChannel(entries []logging.LogEntry, labelFn func(string) string) string {
	grouped := GroupByChannel(entries)

	// チャンネル情報がないか、すべて同一チャンネルならフラットに出力
	if len(grouped) <= 1 {
		return FormatTranscript(entries)
	}

	// ChatID でソートしてから出力（安定した順序のため）
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sortStrings(keys)

	var sb strings.Builder
	for i, key := range keys {
		if i > 0 {
			sb.WriteString("\n")
		}
		label := key
		if labelFn != nil && key != "" {
			label = labelFn(key)
		}
		if label == "" {
			label = "(unknown)"
		}
		fmt.Fprintf(&sb, "## %s\n\n", label)
		sb.WriteString(FormatTranscript(grouped[key]))
	}
	return sb.String()
}

// sortStrings は文字列スライスを昇順ソートする。
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// FormatTranscript は LogEntry をチャット形式の Markdown テキストに変換する。
func FormatTranscript(entries []logging.LogEntry) string {
	if len(entries) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, entry := range entries {
		if i > 0 {
			sb.WriteString("\n")
		}

		name := assistantName
		if entry.Direction == "in" && entry.Sender != "" {
			name = entry.Sender
		}

		timeStr := formatTime(entry.Timestamp)
		fmt.Fprintf(&sb, "**%s** (%s): %s\n", name, timeStr, entry.Content)
	}
	return sb.String()
}

// formatTime はタイムスタンプ文字列を JST の HH:MM 形式に変換する。
// パースに失敗した場合は元の文字列を返す。
func formatTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.In(jst).Format("15:04")
}
