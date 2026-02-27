package distill

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

// PromptData はプロンプトテンプレートに渡す変数。
type PromptData struct {
	Date string // "2026年2月28日"
}

// LoadPrompt はテンプレートファイルを読み込み、変数展開して返す。
func LoadPrompt(templatePath string, data PromptData) (string, error) {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt template %s: %w", templatePath, err)
	}

	tmpl, err := template.New("prompt").Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse prompt template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute prompt template: %w", err)
	}

	return buf.String(), nil
}
