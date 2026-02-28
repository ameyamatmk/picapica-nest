package console

import (
	"log/slog"
	"net/http"
)

// handleWorkspace はワークスペースファイル閲覧画面をフルページで返す。
func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Title:  "ワークスペース",
		Active: "workspace",
	}
	if err := s.workspaceTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("failed to render workspace page", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleWorkspaceContent はファイル内容を HTMX fragment として返す。
func (s *Server) handleWorkspaceContent(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Title:  "ワークスペース",
		Active: "workspace",
	}
	if err := s.workspaceTmpl.ExecuteTemplate(w, "workspace_content", data); err != nil {
		slog.Error("failed to render workspace content", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
