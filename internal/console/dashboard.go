package console

import (
	"log/slog"
	"net/http"
)

// handleDashboard はダッシュボード画面をフルページで返す。
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Title:  "ダッシュボード",
		Active: "dashboard",
	}
	if err := s.dashboardTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("failed to render dashboard page", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
