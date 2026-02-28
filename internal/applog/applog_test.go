package applog

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetup_CreatesLogFile(t *testing.T) {
	// Given: 空の一時ディレクトリ
	workspace := t.TempDir()

	// When: Setup() を呼ぶ
	closer, err := Setup(workspace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer closer.Close()

	// Then: logs/app/YYYY-MM-DD.jsonl ファイルが作成される
	logFileName := time.Now().Format("2006-01-02") + ".jsonl"
	logPath := filepath.Join(workspace, "logs", "app", logFileName)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("expected log file to exist at %s", logPath)
	}
}

func TestSetup_WritesJSONToFile(t *testing.T) {
	// Given: Setup() 済みの環境
	workspace := t.TempDir()
	closer, err := Setup(workspace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When: slog.Info() でログを書く
	slog.Info("test message", "key", "value")
	closer.Close()

	// Then: ログファイルに JSON 行が書き込まれている
	logFileName := time.Now().Format("2006-01-02") + ".jsonl"
	logPath := filepath.Join(workspace, "logs", "app", logFileName)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		t.Fatal("expected log file to have content")
	}

	// JSON としてパースできること
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(content), &entry); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\ncontent: %s", err, content)
	}

	// 必須フィールドの存在確認
	if _, ok := entry["time"]; !ok {
		t.Error("expected 'time' field in JSON log entry")
	}
	if _, ok := entry["level"]; !ok {
		t.Error("expected 'level' field in JSON log entry")
	}
	if msg, ok := entry["msg"]; !ok || msg != "test message" {
		t.Errorf("expected msg='test message', got %v", msg)
	}
	if key, ok := entry["key"]; !ok || key != "value" {
		t.Errorf("expected key='value', got %v", key)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		env      string
		expected slog.Level
	}{
		{"", slog.LevelInfo},
		{"DEBUG", slog.LevelDebug},
		{"debug", slog.LevelDebug},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"INVALID", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run("env="+tt.env, func(t *testing.T) {
			// Given: 環境変数を設定
			t.Setenv("PICAPICA_LOG_LEVEL", tt.env)

			// When: parseLogLevel を呼ぶ
			got := parseLogLevel()

			// Then: 期待するレベルが返る
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}
