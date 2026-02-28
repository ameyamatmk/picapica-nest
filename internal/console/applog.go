package console

import (
	"log/slog"
	"net/http"
)

// handleAppLog はアプリケーションログ画面をフルページで返す。
func (s *Server) handleAppLog(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Title:  "アプリケーションログ",
		Active: "logs",
	}
	if err := s.applogTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("failed to render applog page", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAppLogEntries はログエントリを HTMX fragment として返す。
func (s *Server) handleAppLogEntries(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Title:  "アプリケーションログ",
		Active: "logs",
	}
	if err := s.applogTmpl.ExecuteTemplate(w, "applog_content", data); err != nil {
		slog.Error("failed to render applog entries", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
