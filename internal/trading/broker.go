package trading

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OrderSide indicates buy or sell.
type OrderSide string

const (
	OrderBuy  OrderSide = "buy"
	OrderSell OrderSide = "sell"
)

// OrderType indicates market or limit order.
type OrderType string

const (
	OrderMarket OrderType = "market"
	OrderLimit  OrderType = "limit"
)

// Order is a trading order request.
type Order struct {
	Symbol  string
	Side    OrderSide
	Type    OrderType
	Qty     float64
	LimitPx float64 // 0 = market order
}

// OrderResult records the outcome of an order.
type OrderResult struct {
	OrderID   string
	Symbol    string
	Side      OrderSide
	FilledQty float64
	FilledPx  float64
	At        time.Time
}

// Broker is the interface all broker implementations satisfy.
type Broker interface {
	PlaceOrder(ctx context.Context, o Order) (OrderResult, error)
	CancelOrder(ctx context.Context, orderID string) error
	Orders(ctx context.Context) ([]OrderResult, error)
}

// PaperBroker simulates order execution using live quotes. No real money.
type PaperBroker struct {
	market    MarketDataProvider
	portfolio *InMemoryPortfolio
	risk      *RiskEngine
	orders    []OrderResult
}

// NewPaperBroker creates a PaperBroker.
func NewPaperBroker(market MarketDataProvider, portfolio *InMemoryPortfolio, risk *RiskEngine) *PaperBroker {
	return &PaperBroker{market: market, portfolio: portfolio, risk: risk}
}

// PlaceOrder simulates order execution using the current quote price.
func (b *PaperBroker) PlaceOrder(ctx context.Context, o Order) (OrderResult, error) {
	// Risk check.
	snap, _ := b.portfolio.Snapshot(ctx)
	dec := b.risk.Evaluate(snap, o)
	if !dec.Approved {
		return OrderResult{}, fmt.Errorf("order blocked by risk engine: %s", dec.Reason)
	}

	// Fill at last quote price (market) or limit price.
	fillPx := o.LimitPx
	if o.Type == OrderMarket || fillPx == 0 {
		q, err := b.market.Quote(ctx, o.Symbol)
		if err != nil {
			return OrderResult{}, fmt.Errorf("paper broker: get quote: %w", err)
		}
		if o.Side == OrderBuy {
			fillPx = q.Ask
		} else {
			fillPx = q.Bid
		}
	}

	result := OrderResult{
		OrderID:   uuid.New().String(),
		Symbol:    o.Symbol,
		Side:      o.Side,
		FilledQty: o.Qty,
		FilledPx:  fillPx,
		At:        time.Now(),
	}
	b.portfolio.ApplyOrder(result, o)
	b.orders = append(b.orders, result)
	return result, nil
}

func (b *PaperBroker) CancelOrder(_ context.Context, orderID string) error {
	for i, o := range b.orders {
		if o.OrderID == orderID {
			b.orders = append(b.orders[:i], b.orders[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("order %q not found", orderID)
}

func (b *PaperBroker) Orders(_ context.Context) ([]OrderResult, error) {
	return b.orders, nil
}
