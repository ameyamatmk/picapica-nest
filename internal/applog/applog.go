package applog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// multiHandler は複数の slog.Handler に同時にログを送る。
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, r.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// Setup はアプリケーションロガーを初期化する。
// ログファイルは {workspacePath}/logs/app/YYYY-MM-DD.jsonl に出力される。
// 戻り値の io.Closer を Close() するとログファイルが閉じられる。
func Setup(workspacePath string) (io.Closer, error) {
	logDir := filepath.Join(workspacePath, "logs", "app")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	logFileName := time.Now().Format("2006-01-02") + ".jsonl"
	logFile, err := os.OpenFile(
		filepath.Join(logDir, logFileName),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return nil, err
	}

	level := parseLogLevel()
	opts := &slog.HandlerOptions{Level: level}

	textHandler := slog.NewTextHandler(os.Stdout, opts)
	jsonHandler := slog.NewJSONHandler(logFile, opts)

	multi := &multiHandler{handlers: []slog.Handler{textHandler, jsonHandler}}
	slog.SetDefault(slog.New(multi))

	return logFile, nil
}

// parseLogLevel は環境変数 PICAPICA_LOG_LEVEL からログレベルを取得する。
// 未設定またはパース不能の場合は INFO を返す。
func parseLogLevel() slog.Level {
	switch strings.ToUpper(os.Getenv("PICAPICA_LOG_LEVEL")) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
