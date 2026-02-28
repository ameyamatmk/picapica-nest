package pricing

import "time"

// Pricer はコスト計算と為替変換を統合する。
type Pricer struct {
	costTable map[string]CostEntry
	exchange  *ExchangeRateCache
}

// NewPricer は設定ファイルからコストテーブルを読み込み、為替キャッシュを初期化する。
func NewPricer(configPath string) (*Pricer, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	return &Pricer{
		costTable: cfg.CostTable,
		exchange:  NewExchangeRateCache(1 * time.Hour),
	}, nil
}

// CalcDailyCost はモデル別トークン数から USD コストと JPY コスト（取得できれば）を返す。
func (p *Pricer) CalcDailyCost(model string, promptTokens, completionTokens int) (usd float64, jpy *float64) {
	usd = CalcCost(p.costTable, model, promptTokens, completionTokens)

	rate, err := p.exchange.GetRate()
	if err == nil && rate != nil {
		v := usd * *rate
		jpy = &v
	}
	return usd, jpy
}
