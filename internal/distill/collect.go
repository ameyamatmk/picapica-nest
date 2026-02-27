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
