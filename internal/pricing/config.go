// Package pricing はモデル別コスト計算と為替レート取得を提供する。
package pricing

import (
	"encoding/json"
	"os"
)

// CostEntry は 100万トークンあたりの USD 単価。
type CostEntry struct {
	InputPerMTok  float64 `json:"input_per_mtok"`
	OutputPerMTok float64 `json:"output_per_mtok"`
}

// Config は pricing.json の構造。
type Config struct {
	CostTable map[string]CostEntry `json:"cost_table"`
}

// LoadConfig は指定パスから pricing 設定を読み込む。
// ファイルが存在しない場合は空テーブルの Config を返す。
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{CostTable: map[string]CostEntry{}}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.CostTable == nil {
		cfg.CostTable = map[string]CostEntry{}
	}
	return &cfg, nil
}
