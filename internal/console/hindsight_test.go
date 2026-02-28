package console

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListReports_ReturnsDescendingOrder(t *testing.T) {
	// Given: 日次レポートが3つ存在する
	dir := t.TempDir()
	dailyDir := filepath.Join(dir, "memory", "daily")
	if err := os.MkdirAll(dailyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"2026-02-26.md", "2026-02-28.md", "2026-02-27.md"} {
		if err := os.WriteFile(filepath.Join(dailyDir, name), []byte("# test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// When: listReports を呼ぶ
	files, err := listReports(dir, "daily")

	// Then: 降順で返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	if files[0].Name != "2026-02-28.md" {
		t.Errorf("expected first file '2026-02-28.md', got %q", files[0].Name)
	}
	if files[2].Name != "2026-02-26.md" {
		t.Errorf("expected last file '2026-02-26.md', got %q", files[2].Name)
	}
}

func TestListReports_EmptyDirectory(t *testing.T) {
	// Given: ディレクトリが存在しない
	dir := filepath.Join(t.TempDir(), "nonexistent")

	// When: listReports を呼ぶ
	files, err := listReports(dir, "daily")

	// Then: エラーなしで空スライス
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestListReports_IgnoresNonMarkdown(t *testing.T) {
	// Given: .md 以外のファイルも含むディレクトリ
	dir := t.TempDir()
	dailyDir := filepath.Join(dir, "memory", "daily")
	if err := os.MkdirAll(dailyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dailyDir, "2026-02-28.md"), []byte("# test"), 0o644)
	os.WriteFile(filepath.Join(dailyDir, "notes.txt"), []byte("note"), 0o644)
	os.MkdirAll(filepath.Join(dailyDir, "subdir"), 0o755)

	// When: listReports を呼ぶ
	files, err := listReports(dir, "daily")

	// Then: .md ファイルのみ返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestReadReport_PathTraversal(t *testing.T) {
	// Given: ワークスペース
	dir := t.TempDir()

	// When: パストラバーサル攻撃のファイル名
	_, err := readReport(dir, "daily", "../../../etc/passwd")

	// Then: エラーが返る
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func TestReadReport_RendersMarkdown(t *testing.T) {
	// Given: Markdown レポートファイル
	dir := t.TempDir()
	dailyDir := filepath.Join(dir, "memory", "daily")
	if err := os.MkdirAll(dailyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dailyDir, "2026-02-28.md"), []byte("# Hello\n\n- item1\n- item2\n"), 0o644)

	// When: readReport を呼ぶ
	html, err := readReport(dir, "daily", "2026-02-28.md")

	// Then: HTML に変換されている
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	htmlStr := string(html)
	if !strings.Contains(htmlStr, "<h1>Hello</h1>") {
		t.Errorf("expected <h1>Hello</h1>, got %s", htmlStr)
	}
	if !strings.Contains(htmlStr, "<li>item1</li>") {
		t.Errorf("expected <li>item1</li>, got %s", htmlStr)
	}
}

func TestReportDir_TabMapping(t *testing.T) {
	tests := []struct {
		tab      string
		expected string
	}{
		{"daily", "memory/daily"},
		{"weekly", "memory/weekly"},
		{"monthly", "memory/monthly"},
		{"", "memory/daily"},
		{"unknown", "memory/daily"},
	}

	for _, tt := range tests {
		t.Run("tab="+tt.tab, func(t *testing.T) {
			// When: reportDir を呼ぶ
			got := reportDir("/workspace", tt.tab)

			// Then: 期待するパスが返る
			expected := filepath.Join("/workspace", tt.expected)
			if got != expected {
				t.Errorf("expected %q, got %q", expected, got)
			}
		})
	}
}

func TestHandleHindsight_ReturnsHTML(t *testing.T) {
	// Given: レポートファイルが存在するワークスペース
	dir := t.TempDir()
	dailyDir := filepath.Join(dir, "memory", "daily")
	os.MkdirAll(dailyDir, 0o755)
	os.WriteFile(filepath.Join(dailyDir, "2026-02-28.md"), []byte("# Test Report"), 0o644)

	s := NewServer(dir)

	// When: GET /hindsight にリクエスト
	req := httptest.NewRequest("GET", "/hindsight", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でHTMLが返り、レポート内容を含む
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Hindsight") {
		t.Error("expected page to contain 'Hindsight'")
	}
	if !strings.Contains(body, "Test Report") {
		t.Error("expected page to contain rendered report")
	}
}

func TestBuildCalendar_February2026(t *testing.T) {
	// Given: 2026年2月（28日間、2/1は日曜）
	reportDates := map[string]bool{
		"2026-02-23": true,
		"2026-02-27": true,
		"2026-02-28": true,
	}

	// When: buildCalendar を呼ぶ
	cal := buildCalendar(2026, 2, reportDates, "2026-02-27.md")

	// Then: 正しい年月
	if cal.Year != 2026 || cal.Month != 2 {
		t.Errorf("expected 2026/2, got %d/%d", cal.Year, cal.Month)
	}
	if cal.MonthName != "2026年2月" {
		t.Errorf("expected '2026年2月', got %q", cal.MonthName)
	}

	// Then: レポートがある日と選択状態を確認
	found23 := false
	found27Selected := false
	for _, week := range cal.Weeks {
		for _, day := range week {
			if day.Day == 23 && day.HasReport {
				found23 = true
			}
			if day.Day == 27 && day.Selected {
				found27Selected = true
			}
		}
	}
	if !found23 {
		t.Error("expected day 23 to have HasReport=true")
	}
	if !found27Selected {
		t.Error("expected day 27 to have Selected=true")
	}

	// Then: 前月・翌月
	if cal.PrevYear != 2026 || cal.PrevMonth != 1 {
		t.Errorf("expected prev 2026/1, got %d/%d", cal.PrevYear, cal.PrevMonth)
	}
	if cal.NextYear != 2026 || cal.NextMonth != 3 {
		t.Errorf("expected next 2026/3, got %d/%d", cal.NextYear, cal.NextMonth)
	}
}

func TestBuildCalendar_YearBoundary(t *testing.T) {
	// Given: 2026年1月 → 前月は2025年12月
	cal := buildCalendar(2026, 1, nil, "")

	// Then
	if cal.PrevYear != 2025 || cal.PrevMonth != 12 {
		t.Errorf("expected prev 2025/12, got %d/%d", cal.PrevYear, cal.PrevMonth)
	}
}

func TestBuildCalendar_December_NextIsJanuary(t *testing.T) {
	// Given: 2025年12月 → 翌月は2026年1月
	cal := buildCalendar(2025, 12, nil, "")

	// Then
	if cal.NextYear != 2026 || cal.NextMonth != 1 {
		t.Errorf("expected next 2026/1, got %d/%d", cal.NextYear, cal.NextMonth)
	}
}

func TestBuildCalendar_EmptyReportDates(t *testing.T) {
	// Given: レポートなし
	cal := buildCalendar(2026, 2, nil, "")

	// Then: 全セルの HasReport が false
	for _, week := range cal.Weeks {
		for _, day := range week {
			if day.HasReport {
				t.Errorf("expected no HasReport, but day %d has it", day.Day)
			}
		}
	}
}

func TestListReportDates_ReturnsAllDates(t *testing.T) {
	// Given: 日次レポートが3つ存在する
	dir := t.TempDir()
	dailyDir := filepath.Join(dir, "memory", "daily")
	os.MkdirAll(dailyDir, 0o755)
	for _, name := range []string{"2026-01-15.md", "2026-02-27.md", "2026-02-28.md"} {
		os.WriteFile(filepath.Join(dailyDir, name), []byte("# test"), 0o644)
	}

	// When: listReportDates を呼ぶ
	dates, err := listReportDates(dir)

	// Then: 3日分の日付が返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dates) != 3 {
		t.Fatalf("expected 3 dates, got %d", len(dates))
	}
	if !dates["2026-01-15"] {
		t.Error("expected 2026-01-15 in dates")
	}
}

func TestListReportDates_EmptyDirectory(t *testing.T) {
	// Given: ディレクトリが存在しない
	dir := filepath.Join(t.TempDir(), "nonexistent")

	// When
	dates, err := listReportDates(dir)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dates != nil {
		t.Errorf("expected nil, got %v", dates)
	}
}

func TestFindLatestReportInMonth_Found(t *testing.T) {
	dates := map[string]bool{
		"2026-02-15": true,
		"2026-02-20": true,
		"2026-02-28": true,
	}

	got := findLatestReportInMonth(2026, 2, dates)
	if got != "2026-02-28.md" {
		t.Errorf("expected '2026-02-28.md', got %q", got)
	}
}

func TestFindLatestReportInMonth_NotFound(t *testing.T) {
	dates := map[string]bool{
		"2026-01-15": true,
	}

	got := findLatestReportInMonth(2026, 2, dates)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestParseYearMonth_Valid(t *testing.T) {
	req := httptest.NewRequest("GET", "/hindsight/content?year=2026&month=2", nil)
	year, month := parseYearMonth(req)
	if year != 2026 || month != 2 {
		t.Errorf("expected 2026/2, got %d/%d", year, month)
	}
}

func TestParseYearMonth_Missing(t *testing.T) {
	req := httptest.NewRequest("GET", "/hindsight/content", nil)
	year, month := parseYearMonth(req)
	if year != 0 || month != 0 {
		t.Errorf("expected 0/0, got %d/%d", year, month)
	}
}

func TestParseYearMonth_Invalid(t *testing.T) {
	req := httptest.NewRequest("GET", "/hindsight/content?year=abc&month=2", nil)
	year, month := parseYearMonth(req)
	if year != 0 || month != 0 {
		t.Errorf("expected 0/0, got %d/%d", year, month)
	}
}

func TestParseYearMonth_OutOfRange(t *testing.T) {
	req := httptest.NewRequest("GET", "/hindsight/content?year=2026&month=13", nil)
	year, month := parseYearMonth(req)
	if year != 0 || month != 0 {
		t.Errorf("expected 0/0 for out of range month, got %d/%d", year, month)
	}
}

func TestHandleHindsight_DailyShowsCalendar(t *testing.T) {
	// Given: 日次レポートが存在するワークスペース
	dir := t.TempDir()
	dailyDir := filepath.Join(dir, "memory", "daily")
	os.MkdirAll(dailyDir, 0o755)
	os.WriteFile(filepath.Join(dailyDir, "2026-02-28.md"), []byte("# Daily Report"), 0o644)

	s := NewServer(dir)

	// When: GET /hindsight?tab=daily
	req := httptest.NewRequest("GET", "/hindsight?tab=daily", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: カレンダーUIが含まれる
	body := rec.Body.String()
	if !strings.Contains(body, "calendar-grid") {
		t.Error("expected calendar grid in daily tab")
	}
}

func TestHandleHindsightContent_MonthNavigation(t *testing.T) {
	// Given: 1月にレポートがあるワークスペース
	dir := t.TempDir()
	dailyDir := filepath.Join(dir, "memory", "daily")
	os.MkdirAll(dailyDir, 0o755)
	os.WriteFile(filepath.Join(dailyDir, "2026-01-15.md"), []byte("# Jan"), 0o644)

	s := NewServer(dir)

	// When: 1月のカレンダーをリクエスト
	req := httptest.NewRequest("GET", "/hindsight/content?tab=daily&year=2026&month=1", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 1月のカレンダーが表示される
	body := rec.Body.String()
	if !strings.Contains(body, "2026年1月") {
		t.Error("expected '2026年1月' in response")
	}
}

func TestHandleHindsightContent_WeeklyStillShowsList(t *testing.T) {
	// Given: 週次レポートが存在するワークスペース
	dir := t.TempDir()
	weeklyDir := filepath.Join(dir, "memory", "weekly")
	os.MkdirAll(weeklyDir, 0o755)
	os.WriteFile(filepath.Join(weeklyDir, "2026-02-W09.md"), []byte("# Weekly"), 0o644)

	s := NewServer(dir)

	// When: GET /hindsight/content?tab=weekly
	req := httptest.NewRequest("GET", "/hindsight/content?tab=weekly", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: リスト表示（カレンダーなし）
	body := rec.Body.String()
	if strings.Contains(body, "calendar-grid") {
		t.Error("weekly tab should not contain calendar")
	}
	if !strings.Contains(body, "Weekly") {
		t.Error("expected weekly report content")
	}
}

func TestHandleHindsightContent_Fragment(t *testing.T) {
	// Given: レポートファイルが存在するワークスペース
	dir := t.TempDir()
	weeklyDir := filepath.Join(dir, "memory", "weekly")
	os.MkdirAll(weeklyDir, 0o755)
	os.WriteFile(filepath.Join(weeklyDir, "2026-02-W09.md"), []byte("# Weekly"), 0o644)

	s := NewServer(dir)

	// When: GET /hindsight/content?tab=weekly にリクエスト
	req := httptest.NewRequest("GET", "/hindsight/content?tab=weekly", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でフラグメントが返る（layout を含まない）
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("fragment should not contain DOCTYPE")
	}
	if !strings.Contains(body, "Weekly") {
		t.Error("expected fragment to contain rendered report")
	}
}
