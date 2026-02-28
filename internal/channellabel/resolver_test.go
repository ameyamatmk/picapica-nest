package channellabel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestResolver_Resolve_CacheHit(t *testing.T) {
	// Given: キャッシュにラベルがあるストア
	path := filepath.Join(t.TempDir(), "labels.json")
	store, _ := NewStore(path)
	store.Set("discord_123", "雑談")

	resolver := NewResolver("dummy-token", store)

	// When: Resolve を呼ぶ
	label, err := resolver.Resolve("discord_123")

	// Then: キャッシュから返る（API コールなし）
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if label != "雑談" {
		t.Errorf("expected '雑談', got %q", label)
	}
}

func TestResolver_Resolve_APICall(t *testing.T) {
	// Given: Discord API のモックサーバー
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 認証ヘッダーの確認
		if r.Header.Get("Authorization") != "Bot test-token" {
			t.Errorf("expected 'Bot test-token', got %q", r.Header.Get("Authorization"))
		}
		// パスの確認
		if r.URL.Path != "/channels/1469682598732239006" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := discordChannel{Name: "雑談", Type: 0}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// API ベース URL をモックサーバーに差し替え
	origBase := discordAPIBase
	discordAPIBase = server.URL
	defer func() { discordAPIBase = origBase }()

	path := filepath.Join(t.TempDir(), "labels.json")
	store, _ := NewStore(path)
	resolver := NewResolver("test-token", store)

	// When: 未キャッシュのチャンネルを解決
	label, err := resolver.Resolve("discord_1469682598732239006")

	// Then: API から取得したラベルが返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if label != "雑談" {
		t.Errorf("expected '雑談', got %q", label)
	}

	// And: キャッシュに保存される
	cached, ok := store.Get("discord_1469682598732239006")
	if !ok || cached != "雑談" {
		t.Errorf("expected cache to contain '雑談', got %q (ok=%v)", cached, ok)
	}
}

func TestResolver_Resolve_DMChannel(t *testing.T) {
	// Given: DM チャンネルを返すモック
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := discordChannel{
			Type: 1,
			Recipients: []discordRecipient{
				{Username: "ameyama"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	origBase := discordAPIBase
	discordAPIBase = server.URL
	defer func() { discordAPIBase = origBase }()

	path := filepath.Join(t.TempDir(), "labels.json")
	store, _ := NewStore(path)
	resolver := NewResolver("test-token", store)

	// When: DM チャンネルを解決
	label, err := resolver.Resolve("discord_9876543210")

	// Then: "DM: username" 形式で返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if label != "DM: ameyama" {
		t.Errorf("expected 'DM: ameyama', got %q", label)
	}
}

func TestResolver_Resolve_NonNumericID(t *testing.T) {
	// Given: 非数値 ID のディレクトリ名
	path := filepath.Join(t.TempDir(), "labels.json")
	store, _ := NewStore(path)
	resolver := NewResolver("test-token", store)

	// When: 非数値 ID を解決しようとする
	_, err := resolver.Resolve("discord_test-channel")

	// Then: エラーが返る（API コールしない）
	if err == nil {
		t.Error("expected error for non-numeric ID")
	}
}

func TestResolver_Resolve_APIError(t *testing.T) {
	// Given: 404 を返すモック
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	origBase := discordAPIBase
	discordAPIBase = server.URL
	defer func() { discordAPIBase = origBase }()

	path := filepath.Join(t.TempDir(), "labels.json")
	store, _ := NewStore(path)
	resolver := NewResolver("test-token", store)

	// When: API がエラーを返す
	_, err := resolver.Resolve("discord_999999999999999999")

	// Then: エラーが返る
	if err == nil {
		t.Error("expected error for API failure")
	}
}

func TestResolver_Resolve_NoPrefix(t *testing.T) {
	// Given: アンダースコアなしのディレクトリ名
	path := filepath.Join(t.TempDir(), "labels.json")
	store, _ := NewStore(path)
	resolver := NewResolver("test-token", store)

	// When: プレフィックスなしの名前を解決
	_, err := resolver.Resolve("nounderscore")

	// Then: エラーが返る
	if err == nil {
		t.Error("expected error for name without underscore")
	}
}

func TestExtractChannelID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantID   string
		wantBool bool
	}{
		{"数値 ID", "discord_1469682598732239006", "1469682598732239006", true},
		{"非数値 ID", "discord_test-channel", "", false},
		{"アンダースコアなし", "simple", "", false},
		{"空の ID", "discord_", "", false},
		{"複数アンダースコア", "discord_123_456", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotBool := extractChannelID(tt.input)
			if gotID != tt.wantID || gotBool != tt.wantBool {
				t.Errorf("extractChannelID(%q) = (%q, %v), want (%q, %v)",
					tt.input, gotID, gotBool, tt.wantID, tt.wantBool)
			}
		})
	}
}
