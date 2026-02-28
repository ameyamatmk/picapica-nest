package console

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFormatComma(t *testing.T) {
	tests := []struct {
		input    any
		expected string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{12345, "12,345"},
		{1234567, "1,234,567"},
		{int64(38534), "38,534"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatComma(tt.input)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestHandleIndex_RedirectsToDashboard(t *testing.T) {
	// Given: Console サーバー
	s := NewServer(t.TempDir())

	// When: GET / にリクエスト
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: /dashboard にリダイレクト
	if rec.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %q", loc)
	}
}

func TestStaticFiles_ServesHTMX(t *testing.T) {
	// Given: Console サーバー
	s := NewServer(t.TempDir())

	// When: GET /static/htmx.min.js にリクエスト
	req := httptest.NewRequest("GET", "/static/htmx.min.js", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 で JavaScript が返る
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "htmx") {
		t.Error("expected response to contain 'htmx'")
	}
}

func TestStaticFiles_ServesPicoCSS(t *testing.T) {
	// Given: Console サーバー
	s := NewServer(t.TempDir())

	// When: GET /static/pico.min.css にリクエスト
	req := httptest.NewRequest("GET", "/static/pico.min.css", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: 200 で CSS が返る
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
