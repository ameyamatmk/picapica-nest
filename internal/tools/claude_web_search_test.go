package tools

import (
	"context"
	"testing"
)

func TestClaudeWebSearchTool_Interface(t *testing.T) {
	tool := NewClaudeWebSearchTool("", nil)

	// Given: ツール名と説明が設定されている
	// Then: 正しい名前と説明を返す
	if got := tool.Name(); got != "web_search" {
		t.Errorf("Name() = %q, want %q", got, "web_search")
	}

	if got := tool.Description(); got == "" {
		t.Error("Description() should not be empty")
	}
}

func TestClaudeWebSearchTool_Parameters(t *testing.T) {
	tool := NewClaudeWebSearchTool("", nil)

	// Given: パラメータ定義を取得
	params := tool.Parameters()

	// Then: query が required に含まれる
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("required field should be []string")
	}
	if len(required) != 1 || required[0] != "query" {
		t.Errorf("required = %v, want [query]", required)
	}

	// Then: properties に query がある
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties field should be map[string]any")
	}
	if _, ok := props["query"]; !ok {
		t.Error("properties should contain query")
	}
}

func TestClaudeWebSearchTool_MissingQuery(t *testing.T) {
	tool := NewClaudeWebSearchTool("", nil)

	// Given: query が空
	args := map[string]any{}

	// When: Execute を呼ぶ
	result := tool.Execute(context.Background(), args)

	// Then: エラーを返す
	if !result.IsError {
		t.Error("should return error for missing query")
	}
}
