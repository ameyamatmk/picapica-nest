package tools

import (
	"context"
	"testing"
)

func TestClaudeWebFetchTool_Interface(t *testing.T) {
	tool := NewClaudeWebFetchTool("")

	// Given: ツール名と説明が設定されている
	// Then: 正しい名前と説明を返す
	if got := tool.Name(); got != "web_fetch" {
		t.Errorf("Name() = %q, want %q", got, "web_fetch")
	}

	if got := tool.Description(); got == "" {
		t.Error("Description() should not be empty")
	}
}

func TestClaudeWebFetchTool_Parameters(t *testing.T) {
	tool := NewClaudeWebFetchTool("")

	// Given: パラメータ定義を取得
	params := tool.Parameters()

	// Then: url が required に含まれる
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("required field should be []string")
	}
	if len(required) != 1 || required[0] != "url" {
		t.Errorf("required = %v, want [url]", required)
	}

	// Then: properties に url と question がある
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties field should be map[string]any")
	}
	if _, ok := props["url"]; !ok {
		t.Error("properties should contain url")
	}
	if _, ok := props["question"]; !ok {
		t.Error("properties should contain question")
	}
}

func TestClaudeWebFetchTool_MissingURL(t *testing.T) {
	tool := NewClaudeWebFetchTool("")

	// Given: url が空
	args := map[string]any{}

	// When: Execute を呼ぶ
	result := tool.Execute(context.Background(), args)

	// Then: エラーを返す
	if !result.IsError {
		t.Error("should return error for missing url")
	}
}

func TestBuildFetchPrompt(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		question string
		wantHas  string
	}{
		{
			name:     "質問なし",
			url:      "https://example.com",
			question: "",
			wantHas:  "https://example.com",
		},
		{
			name:     "質問あり",
			url:      "https://example.com",
			question: "何について書かれている？",
			wantHas:  "何について書かれている？",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFetchPrompt(tt.url, tt.question)
			if got == "" {
				t.Error("prompt should not be empty")
			}
			if !contains(got, tt.wantHas) {
				t.Errorf("prompt = %q, want to contain %q", got, tt.wantHas)
			}
		})
	}
}
