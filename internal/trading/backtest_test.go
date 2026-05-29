package trading

import (
	"context"
	"testing"
)

// buyAndHoldStrategy buys all available cash on the first bar, holds forever.
func buyAndHoldStrategy(bars []OHLCV, portfolio Portfolio) []Order {
	if len(bars) != 1 {
		return nil
	}
	if len(bars) > 0 && portfolio.Cash > 0 {
		symbol := bars[0].Symbol
		qty := portfolio.Cash / bars[0].Close
		return []Order{{Symbol: symbol, Side: OrderBuy, Type: OrderMarket, Qty: qty, LimitPx: bars[0].Close}}
	}
	return nil
}

func TestBacktestBuyAndHold(t *testing.T) {
	market := NewMockMarketProvider()
	cfg := BacktestConfig{
		Symbol:         "AAPL",
		Interval:       "1d",
		InitialCapital: 10000,
		Strategy:       buyAndHoldStrategy,
	}
	result, err := Backtest(context.Background(), cfg, market)
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalCapital <= 0 {
		t.Errorf("expected positive final capital, got %.2f", result.FinalCapital)
	}
	// Total return should be finite and non-NaN.
	if result.TotalReturn != result.TotalReturn { // NaN check
		t.Error("TotalReturn is NaN")
	}
	if result.MaxDrawdown < 0 || result.MaxDrawdown > 1 {
		t.Errorf("MaxDrawdown %.2f out of [0,1]", result.MaxDrawdown)
	}
}

func TestBacktestNoStrategy(t *testing.T) {
	market := NewMockMarketProvider()
	_, err := Backtest(context.Background(), BacktestConfig{
		Symbol:         "AAPL",
		Interval:       "1d",
		InitialCapital: 10000,
	}, market)
	if err == nil {
		t.Error("expected error when strategy is nil")
	}
}
