package trading

import (
	"context"
	"time"
)

// Position is one holding in the portfolio.
type Position struct {
	Symbol    string
	Qty       float64
	AvgPrice  float64
	CostBasis float64
}

// Portfolio is the current state of the account.
type Portfolio struct {
	Positions map[string]Position
	Cash      float64
	Currency  string
	UpdatedAt time.Time
}

// TotalValue estimates portfolio value using last price from the provider.
func (p *Portfolio) TotalValue(ctx context.Context, market MarketDataProvider) float64 {
	total := p.Cash
	for sym, pos := range p.Positions {
		q, err := market.Quote(ctx, sym)
		if err == nil {
			total += pos.Qty * q.Last
		} else {
			total += pos.CostBasis
		}
	}
	return total
}

// PortfolioManager is the interface for portfolio access.
type PortfolioManager interface {
	Snapshot(ctx context.Context) (Portfolio, error)
	PnL(ctx context.Context) (float64, error)
	Allocation(ctx context.Context) map[string]float64
}

// InMemoryPortfolio is a simple in-process portfolio manager.
type InMemoryPortfolio struct {
	portfolio Portfolio
	market    MarketDataProvider
}

// NewInMemoryPortfolio creates a portfolio with the given starting cash.
func NewInMemoryPortfolio(cash float64, market MarketDataProvider) *InMemoryPortfolio {
	return &InMemoryPortfolio{
		portfolio: Portfolio{
			Positions: make(map[string]Position),
			Cash:      cash,
			Currency:  "USD",
			UpdatedAt: time.Now(),
		},
		market: market,
	}
}

func (p *InMemoryPortfolio) Snapshot(_ context.Context) (Portfolio, error) {
	return p.portfolio, nil
}

func (p *InMemoryPortfolio) PnL(ctx context.Context) (float64, error) {
	total := p.portfolio.TotalValue(ctx, p.market)
	var costBasis float64
	for _, pos := range p.portfolio.Positions {
		costBasis += pos.CostBasis
	}
	return total - p.portfolio.Cash - costBasis, nil
}

func (p *InMemoryPortfolio) Allocation(ctx context.Context) map[string]float64 {
	total := p.portfolio.TotalValue(ctx, p.market)
	if total == 0 {
		return nil
	}
	out := make(map[string]float64, len(p.portfolio.Positions))
	for sym, pos := range p.portfolio.Positions {
		q, err := p.market.Quote(ctx, sym)
		if err == nil {
			out[sym] = pos.Qty * q.Last / total
		}
	}
	return out
}

// ApplyOrder updates the portfolio after an order is filled.
func (p *InMemoryPortfolio) ApplyOrder(result OrderResult, order Order) {
	filled := result.FilledQty * result.FilledPx
	if order.Side == OrderBuy {
		p.portfolio.Cash -= filled
		pos := p.portfolio.Positions[order.Symbol]
		newQty := pos.Qty + result.FilledQty
		newCost := pos.CostBasis + filled
		p.portfolio.Positions[order.Symbol] = Position{
			Symbol:    order.Symbol,
			Qty:       newQty,
			AvgPrice:  newCost / newQty,
			CostBasis: newCost,
		}
	} else {
		p.portfolio.Cash += filled
		pos := p.portfolio.Positions[order.Symbol]
		pos.Qty -= result.FilledQty
		pos.CostBasis -= result.FilledQty * pos.AvgPrice
		if pos.Qty <= 0 {
			delete(p.portfolio.Positions, order.Symbol)
		} else {
			p.portfolio.Positions[order.Symbol] = pos
		}
	}
	p.portfolio.UpdatedAt = time.Now()
}
