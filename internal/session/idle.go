package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/session"
)

// sessionFile は sessions ディレクトリ内の JSON ファイルから
// Updated フィールドだけを読み取るための軽量構造体。
type sessionFile struct {
	Key     string    `json:"key"`
	Updated time.Time `json:"updated"`
}

// IdleMonitor はセッションの idle timeout を監視し、
// 一定時間操作がないセッションをクリアする。
type IdleMonitor struct {
	sessionsDirs []string
	timeout      time.Duration
	interval     time.Duration
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewIdleMonitor は新しい IdleMonitor を作成する。
// sessionsDirs には各 Agent の sessions ディレクトリパスを渡す。
// timeout は idle 判定の閾値、interval はチェック間隔。
func NewIdleMonitor(sessionsDirs []string, timeout, interval time.Duration) *IdleMonitor {
	if interval == 0 {
		interval = 1 * time.Minute
	}
	return &IdleMonitor{
		sessionsDirs: sessionsDirs,
		timeout:      timeout,
		interval:     interval,
	}
}

// Start はバックグラウンド goroutine で定期チェックを開始する。
func (m *IdleMonitor) Start(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)
	m.wg.Add(1)
	go m.run(ctx)
}

// Stop はバックグラウンド goroutine を停止し、完了を待つ。
func (m *IdleMonitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
}

// run は定期的にセッションの idle チェックを行うループ。
func (m *IdleMonitor) run(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAll()
		}
	}
}

// checkAll は全 sessions ディレクトリのセッションをチェックする。
func (m *IdleMonitor) checkAll() {
	for _, dir := range m.sessionsDirs {
		m.checkDir(dir)
	}
}

// checkDir は指定した sessions ディレクトリ内のセッションを確認し、
// idle timeout を超過しているセッションをクリアする。
func (m *IdleMonitor) checkDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// ディレクトリが存在しない場合は静かにスキップ
		return
	}

	now := time.Now()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		sf, err := readSessionFile(filePath)
		if err != nil {
			continue
		}

		// idle timeout 超過チェック
		idle := now.Sub(sf.Updated)
		if idle <= m.timeout {
			continue
		}

		// セッションをクリア
		fmt.Printf("IdleMonitor: session %q idle for %v, clearing\n", sf.Key, idle.Truncate(time.Second))
		m.clearSession(dir, sf.Key)
	}
}

// clearSession は指定セッションの履歴とサマリーをクリアし、永続化する。
//
// 処理フロー:
//  1. Save(key)               — 現在の状態を保存（安全のため）
//  2. TruncateHistory(key, 0) — インメモリの履歴をクリア
//  3. SetSummary(key, "")     — サマリーもクリア
//  4. Save(key)               — クリア後の状態を保存
//
// Save が失敗した場合はスキップし、次のチェックで再試行する。
func (m *IdleMonitor) clearSession(dir, key string) {
	// SessionManager を sessions ディレクトリから作成し、
	// 既存のセッションファイルをロードする。
	sm := session.NewSessionManager(dir)

	// GetOrCreate でインメモリにセッションを確保
	// （loadSessions で既にロード済みのはずだが念のため）
	sm.GetOrCreate(key)

	// 1. 現在の状態を保存
	if err := sm.Save(key); err != nil {
		fmt.Printf("IdleMonitor: failed to save session %q before clear: %v\n", key, err)
		return
	}

	// 2. 履歴をクリア
	sm.TruncateHistory(key, 0)

	// 3. サマリーをクリア
	sm.SetSummary(key, "")

	// 4. クリア後の状態を保存
	if err := sm.Save(key); err != nil {
		fmt.Printf("IdleMonitor: failed to save session %q after clear: %v\n", key, err)
		return
	}

	fmt.Printf("IdleMonitor: session %q cleared successfully\n", key)
}

// SessionsDirsFromConfig は config から全 Agent の sessions ディレクトリパスを計算する。
// PicoClaw の resolveAgentWorkspace ロジックを再現して workspace を解決し、
// その中の sessions/ サブディレクトリのパスを返す。
func SessionsDirsFromConfig(cfg *config.Config) []string {
	seen := make(map[string]struct{})
	var dirs []string

	addDir := func(workspace string) {
		dir := filepath.Join(workspace, "sessions")
		if _, ok := seen[dir]; !ok {
			seen[dir] = struct{}{}
			dirs = append(dirs, dir)
		}
	}

	defaults := &cfg.Agents.Defaults

	if len(cfg.Agents.List) == 0 {
		// 暗黙の main agent: defaults.Workspace を使用
		ws := expandHome(defaults.Workspace)
		addDir(ws)
	} else {
		for i := range cfg.Agents.List {
			ac := &cfg.Agents.List[i]
			ws := resolveWorkspace(ac, defaults)
			addDir(ws)
		}
	}

	return dirs
}

// resolveWorkspace は PicoClaw の resolveAgentWorkspace と同じロジックで
// Agent の workspace パスを解決する。
func resolveWorkspace(ac *config.AgentConfig, defaults *config.AgentDefaults) string {
	if ac != nil && strings.TrimSpace(ac.Workspace) != "" {
		return expandHome(strings.TrimSpace(ac.Workspace))
	}
	if ac == nil || ac.Default || ac.ID == "" || normalizeAgentID(ac.ID) == "main" {
		return expandHome(defaults.Workspace)
	}
	home, _ := os.UserHomeDir()
	id := normalizeAgentID(ac.ID)
	return filepath.Join(home, ".picoclaw", "workspace-"+id)
}

// normalizeAgentID は agent ID を正規化する（小文字化、トリム）。
func normalizeAgentID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

// expandHome はパス先頭の ~ をホームディレクトリに展開する。
func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return home + path[1:]
		}
		return home
	}
	return path
}

// readSessionFile は JSON ファイルからセッション情報を読み取る。
func readSessionFile(path string) (*sessionFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sf sessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, err
	}

	return &sf, nil
}
