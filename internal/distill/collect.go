package distill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// isoWeekStart は ISO 週番号から週の開始日（月曜日）を返す。
func isoWeekStart(year, week int) time.Time {
	// 1月4日は必ず ISO 第1週に含まれる
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	// 第1週の月曜日を求める
	offset := int(time.Monday - jan4.Weekday())
	if offset > 0 {
		offset -= 7
	}
	weekOneMonday := jan4.AddDate(0, 0, offset)
	// 指定週の月曜日
	return weekOneMonday.AddDate(0, 0, (week-1)*7)
}

// isoWeekEnd は ISO 週番号から週の終了日（日曜日）を返す。
func isoWeekEnd(year, week int) time.Time {
	return isoWeekStart(year, week).AddDate(0, 0, 6)
}

// weekStartOffset は週の開始曜日の ISO 月曜日からのオフセット（日数）。
// -2 = 土曜日, -1 = 日曜日, 0 = 月曜日
const weekStartOffset = -2

// WeekEndDay は週の終了曜日を返す。
// cmd 側でデフォルト週を計算する際に使用する。
func WeekEndDay() time.Weekday {
	// ISO 月曜 + weekStartOffset + 6
	return time.Weekday((int(time.Monday) + weekStartOffset + 6 + 7) % 7)
}

// weekStartSat は ISO 週番号に対応する週の開始日を返す。
// 開始曜日は weekStartOffset で決まる（デフォルト: 土曜日）。
func weekStartSat(year, week int) time.Time {
	return isoWeekStart(year, week).AddDate(0, 0, weekStartOffset)
}

// weekEndFri は ISO 週番号に対応する週の終了日を返す。
// 開始日の6日後（デフォルト: 金曜日）。
func weekEndFri(year, week int) time.Time {
	return isoWeekStart(year, week).AddDate(0, 0, weekStartOffset+6)
}

// weekFileName は週次レポートのファイル名を返す。
// 形式: YYYY-MM-WNN.md（月は週の終了日=金曜日の月）。
// 金曜の月を使うことで、月次レポート作成時に作成済みの週次レポートのみが
// 当月のファイル名パターンにマッチする。
func weekFileName(year, week int) string {
	end := weekEndFri(year, week)
	return fmt.Sprintf("%d-%02d-W%02d.md", year, int(end.Month()), week)
}

// readReportFile は Markdown レポートファイルを読み込む。
// ファイルが存在しない場合は ("", nil) を返す。
func readReportFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read report %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// writeReportFile は蒸留結果をファイルに保存する。
// ファイル名を文字列で受け取る（週次の YYYY-WNN.md や月次の YYYY-MM.md に対応）。
func writeReportFile(outputDir string, fileName string, content string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	filePath := filepath.Join(outputDir, fileName)
	return os.WriteFile(filePath, []byte(content+"\n"), 0o644)
}
