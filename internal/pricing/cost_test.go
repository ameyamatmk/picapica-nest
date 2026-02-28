package pricing

import (
	"math"
	"testing"
)

func TestCalcCost_PrefixMatch(t *testing.T) {
	// Given: claude-haiku-4-5 のコストテーブル
	table := map[string]CostEntry{
		"claude-haiku-4-5": {InputPerMTok: 0.80, OutputPerMTok: 4.00},
	}

	// When: フルモデル名（サフィックス付き）で計算
	cost := CalcCost(table, "claude-haiku-4-5-20250514", 1_000_000, 500_000)

	// Then: prefix 一致で正しく計算される（$0.80 + $2.00 = $2.80）
	expected := 2.80
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("expected %.4f, got %.4f", expected, cost)
	}
}

func TestCalcCost_LongestPrefixWins(t *testing.T) {
	// Given: 短い prefix と長い prefix が両方ある
	table := map[string]CostEntry{
		"claude":           {InputPerMTok: 1.00, OutputPerMTok: 5.00},
		"claude-sonnet-4":  {InputPerMTok: 3.00, OutputPerMTok: 15.00},
	}

	// When: claude-sonnet-4-20250514 で計算
	cost := CalcCost(table, "claude-sonnet-4-20250514", 1_000_000, 1_000_000)

	// Then: 最長一致で claude-sonnet-4 が使われる（$3.00 + $15.00 = $18.00）
	expected := 18.00
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("expected %.4f, got %.4f", expected, cost)
	}
}

func TestCalcCost_UnknownModel(t *testing.T) {
	// Given: コストテーブルにないモデル
	table := map[string]CostEntry{
		"claude-haiku-4-5": {InputPerMTok: 0.80, OutputPerMTok: 4.00},
	}

	// When: 未知のモデルで計算
	cost := CalcCost(table, "gpt-4o", 1_000_000, 500_000)

	// Then: 0 が返る
	if cost != 0 {
		t.Errorf("expected 0, got %.4f", cost)
	}
}

func TestCalcCost_EmptyTable(t *testing.T) {
	// Given: 空のコストテーブル
	table := map[string]CostEntry{}

	// When: 任意のモデルで計算
	cost := CalcCost(table, "claude-haiku-4-5", 1_000_000, 500_000)

	// Then: 0 が返る
	if cost != 0 {
		t.Errorf("expected 0, got %.4f", cost)
	}
}

func TestCalcCost_SmallTokens(t *testing.T) {
	// Given: コストテーブル
	table := map[string]CostEntry{
		"claude-haiku-4-5": {InputPerMTok: 0.80, OutputPerMTok: 4.00},
	}

	// When: 少量トークンで計算
	cost := CalcCost(table, "claude-haiku-4-5", 1000, 500)

	// Then: 正確に計算される（$0.0008 + $0.002 = $0.0028）
	expected := 0.0028
	if math.Abs(cost-expected) > 0.00001 {
		t.Errorf("expected %.6f, got %.6f", expected, cost)
	}
}
