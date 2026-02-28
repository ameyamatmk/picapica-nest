package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/providers"
)

// recentDailyDays は system prompt に埋め込む日次レポートの日数。
const recentDailyDays = 2

// 曜日の日本語表記
var weekdayJP = [...]string{
	"日曜日", "月曜日", "火曜日", "水曜日", "木曜日", "金曜日", "土曜日",
}

// PromptRewriteProvider は LLMProvider をラップし、
// system prompt の末尾に動的コンテキストを追加する。
type PromptRewriteProvider struct {
	inner         providers.LLMProvider
	location      *time.Location
	workspacePath string
}

// NewPromptRewriteProvider は inner Provider をラップする PromptRewriteProvider を返す。
// タイムゾーンは Asia/Tokyo を使用する。
func NewPromptRewriteProvider(inner providers.LLMProvider, workspacePath string) *PromptRewriteProvider {
	loc, _ := time.LoadLocation("Asia/Tokyo")
	if loc == nil {
		loc = time.FixedZone("JST", 9*60*60)
	}
	return &PromptRewriteProvider{
		inner:         inner,
		location:      loc,
		workspacePath: workspacePath,
	}
}

func (p *PromptRewriteProvider) Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, options map[string]interface{}) (*providers.LLMResponse, error) {
	rewritten := p.rewriteMessages(messages)
	return p.inner.Chat(ctx, rewritten, tools, model, options)
}

func (p *PromptRewriteProvider) GetDefaultModel() string {
	return p.inner.GetDefaultModel()
}

// rewriteMessages は messages をコピーし、system prompt の末尾に動的セクションを追加する。
// 元の slice は変更しない。
func (p *PromptRewriteProvider) rewriteMessages(messages []providers.Message) []providers.Message {
	if len(messages) == 0 {
		return messages
	}

	copied := make([]providers.Message, len(messages))
	copy(copied, messages)

	for i := range copied {
		if copied[i].Role == "system" {
			copied[i].Content += p.buildDynamicSection()
			break
		}
	}

	return copied
}

// buildDynamicSection は system prompt 末尾に追加する動的セクションを生成する。
// 時刻情報に加え、Hindsight レポートを注入する。
func (p *PromptRewriteProvider) buildDynamicSection() string {
	now := time.Now().In(p.location)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n\n---\n## Current Situation\n- 現在時刻: %s\n- 曜日: %s",
		now.Format("2006-01-02 15:04 (MST)"),
		weekdayJP[now.Weekday()],
	))

	// 日次レポート（内容埋め込み）
	if daily := p.buildDailySection(now); daily != "" {
		sb.WriteString("\n\n")
		sb.WriteString(daily)
	}

	// 週次・月次レポート（パス参照のみ）
	if refs := p.buildReportRefsSection(); refs != "" {
		sb.WriteString("\n\n")
		sb.WriteString(refs)
	}

	return sb.String()
}

// buildDailySection は直近の日次レポートを読み込み、見出し付きで結合する。
// ファイルが1件も見つからなければ空文字を返す。
func (p *PromptRewriteProvider) buildDailySection(now time.Time) string {
	if p.workspacePath == "" {
		return ""
	}
	dailyDir := filepath.Join(p.workspacePath, "memory", "daily")

	var sb strings.Builder
	count := 0

	for i := range recentDailyDays {
		date := now.AddDate(0, 0, -i)
		fileName := date.Format("2006-01-02") + ".md"
		content, err := readFile(filepath.Join(dailyDir, fileName))
		if err != nil || content == "" {
			continue
		}

		if count == 0 {
			sb.WriteString("## 直近の出来事")
		}
		sb.WriteString(fmt.Sprintf("\n\n### %d/%d（%s）\n",
			int(date.Month()), date.Day(), weekdayJP[date.Weekday()]))
		sb.WriteString(content)
		count++
	}

	return sb.String()
}

// buildReportRefsSection は週次・月次レポートの最新ファイルパスを参照として返す。
// ファイルが1件も見つからなければ空文字を返す。
func (p *PromptRewriteProvider) buildReportRefsSection() string {
	if p.workspacePath == "" {
		return ""
	}

	var refs []string

	// 週次レポート（最新1件）
	weeklyDir := filepath.Join(p.workspacePath, "memory", "weekly")
	if latest := latestFileByGlob(filepath.Join(weeklyDir, "*.md")); latest != "" {
		refs = append(refs, fmt.Sprintf("- 週次: %s", latest))
	}

	// 月次レポート（最新1件）
	monthlyDir := filepath.Join(p.workspacePath, "memory", "monthly")
	if latest := latestFileByGlob(filepath.Join(monthlyDir, "*.md")); latest != "" {
		refs = append(refs, fmt.Sprintf("- 月次: %s", latest))
	}

	if len(refs) == 0 {
		return ""
	}

	return "## 参照可能なレポート\n" + strings.Join(refs, "\n")
}

// readFile はファイルを読み込み、TrimSpace した内容を返す。
// ファイルが存在しない場合は ("", nil) を返す。
func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// latestFileByGlob は glob パターンにマッチするファイルのうち、
// ファイル名順で最後のもの（＝最新）のパスを返す。
func latestFileByGlob(pattern string) string {
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[len(matches)-1]
}
