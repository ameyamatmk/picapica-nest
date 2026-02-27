package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/sipeed/picoclaw/pkg/providers"
)

// 曜日の日本語表記
var weekdayJP = [...]string{
	"日曜日", "月曜日", "火曜日", "水曜日", "木曜日", "金曜日", "土曜日",
}

// PromptRewriteProvider は LLMProvider をラップし、
// system prompt の末尾に動的コンテキストを追加する。
type PromptRewriteProvider struct {
	inner    providers.LLMProvider
	location *time.Location
}

// NewPromptRewriteProvider は inner Provider をラップする PromptRewriteProvider を返す。
// タイムゾーンは Asia/Tokyo を使用する。
func NewPromptRewriteProvider(inner providers.LLMProvider) *PromptRewriteProvider {
	loc, _ := time.LoadLocation("Asia/Tokyo")
	if loc == nil {
		loc = time.FixedZone("JST", 9*60*60)
	}
	return &PromptRewriteProvider{
		inner:    inner,
		location: loc,
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
// 初期実装では時刻情報のみ。P2 で Recent Context / Priority Override を追加予定。
func (p *PromptRewriteProvider) buildDynamicSection() string {
	now := time.Now().In(p.location)
	return fmt.Sprintf("\n\n---\n## Current Situation\n- 現在時刻: %s\n- 曜日: %s",
		now.Format("2006-01-02 15:04 (MST)"),
		weekdayJP[now.Weekday()],
	)
}
