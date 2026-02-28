package pricing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExchangeRateCache_FetchesFromAPI(t *testing.T) {
	// Given: モック API サーバー
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(exchangeResponse{
			Rates: map[string]float64{"JPY": 150.25},
		})
	}))
	defer srv.Close()

	cache := NewExchangeRateCache(1 * time.Hour)
	cache.fetchRate = func() (float64, error) {
		return fetchRateFromURL(srv.URL, cache.client)
	}

	// When: GetRate を呼ぶ
	rate, err := cache.GetRate()

	// Then: レートが取得できる
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate == nil {
		t.Fatal("expected non-nil rate")
	}
	if *rate != 150.25 {
		t.Errorf("expected 150.25, got %.2f", *rate)
	}
}

func TestExchangeRateCache_UsesCache(t *testing.T) {
	// Given: 既にレートがキャッシュされている
	cache := NewExchangeRateCache(1 * time.Hour)
	cache.rate = 148.50
	cache.hasRate = true
	cache.fetchedAt = time.Now()

	callCount := 0
	cache.fetchRate = func() (float64, error) {
		callCount++
		return 0, nil
	}

	// When: GetRate を呼ぶ
	rate, err := cache.GetRate()

	// Then: キャッシュされたレートが返る（API は呼ばれない）
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate == nil {
		t.Fatal("expected non-nil rate")
	}
	if *rate != 148.50 {
		t.Errorf("expected 148.50, got %.2f", *rate)
	}
	if callCount != 0 {
		t.Errorf("expected fetchRate not to be called, but was called %d times", callCount)
	}
}

func TestExchangeRateCache_GracefulDegradation(t *testing.T) {
	// Given: 古いキャッシュがあり、API が失敗する
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cache := NewExchangeRateCache(1 * time.Millisecond)
	cache.rate = 145.00
	cache.hasRate = true
	cache.fetchedAt = time.Now().Add(-1 * time.Hour) // 期限切れ
	cache.fetchRate = func() (float64, error) {
		return fetchRateFromURL(srv.URL, cache.client)
	}

	// TTL を過ぎるのを待つ
	time.Sleep(2 * time.Millisecond)

	// When: GetRate を呼ぶ
	rate, err := cache.GetRate()

	// Then: 古いキャッシュが返る（graceful degradation）
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate == nil {
		t.Fatal("expected non-nil rate")
	}
	if *rate != 145.00 {
		t.Errorf("expected 145.00, got %.2f", *rate)
	}
}

func TestExchangeRateCache_NoCache_APIFails(t *testing.T) {
	// Given: キャッシュなし、API も失敗
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cache := NewExchangeRateCache(1 * time.Hour)
	cache.fetchRate = func() (float64, error) {
		return fetchRateFromURL(srv.URL, cache.client)
	}

	// When: GetRate を呼ぶ
	rate, err := cache.GetRate()

	// Then: nil が返る（エラー）
	if err == nil {
		t.Fatal("expected error")
	}
	if rate != nil {
		t.Errorf("expected nil rate, got %v", *rate)
	}
}
