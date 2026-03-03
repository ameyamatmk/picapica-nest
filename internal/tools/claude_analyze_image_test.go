package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestClaudeAnalyzeImageTool_Interface(t *testing.T) {
	tool := NewClaudeAnalyzeImageTool(os.TempDir(), "")

	// Given: ツール名と説明が設定されている
	// Then: 正しい名前と説明を返す
	if got := tool.Name(); got != "claude_analyze_image" {
		t.Errorf("Name() = %q, want %q", got, "claude_analyze_image")
	}

	if got := tool.Description(); got == "" {
		t.Error("Description() should not be empty")
	}
}

func TestClaudeAnalyzeImageTool_Parameters(t *testing.T) {
	tool := NewClaudeAnalyzeImageTool(os.TempDir(), "")

	// Given: パラメータ定義を取得
	params := tool.Parameters()

	// Then: image_url が required に含まれる
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("required field should be []string")
	}
	if len(required) != 1 || required[0] != "image_url" {
		t.Errorf("required = %v, want [image_url]", required)
	}

	// Then: properties に image_url と question がある
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties field should be map[string]any")
	}
	if _, ok := props["image_url"]; !ok {
		t.Error("properties should contain image_url")
	}
	if _, ok := props["question"]; !ok {
		t.Error("properties should contain question")
	}
}

func TestClaudeAnalyzeImageTool_MissingImageURL(t *testing.T) {
	tool := NewClaudeAnalyzeImageTool(os.TempDir(), "")

	// Given: image_url が空
	args := map[string]any{}

	// When: Execute を呼ぶ
	result := tool.Execute(context.Background(), args)

	// Then: エラーを返す
	if !result.IsError {
		t.Error("should return error for missing image_url")
	}
}

func TestDownloadImage(t *testing.T) {
	// Given: 画像を返すテストサーバー
	imageData := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG マジックバイト
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(imageData)
	}))
	defer srv.Close()

	// When: downloadImage を呼ぶ
	tmpFile, err := downloadImage(context.Background(), srv.URL+"/test.jpg", os.TempDir())
	if err != nil {
		t.Fatalf("downloadImage() error: %v", err)
	}
	defer os.Remove(tmpFile)

	// Then: ファイルが作成され、内容が一致する
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if len(data) != len(imageData) {
		t.Errorf("downloaded file size = %d, want %d", len(data), len(imageData))
	}
}

func TestDownloadImage_NotFound(t *testing.T) {
	// Given: 404 を返すテストサーバー
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// When: downloadImage を呼ぶ
	_, err := downloadImage(context.Background(), srv.URL+"/missing.jpg", os.TempDir())

	// Then: エラーを返す
	if err == nil {
		t.Error("should return error for 404 response")
	}
}

func TestBuildVisionPrompt(t *testing.T) {
	tests := []struct {
		name      string
		imagePath string
		question  string
		wantHas   string
	}{
		{
			name:      "質問なし",
			imagePath: "/tmp/test.jpg",
			question:  "",
			wantHas:   "/tmp/test.jpg",
		},
		{
			name:      "質問あり",
			imagePath: "/tmp/test.jpg",
			question:  "何の写真？",
			wantHas:   "何の写真？",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildVisionPrompt(tt.imagePath, tt.question)
			if got == "" {
				t.Error("prompt should not be empty")
			}
			if !contains(got, tt.wantHas) {
				t.Errorf("prompt = %q, want to contain %q", got, tt.wantHas)
			}
		})
	}
}

func contains(s, substr string) bool {
	return fmt.Sprintf("%s", s) != "" && len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
