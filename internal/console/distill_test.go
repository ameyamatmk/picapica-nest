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

func TestHandleDistill_ReturnsHTML(t *testing.T) {
	// Given: レポートファイルが存在するワークスペース
	dir := t.TempDir()
	dailyDir := filepath.Join(dir, "memory", "daily")
	os.MkdirAll(dailyDir, 0o755)
	os.WriteFile(filepath.Join(dailyDir, "2026-02-28.md"), []byte("# Test Report"), 0o644)

	s := NewServer(dir)

	// When: GET /distill にリクエスト
	req := httptest.NewRequest("GET", "/distill", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でHTMLが返り、レポート内容を含む
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "蒸留レポート") {
		t.Error("expected page to contain '蒸留レポート'")
	}
	if !strings.Contains(body, "Test Report") {
		t.Error("expected page to contain rendered report")
	}
}

func TestHandleDistillContent_Fragment(t *testing.T) {
	// Given: レポートファイルが存在するワークスペース
	dir := t.TempDir()
	weeklyDir := filepath.Join(dir, "memory", "weekly")
	os.MkdirAll(weeklyDir, 0o755)
	os.WriteFile(filepath.Join(weeklyDir, "2026-02-W09.md"), []byte("# Weekly"), 0o644)

	s := NewServer(dir)

	// When: GET /distill/content?tab=weekly にリクエスト
	req := httptest.NewRequest("GET", "/distill/content?tab=weekly", nil)
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
