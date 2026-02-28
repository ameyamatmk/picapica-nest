package console

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadUsage_AggregatesByDay(t *testing.T) {
	// Given: 2日分のレコードが含まれる usage.jsonl
	dir := t.TempDir()
	records := strings.Join([]string{
		`{"ts":"2026-02-27T10:00:00Z","model":"claude","prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"latency_ms":500}`,
		`{"ts":"2026-02-27T11:00:00Z","model":"claude","prompt_tokens":200,"completion_tokens":100,"total_tokens":300,"latency_ms":700}`,
		`{"ts":"2026-02-28T09:00:00Z","model":"claude","prompt_tokens":50,"completion_tokens":25,"total_tokens":75,"latency_ms":300}`,
	}, "\n")
	os.WriteFile(filepath.Join(dir, "usage.jsonl"), []byte(records), 0o644)

	// When: loadUsage を呼ぶ
	days, err := loadUsage(dir)

	// Then: 2日分の集計結果が降順で返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(days))
	}
	if days[0].Date != "2026-02-28" {
		t.Errorf("expected first day '2026-02-28', got %q", days[0].Date)
	}

	// 2/27: 2 calls, prompt=300, completion=150, total=450, avg_latency=600
	day27 := days[1]
	if day27.CallCount != 2 {
		t.Errorf("expected 2 calls for 2/27, got %d", day27.CallCount)
	}
	if day27.PromptTokens != 300 {
		t.Errorf("expected prompt_tokens=300, got %d", day27.PromptTokens)
	}
	if day27.TotalTokens != 450 {
		t.Errorf("expected total_tokens=450, got %d", day27.TotalTokens)
	}
	if day27.AvgLatencyMs != 600 {
		t.Errorf("expected avg_latency=600, got %d", day27.AvgLatencyMs)
	}
}

func TestLoadUsage_FileNotFound(t *testing.T) {
	// Given: usage.jsonl が存在しない
	dir := t.TempDir()

	// When: loadUsage を呼ぶ
	days, err := loadUsage(dir)

	// Then: エラーなしで nil
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if days != nil {
		t.Errorf("expected nil, got %v", days)
	}
}

func TestLoadUsage_SkipsInvalidLines(t *testing.T) {
	// Given: 不正な行を含む usage.jsonl
	dir := t.TempDir()
	records := strings.Join([]string{
		`{"ts":"2026-02-28T09:00:00Z","model":"claude","prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"latency_ms":300}`,
		`invalid json line`,
		``,
		`{"ts":"2026-02-28T10:00:00Z","model":"claude","prompt_tokens":200,"completion_tokens":100,"total_tokens":300,"latency_ms":500}`,
	}, "\n")
	os.WriteFile(filepath.Join(dir, "usage.jsonl"), []byte(records), 0o644)

	// When: loadUsage を呼ぶ
	days, err := loadUsage(dir)

	// Then: 有効なレコードのみ集計される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(days))
	}
	if days[0].CallCount != 2 {
		t.Errorf("expected 2 calls, got %d", days[0].CallCount)
	}
}

func TestLoadUsage_CountsErrors(t *testing.T) {
	// Given: エラーありのレコードを含む usage.jsonl
	dir := t.TempDir()
	records := strings.Join([]string{
		`{"ts":"2026-02-28T09:00:00Z","model":"claude","prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"latency_ms":300}`,
		`{"ts":"2026-02-28T10:00:00Z","model":"claude","prompt_tokens":0,"completion_tokens":0,"total_tokens":0,"latency_ms":100,"error":"rate limit"}`,
	}, "\n")
	os.WriteFile(filepath.Join(dir, "usage.jsonl"), []byte(records), 0o644)

	// When: loadUsage を呼ぶ
	days, err := loadUsage(dir)

	// Then: エラー数が集計される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if days[0].ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", days[0].ErrorCount)
	}
}

func TestExtractDate(t *testing.T) {
	tests := []struct {
		ts       string
		expected string
	}{
		{"2026-02-28T10:00:00Z", "2026-02-28"},
		{"2026-02-28T10:00:00+09:00", "2026-02-28"},
		{"invalid", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run("ts="+tt.ts, func(t *testing.T) {
			got := extractDate(tt.ts)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestHandleUsage_ReturnsHTML(t *testing.T) {
	// Given: usage.jsonl が存在するワークスペース
	dir := t.TempDir()
	records := `{"ts":"2026-02-28T09:00:00Z","model":"claude","prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"latency_ms":300}`
	os.WriteFile(filepath.Join(dir, "usage.jsonl"), []byte(records), 0o644)

	s := NewServer(dir, nil)

	// When: GET /usage にリクエスト
	req := httptest.NewRequest("GET", "/usage", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でテーブルが含まれるHTMLが返る
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "API Usage") {
		t.Error("expected page to contain 'API Usage'")
	}
	if !strings.Contains(body, "2026-02-28") {
		t.Error("expected page to contain date")
	}
}

func TestHandleUsage_EmptyData(t *testing.T) {
	// Given: usage.jsonl が存在しない
	s := NewServer(t.TempDir(), nil)

	// When: GET /usage にリクエスト
	req := httptest.NewRequest("GET", "/usage", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 で「データがありません」が表示される
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "データがありません") {
		t.Error("expected page to contain empty state message")
	}
}
