package pricing

import "strings"

// CalcCost はモデル名と入出力トークン数から USD コストを計算する。
// CostTable のキーは prefix 最長一致で検索する。
// マッチしない場合は 0 を返す。
func CalcCost(table map[string]CostEntry, model string, promptTokens, completionTokens int) float64 {
	entry, ok := lookupCostEntry(table, model)
	if !ok {
		return 0
	}
	input := float64(promptTokens) / 1_000_000 * entry.InputPerMTok
	output := float64(completionTokens) / 1_000_000 * entry.OutputPerMTok
	return input + output
}

// lookupCostEntry は最長 prefix 一致で CostEntry を検索する。
func lookupCostEntry(table map[string]CostEntry, model string) (CostEntry, bool) {
	var bestKey string
	for key := range table {
		if strings.HasPrefix(model, key) && len(key) > len(bestKey) {
			bestKey = key
		}
	}
	if bestKey == "" {
		return CostEntry{}, false
	}
	return table[bestKey], true
}
