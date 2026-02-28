package pricing

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const frankfurterURL = "https://api.frankfurter.dev/v1/latest?base=USD&symbols=JPY"

// exchangeResponse は Frankfurter API のレスポンス。
type exchangeResponse struct {
	Rates map[string]float64 `json:"rates"`
}

// ExchangeRateCache は USD/JPY 為替レートのキャッシュ。
type ExchangeRateCache struct {
	rate      float64
	hasRate   bool
	fetchedAt time.Time
	ttl       time.Duration
	mu        sync.Mutex
	client    *http.Client
	fetchRate func() (float64, error) // テスト用に差し替え可能
}

// NewExchangeRateCache は指定 TTL のキャッシュを作成する。
func NewExchangeRateCache(ttl time.Duration) *ExchangeRateCache {
	c := &ExchangeRateCache{
		ttl: ttl,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	c.fetchRate = func() (float64, error) {
		return fetchRateFromURL(frankfurterURL, c.client)
	}
	return c
}

// GetRate は USD/JPY レートを返す。
// キャッシュが有効ならキャッシュから返し、期限切れなら API から取得する。
// API 失敗時は古いキャッシュがあればそれを返す（graceful degradation）。
// レートが一度も取得できていない場合は nil を返す。
func (c *ExchangeRateCache) GetRate() (*float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hasRate && time.Since(c.fetchedAt) < c.ttl {
		rate := c.rate
		return &rate, nil
	}

	rate, err := c.fetchRate()
	if err != nil {
		slog.Warn("failed to fetch exchange rate", "component", "pricing", "error", err)
		if c.hasRate {
			// graceful degradation: 古いキャッシュを返す
			rate := c.rate
			return &rate, nil
		}
		return nil, err
	}

	c.rate = rate
	c.hasRate = true
	c.fetchedAt = time.Now()
	return &rate, nil
}

// fetchRateFromURL は指定 URL から USD/JPY レートを取得する。
func fetchRateFromURL(url string, client *http.Client) (float64, error) {
	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("exchange rate API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("exchange rate API returned status %d", resp.StatusCode)
	}

	var result exchangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode exchange rate response: %w", err)
	}

	rate, ok := result.Rates["JPY"]
	if !ok {
		return 0, fmt.Errorf("JPY rate not found in response")
	}
	return rate, nil
}
