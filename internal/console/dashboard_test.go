package console

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleDashboard_ReturnsHTML(t *testing.T) {
	// Given: レポートと usage.jsonl が存在するワークスペース
	dir := t.TempDir()
	dailyDir := filepath.Join(dir, "memory", "daily")
	os.MkdirAll(dailyDir, 0o755)
	os.WriteFile(filepath.Join(dailyDir, "2026-02-28.md"), []byte("# 日次レポート\n\nテスト本文です。"), 0o644)

	records := `{"ts":"2026-02-28T09:00:00Z","model":"claude","prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"latency_ms":300}`
	os.WriteFile(filepath.Join(dir, "usage.jsonl"), []byte(records), 0o644)

	s := NewServer(dir)

	// When: GET /dashboard にリクエスト
	req := httptest.NewRequest("GET", "/dashboard", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でダッシュボードHTMLが返る
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "ダッシュボード") {
		t.Error("expected page to contain 'ダッシュボード'")
	}
	if !strings.Contains(body, "蒸留レポート") {
		t.Error("expected page to contain '蒸留レポート'")
	}
	if !strings.Contains(body, "Usage サマリ") {
		t.Error("expected page to contain 'Usage サマリ'")
	}
}

func TestHandleDashboard_EmptyWorkspace(t *testing.T) {
	// Given: 空のワークスペース（何もファイルがない）
	s := NewServer(t.TempDir())

	// When: GET /dashboard にリクエスト
	req := httptest.NewRequest("GET", "/dashboard", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 でフォールバック表示が返る
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "レポートなし") {
		t.Error("expected page to contain 'レポートなし' for empty workspace")
	}
}

func TestBuildDashboardData_WithReports(t *testing.T) {
	// Given: 日次/週次/月次にそれぞれレポートが存在する
	dir := t.TempDir()
	for _, sub := range []string{"daily", "weekly", "monthly"} {
		d := filepath.Join(dir, "memory", sub)
		os.MkdirAll(d, 0o755)
	}
	os.WriteFile(
		filepath.Join(dir, "memory", "daily", "2026-02-28.md"),
		[]byte("# 日次\n\n今日はテストを書きました。良い一日でした。"),
		0o644,
	)
	os.WriteFile(
		filepath.Join(dir, "memory", "weekly", "2026-02-W09.md"),
		[]byte("# 週次まとめ\n\n今週の振り返りです。"),
		0o644,
	)
	os.WriteFile(
		filepath.Join(dir, "memory", "monthly", "2026-02.md"),
		[]byte("# 月次まとめ\n\n2月の振り返りです。"),
		0o644,
	)

	s := NewServer(dir)

	// When: buildDashboardData を呼ぶ
	data := s.buildDashboardData()

	// Then: 3つのタブの最新レポートが取得できる
	if len(data.LatestReports) != 3 {
		t.Fatalf("expected 3 reports, got %d", len(data.LatestReports))
	}

	// 日次レポート
	daily := data.LatestReports[0]
	if daily.Tab != "daily" {
		t.Errorf("expected tab 'daily', got %q", daily.Tab)
	}
	if daily.TabLabel != "日次" {
		t.Errorf("expected label '日次', got %q", daily.TabLabel)
	}
	if daily.FileName != "2026-02-28.md" {
		t.Errorf("expected file '2026-02-28.md', got %q", daily.FileName)
	}
	if daily.Label != "2026-02-28" {
		t.Errorf("expected label '2026-02-28', got %q", daily.Label)
	}
	if !strings.Contains(daily.Preview, "今日はテストを書きました") {
		t.Errorf("expected preview to contain '今日はテストを書きました', got %q", daily.Preview)
	}

	// 週次レポート
	weekly := data.LatestReports[1]
	if weekly.Tab != "weekly" {
		t.Errorf("expected tab 'weekly', got %q", weekly.Tab)
	}
	if !strings.Contains(weekly.Preview, "今週の振り返り") {
		t.Errorf("expected preview to contain '今週の振り返り', got %q", weekly.Preview)
	}

	// 月次レポート
	monthly := data.LatestReports[2]
	if monthly.Tab != "monthly" {
		t.Errorf("expected tab 'monthly', got %q", monthly.Tab)
	}
	if monthly.FileName != "2026-02.md" {
		t.Errorf("expected file '2026-02.md', got %q", monthly.FileName)
	}
}

func TestBuildDashboardData_WithUsage(t *testing.T) {
	// Given: 当日と昨日のレコードを含む usage.jsonl
	dir := t.TempDir()
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	records := strings.Join([]string{
		`{"ts":"` + today + `T10:00:00Z","model":"claude","prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"latency_ms":500}`,
		`{"ts":"` + today + `T11:00:00Z","model":"claude","prompt_tokens":200,"completion_tokens":100,"total_tokens":300,"latency_ms":700}`,
		`{"ts":"` + yesterday + `T09:00:00Z","model":"claude","prompt_tokens":50,"completion_tokens":25,"total_tokens":75,"latency_ms":300}`,
	}, "\n")
	os.WriteFile(filepath.Join(dir, "usage.jsonl"), []byte(records), 0o644)

	s := NewServer(dir)

	// When: buildDashboardData を呼ぶ
	data := s.buildDashboardData()

	// Then: Usage サマリが正しく集計される
	if data.UsageSummary == nil {
		t.Fatal("expected UsageSummary to be non-nil")
	}

	// 本日: 2 calls, 450 tokens
	if data.UsageSummary.TodayCalls != 2 {
		t.Errorf("expected TodayCalls=2, got %d", data.UsageSummary.TodayCalls)
	}
	if data.UsageSummary.TodayTokens != 450 {
		t.Errorf("expected TodayTokens=450, got %d", data.UsageSummary.TodayTokens)
	}

	// 直近7日間: 3 calls, 525 tokens
	if data.UsageSummary.WeekCalls != 3 {
		t.Errorf("expected WeekCalls=3, got %d", data.UsageSummary.WeekCalls)
	}
	if data.UsageSummary.WeekTokens != 525 {
		t.Errorf("expected WeekTokens=525, got %d", data.UsageSummary.WeekTokens)
	}
}

func TestReadReportPreview_TruncatesLongText(t *testing.T) {
	// Given: 100文字を超えるプレビューのレポート
	dir := t.TempDir()
	dailyDir := filepath.Join(dir, "memory", "daily")
	os.MkdirAll(dailyDir, 0o755)

	longText := strings.Repeat("あ", 150)
	os.WriteFile(filepath.Join(dailyDir, "2026-02-28.md"),
		[]byte("# タイトル\n\n"+longText), 0o644)

	// When: readReportPreview を呼ぶ
	preview, err := readReportPreview(dir, "daily", "2026-02-28.md")

	// Then: 100文字 + "..." で切られている
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	runes := []rune(preview)
	// 100文字 + "..." の 3文字 = 103文字
	if len(runes) != 103 {
		t.Errorf("expected 103 runes, got %d", len(runes))
	}
	if !strings.HasSuffix(preview, "...") {
		t.Errorf("expected preview to end with '...', got %q", preview[len(preview)-10:])
	}
}

func TestReadReportPreview_SkipsHeadings(t *testing.T) {
	// Given: 見出しだけの後にテキストがあるレポート
	dir := t.TempDir()
	dailyDir := filepath.Join(dir, "memory", "daily")
	os.MkdirAll(dailyDir, 0o755)
	os.WriteFile(filepath.Join(dailyDir, "2026-02-28.md"),
		[]byte("# 見出し1\n\n## 見出し2\n\n本文テキストです。"), 0o644)

	// When: readReportPreview を呼ぶ
	preview, err := readReportPreview(dir, "daily", "2026-02-28.md")

	// Then: 見出しではなく本文が返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preview != "本文テキストです。" {
		t.Errorf("expected '本文テキストです。', got %q", preview)
	}
}

func TestBuildUsageSummary_EmptyData(t *testing.T) {
	// Given: usage.jsonl が存在しない
	dir := t.TempDir()

	// When: buildUsageSummary を呼ぶ
	summary, err := buildUsageSummary(dir)

	// Then: ゼロ値のサマリが返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.TodayCalls != 0 || summary.WeekCalls != 0 {
		t.Errorf("expected zero calls, got today=%d week=%d",
			summary.TodayCalls, summary.WeekCalls)
	}
}
