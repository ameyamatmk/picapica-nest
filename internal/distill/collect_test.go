package distill

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsoWeekStart_NormalWeek(t *testing.T) {
	// Given: 2026-W09
	// When: isoWeekStart を呼ぶ
	got := isoWeekStart(2026, 9)

	// Then: 2026-02-23（月曜）が返る
	expected := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)
	if !got.Equal(expected) {
		t.Errorf("expected %s, got %s", expected.Format("2006-01-02"), got.Format("2006-01-02"))
	}
}

func TestIsoWeekStart_FirstWeek(t *testing.T) {
	// Given: 2026-W01
	// When: isoWeekStart を呼ぶ
	got := isoWeekStart(2026, 1)

	// Then: 2025-12-29（月曜）が返る（ISO 週は前年にまたがることがある）
	expected := time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC)
	if !got.Equal(expected) {
		t.Errorf("expected %s, got %s", expected.Format("2006-01-02"), got.Format("2006-01-02"))
	}
}

func TestIsoWeekStart_LastWeek(t *testing.T) {
	// Given: 2025-W52（2025年は52週）
	// When: isoWeekStart を呼ぶ
	got := isoWeekStart(2025, 52)

	// Then: 2025-12-22（月曜）が返る
	expected := time.Date(2025, 12, 22, 0, 0, 0, 0, time.UTC)
	if !got.Equal(expected) {
		t.Errorf("expected %s, got %s", expected.Format("2006-01-02"), got.Format("2006-01-02"))
	}
}

func TestIsoWeekEnd_NormalWeek(t *testing.T) {
	// Given: 2026-W09
	// When: isoWeekEnd を呼ぶ
	got := isoWeekEnd(2026, 9)

	// Then: 2026-03-01（日曜）が返る
	expected := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(expected) {
		t.Errorf("expected %s, got %s", expected.Format("2006-01-02"), got.Format("2006-01-02"))
	}
}

func TestIsoWeekStartEnd_Consistency(t *testing.T) {
	// Given: 複数の週について
	// When: start と end を計算する
	// Then: end - start = 6日、start.Weekday() = Monday、end.Weekday() = Sunday
	testCases := []struct {
		year, week int
	}{
		{2026, 1},
		{2026, 9},
		{2026, 52},
		{2025, 1},
	}
	for _, tc := range testCases {
		start := isoWeekStart(tc.year, tc.week)
		end := isoWeekEnd(tc.year, tc.week)

		if start.Weekday() != time.Monday {
			t.Errorf("W%02d: start weekday = %s, want Monday", tc.week, start.Weekday())
		}
		if end.Weekday() != time.Sunday {
			t.Errorf("W%02d: end weekday = %s, want Sunday", tc.week, end.Weekday())
		}
		diff := end.Sub(start)
		if diff != 6*24*time.Hour {
			t.Errorf("W%02d: end - start = %v, want 6 days", tc.week, diff)
		}
	}
}

func TestWeekStartSat_NormalWeek(t *testing.T) {
	// Given: 2026-W09（ISO 月曜 = 2/23）
	// When: weekStartSat を呼ぶ
	got := weekStartSat(2026, 9)

	// Then: 2026-02-21（土曜）が返る
	expected := time.Date(2026, 2, 21, 0, 0, 0, 0, time.UTC)
	if !got.Equal(expected) {
		t.Errorf("expected %s (%s), got %s (%s)",
			expected.Format("2006-01-02"), expected.Weekday(),
			got.Format("2006-01-02"), got.Weekday())
	}
}

func TestWeekEndFri_NormalWeek(t *testing.T) {
	// Given: 2026-W09
	// When: weekEndFri を呼ぶ
	got := weekEndFri(2026, 9)

	// Then: 2026-02-27（金曜）が返る
	expected := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)
	if !got.Equal(expected) {
		t.Errorf("expected %s (%s), got %s (%s)",
			expected.Format("2006-01-02"), expected.Weekday(),
			got.Format("2006-01-02"), got.Weekday())
	}
}

func TestWeekStartEndSat_Consistency(t *testing.T) {
	// Given: 複数の週について
	// Then: start = Saturday, end = Friday, end - start = 6日
	testCases := []struct{ year, week int }{
		{2026, 1}, {2026, 9}, {2026, 52}, {2025, 1},
	}
	for _, tc := range testCases {
		start := weekStartSat(tc.year, tc.week)
		end := weekEndFri(tc.year, tc.week)

		if start.Weekday() != time.Saturday {
			t.Errorf("W%02d: start weekday = %s, want Saturday", tc.week, start.Weekday())
		}
		if end.Weekday() != time.Friday {
			t.Errorf("W%02d: end weekday = %s, want Friday", tc.week, end.Weekday())
		}
		diff := end.Sub(start)
		if diff != 6*24*time.Hour {
			t.Errorf("W%02d: end - start = %v, want 6 days", tc.week, diff)
		}
	}
}

func TestWeekFileName(t *testing.T) {
	// Given: 2026-W09（金曜 = 2/27 → 2月）
	got := weekFileName(2026, 9)
	expected := "2026-02-W09.md"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	// W10 の金曜 = 3/6 → 3月
	got = weekFileName(2026, 10)
	expected = "2026-03-W10.md"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestReadReportFile_Exists(t *testing.T) {
	// Given: 内容のあるレポートファイル
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	os.WriteFile(path, []byte("# Report\n\nContent here.\n"), 0o644)

	// When: readReportFile を呼ぶ
	content, err := readReportFile(path)

	// Then: 内容が返る（末尾の改行はトリムされる）
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "# Report\n\nContent here." {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestReadReportFile_NotExists(t *testing.T) {
	// Given: 存在しないファイルパス
	// When: readReportFile を呼ぶ
	content, err := readReportFile("/nonexistent/report.md")

	// Then: 空文字列・エラーなし
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string, got %q", content)
	}
}

func TestWriteReportFile_CreatesDirectoryAndFile(t *testing.T) {
	// Given: 存在しないディレクトリパス
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "new", "nested")

	// When: writeReportFile を呼ぶ
	err := writeReportFile(outputDir, "2026-W09.md", "# Weekly Report")

	// Then: ディレクトリとファイルが作成される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(outputDir, "2026-W09.md"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "# Weekly Report\n" {
		t.Errorf("unexpected content: %q", string(data))
	}
}
