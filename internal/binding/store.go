// Package binding は Discord チャンネルと Agent の紐付けを永続化する。
package binding

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

// Entry は 1 つの Binding 永続化レコード。
type Entry struct {
	AgentID   string `json:"agent_id"`
	ChannelID string `json:"channel_id"` // Discord channel ID
	CreatedAt string `json:"created_at"`
}

// Store は Binding の永続化ストア。
type Store struct {
	path    string
	mu      sync.RWMutex
	entries []Entry
}

// LoadOrNew は path から Binding を読み込む。ファイルが無ければ空の Store を返す。
func LoadOrNew(path string) (*Store, error) {
	s := &Store{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("binding store: read %s: %w", path, err)
	}

	if len(data) == 0 {
		return s, nil
	}

	if err := json.Unmarshal(data, &s.entries); err != nil {
		return nil, fmt.Errorf("binding store: parse %s: %w", path, err)
	}

	return s, nil
}

// Save は現在のエントリをファイルに書き出す。
func (s *Store) Save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.entries, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("binding store: marshal: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("binding store: mkdir %s: %w", dir, err)
	}

	return os.WriteFile(s.path, data, 0o644)
}

// Add は新しい Binding を追加する。同じ ChannelID が既にあればエラーを返す。
func (s *Store) Add(agentID, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range s.entries {
		if e.ChannelID == channelID {
			return fmt.Errorf("channel %s is already bound to agent %s", channelID, e.AgentID)
		}
	}

	s.entries = append(s.entries, Entry{
		AgentID:   agentID,
		ChannelID: channelID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

// Remove は指定 ChannelID の Binding を削除する。存在しなければ false を返す。
func (s *Store) Remove(channelID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, e := range s.entries {
		if e.ChannelID == channelID {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			return true
		}
	}
	return false
}

// List は全エントリのコピーを返す。
func (s *Store) List() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

// FindByChannel は ChannelID に一致するエントリを返す。見つからなければ nil。
func (s *Store) FindByChannel(channelID string) *Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range s.entries {
		if e.ChannelID == channelID {
			cp := e
			return &cp
		}
	}
	return nil
}

// ToBindings は保存済みエントリを PicoClaw の AgentBinding スライスに変換する。
// channel は "discord" 固定、accountID は "*"（ワイルドカード）。
func (s *Store) ToBindings() []config.AgentBinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bindings := make([]config.AgentBinding, 0, len(s.entries))
	for _, e := range s.entries {
		bindings = append(bindings, config.AgentBinding{
			AgentID: e.AgentID,
			Match: config.BindingMatch{
				Channel:   "discord",
				AccountID: "*",
				Peer: &config.PeerMatch{
					Kind: "channel",
					ID:   e.ChannelID,
				},
			},
		})
	}
	return bindings
}
