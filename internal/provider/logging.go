package provider

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/providers"
)

// jst は日本標準時 (UTC+9)。
var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

// UsageRecord は LLM 呼び出し1回分の usage 情報。
// usage.jsonl に JSONL 形式で追記される。
type UsageRecord struct {
	Timestamp        string `json:"ts"`
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	LatencyMs        int64  `json:"latency_ms"`
	Error            string `json:"error,omitempty"`
}

// LoggingProvider は LLMProvider をラップし、
// 各 Chat() 呼び出しの usage / レイテンシを JSONL ファイルに記録する。
type LoggingProvider struct {
	inner   providers.LLMProvider
	logPath string
	mu      sync.Mutex
}

// NewLoggingProvider は inner Provider をラップする LoggingProvider を返す。
// logPath は usage.jsonl の出力先パス。
func NewLoggingProvider(inner providers.LLMProvider, logPath string) *LoggingProvider {
	return &LoggingProvider{
		inner:   inner,
		logPath: logPath,
	}
}

func (p *LoggingProvider) Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, options map[string]interface{}) (*providers.LLMResponse, error) {
	start := time.Now()
	resp, err := p.inner.Chat(ctx, messages, tools, model, options)
	latency := time.Since(start)

	record := UsageRecord{
		Timestamp: start.In(jst).Format(time.RFC3339),
		Model:     model,
		LatencyMs: latency.Milliseconds(),
	}

	if err != nil {
		record.Error = err.Error()
	}
	if resp != nil && resp.Usage != nil {
		record.PromptTokens = resp.Usage.PromptTokens
		record.CompletionTokens = resp.Usage.CompletionTokens
		record.TotalTokens = resp.Usage.TotalTokens
	}

	// ログ書き込みの失敗は本体の処理をブロックしない
	_ = p.appendRecord(record)

	return resp, err
}

func (p *LoggingProvider) GetDefaultModel() string {
	return p.inner.GetDefaultModel()
}

func (p *LoggingProvider) appendRecord(record UsageRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	p.mu.Lock()
	defer p.mu.Unlock()

	f, err := os.OpenFile(p.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}
