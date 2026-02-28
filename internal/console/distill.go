package console

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ReportFile はレポートファイルのメタデータ。
type ReportFile struct {
	Name  string // ファイル名 (例: "2026-02-28.md")
	Label string // 表示用ラベル (例: "2026-02-28")
}

// calendarDay はカレンダーの1日分のセルデータ。
type calendarDay struct {
	Day       int    // 日（0 = 空セル: 前月・翌月のパディング）
	HasReport bool   // レポートファイルが存在するか
	Selected  bool   // 現在選択中の日付か
	FileName  string // レポートファイル名 (例: "2026-02-28.md")
}

// calendarMonth はカレンダーの1か月分のデータ。
type calendarMonth struct {
	Year      int
	Month     int
	MonthName string          // 表示用 (例: "2026年2月")
	Weeks     [][]calendarDay // 可変行 x 7列（日〜土）
	PrevYear  int
	PrevMonth int
	NextYear  int
	NextMonth int
}

// distillData は蒸留レポート画面のテンプレートデータ。
type distillData struct {
	pageData
	Tab         string
	Files       []ReportFile   // 週次・月次用
	Calendar    *calendarMonth // 日次用
	CurrentFile string
	Content     template.HTML
}

// handleDistill は蒸留レポート画面をフルページで返す。
func (s *Server) handleDistill(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "daily"
	}

	year, month := parseYearMonth(r)
	data := s.buildDistillData(tab, "", year, month)

	if err := s.distillTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("failed to render distill page", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleDistillContent は HTMX fragment としてレポートリスト + 内容を返す。
func (s *Server) handleDistillContent(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "daily"
	}
	file := r.URL.Query().Get("file")

	year, month := parseYearMonth(r)
	data := s.buildDistillData(tab, file, year, month)

	if err := s.distillTmpl.ExecuteTemplate(w, "distill_content", data); err != nil {
		slog.Error("failed to render distill content", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// buildDistillData はテンプレートに渡すデータを構築する。
func (s *Server) buildDistillData(tab, file string, year, month int) distillData {
	data := distillData{
		pageData: pageData{
			Title:  "蒸留レポート",
			Active: "distill",
		},
		Tab: tab,
	}

	if tab == "daily" {
		// 日次: カレンダー表示
		now := time.Now()
		if year == 0 || month == 0 {
			year = now.Year()
			month = int(now.Month())
		}

		reportDates, err := listReportDates(s.workspacePath)
		if err != nil {
			slog.Error("failed to list report dates", "component", "console", "error", err)
		}

		if file == "" && reportDates != nil {
			file = findLatestReportInMonth(year, month, reportDates)
		}

		data.Calendar = buildCalendar(year, month, reportDates, file)
	} else {
		// 週次・月次: リスト表示
		files, err := listReports(s.workspacePath, tab)
		if err != nil {
			slog.Error("failed to list reports", "component", "console", "tab", tab, "error", err)
		}
		if file == "" && len(files) > 0 {
			file = files[0].Name
		}
		data.Files = files
	}

	data.CurrentFile = file

	if file != "" {
		content, err := readReport(s.workspacePath, tab, file)
		if err != nil {
			slog.Error("failed to read report", "component", "console", "file", file, "error", err)
		}
		data.Content = content
	}

	return data
}

// maxReportFiles はレポートリストの最大表示件数。
const maxReportFiles = 30

// listReports は指定タブのレポートファイル一覧を降順で返す。
// 最大 maxReportFiles 件まで返す。
func listReports(workspacePath, tab string) ([]ReportFile, error) {
	dir := reportDir(workspacePath, tab)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []ReportFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		files = append(files, ReportFile{
			Name:  e.Name(),
			Label: strings.TrimSuffix(e.Name(), ".md"),
		})
	}

	// 降順ソート（新しい順）
	slices.SortFunc(files, func(a, b ReportFile) int {
		return strings.Compare(b.Name, a.Name)
	})
	if len(files) > maxReportFiles {
		files = files[:maxReportFiles]
	}
	return files, nil
}

// readReport は指定レポートファイルを読み込み、Markdown → HTML 変換して返す。
func readReport(workspacePath, tab, fileName string) (template.HTML, error) {
	// パストラバーサル対策
	safeName := filepath.Base(fileName)
	if safeName != fileName || !strings.HasSuffix(safeName, ".md") {
		return "", fmt.Errorf("invalid file name: %s", fileName)
	}

	path := filepath.Join(reportDir(workspacePath, tab), safeName)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	html, err := renderMarkdown(data)
	if err != nil {
		return "", err
	}
	return template.HTML(html), nil
}

// reportDir はタブ名からレポートディレクトリのパスを返す。
func reportDir(workspacePath, tab string) string {
	switch tab {
	case "weekly":
		return filepath.Join(workspacePath, "memory", "weekly")
	case "monthly":
		return filepath.Join(workspacePath, "memory", "monthly")
	default:
		return filepath.Join(workspacePath, "memory", "daily")
	}
}

// listReportDates は日次レポートの日付一覧を map で返す。
// キーは "YYYY-MM-DD" 形式の日付文字列。
func listReportDates(workspacePath string) (map[string]bool, error) {
	dir := reportDir(workspacePath, "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	dates := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		date := strings.TrimSuffix(e.Name(), ".md")
		dates[date] = true
	}
	return dates, nil
}

// buildCalendar は指定年月のカレンダーデータを生成する。
func buildCalendar(year, month int, reportDates map[string]bool, selectedFile string) *calendarMonth {
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)
	daysInMonth := lastDay.Day()

	startWeekday := int(firstDay.Weekday()) // Sunday=0

	selectedDate := strings.TrimSuffix(selectedFile, ".md")

	var weeks [][]calendarDay
	day := 1
	for week := 0; week < 6; week++ {
		row := make([]calendarDay, 7)
		for col := 0; col < 7; col++ {
			cellIndex := week*7 + col
			if cellIndex < startWeekday || day > daysInMonth {
				continue
			}
			dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
			row[col] = calendarDay{
				Day:       day,
				HasReport: reportDates[dateStr],
				Selected:  dateStr == selectedDate,
				FileName:  dateStr + ".md",
			}
			day++
		}
		weeks = append(weeks, row)
		if day > daysInMonth {
			break
		}
	}

	prevMonth := firstDay.AddDate(0, -1, 0)
	nextMonth := firstDay.AddDate(0, 1, 0)

	return &calendarMonth{
		Year:      year,
		Month:     month,
		MonthName: fmt.Sprintf("%d年%d月", year, month),
		Weeks:     weeks,
		PrevYear:  prevMonth.Year(),
		PrevMonth: int(prevMonth.Month()),
		NextYear:  nextMonth.Year(),
		NextMonth: int(nextMonth.Month()),
	}
}

// findLatestReportInMonth は指定年月内で最も新しいレポートのファイル名を返す。
func findLatestReportInMonth(year, month int, reportDates map[string]bool) string {
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	for d := lastDay.Day(); d >= 1; d-- {
		dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, d)
		if reportDates[dateStr] {
			return dateStr + ".md"
		}
	}
	return ""
}

// parseYearMonth は URL クエリから年月を解析する。
// 不正値の場合は 0, 0 を返す。
func parseYearMonth(r *http.Request) (int, int) {
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")
	if yearStr == "" || monthStr == "" {
		return 0, 0
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2100 {
		return 0, 0
	}

	month, err := strconv.Atoi(monthStr)
	if err != nil || month < 1 || month > 12 {
		return 0, 0
	}

	return year, month
}
