package binding

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrNew_FileNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bindings.json")

	s, err := LoadOrNew(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(s.List()); got != 0 {
		t.Fatalf("expected 0 entries, got %d", got)
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bindings.json")

	s, err := LoadOrNew(path)
	if err != nil {
		t.Fatalf("LoadOrNew: %v", err)
	}

	if err := s.Add("worklog", "111111111"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Add("daily", "222222222"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 別インスタンスで読み直し
	s2, err := LoadOrNew(path)
	if err != nil {
		t.Fatalf("LoadOrNew (reload): %v", err)
	}
	entries := s2.List()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].AgentID != "worklog" || entries[0].ChannelID != "111111111" {
		t.Errorf("entry[0] mismatch: %+v", entries[0])
	}
	if entries[1].AgentID != "daily" || entries[1].ChannelID != "222222222" {
		t.Errorf("entry[1] mismatch: %+v", entries[1])
	}
}

func TestAdd_DuplicateChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bindings.json")

	s, err := LoadOrNew(path)
	if err != nil {
		t.Fatalf("LoadOrNew: %v", err)
	}

	if err := s.Add("worklog", "111111111"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Add("other", "111111111"); err == nil {
		t.Fatal("expected error on duplicate channel, got nil")
	}
}

func TestRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bindings.json")

	s, err := LoadOrNew(path)
	if err != nil {
		t.Fatalf("LoadOrNew: %v", err)
	}

	if err := s.Add("worklog", "111111111"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Add("daily", "222222222"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if !s.Remove("111111111") {
		t.Fatal("Remove returned false for existing entry")
	}
	if s.Remove("111111111") {
		t.Fatal("Remove returned true for already-removed entry")
	}

	entries := s.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after remove, got %d", len(entries))
	}
	if entries[0].ChannelID != "222222222" {
		t.Errorf("remaining entry mismatch: %+v", entries[0])
	}
}

func TestFindByChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bindings.json")

	s, err := LoadOrNew(path)
	if err != nil {
		t.Fatalf("LoadOrNew: %v", err)
	}

	if err := s.Add("worklog", "111111111"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	found := s.FindByChannel("111111111")
	if found == nil {
		t.Fatal("FindByChannel returned nil for existing entry")
	}
	if found.AgentID != "worklog" {
		t.Errorf("expected agent_id 'worklog', got %q", found.AgentID)
	}

	notFound := s.FindByChannel("999999999")
	if notFound != nil {
		t.Fatalf("FindByChannel returned non-nil for missing entry: %+v", notFound)
	}
}

func TestToBindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bindings.json")

	s, err := LoadOrNew(path)
	if err != nil {
		t.Fatalf("LoadOrNew: %v", err)
	}

	if err := s.Add("worklog", "111111111"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	bindings := s.ToBindings()
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings))
	}

	b := bindings[0]
	if b.AgentID != "worklog" {
		t.Errorf("AgentID: got %q, want %q", b.AgentID, "worklog")
	}
	if b.Match.Channel != "discord" {
		t.Errorf("Channel: got %q, want %q", b.Match.Channel, "discord")
	}
	if b.Match.AccountID != "*" {
		t.Errorf("AccountID: got %q, want %q", b.Match.AccountID, "*")
	}
	if b.Match.Peer == nil {
		t.Fatal("Peer is nil")
	}
	if b.Match.Peer.Kind != "channel" {
		t.Errorf("Peer.Kind: got %q, want %q", b.Match.Peer.Kind, "channel")
	}
	if b.Match.Peer.ID != "111111111" {
		t.Errorf("Peer.ID: got %q, want %q", b.Match.Peer.ID, "111111111")
	}
}

func TestLoadOrNew_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bindings.json")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadOrNew(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(s.List()); got != 0 {
		t.Fatalf("expected 0 entries, got %d", got)
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "bindings.json")

	s, err := LoadOrNew(path)
	if err != nil {
		t.Fatalf("LoadOrNew: %v", err)
	}

	if err := s.Add("test", "123"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}
