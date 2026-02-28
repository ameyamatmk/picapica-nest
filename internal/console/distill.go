package console

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ReportFile はレポートファイルのメタデータ。
type ReportFile struct {
	Name  string // ファイル名 (例: "2026-02-28.md")
	Label string // 表示用ラベル (例: "2026-02-28")
}

// distillData は蒸留レポート画面のテンプレートデータ。
type distillData struct {
	pageData
	Tab         string
	Files       []ReportFile
	CurrentFile string
	Content     template.HTML
}

// handleDistill は蒸留レポート画面をフルページで返す。
func (s *Server) handleDistill(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "daily"
	}

	data := s.buildDistillData(tab, "")

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

	data := s.buildDistillData(tab, file)

	if err := s.distillTmpl.ExecuteTemplate(w, "distill_content", data); err != nil {
		slog.Error("failed to render distill content", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// buildDistillData はテンプレートに渡すデータを構築する。
func (s *Server) buildDistillData(tab, file string) distillData {
	files, err := listReports(s.workspacePath, tab)
	if err != nil {
		slog.Error("failed to list reports", "component", "console", "tab", tab, "error", err)
	}

	// ファイル指定がなければ最新（先頭）を選択
	if file == "" && len(files) > 0 {
		file = files[0].Name
	}

	var content template.HTML
	if file != "" {
		content, err = readReport(s.workspacePath, tab, file)
		if err != nil {
			slog.Error("failed to read report", "component", "console", "file", file, "error", err)
		}
	}

	return distillData{
		pageData: pageData{
			Title:  "蒸留レポート",
			Active: "distill",
		},
		Tab:         tab,
		Files:       files,
		CurrentFile: file,
		Content:     content,
	}
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
