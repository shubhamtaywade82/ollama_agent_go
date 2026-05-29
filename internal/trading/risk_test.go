package trading

import (
	"testing"
)

func TestRiskBlocksOversizedOrder(t *testing.T) {
	limits := RiskLimits{MaxOrderValue: 1000}
	r := NewRiskEngine(limits)
	p := Portfolio{Positions: map[string]Position{}, Cash: 10000}
	o := Order{Symbol: "AAPL", Side: OrderBuy, Type: OrderMarket, Qty: 100, LimitPx: 150}
	dec := r.Evaluate(p, o)
	if dec.Approved {
		t.Errorf("expected order to be blocked, got approved: %s", dec.Reason)
	}
}

func TestRiskBlocksBlockedSymbol(t *testing.T) {
	limits := RiskLimits{BlockedSymbols: []string{"TSLA"}}
	r := NewRiskEngine(limits)
	p := Portfolio{Positions: map[string]Position{}, Cash: 10000}
	o := Order{Symbol: "TSLA", Side: OrderBuy, Type: OrderMarket, Qty: 1, LimitPx: 200}
	dec := r.Evaluate(p, o)
	if dec.Approved {
		t.Error("expected TSLA to be blocked")
	}
}

func TestRiskApprovesSafeOrder(t *testing.T) {
	limits := DefaultRiskLimits
	r := NewRiskEngine(limits)
	p := Portfolio{Positions: map[string]Position{}, Cash: 100000}
	o := Order{Symbol: "AAPL", Side: OrderBuy, Type: OrderMarket, Qty: 5, LimitPx: 150}
	dec := r.Evaluate(p, o)
	if !dec.Approved {
		t.Errorf("expected order to be approved, got blocked: %s", dec.Reason)
	}
}

func TestRiskBlocksAllowlistViolation(t *testing.T) {
	limits := RiskLimits{AllowedSymbols: []string{"AAPL", "GOOG"}}
	r := NewRiskEngine(limits)
	p := Portfolio{Positions: map[string]Position{}, Cash: 10000}
	o := Order{Symbol: "TSLA", Side: OrderBuy, Type: OrderMarket, Qty: 1, LimitPx: 200}
	dec := r.Evaluate(p, o)
	if dec.Approved {
		t.Error("expected TSLA to be blocked (not in allowlist)")
	}
}
