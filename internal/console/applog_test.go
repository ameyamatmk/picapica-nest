package console

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleAppLog_ReturnsHTML(t *testing.T) {
	// Given: ログファイルが存在するワークスペース
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs", "app")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logData := `{"time":"2026-02-28T10:20:21.679+09:00","level":"INFO","msg":"server started","component":"main"}
{"time":"2026-02-28T10:20:22.000+09:00","level":"ERROR","msg":"connection failed","component":"db","error":"timeout"}
`
	os.WriteFile(filepath.Join(logDir, "2026-02-28.jsonl"), []byte(logData), 0o644)

	s := NewServer(dir)

	// When: GET /logs にリクエスト
	req := httptest.NewRequest("GET", "/logs", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でHTMLが返り、ログ内容を含む
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "アプリケーションログ") {
		t.Error("expected page to contain 'アプリケーションログ'")
	}
	if !strings.Contains(body, "server started") {
		t.Error("expected page to contain log message 'server started'")
	}
	if !strings.Contains(body, "connection failed") {
		t.Error("expected page to contain log message 'connection failed'")
	}
}

func TestHandleAppLog_EmptyLogs(t *testing.T) {
	// Given: ログディレクトリが存在しないワークスペース
	dir := t.TempDir()

	s := NewServer(dir)

	// When: GET /logs にリクエスト
	req := httptest.NewRequest("GET", "/logs", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でHTMLが返る（エラーにならない）
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "アプリケーションログ") {
		t.Error("expected page to contain 'アプリケーションログ'")
	}
	if !strings.Contains(body, "ログエントリがありません") {
		t.Error("expected page to contain empty message")
	}
}

func TestHandleAppLogEntries_WithData(t *testing.T) {
	// Given: ログファイルが存在するワークスペース
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs", "app")
	os.MkdirAll(logDir, 0o755)
	logData := `{"time":"2026-02-28T10:20:21.679+09:00","level":"INFO","msg":"running weekly distillation","component":"distill","week":"2026-W09"}
{"time":"2026-02-28T10:20:22.000+09:00","level":"ERROR","msg":"health server error","error":"bind: address already in use"}
`
	os.WriteFile(filepath.Join(logDir, "2026-02-28.jsonl"), []byte(logData), 0o644)

	s := NewServer(dir)

	// When: GET /logs/entries?date=2026-02-28.jsonl にリクエスト
	req := httptest.NewRequest("GET", "/logs/entries?date=2026-02-28.jsonl", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でフラグメントが返り、エントリを含む
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	// フラグメントなので DOCTYPE を含まない
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("fragment should not contain DOCTYPE")
	}
	if !strings.Contains(body, "running weekly distillation") {
		t.Error("expected fragment to contain log message")
	}
	if !strings.Contains(body, "10:20:21") {
		t.Error("expected time display HH:MM:SS")
	}
	// Extra フィールドが表示されること
	if !strings.Contains(body, "week") {
		t.Error("expected extra field 'week' to be displayed")
	}
}

func TestHandleAppLogEntries_FilterByLevel(t *testing.T) {
	// Given: 複数レベルのログエントリ
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs", "app")
	os.MkdirAll(logDir, 0o755)
	logData := `{"time":"2026-02-28T10:00:00.000+09:00","level":"DEBUG","msg":"debug message"}
{"time":"2026-02-28T10:00:01.000+09:00","level":"INFO","msg":"info message"}
{"time":"2026-02-28T10:00:02.000+09:00","level":"WARN","msg":"warn message"}
{"time":"2026-02-28T10:00:03.000+09:00","level":"ERROR","msg":"error message"}
`
	os.WriteFile(filepath.Join(logDir, "2026-02-28.jsonl"), []byte(logData), 0o644)

	s := NewServer(dir)

	// When: WARN レベル以上でフィルタ
	req := httptest.NewRequest("GET", "/logs/entries?date=2026-02-28.jsonl&level=WARN", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: WARN と ERROR のみ含む
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "debug message") {
		t.Error("expected DEBUG to be filtered out")
	}
	if strings.Contains(body, "info message") {
		t.Error("expected INFO to be filtered out")
	}
	if !strings.Contains(body, "warn message") {
		t.Error("expected WARN to be included")
	}
	if !strings.Contains(body, "error message") {
		t.Error("expected ERROR to be included")
	}
}

func TestLoadAppLogEntries(t *testing.T) {
	// Given: JSONL ログファイル
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs", "app")
	os.MkdirAll(logDir, 0o755)
	logData := `{"time":"2026-02-28T10:20:21.679+09:00","level":"INFO","msg":"running weekly distillation","component":"distill","week":"2026-W09"}
{"time":"2026-02-28T10:20:22.000+09:00","level":"ERROR","msg":"health server error","component":"http","error":"bind: address already in use"}
{"time":"2026-02-28T10:30:00.000+09:00","level":"INFO","msg":"session started","component":"distill"}
`
	os.WriteFile(filepath.Join(logDir, "2026-02-28.jsonl"), []byte(logData), 0o644)

	// When: loadAppLogEntries を呼ぶ
	entries, components, err := loadAppLogEntries(dir, "2026-02-28.jsonl")

	// Then: 正しくパースされる
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// 1行目の確認
	e := entries[0]
	if e.Time != "10:20:21" {
		t.Errorf("expected time '10:20:21', got %q", e.Time)
	}
	if e.Level != "INFO" {
		t.Errorf("expected level 'INFO', got %q", e.Level)
	}
	if e.Message != "running weekly distillation" {
		t.Errorf("expected message 'running weekly distillation', got %q", e.Message)
	}
	if e.Component != "distill" {
		t.Errorf("expected component 'distill', got %q", e.Component)
	}
	if e.Extra["week"] != "2026-W09" {
		t.Errorf("expected extra field week='2026-W09', got %q", e.Extra["week"])
	}

	// コンポーネント一覧: ソート済みで重複排除
	if len(components) != 2 {
		t.Fatalf("expected 2 components, got %d: %v", len(components), components)
	}
	if components[0] != "distill" || components[1] != "http" {
		t.Errorf("expected [distill, http], got %v", components)
	}
}

func TestLoadAppLogEntries_PathTraversal(t *testing.T) {
	// Given: ワークスペース
	dir := t.TempDir()

	// When: パストラバーサル攻撃のファイル名
	_, _, err := loadAppLogEntries(dir, "../../../etc/passwd")

	// Then: エラーが返る
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func TestFilterLogEntries(t *testing.T) {
	entries := []appLogEntry{
		{Level: "DEBUG", Message: "debug", Component: "db"},
		{Level: "INFO", Message: "info", Component: "http"},
		{Level: "WARN", Message: "warn", Component: "db"},
		{Level: "ERROR", Message: "error", Component: "http"},
	}

	t.Run("no filter", func(t *testing.T) {
		// When: フィルタなし
		result := filterLogEntries(entries, "", "")

		// Then: 全件返る
		if len(result) != 4 {
			t.Errorf("expected 4, got %d", len(result))
		}
	})

	t.Run("filter by level WARN", func(t *testing.T) {
		// When: WARN 以上でフィルタ
		result := filterLogEntries(entries, "WARN", "")

		// Then: WARN + ERROR の2件
		if len(result) != 2 {
			t.Fatalf("expected 2, got %d", len(result))
		}
		if result[0].Level != "WARN" {
			t.Errorf("expected WARN, got %q", result[0].Level)
		}
		if result[1].Level != "ERROR" {
			t.Errorf("expected ERROR, got %q", result[1].Level)
		}
	})

	t.Run("filter by level ERROR", func(t *testing.T) {
		// When: ERROR のみ
		result := filterLogEntries(entries, "ERROR", "")

		// Then: 1件
		if len(result) != 1 {
			t.Fatalf("expected 1, got %d", len(result))
		}
		if result[0].Message != "error" {
			t.Errorf("expected 'error', got %q", result[0].Message)
		}
	})

	t.Run("filter by component", func(t *testing.T) {
		// When: コンポーネント "db" でフィルタ
		result := filterLogEntries(entries, "", "db")

		// Then: db の2件
		if len(result) != 2 {
			t.Fatalf("expected 2, got %d", len(result))
		}
		for _, e := range result {
			if e.Component != "db" {
				t.Errorf("expected component 'db', got %q", e.Component)
			}
		}
	})

	t.Run("filter by level and component", func(t *testing.T) {
		// When: WARN 以上 + コンポーネント "http"
		result := filterLogEntries(entries, "WARN", "http")

		// Then: ERROR の1件のみ（http の WARN はない）
		if len(result) != 1 {
			t.Fatalf("expected 1, got %d", len(result))
		}
		if result[0].Message != "error" {
			t.Errorf("expected 'error', got %q", result[0].Message)
		}
	})
}

func TestListAppLogDates(t *testing.T) {
	// Given: 複数の日付ファイル
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs", "app")
	os.MkdirAll(logDir, 0o755)
	for _, name := range []string{"2026-02-26.jsonl", "2026-02-28.jsonl", "2026-02-27.jsonl"} {
		os.WriteFile(filepath.Join(logDir, name), []byte("{}"), 0o644)
	}

	// When: listAppLogDates を呼ぶ
	dates, err := listAppLogDates(dir)

	// Then: 降順で返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dates) != 3 {
		t.Fatalf("expected 3 dates, got %d", len(dates))
	}
	if dates[0].FileName != "2026-02-28.jsonl" {
		t.Errorf("expected first '2026-02-28.jsonl', got %q", dates[0].FileName)
	}
	if dates[0].Label != "2026-02-28" {
		t.Errorf("expected label '2026-02-28', got %q", dates[0].Label)
	}
	if dates[2].FileName != "2026-02-26.jsonl" {
		t.Errorf("expected last '2026-02-26.jsonl', got %q", dates[2].FileName)
	}
}

func TestListAppLogDates_EmptyDirectory(t *testing.T) {
	// Given: ディレクトリが存在しない
	dir := filepath.Join(t.TempDir(), "nonexistent")

	// When: listAppLogDates を呼ぶ
	dates, err := listAppLogDates(dir)

	// Then: エラーなしで空スライス
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dates) != 0 {
		t.Errorf("expected 0 dates, got %d", len(dates))
	}
}

func TestListAppLogDates_IgnoresNonJSONL(t *testing.T) {
	// Given: .jsonl 以外のファイルも含むディレクトリ
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs", "app")
	os.MkdirAll(logDir, 0o755)
	os.WriteFile(filepath.Join(logDir, "2026-02-28.jsonl"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(logDir, "notes.txt"), []byte("note"), 0o644)
	os.MkdirAll(filepath.Join(logDir, "subdir"), 0o755)

	// When: listAppLogDates を呼ぶ
	dates, err := listAppLogDates(dir)

	// Then: .jsonl ファイルのみ返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dates) != 1 {
		t.Errorf("expected 1 date, got %d", len(dates))
	}
}

func TestExtractTime(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2026-02-28T10:20:21.679+09:00", "10:20:21"},
		{"2026-02-28T23:59:59Z", "23:59:59"},
		{"invalid", "invalid"},
		{"2026-02-28T10", "2026-02-28T10"}, // T の後が短すぎる
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractTime(tt.input)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
