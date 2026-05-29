package trading

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ollama_agent_go/internal/tools"
)

// RegisterTradingTools adds trading-specific tools to the given registry.
func RegisterTradingTools(reg *tools.Registry, market MarketDataProvider, broker Broker, risk *RiskEngine, portfolio PortfolioManager) {
	reg.Register(&getQuoteTool{market: market})
	reg.Register(&getOHLCVTool{market: market})
	reg.Register(&getSignalsTool{market: market})
	reg.Register(&getPortfolioTool{portfolio: portfolio})
	reg.Register(&placeOrderTool{broker: broker})
	reg.Register(&cancelOrderTool{broker: broker})
	reg.Register(&getNewsTool{market: market})
}

// ─── get_quote ────────────────────────────────────────────────────────────────

type getQuoteTool struct{ market MarketDataProvider }

func (t *getQuoteTool) Name() string        { return "get_quote" }
func (t *getQuoteTool) Description() string { return "Get the current bid/ask/last quote for a symbol." }
func (t *getQuoteTool) Parameters() map[string]any {
	return jsonSchema("object", map[string]any{
		"symbol": prop("string", "Ticker symbol (e.g. AAPL)"),
	}, "symbol")
}
func (t *getQuoteTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	symbol, _ := args["symbol"].(string)
	q, err := t.market.Quote(ctx, symbol)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Symbol: %s  Bid: %.2f  Ask: %.2f  Last: %.2f  Vol: %d", q.Symbol, q.Bid, q.Ask, q.Last, q.Volume), nil
}

// ─── get_ohlcv ────────────────────────────────────────────────────────────────

type getOHLCVTool struct{ market MarketDataProvider }

func (t *getOHLCVTool) Name() string        { return "get_ohlcv" }
func (t *getOHLCVTool) Description() string { return "Get OHLCV candlestick bars for a symbol." }
func (t *getOHLCVTool) Parameters() map[string]any {
	return jsonSchema("object", map[string]any{
		"symbol":   prop("string", "Ticker symbol"),
		"interval": prop("string", "Bar interval: 1m, 5m, 1h, 1d"),
		"limit":    prop("integer", "Number of bars to return (max 200)"),
	}, "symbol", "interval")
}
func (t *getOHLCVTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	symbol, _ := args["symbol"].(string)
	interval, _ := args["interval"].(string)
	limit := 20
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	bars, err := t.market.OHLCV(ctx, symbol, interval, limit)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "OHLCV for %s (%s), last %d bars:\n", symbol, interval, len(bars))
	for _, b := range bars {
		fmt.Fprintf(&sb, "  %s  O:%.2f H:%.2f L:%.2f C:%.2f V:%d\n",
			b.Timestamp.Format("2006-01-02 15:04"), b.Open, b.High, b.Low, b.Close, b.Volume)
	}
	return sb.String(), nil
}

// ─── get_signals ──────────────────────────────────────────────────────────────

type getSignalsTool struct{ market MarketDataProvider }

func (t *getSignalsTool) Name() string        { return "get_signals" }
func (t *getSignalsTool) Description() string { return "Compute technical indicators for a symbol." }
func (t *getSignalsTool) Parameters() map[string]any {
	return jsonSchema("object", map[string]any{
		"symbol":     prop("string", "Ticker symbol"),
		"indicators": prop("array", "List of indicators: SMA, EMA, RSI, MACD, BB"),
	}, "symbol")
}
func (t *getSignalsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	symbol, _ := args["symbol"].(string)
	bars, err := t.market.OHLCV(ctx, symbol, "1d", 50)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Signals for %s:\n", symbol)
	sma := SMA(bars, 20)
	if len(sma) > 0 {
		fmt.Fprintf(&sb, "  SMA(20): %.2f\n", sma[len(sma)-1])
	}
	rsi := RSI(bars, 14)
	if len(rsi) > 0 {
		fmt.Fprintf(&sb, "  RSI(14): %.2f\n", rsi[len(rsi)-1])
	}
	return sb.String(), nil
}

// ─── get_portfolio ────────────────────────────────────────────────────────────

type getPortfolioTool struct{ portfolio PortfolioManager }

func (t *getPortfolioTool) Name() string        { return "get_portfolio" }
func (t *getPortfolioTool) Description() string { return "Get current portfolio positions and cash balance." }
func (t *getPortfolioTool) Parameters() map[string]any {
	return jsonSchema("object", map[string]any{}, "")
}
func (t *getPortfolioTool) Execute(ctx context.Context, _ map[string]any) (string, error) {
	p, err := t.portfolio.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Cash: %.2f %s\n", p.Cash, p.Currency)
	for sym, pos := range p.Positions {
		fmt.Fprintf(&sb, "  %s: qty=%.4f avg=%.2f cost=%.2f\n", sym, pos.Qty, pos.AvgPrice, pos.CostBasis)
	}
	return sb.String(), nil
}

// ─── place_order ─────────────────────────────────────────────────────────────

type placeOrderTool struct{ broker Broker }

func (t *placeOrderTool) Name() string        { return "place_order" }
func (t *placeOrderTool) Description() string { return "Place a buy or sell order. Risk checks run automatically." }
func (t *placeOrderTool) Parameters() map[string]any {
	return jsonSchema("object", map[string]any{
		"symbol":   prop("string", "Ticker symbol"),
		"side":     prop("string", "buy or sell"),
		"order_type": prop("string", "market or limit"),
		"qty":      prop("number", "Number of shares/units"),
		"limit_px": prop("number", "Limit price (required for limit orders)"),
	}, "symbol", "side", "order_type", "qty")
}
func (t *placeOrderTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	o := Order{
		Symbol:  str(args, "symbol"),
		Side:    OrderSide(str(args, "side")),
		Type:    OrderType(str(args, "order_type")),
		Qty:     flt(args, "qty"),
		LimitPx: flt(args, "limit_px"),
	}
	result, err := t.broker.PlaceOrder(ctx, o)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Filled: %s %s %.4f @ %.2f  ID=%s", result.Side, result.Symbol, result.FilledQty, result.FilledPx, result.OrderID), nil
}

// ─── cancel_order ─────────────────────────────────────────────────────────────

type cancelOrderTool struct{ broker Broker }

func (t *cancelOrderTool) Name() string        { return "cancel_order" }
func (t *cancelOrderTool) Description() string { return "Cancel an open order by ID." }
func (t *cancelOrderTool) Parameters() map[string]any {
	return jsonSchema("object", map[string]any{
		"order_id": prop("string", "Order ID to cancel"),
	}, "order_id")
}
func (t *cancelOrderTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	id := str(args, "order_id")
	if err := t.broker.CancelOrder(ctx, id); err != nil {
		return "", err
	}
	return "Order " + id + " cancelled.", nil
}

// ─── get_news ─────────────────────────────────────────────────────────────────

type getNewsTool struct{ market MarketDataProvider }

func (t *getNewsTool) Name() string        { return "get_news" }
func (t *getNewsTool) Description() string { return "Get recent news for a symbol with sentiment scores." }
func (t *getNewsTool) Parameters() map[string]any {
	return jsonSchema("object", map[string]any{
		"symbol": prop("string", "Ticker symbol"),
		"limit":  prop("integer", "Number of news items (default 5)"),
	}, "symbol")
}
func (t *getNewsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	symbol := str(args, "symbol")
	limit := 5
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	items, err := t.market.News(ctx, symbol, limit)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "News for %s:\n", symbol)
	for _, item := range items {
		fmt.Fprintf(&sb, "  [%.2f] %s (%s)\n", item.Sentiment, item.Headline, item.Source)
	}
	return sb.String(), nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func jsonSchema(typ string, props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": typ, "properties": props}
	var req []string
	for _, r := range required {
		if r != "" {
			req = append(req, r)
		}
	}
	if len(req) > 0 {
		m["required"] = req
	}
	return m
}

func prop(typ, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}

func str(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func flt(args map[string]any, key string) float64 {
	v, _ := args[key].(float64)
	return v
}

// marshalJSON is a convenience wrapper used in test assertions.
func marshalJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

var _ = marshalJSON // suppress unused warning
