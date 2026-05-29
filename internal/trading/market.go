// Package trading provides domain-specific types and tools for financial
// market data, signal generation, portfolio management, risk controls,
// order execution, and backtesting.
package trading

import (
	"context"
	"time"
)

// Quote is a real-time price snapshot for a symbol.
type Quote struct {
	Symbol    string
	Bid       float64
	Ask       float64
	Last      float64
	Volume    int64
	Timestamp time.Time
}

// OHLCV is one candlestick bar.
type OHLCV struct {
	Symbol    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
	Timestamp time.Time
	Interval  string // "1m", "5m", "1h", "1d"
}

// NewsItem is one news article with sentiment score.
type NewsItem struct {
	Headline  string
	Source    string
	URL       string
	Sentiment float32 // -1.0 (bearish) to 1.0 (bullish)
	At        time.Time
}

// MarketDataProvider is the interface all market data backends implement.
type MarketDataProvider interface {
	Quote(ctx context.Context, symbol string) (Quote, error)
	OHLCV(ctx context.Context, symbol, interval string, limit int) ([]OHLCV, error)
	News(ctx context.Context, symbol string, limit int) ([]NewsItem, error)
}
