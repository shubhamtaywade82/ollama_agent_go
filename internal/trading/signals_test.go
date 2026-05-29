package trading

import (
	"context"
	"testing"
)

func syntheticBars(prices []float64) []OHLCV {
	bars := make([]OHLCV, len(prices))
	for i, p := range prices {
		bars[i] = OHLCV{Close: p, Open: p, High: p, Low: p}
	}
	return bars
}

func TestSMA(t *testing.T) {
	prices := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	bars := syntheticBars(prices)
	sma := SMA(bars, 3)
	// Expected: [2, 3, 4, 5, 6, 7, 8, 9] (mean of every 3 consecutive)
	if len(sma) != 8 {
		t.Fatalf("expected 8 values, got %d", len(sma))
	}
	if sma[0] != 2.0 {
		t.Errorf("SMA[0]: want 2.0, got %.2f", sma[0])
	}
	if sma[len(sma)-1] != 9.0 {
		t.Errorf("SMA last: want 9.0, got %.2f", sma[len(sma)-1])
	}
}

func TestRSI(t *testing.T) {
	m := NewMockMarketProvider()
	bars, _ := m.OHLCV(context.Background(), "AAPL", "1d", 30)
	rsi := RSI(bars, 14)
	if len(rsi) == 0 {
		t.Fatal("expected non-empty RSI")
	}
	for i, v := range rsi {
		if v < 0 || v > 100 {
			t.Errorf("RSI[%d] = %.2f out of [0,100]", i, v)
		}
	}
}

func TestMACD(t *testing.T) {
	m := NewMockMarketProvider()
	bars, _ := m.OHLCV(context.Background(), "AAPL", "1d", 50)
	macdLine, signalLine, histogram := MACD(bars, 12, 26, 9)
	if macdLine == nil || signalLine == nil || histogram == nil {
		t.Fatal("expected non-nil MACD output")
	}
	if len(histogram) != len(signalLine) {
		t.Errorf("histogram len %d != signal len %d", len(histogram), len(signalLine))
	}
}

func TestBollingerBands(t *testing.T) {
	prices := make([]float64, 30)
	for i := range prices {
		prices[i] = float64(100 + i)
	}
	bars := syntheticBars(prices)
	upper, mid, lower := BollingerBands(bars, 20, 2.0)
	if upper == nil || mid == nil || lower == nil {
		t.Fatal("expected non-nil Bollinger Bands")
	}
	for i := range upper {
		if upper[i] < mid[i] || mid[i] < lower[i] {
			t.Errorf("BB ordering violated at %d: upper=%.2f mid=%.2f lower=%.2f", i, upper[i], mid[i], lower[i])
		}
	}
}
