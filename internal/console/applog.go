package console

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// appLogEntry はアプリログの1行分のデータ。
type appLogEntry struct {
	Time      string            // 時刻表示 (HH:MM:SS)
	Level     string            // "INFO", "ERROR" 等
	Message   string            // ログメッセージ
	Component string            // コンポーネント名
	Extra     map[string]string // その他のフィールド
}

// appLogDate はログファイルの日付メタデータ。
type appLogDate struct {
	FileName string // "2026-02-28.jsonl"
	Label    string // "2026-02-28"
}

// applogData はアプリケーションログ画面のテンプレートデータ。
type applogData struct {
	pageData
	Dates           []appLogDate
	CurrentDate     string
	Entries         []appLogEntry
	Levels          []string // フィルタ用レベルリスト
	Components      []string // 抽出されたコンポーネント一覧
	FilterLevel     string   // 現在のフィルタレベル（空=全て）
	FilterComponent string   // 現在のフィルタコンポーネント（空=全て）
}

// allLevels はフィルタ用のレベル一覧（優先度昇順）。
var allLevels = []string{"DEBUG", "INFO", "WARN", "ERROR"}

// levelPriority はログレベルの優先度マップ。数値が大きいほど重要。
var levelPriority = map[string]int{
	"DEBUG": 0,
	"INFO":  1,
	"WARN":  2,
	"ERROR": 3,
}

// handleAppLog はアプリケーションログ画面をフルページで返す。
func (s *Server) handleAppLog(w http.ResponseWriter, r *http.Request) {
	data := s.buildAppLogData("", "", "")

	if err := s.applogTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("failed to render applog page", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAppLogEntries はログエントリを HTMX fragment として返す。
func (s *Server) handleAppLogEntries(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	level := r.URL.Query().Get("level")
	component := r.URL.Query().Get("component")

	data := s.buildAppLogData(date, level, component)

	if err := s.applogTmpl.ExecuteTemplate(w, "applog_content", data); err != nil {
		slog.Error("failed to render applog entries", "component", "console", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// buildAppLogData はテンプレートに渡すデータを構築する。
func (s *Server) buildAppLogData(date, level, component string) applogData {
	data := applogData{
		pageData: pageData{
			Title:  "アプリケーションログ",
			Active: "logs",
		},
		Levels:          allLevels,
		FilterLevel:     level,
		FilterComponent: component,
	}

	// ログファイル一覧を取得（降順）
	dates, err := listAppLogDates(s.workspacePath)
	if err != nil {
		slog.Error("failed to list applog dates", "component", "console", "error", err)
		return data
	}
	data.Dates = dates

	// 日付未指定の場合は最新日付を自動選択
	if date == "" && len(dates) > 0 {
		date = dates[0].FileName
	}
	data.CurrentDate = date

	if date == "" {
		return data
	}

	// ログエントリを読み込み
	entries, components, err := loadAppLogEntries(s.workspacePath, date)
	if err != nil {
		slog.Error("failed to load applog entries", "component", "console", "date", date, "error", err)
		return data
	}
	data.Components = components

	// フィルタ適用
	data.Entries = filterLogEntries(entries, level, component)

	return data
}

// appLogDir はアプリログのディレクトリパスを返す。
func appLogDir(workspacePath string) string {
	return filepath.Join(workspacePath, "logs", "app")
}

// listAppLogDates はアプリログの日付ファイル一覧を降順で返す。
func listAppLogDates(workspacePath string) ([]appLogDate, error) {
	dir := appLogDir(workspacePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var dates []appLogDate
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		dates = append(dates, appLogDate{
			FileName: e.Name(),
			Label:    strings.TrimSuffix(e.Name(), ".jsonl"),
		})
	}

	// 降順ソート（新しい順）
	slices.SortFunc(dates, func(a, b appLogDate) int {
		return strings.Compare(b.FileName, a.FileName)
	})

	return dates, nil
}

// rawLogEntry は JSONL パース用の中間構造体。
type rawLogEntry struct {
	Time      string `json:"time"`
	Level     string `json:"level"`
	Msg       string `json:"msg"`
	Component string `json:"component"`
}

// knownFields は rawLogEntry に対応する既知フィールド名。Extra に含めない。
var knownFields = map[string]bool{
	"time":      true,
	"level":     true,
	"msg":       true,
	"component": true,
}

// loadAppLogEntries は指定日付の JSONL ファイルを読み込み、エントリとコンポーネント一覧を返す。
func loadAppLogEntries(workspacePath, dateFile string) ([]appLogEntry, []string, error) {
	// パストラバーサル対策
	safeName := filepath.Base(dateFile)
	if safeName != dateFile || !strings.HasSuffix(safeName, ".jsonl") {
		return nil, nil, os.ErrInvalid
	}

	path := filepath.Join(appLogDir(workspacePath), safeName)
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var entries []appLogEntry
	componentSet := make(map[string]bool)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// まず既知フィールドをパース
		var raw rawLogEntry
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		// Extra フィールドの抽出: 全フィールドをパースして既知を除外
		var allFields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &allFields); err != nil {
			continue
		}

		extra := make(map[string]string)
		for k, v := range allFields {
			if knownFields[k] {
				continue
			}
			// JSON の値を文字列に変換（引用符を外す）
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				// 文字列でないなら生 JSON をそのまま使う
				extra[k] = string(v)
			} else {
				extra[k] = s
			}
		}

		// 時刻表示を HH:MM:SS に変換
		timeDisplay := extractTime(raw.Time)

		entry := appLogEntry{
			Time:      timeDisplay,
			Level:     raw.Level,
			Message:   raw.Msg,
			Component: raw.Component,
			Extra:     extra,
		}
		entries = append(entries, entry)

		if raw.Component != "" {
			componentSet[raw.Component] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	// コンポーネント一覧をソート
	components := make([]string, 0, len(componentSet))
	for c := range componentSet {
		components = append(components, c)
	}
	slices.Sort(components)

	return entries, components, nil
}

// extractTime はタイムスタンプから HH:MM:SS 部分を抽出する。
func extractTime(ts string) string {
	// "T" の後ろから "+" または "Z" の前まで、最初の8文字 (HH:MM:SS)
	idx := strings.IndexByte(ts, 'T')
	if idx < 0 || idx+9 > len(ts) {
		return ts
	}
	return ts[idx+1 : idx+9]
}

// filterLogEntries はレベルとコンポーネントでエントリをフィルタリングする。
// level が指定された場合、そのレベル以上のエントリのみ返す。
// component が指定された場合、完全一致でフィルタする。
func filterLogEntries(entries []appLogEntry, level, component string) []appLogEntry {
	if level == "" && component == "" {
		return entries
	}

	minPriority := 0
	if level != "" {
		if p, ok := levelPriority[level]; ok {
			minPriority = p
		}
	}

	var filtered []appLogEntry
	for _, e := range entries {
		// レベルフィルタ
		if level != "" {
			p, ok := levelPriority[e.Level]
			if !ok || p < minPriority {
				continue
			}
		}
		// コンポーネントフィルタ
		if component != "" && e.Component != component {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}
