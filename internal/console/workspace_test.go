package console

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleWorkspace_ReturnsHTML(t *testing.T) {
	// Given: .md ファイルが存在するワークスペース
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("# Soul\n\nHello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(dir)

	// When: GET /workspace にリクエスト
	req := httptest.NewRequest("GET", "/workspace", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でHTMLが返り、ページタイトルとファイル内容を含む
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "ワークスペース") {
		t.Error("expected page to contain 'ワークスペース'")
	}
	if !strings.Contains(body, "SOUL.md") {
		t.Error("expected page to contain file name 'SOUL.md'")
	}
	if !strings.Contains(body, "Hello world") {
		t.Error("expected page to contain rendered markdown content")
	}
}

func TestHandleWorkspace_EmptyWorkspace(t *testing.T) {
	// Given: 空のワークスペース
	dir := t.TempDir()

	s := NewServer(dir)

	// When: GET /workspace にリクエスト
	req := httptest.NewRequest("GET", "/workspace", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 が返る（エラーにならない）
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "ワークスペース") {
		t.Error("expected page to contain 'ワークスペース'")
	}
	if !strings.Contains(body, "ファイルを選択してください") {
		t.Error("expected empty state message")
	}
}

func TestHandleWorkspaceContent_ReadsFile(t *testing.T) {
	// Given: .md ファイルが存在するワークスペース
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Agents\n\n- agent1\n- agent2"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(dir)

	// When: GET /workspace/content?file=AGENTS.md にリクエスト
	req := httptest.NewRequest("GET", "/workspace/content?file=AGENTS.md", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でフラグメントが返り、ファイル内容を含む
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	// HTMX fragment なので DOCTYPE を含まない
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("fragment should not contain DOCTYPE")
	}
	if !strings.Contains(body, "Agents") {
		t.Error("expected fragment to contain rendered file content")
	}
	if !strings.Contains(body, "agent1") {
		t.Error("expected fragment to contain list items")
	}
	// 選択中ファイルが aria-current="page" で強調される
	if !strings.Contains(body, `aria-current="page"`) {
		t.Error("expected current file to have aria-current attribute")
	}
}

func TestHandleWorkspaceContent_SubdirectoryFile(t *testing.T) {
	// Given: prompts/ 配下にファイルがあるワークスペース
	dir := t.TempDir()
	promptsDir := filepath.Join(dir, "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "rewrite.md"), []byte("# Rewrite Prompt"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(dir)

	// When: サブディレクトリのファイルを指定してリクエスト
	req := httptest.NewRequest("GET", "/workspace/content?file=prompts/rewrite.md", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: ファイル内容が正しく表示される
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Rewrite Prompt") {
		t.Error("expected fragment to contain subdirectory file content")
	}
}

func TestListWorkspaceFiles(t *testing.T) {
	// Given: 複数のファイルとディレクトリがあるワークスペース
	dir := t.TempDir()

	// ルートの .md ファイル
	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("# Soul"), 0o644)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Agents"), 0o644)
	os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("# Memory"), 0o644)

	// 非 .md ファイル（除外対象）
	os.WriteFile(filepath.Join(dir, "usage.jsonl"), []byte("{}"), 0o644)

	// prompts/ ディレクトリ（表示対象）
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0o755)
	os.WriteFile(filepath.Join(promptsDir, "rewrite.md"), []byte("# Rewrite"), 0o644)
	os.WriteFile(filepath.Join(promptsDir, "system.md"), []byte("# System"), 0o644)

	// 除外ディレクトリ
	os.MkdirAll(filepath.Join(dir, "memory", "daily"), 0o755)
	os.WriteFile(filepath.Join(dir, "memory", "daily", "2026-02-28.md"), []byte("# Daily"), 0o644)
	os.MkdirAll(filepath.Join(dir, "sessions"), 0o755)
	os.MkdirAll(filepath.Join(dir, "state"), 0o755)
	os.MkdirAll(filepath.Join(dir, "logs"), 0o755)

	// When: listWorkspaceFiles を呼ぶ
	files, err := listWorkspaceFiles(dir)

	// Then: エラーなし
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Then: ルート .md ファイル3つ + prompts ディレクトリヘッダー1つ + prompts 内 .md ファイル2つ = 6
	if len(files) != 6 {
		t.Fatalf("expected 6 entries, got %d: %+v", len(files), files)
	}

	// Then: ルートファイルはアルファベット順
	if files[0].Path != "AGENTS.md" {
		t.Errorf("expected first file 'AGENTS.md', got %q", files[0].Path)
	}
	if files[1].Path != "MEMORY.md" {
		t.Errorf("expected second file 'MEMORY.md', got %q", files[1].Path)
	}
	if files[2].Path != "SOUL.md" {
		t.Errorf("expected third file 'SOUL.md', got %q", files[2].Path)
	}

	// Then: prompts ディレクトリヘッダー
	if !files[3].IsDir || files[3].Name != "prompts" {
		t.Errorf("expected prompts dir header, got %+v", files[3])
	}

	// Then: prompts 内のファイルは Depth=1
	if files[4].Path != "prompts/rewrite.md" || files[4].Depth != 1 {
		t.Errorf("expected prompts/rewrite.md with depth 1, got %+v", files[4])
	}
	if files[5].Path != "prompts/system.md" || files[5].Depth != 1 {
		t.Errorf("expected prompts/system.md with depth 1, got %+v", files[5])
	}

	// Then: 除外ディレクトリのファイルが含まれない
	for _, f := range files {
		if strings.HasPrefix(f.Path, "memory/") || strings.HasPrefix(f.Path, "sessions/") ||
			strings.HasPrefix(f.Path, "state/") || strings.HasPrefix(f.Path, "logs/") {
			t.Errorf("excluded directory file should not be listed: %s", f.Path)
		}
	}

	// Then: .jsonl ファイルが含まれない
	for _, f := range files {
		if f.Path == "usage.jsonl" {
			t.Error("non-.md file should not be listed")
		}
	}
}

func TestListWorkspaceFiles_EmptyDirectory(t *testing.T) {
	// Given: 空のワークスペース
	dir := t.TempDir()

	// When: listWorkspaceFiles を呼ぶ
	files, err := listWorkspaceFiles(dir)

	// Then: エラーなし、空スライス
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestListWorkspaceFiles_NonexistentDirectory(t *testing.T) {
	// Given: 存在しないディレクトリ
	dir := filepath.Join(t.TempDir(), "nonexistent")

	// When: listWorkspaceFiles を呼ぶ
	files, err := listWorkspaceFiles(dir)

	// Then: エラーなし、nil
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if files != nil {
		t.Errorf("expected nil, got %v", files)
	}
}

func TestReadWorkspaceFile_PathTraversal(t *testing.T) {
	// Given: ワークスペース
	dir := t.TempDir()

	tests := []struct {
		name     string
		filePath string
	}{
		{"parent directory", "../../../etc/passwd"},
		{"dot-dot in middle", "prompts/../../etc/passwd"},
		{"dot-dot only", ".."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: パストラバーサル攻撃のファイルパス
			_, err := readWorkspaceFile(dir, tt.filePath)

			// Then: エラーが返る
			if err == nil {
				t.Fatal("expected error for path traversal, got nil")
			}
		})
	}
}

func TestReadWorkspaceFile_RejectsNonMarkdown(t *testing.T) {
	// Given: ワークスペース
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "usage.jsonl"), []byte("{}"), 0o644)

	// When: 非 .md ファイルを指定
	_, err := readWorkspaceFile(dir, "usage.jsonl")

	// Then: エラーが返る
	if err == nil {
		t.Fatal("expected error for non-markdown file, got nil")
	}
}

func TestReadWorkspaceFile_RendersMarkdown(t *testing.T) {
	// Given: Markdown ファイル
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("# My Soul\n\n- trait1\n- trait2\n"), 0o644)

	// When: readWorkspaceFile を呼ぶ
	html, err := readWorkspaceFile(dir, "SOUL.md")

	// Then: HTML に変換されている
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	htmlStr := string(html)
	if !strings.Contains(htmlStr, "<h1>My Soul</h1>") {
		t.Errorf("expected <h1>My Soul</h1>, got %s", htmlStr)
	}
	if !strings.Contains(htmlStr, "<li>trait1</li>") {
		t.Errorf("expected <li>trait1</li>, got %s", htmlStr)
	}
}

func TestReadWorkspaceFile_SubdirectoryFile(t *testing.T) {
	// Given: サブディレクトリ内の Markdown ファイル
	dir := t.TempDir()
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0o755)
	os.WriteFile(filepath.Join(promptsDir, "rewrite.md"), []byte("# Rewrite\n\nContent here"), 0o644)

	// When: サブディレクトリのファイルを読む
	html, err := readWorkspaceFile(dir, "prompts/rewrite.md")

	// Then: 正しく読み込める
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	htmlStr := string(html)
	if !strings.Contains(htmlStr, "Rewrite") {
		t.Errorf("expected 'Rewrite', got %s", htmlStr)
	}
}
