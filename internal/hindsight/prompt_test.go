package hindsight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPrompt_Basic(t *testing.T) {
	// Given: テンプレートファイル
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "test.md")
	os.WriteFile(templatePath, []byte("Report for {{.Date}}.\nDone."), 0o644)

	// When: LoadPrompt を呼ぶ
	result, err := LoadPrompt(templatePath, PromptData{Date: "2026年2月28日"})

	// Then: 変数が展開される
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "2026年2月28日") {
		t.Errorf("expected date in output, got:\n%s", result)
	}
	if !strings.Contains(result, "Done.") {
		t.Errorf("expected template content preserved, got:\n%s", result)
	}
}

func TestLoadPrompt_FileNotFound(t *testing.T) {
	// Given: 存在しないファイル
	// When: LoadPrompt を呼ぶ
	_, err := LoadPrompt("/nonexistent/template.md", PromptData{Date: "test"})

	// Then: エラーが返る
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadPrompt_InvalidTemplate(t *testing.T) {
	// Given: 不正なテンプレート構文
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "bad.md")
	os.WriteFile(templatePath, []byte("{{.Unknown}}"), 0o644)

	// When: LoadPrompt を呼ぶ
	_, err := LoadPrompt(templatePath, PromptData{Date: "test"})

	// Then: テンプレート実行エラー（Unknown フィールドは存在しない）
	if err == nil {
		t.Fatal("expected error for invalid template field")
	}
}
