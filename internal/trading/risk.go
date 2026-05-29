package trading

import "fmt"

// RiskLimits defines hard limits checked before any order is placed.
type RiskLimits struct {
	MaxPositionPct float64  // maximum % of portfolio in one symbol (0 = unlimited)
	MaxDrawdownPct float64  // halt trading if portfolio drawdown exceeds this (0 = unlimited)
	MaxOrderValue  float64  // maximum single order value in currency (0 = unlimited)
	AllowedSymbols []string // whitelist; nil = all symbols allowed
	BlockedSymbols []string // blacklist; takes precedence over AllowedSymbols
}

// DefaultRiskLimits is a conservative default suitable for paper trading.
var DefaultRiskLimits = RiskLimits{
	MaxPositionPct: 0.25, // 25% of portfolio in one symbol
	MaxOrderValue:  10_000,
	MaxDrawdownPct: 0.20, // stop at 20% drawdown
}

// RiskDecision is the outcome of a risk evaluation.
type RiskDecision struct {
	Approved bool
	Reason   string
}

// RiskEngine evaluates orders against a set of risk limits.
type RiskEngine struct {
	limits RiskLimits
}

// NewRiskEngine creates a RiskEngine with the given limits.
func NewRiskEngine(limits RiskLimits) *RiskEngine {
	return &RiskEngine{limits: limits}
}

// Evaluate returns a RiskDecision for the proposed order given the current portfolio.
func (r *RiskEngine) Evaluate(portfolio Portfolio, order Order) RiskDecision {
	// Blacklist check.
	for _, blocked := range r.limits.BlockedSymbols {
		if blocked == order.Symbol {
			return RiskDecision{false, fmt.Sprintf("symbol %q is blocked", order.Symbol)}
		}
	}

	// Whitelist check (nil = all allowed).
	if len(r.limits.AllowedSymbols) > 0 {
		allowed := false
		for _, s := range r.limits.AllowedSymbols {
			if s == order.Symbol {
				allowed = true
				break
			}
		}
		if !allowed {
			return RiskDecision{false, fmt.Sprintf("symbol %q is not in the allowed list", order.Symbol)}
		}
	}

	// Max order value check.
	if r.limits.MaxOrderValue > 0 {
		orderValue := order.Qty * order.LimitPx
		if orderValue <= 0 {
			orderValue = order.Qty * 100 // estimate if no limit price
		}
		if orderValue > r.limits.MaxOrderValue {
			return RiskDecision{false, fmt.Sprintf("order value %.2f exceeds max %.2f", orderValue, r.limits.MaxOrderValue)}
		}
	}

	// Max position concentration check.
	if r.limits.MaxPositionPct > 0 && order.Side == OrderBuy {
		totalValue := portfolio.Cash
		for _, pos := range portfolio.Positions {
			totalValue += pos.CostBasis
		}
		if totalValue > 0 {
			existingPos := portfolio.Positions[order.Symbol]
			newCost := existingPos.CostBasis + order.Qty*order.LimitPx
			if newCost/totalValue > r.limits.MaxPositionPct {
				return RiskDecision{false, fmt.Sprintf("position in %q would exceed %.0f%% limit", order.Symbol, r.limits.MaxPositionPct*100)}
			}
		}
	}

	return RiskDecision{Approved: true, Reason: "all checks passed"}
}
