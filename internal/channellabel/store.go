// Package channellabel はログディレクトリ名から人間が読めるラベルへの
// マッピングを管理する。Discord API でチャンネル名を解決し、
// JSON ファイルにキャッシュする。
package channellabel

import (
	"encoding/json"
	"os"
	"sync"
)

// Store はディレクトリ名 → 表示ラベルのマッピングを永続化する。
// キャッシュファイル形式: {"discord_123456": "雑談", ...}
type Store struct {
	path   string
	labels map[string]string
	mu     sync.RWMutex
}

// NewStore は新しい Store を作成する。
// ファイルが存在すればロードし、存在しなければ空のマップで初期化する。
func NewStore(path string) (*Store, error) {
	s := &Store{
		path:   path,
		labels: make(map[string]string),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, &s.labels); err != nil {
		return nil, err
	}

	return s, nil
}

// Get はディレクトリ名に対応するラベルを返す。
func (s *Store) Get(dirName string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	label, ok := s.labels[dirName]
	return label, ok
}

// Set はラベルを設定し、ファイルに保存する。
func (s *Store) Set(dirName, label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.labels[dirName] = label

	data, err := json.MarshalIndent(s.labels, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o644)
}
