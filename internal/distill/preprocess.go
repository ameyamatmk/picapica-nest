// Package distill は蒸留パイプラインの前処理とオーケストレーションを提供する。
package distill

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ameyamatmk/picapica-nest/internal/logging"
)

// アシスタント側の表示名
const assistantName = "picapica"

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
// logsDir 配下のサブディレクトリを走査し、{YYYY-MM-DD}.jsonl を読み込む。
// エントリはタイムスタンプ順にソートされずそのまま返す（JSONL は時系列順に書き込まれるため）。
func CollectLogs(logsDir string, date time.Time) ([]logging.LogEntry, error) {
	dateStr := date.Format("2006-01-02")
	fileName := dateStr + ".jsonl"

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read logs directory: %w", err)
	}

	var allEntries []logging.LogEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		filePath := filepath.Join(logsDir, entry.Name(), fileName)
		logEntries, err := readJSONL(filePath)
		if err != nil {
			// ファイルが存在しない場合はスキップ
			if os.IsNotExist(err) {
				continue
			}
			fmt.Fprintf(os.Stderr, "warning: failed to read %s: %v\n", filePath, err)
			continue
		}
		allEntries = append(allEntries, logEntries...)
	}

	return allEntries, nil
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
			fmt.Fprintf(os.Stderr, "warning: %s line %d: invalid JSON, skipping: %v\n", path, lineNum, err)
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return entries, fmt.Errorf("error reading %s: %w", path, err)
	}

	return entries, nil
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
