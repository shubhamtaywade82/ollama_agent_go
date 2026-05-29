package trading

import (
	"context"
	"fmt"
	"math"
	"time"
)

// MockMarketProvider is a deterministic market data provider for tests.
// Prices follow a sine wave seeded by the symbol name.
type MockMarketProvider struct {
	BasePrice map[string]float64 // symbol → starting price
}

func NewMockMarketProvider() *MockMarketProvider {
	return &MockMarketProvider{BasePrice: map[string]float64{
		"AAPL": 150.0,
		"TSLA": 200.0,
		"GOOG": 130.0,
	}}
}

func (m *MockMarketProvider) Quote(_ context.Context, symbol string) (Quote, error) {
	base := m.base(symbol)
	return Quote{
		Symbol:    symbol,
		Bid:       base - 0.01,
		Ask:       base + 0.01,
		Last:      base,
		Volume:    1_000_000,
		Timestamp: time.Now(),
	}, nil
}

func (m *MockMarketProvider) OHLCV(_ context.Context, symbol, interval string, limit int) ([]OHLCV, error) {
	base := m.base(symbol)
	bars := make([]OHLCV, limit)
	now := time.Now()
	for i := 0; i < limit; i++ {
		t := now.Add(-time.Duration(limit-i) * intervalDuration(interval))
		offset := math.Sin(float64(i) * 0.3)
		c := base + offset*5
		bars[i] = OHLCV{
			Symbol:    symbol,
			Open:      c - 1,
			High:      c + 2,
			Low:       c - 2,
			Close:     c,
			Volume:    500_000,
			Timestamp: t,
			Interval:  interval,
		}
	}
	return bars, nil
}

func (m *MockMarketProvider) News(_ context.Context, symbol string, limit int) ([]NewsItem, error) {
	items := make([]NewsItem, limit)
	for i := 0; i < limit; i++ {
		items[i] = NewsItem{
			Headline:  fmt.Sprintf("Mock headline %d for %s", i+1, symbol),
			Source:    "MockNews",
			Sentiment: float32(math.Sin(float64(i)*0.5)) * 0.5,
			At:        time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	return items, nil
}

func (m *MockMarketProvider) base(symbol string) float64 {
	if p, ok := m.BasePrice[symbol]; ok {
		return p
	}
	// Deterministic price from symbol string.
	var h float64
	for _, c := range symbol {
		h += float64(c)
	}
	return 50 + math.Mod(h, 200)
}

func intervalDuration(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "1h":
		return time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return time.Hour
	}
}
