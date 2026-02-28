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

// workspaceFile はワークスペース内のファイルまたはディレクトリのメタデータ。
type workspaceFile struct {
	Path  string // 相対パス (例: "SOUL.md", "prompts/rewrite.md")
	Name  string // ファイル名 (例: "SOUL.md")
	IsDir bool   // ディレクトリか
	Depth int    // ネスト深さ（0=ルート）
}

// workspaceData はワークスペース画面のテンプレートデータ。
type workspaceData struct {
	pageData
	Files       []workspaceFile
	CurrentFile string
	Content     template.HTML // Markdown → HTML 変換済み
}

// excludeDirs はワークスペースのファイルリストから除外するディレクトリ。
// memory/, sessions/, state/, logs/ は他の画面で表示するため除外する。
var excludeDirs = map[string]bool{
	"memory":   true,
	"sessions": true,
	"state":    true,
	"logs":     true,
}

// handleWorkspace はワークスペースファイル閲覧画面をフルページで返す。
func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	data := s.buildWorkspaceData("")

	if err := s.workspaceTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("failed to render workspace page", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleWorkspaceContent はファイル内容を HTMX fragment として返す。
func (s *Server) handleWorkspaceContent(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	data := s.buildWorkspaceData(file)

	if err := s.workspaceTmpl.ExecuteTemplate(w, "workspace_content", data); err != nil {
		slog.Error("failed to render workspace content", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// buildWorkspaceData はテンプレートに渡すデータを構築する。
func (s *Server) buildWorkspaceData(file string) workspaceData {
	data := workspaceData{
		pageData: pageData{
			Title:  "ワークスペース",
			Active: "workspace",
		},
	}

	files, err := listWorkspaceFiles(s.workspacePath)
	if err != nil {
		slog.Error("failed to list workspace files", "component", "console", "error", err)
	}
	data.Files = files

	// ファイル未指定の場合、最初の非ディレクトリファイルを自動選択
	if file == "" {
		for _, f := range files {
			if !f.IsDir {
				file = f.Path
				break
			}
		}
	}

	data.CurrentFile = file

	if file != "" {
		content, err := readWorkspaceFile(s.workspacePath, file)
		if err != nil {
			slog.Error("failed to read workspace file", "component", "console", "file", file, "error", err)
		}
		data.Content = content
	}

	return data
}

// listWorkspaceFiles はワークスペース内の表示対象 .md ファイルをリストする。
// ルート直下の .md ファイルと prompts/ ディレクトリ内の .md ファイルを返す。
// memory/, sessions/, state/, logs/ は除外する。
func listWorkspaceFiles(workspacePath string) ([]workspaceFile, error) {
	var files []workspaceFile

	// ルート直下の .md ファイルを収集
	rootEntries, err := os.ReadDir(workspacePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var rootFiles []workspaceFile
	var subDirs []string

	for _, e := range rootEntries {
		if e.IsDir() {
			name := e.Name()
			// 除外ディレクトリをスキップ
			if excludeDirs[name] {
				continue
			}
			// ドットディレクトリをスキップ
			if strings.HasPrefix(name, ".") {
				continue
			}
			subDirs = append(subDirs, name)
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		rootFiles = append(rootFiles, workspaceFile{
			Path:  e.Name(),
			Name:  e.Name(),
			IsDir: false,
			Depth: 0,
		})
	}

	// ルート .md ファイルをソート
	slices.SortFunc(rootFiles, func(a, b workspaceFile) int {
		return strings.Compare(a.Name, b.Name)
	})
	files = append(files, rootFiles...)

	// サブディレクトリをソートして処理
	slices.Sort(subDirs)

	for _, dirName := range subDirs {
		subFiles := listSubdirFiles(workspacePath, dirName)
		if len(subFiles) == 0 {
			continue
		}

		// ディレクトリヘッダーを追加
		files = append(files, workspaceFile{
			Path:  dirName,
			Name:  dirName,
			IsDir: true,
			Depth: 0,
		})
		files = append(files, subFiles...)
	}

	return files, nil
}


// promptsFileOrder は prompts/ ディレクトリ内のファイル表示順序。
var promptsFileOrder = []string{
	"daily_hindsight.md",
	"weekly_hindsight.md",
	"monthly_hindsight.md",
}

// listSubdirFiles はサブディレクトリ内の .md ファイルを返す。
// prompts/ ディレクトリはハードコードされた順序で返す。
func listSubdirFiles(workspacePath, dirName string) []workspaceFile {
	dirPath := filepath.Join(workspacePath, dirName)

	if dirName == "prompts" {
		return listOrderedFiles(dirPath, dirName, promptsFileOrder)
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		slog.Error("failed to read subdirectory", "component", "console", "dir", dirName, "error", err)
		return nil
	}

	var subFiles []workspaceFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		subFiles = append(subFiles, workspaceFile{
			Path:  dirName + "/" + e.Name(),
			Name:  e.Name(),
			IsDir: false,
			Depth: 1,
		})
	}

	slices.SortFunc(subFiles, func(a, b workspaceFile) int {
		return strings.Compare(a.Name, b.Name)
	})
	return subFiles
}

// listOrderedFiles は指定された順序でファイルを返す。存在しないファイルはスキップする。
func listOrderedFiles(dirPath, dirName string, order []string) []workspaceFile {
	var files []workspaceFile
	for _, name := range order {
		path := filepath.Join(dirPath, name)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		files = append(files, workspaceFile{
			Path:  dirName + "/" + name,
			Name:  name,
			IsDir: false,
			Depth: 1,
		})
	}
	return files
}

// readWorkspaceFile はワークスペース内のファイルを読み込み、Markdown → HTML 変換して返す。
// パストラバーサル対策として、ワークスペースパス配下にパスが収まることを検証する。
func readWorkspaceFile(workspacePath, filePath string) (template.HTML, error) {
	// パストラバーサル対策
	cleaned := filepath.Clean(filePath)
	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("invalid file path: %s", filePath)
	}

	// .md ファイルのみ許可
	if !strings.HasSuffix(cleaned, ".md") {
		return "", fmt.Errorf("unsupported file type: %s", filePath)
	}

	absPath := filepath.Join(workspacePath, cleaned)

	// ワークスペースパス配下に収まることを確認
	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve workspace path: %w", err)
	}
	absFile, err := filepath.Abs(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}
	if !strings.HasPrefix(absFile, absWorkspace+string(filepath.Separator)) {
		return "", fmt.Errorf("file path outside workspace: %s", filePath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	html, err := renderMarkdown(data)
	if err != nil {
		return "", err
	}
	return template.HTML(html), nil
}
