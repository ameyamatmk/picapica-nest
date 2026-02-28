package console

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleIndex_RedirectsToDistill(t *testing.T) {
	// Given: Console サーバー
	s := NewServer(t.TempDir())

	// When: GET / にリクエスト
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	// Then: /distill にリダイレクト
	if rec.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/distill" {
		t.Errorf("expected redirect to /distill, got %q", loc)
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
