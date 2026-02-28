package console

import (
	"log/slog"
	"net/http"
)

// handleConversations は会話ログ画面をフルページで返す。
func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Title:  "会話ログ",
		Active: "conversations",
	}
	if err := s.conversationsTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("failed to render conversations page", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleConversationMessages は会話メッセージを HTMX fragment として返す。
func (s *Server) handleConversationMessages(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Title:  "会話ログ",
		Active: "conversations",
	}
	if err := s.conversationsTmpl.ExecuteTemplate(w, "conversations_content", data); err != nil {
		slog.Error("failed to render conversation messages", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
