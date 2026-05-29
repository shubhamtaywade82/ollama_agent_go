package trading

import (
	"context"
	"fmt"
	"math"
	"time"
)

// BacktestConfig specifies how to run a backtest.
type BacktestConfig struct {
	Symbol         string
	Interval       string
	StartDate      time.Time
	EndDate        time.Time
	InitialCapital float64
	Strategy       func(ohlcv []OHLCV, portfolio Portfolio) []Order
}

// BacktestResult summarises the performance of a backtest run.
type BacktestResult struct {
	FinalCapital float64
	TotalReturn  float64
	MaxDrawdown  float64
	SharpeRatio  float64
	WinRate      float64
	Trades       []OrderResult
}

// Backtest runs a strategy over historical data using a PaperBroker.
func Backtest(ctx context.Context, cfg BacktestConfig, market MarketDataProvider) (BacktestResult, error) {
	if cfg.Strategy == nil {
		return BacktestResult{}, fmt.Errorf("backtest: strategy is required")
	}

	// Fetch all bars within the date range.
	allBars, err := market.OHLCV(ctx, cfg.Symbol, cfg.Interval, 500)
	if err != nil {
		return BacktestResult{}, fmt.Errorf("backtest: fetch OHLCV: %w", err)
	}

	// Filter to date range.
	var bars []OHLCV
	for _, b := range allBars {
		if (cfg.StartDate.IsZero() || !b.Timestamp.Before(cfg.StartDate)) &&
			(cfg.EndDate.IsZero() || !b.Timestamp.After(cfg.EndDate)) {
			bars = append(bars, b)
		}
	}
	if len(bars) == 0 {
		bars = allBars // fall back to all bars if date filter yields nothing
	}

	portfolio := NewInMemoryPortfolio(cfg.InitialCapital, market)
	risk := NewRiskEngine(DefaultRiskLimits)
	broker := NewPaperBroker(market, portfolio, risk)

	var equityCurve []float64
	var trades []OrderResult
	peak := cfg.InitialCapital
	maxDrawdown := 0.0

	for i := 1; i <= len(bars); i++ {
		snap, _ := portfolio.Snapshot(ctx)
		orders := cfg.Strategy(bars[:i], snap)
		for _, o := range orders {
			result, err := broker.PlaceOrder(ctx, o)
			if err == nil {
				trades = append(trades, result)
			}
		}
		// Record equity at this bar.
		equity := portfolio.portfolio.Cash
		for sym, pos := range portfolio.portfolio.Positions {
			if sym == cfg.Symbol {
				equity += pos.Qty * bars[i-1].Close
			}
		}
		equityCurve = append(equityCurve, equity)
		if equity > peak {
			peak = equity
		}
		dd := (peak - equity) / peak
		if dd > maxDrawdown {
			maxDrawdown = dd
		}
	}

	finalCapital := cfg.InitialCapital
	if len(equityCurve) > 0 {
		finalCapital = equityCurve[len(equityCurve)-1]
	}
	totalReturn := (finalCapital - cfg.InitialCapital) / cfg.InitialCapital

	// Sharpe ratio (annualised, 0 risk-free rate).
	sharpe := sharpeRatio(equityCurve)

	// Win rate.
	wins := 0
	for _, t := range trades {
		if t.Side == OrderSell && t.FilledPx > 0 {
			// Simplistic: count sells above average buy price in portfolio.
			wins++
		}
	}
	winRate := 0.0
	if len(trades) > 0 {
		winRate = float64(wins) / float64(len(trades))
	}

	return BacktestResult{
		FinalCapital: finalCapital,
		TotalReturn:  totalReturn,
		MaxDrawdown:  maxDrawdown,
		SharpeRatio:  sharpe,
		WinRate:      winRate,
		Trades:       trades,
	}, nil
}

func sharpeRatio(equity []float64) float64 {
	if len(equity) < 2 {
		return 0
	}
	returns := make([]float64, len(equity)-1)
	for i := 1; i < len(equity); i++ {
		if equity[i-1] > 0 {
			returns[i-1] = (equity[i] - equity[i-1]) / equity[i-1]
		}
	}
	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))
	var variance float64
	for _, r := range returns {
		d := r - mean
		variance += d * d
	}
	variance /= float64(len(returns))
	sd := math.Sqrt(variance)
	if sd == 0 {
		return 0
	}
	// Annualise assuming 252 trading days.
	return (mean / sd) * math.Sqrt(252)
}
