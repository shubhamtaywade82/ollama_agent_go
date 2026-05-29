package trading

import (
	"math"
	"time"
)

// Signal is one computed technical indicator value.
type Signal struct {
	Symbol    string
	Indicator string
	Value     float64
	Extra     map[string]float64
	At        time.Time
}

// closes extracts Close prices from OHLCV bars.
func closes(bars []OHLCV) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Close
	}
	return out
}

// SMA returns the simple moving average of Close prices over period.
// Returns nil if len(bars) < period.
func SMA(bars []OHLCV, period int) []float64 {
	prices := closes(bars)
	if len(prices) < period {
		return nil
	}
	out := make([]float64, len(prices)-period+1)
	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	out[0] = sum / float64(period)
	for i := period; i < len(prices); i++ {
		sum += prices[i] - prices[i-period]
		out[i-period+1] = sum / float64(period)
	}
	return out
}

// EMA returns exponential moving average of Close prices over period.
func EMA(bars []OHLCV, period int) []float64 {
	prices := closes(bars)
	if len(prices) < period {
		return nil
	}
	out := make([]float64, len(prices))
	k := 2.0 / float64(period+1)
	// Seed with SMA of first period bars.
	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	out[period-1] = sum / float64(period)
	for i := period; i < len(prices); i++ {
		out[i] = prices[i]*k + out[i-1]*(1-k)
	}
	return out[period-1:]
}

// RSI returns the relative strength index (0–100) over period.
func RSI(bars []OHLCV, period int) []float64 {
	prices := closes(bars)
	if len(prices) <= period {
		return nil
	}
	gains := make([]float64, len(prices)-1)
	losses := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		diff := prices[i] - prices[i-1]
		if diff > 0 {
			gains[i-1] = diff
		} else {
			losses[i-1] = -diff
		}
	}
	// Initial average gain/loss.
	var avgGain, avgLoss float64
	for i := 0; i < period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	out := make([]float64, len(gains)-period+1)
	rsi := func(g, l float64) float64 {
		if l == 0 {
			return 100
		}
		rs := g / l
		return 100 - 100/(1+rs)
	}
	out[0] = rsi(avgGain, avgLoss)
	for i := period; i < len(gains); i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)
		out[i-period+1] = rsi(avgGain, avgLoss)
	}
	return out
}

// MACD returns MACD line, signal line, and histogram.
// fast, slow, signal are the typical periods (12, 26, 9).
func MACD(bars []OHLCV, fast, slow, signalPeriod int) (macdLine, signalLine, histogram []float64) {
	fastEMA := EMA(bars, fast)
	slowEMA := EMA(bars, slow)
	if fastEMA == nil || slowEMA == nil {
		return nil, nil, nil
	}
	// Align to the shorter series (slow EMA is shorter).
	offset := len(fastEMA) - len(slowEMA)
	if offset < 0 {
		offset = 0
	}
	n := len(slowEMA)
	macdLine = make([]float64, n)
	fastAligned := fastEMA[offset:]
	for i := 0; i < n; i++ {
		macdLine[i] = fastAligned[i] - slowEMA[i]
	}
	// Signal line = EMA of MACD line.
	if len(macdLine) < signalPeriod {
		return macdLine, nil, nil
	}
	// Build synthetic OHLCV from MACD values for EMA calculation.
	synth := make([]OHLCV, len(macdLine))
	for i, v := range macdLine {
		synth[i] = OHLCV{Close: v}
	}
	signalLine = EMA(synth, signalPeriod)
	if signalLine == nil {
		return macdLine, nil, nil
	}
	// Align histogram.
	histOffset := len(macdLine) - len(signalLine)
	histogram = make([]float64, len(signalLine))
	for i := range signalLine {
		histogram[i] = macdLine[histOffset+i] - signalLine[i]
	}
	return macdLine, signalLine, histogram
}

// BollingerBands returns upper, middle (SMA), and lower bands.
func BollingerBands(bars []OHLCV, period int, stdDevMult float64) (upper, middle, lower []float64) {
	middle = SMA(bars, period)
	if middle == nil {
		return nil, nil, nil
	}
	prices := closes(bars)
	upper = make([]float64, len(middle))
	lower = make([]float64, len(middle))
	for i, sma := range middle {
		start := i
		end := i + period
		var variance float64
		for _, p := range prices[start:end] {
			d := p - sma
			variance += d * d
		}
		sd := math.Sqrt(variance / float64(period))
		upper[i] = sma + stdDevMult*sd
		lower[i] = sma - stdDevMult*sd
	}
	return upper, middle, lower
}
