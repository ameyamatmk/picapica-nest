package channellabel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore_EmptyFile(t *testing.T) {
	// Given: ファイルが存在しないパス
	path := filepath.Join(t.TempDir(), "labels.json")

	// When: NewStore を呼ぶ
	store, err := NewStore(path)

	// Then: エラーなし、空のストア
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.Get("anything"); ok {
		t.Error("expected empty store")
	}
}

func TestNewStore_ExistingFile(t *testing.T) {
	// Given: 既存のキャッシュファイル
	path := filepath.Join(t.TempDir(), "labels.json")
	data := `{"discord_123": "雑談", "discord_456": "開発"}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	// When: NewStore を呼ぶ
	store, err := NewStore(path)

	// Then: ファイルの内容がロードされる
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	label, ok := store.Get("discord_123")
	if !ok || label != "雑談" {
		t.Errorf("expected '雑談', got %q (ok=%v)", label, ok)
	}
	label, ok = store.Get("discord_456")
	if !ok || label != "開発" {
		t.Errorf("expected '開発', got %q (ok=%v)", label, ok)
	}
}

func TestStore_SetAndGet(t *testing.T) {
	// Given: 空のストア
	path := filepath.Join(t.TempDir(), "labels.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}

	// When: Set を呼ぶ
	if err := store.Set("discord_789", "テスト"); err != nil {
		t.Fatal(err)
	}

	// Then: Get で取得できる
	label, ok := store.Get("discord_789")
	if !ok || label != "テスト" {
		t.Errorf("expected 'テスト', got %q (ok=%v)", label, ok)
	}

	// And: ファイルに永続化されている
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected file to be written")
	}
}

func TestStore_SetPersists(t *testing.T) {
	// Given: Set でラベルを保存したストア
	path := filepath.Join(t.TempDir(), "labels.json")
	store1, _ := NewStore(path)
	store1.Set("discord_100", "永続化テスト")

	// When: 同じファイルから新しい Store を作成
	store2, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}

	// Then: 前回のデータが読み込まれる
	label, ok := store2.Get("discord_100")
	if !ok || label != "永続化テスト" {
		t.Errorf("expected '永続化テスト', got %q (ok=%v)", label, ok)
	}
}

func TestNewStore_InvalidJSON(t *testing.T) {
	// Given: 不正な JSON ファイル
	path := filepath.Join(t.TempDir(), "labels.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// When: NewStore を呼ぶ
	_, err := NewStore(path)

	// Then: エラーが返る
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
