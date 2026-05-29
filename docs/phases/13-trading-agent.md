# Phase 13 — Trading Agent

Tags: `#trading` `#finance` `#market-data` `#signals` `#portfolio` `#risk` `#backtest` `#p3` `#status/done`

Prerequisites: Phases 01–12 (full runtime kernel, orchestration, RAG, governance).

---

## Goal

A domain-specific agent layer for financial markets: real-time market data ingestion,
signal generation, portfolio management, risk controls, and backtesting. The trading agent
runs as a specialized agent role on top of the existing runtime — it does not replace the
general agent, it extends it.

---

## Architecture

```
TUI / API
  └── Engine.Submit()
        └── Selector → RoleTrading
              └── TradingAgent
                    ├── MarketDataTool  (quotes, OHLCV, news)
                    ├── SignalEngine    (technical indicators)
                    ├── PortfolioTool   (positions, P&L)
                    ├── RiskEngine      (limits, drawdown checks)
                    └── BrokerTool      (order execution — paper/live)
```

---

## Files to create (in order)

### Step 1 — Market data types
**`internal/agents/trading/market.go`**

```go
package trading

type Quote struct {
    Symbol    string
    Bid       float64
    Ask       float64
    Last      float64
    Volume    int64
    Timestamp time.Time
}

type OHLCV struct {
    Symbol    string
    Open      float64
    High      float64
    Low       float64
    Close     float64
    Volume    int64
    Timestamp time.Time
    Interval  string   // "1m", "5m", "1h", "1d"
}

type MarketDataProvider interface {
    Quote(ctx context.Context, symbol string) (Quote, error)
    OHLCV(ctx context.Context, symbol, interval string, limit int) ([]OHLCV, error)
    News(ctx context.Context, symbol string, limit int) ([]NewsItem, error)
}

type NewsItem struct {
    Headline  string
    Source    string
    URL       string
    Sentiment float32  // -1.0 to 1.0
    At        time.Time
}
```

Tags: `#trading/market-data`

---

### Step 2 — Market data providers
**`internal/agents/trading/providers/`**

- `alpaca.go` — Alpaca Markets REST API (stocks, crypto)
- `yfinance.go` — Yahoo Finance HTTP scraper (free, no auth required)
- `mock.go` — deterministic mock for tests

```go
type AlpacaProvider struct {
    apiKey    string
    apiSecret string
    baseURL   string
    client    *http.Client
}

func NewAlpacaProvider(key, secret, baseURL string) *AlpacaProvider
```

Tags: `#trading/providers`

---

### Step 3 — Signal engine
**`internal/agents/trading/signals.go`**

Technical indicators computed from OHLCV data:

```go
type Signal struct {
    Symbol    string
    Indicator string   // "SMA", "EMA", "RSI", "MACD", "BB"
    Value     float64
    Extra     map[string]float64
    At        time.Time
}

// SMA returns simple moving average over period.
func SMA(ohlcv []OHLCV, period int) []float64

// EMA returns exponential moving average.
func EMA(ohlcv []OHLCV, period int) []float64

// RSI returns relative strength index (0–100).
func RSI(ohlcv []OHLCV, period int) []float64

// MACD returns MACD line, signal line, histogram.
func MACD(ohlcv []OHLCV, fast, slow, signal int) ([]float64, []float64, []float64)

// BollingerBands returns upper, middle, lower bands.
func BollingerBands(ohlcv []OHLCV, period int, stdDevMult float64) ([]float64, []float64, []float64)
```

Tags: `#trading/signals`

---

### Step 4 — Portfolio manager
**`internal/agents/trading/portfolio.go`**

```go
type Position struct {
    Symbol   string
    Qty      float64
    AvgPrice float64
    CostBasis float64
}

type Portfolio struct {
    Positions  map[string]Position
    Cash       float64
    Currency   string
    UpdatedAt  time.Time
}

type PortfolioManager interface {
    Snapshot(ctx context.Context) (Portfolio, error)
    PnL(ctx context.Context) (float64, error)
    Allocation(ctx context.Context) map[string]float64   // % per symbol
}
```

Tags: `#trading/portfolio`

---

### Step 5 — Risk engine
**`internal/agents/trading/risk.go`**

Hard limits evaluated before any order is placed:

```go
type RiskLimits struct {
    MaxPositionPct   float64  // max % of portfolio in one symbol
    MaxDrawdownPct   float64  // halt trading if drawdown exceeds this
    MaxOrderValue    float64  // max single order value in currency
    AllowedSymbols   []string // whitelist; nil = all allowed
    BlockedSymbols   []string // blacklist
}

type RiskEngine struct {
    limits RiskLimits
}

type RiskDecision struct {
    Approved bool
    Reason   string
}

func (r *RiskEngine) Evaluate(portfolio Portfolio, order Order) RiskDecision
```

Tags: `#trading/risk`

---

### Step 6 — Order types and broker
**`internal/agents/trading/broker.go`**

```go
type OrderSide string
const (OrderBuy OrderSide = "buy"; OrderSell OrderSide = "sell")

type OrderType string
const (OrderMarket OrderType = "market"; OrderLimit OrderType = "limit")

type Order struct {
    Symbol   string
    Side     OrderSide
    Type     OrderType
    Qty      float64
    LimitPx  float64
}

type OrderResult struct {
    OrderID   string
    FilledQty float64
    FilledPx  float64
    At        time.Time
}

type Broker interface {
    PlaceOrder(ctx context.Context, o Order) (OrderResult, error)
    CancelOrder(ctx context.Context, orderID string) error
    Orders(ctx context.Context) ([]OrderResult, error)
}

// PaperBroker: simulates order execution using last quote price. No real money.
type PaperBroker struct { market MarketDataProvider; portfolio *Portfolio }

// AlpacaBroker: live/paper trading via Alpaca API.
type AlpacaBroker struct { apiKey, apiSecret, baseURL string }
```

Tags: `#trading/broker`

---

### Step 7 — Trading tools (registered with tools.Registry)
**`internal/agents/trading/tools.go`**

```go
// RegisterTradingTools adds trading-specific tools to a registry.
func RegisterTradingTools(reg *tools.Registry, market MarketDataProvider, broker Broker, risk *RiskEngine, portfolio PortfolioManager)
```

Tools exposed to the LLM:
| Tool name | Args | Description |
|---|---|---|
| `get_quote` | `symbol` | Get live quote |
| `get_ohlcv` | `symbol, interval, limit` | Get OHLCV bars |
| `get_signals` | `symbol, indicators[]` | Compute technical signals |
| `get_portfolio` | — | Current positions + P&L |
| `place_order` | `symbol, side, type, qty, limit_px?` | Place order (risk check first) |
| `cancel_order` | `order_id` | Cancel open order |
| `get_news` | `symbol, limit` | Recent news with sentiment |

Tags: `#trading/tools`

---

### Step 8 — Agent role
**`internal/agent/roles/role.go`** — add:

```go
RoleTrading  // financial market agent
```

**`internal/agent/roles/prompts.go`** — add `tradingPrompt`:

```
You are a trading agent. Analyze market data, generate signals, and manage a portfolio.
Always check risk limits before placing orders. Explain your reasoning.
Never place orders above allowed limits. Paper trading mode unless explicitly told otherwise.
Report positions and P&L after each action.
```

Tags: `#trading/role`

---

### Step 9 — Backtester
**`internal/agents/trading/backtest.go`**

```go
type BacktestConfig struct {
    Symbol    string
    Interval  string
    StartDate time.Time
    EndDate   time.Time
    InitialCapital float64
    Strategy  func(ohlcv []OHLCV, portfolio Portfolio) []Order
}

type BacktestResult struct {
    FinalCapital  float64
    TotalReturn   float64
    MaxDrawdown   float64
    SharpeRatio   float64
    WinRate       float64
    Trades        []OrderResult
}

func Backtest(ctx context.Context, cfg BacktestConfig, market MarketDataProvider) (BacktestResult, error)
```

Tags: `#trading/backtest`

---

### Step 10 — Config
**`internal/config/config.go`** — add:

```go
type TradingConfig struct {
    Enabled        bool
    Mode           string   // "paper" | "live"
    Provider       string   // "alpaca" | "yfinance"
    AlpacaKey      string
    AlpacaSecret   string
    AlpacaBaseURL  string
    RiskLimits     trading.RiskLimits
}
```

Tags: `#trading/config`

---

## Tests

**`internal/agents/trading/signals_test.go`**
- `TestSMA` — known input → known output
- `TestRSI` — value stays in [0,100]
- `TestMACD`

**`internal/agents/trading/risk_test.go`**
- `TestRiskBlocksOversizedOrder`
- `TestRiskBlocksBlockedSymbol`
- `TestRiskApprovesSafeOrder`

**`internal/agents/trading/backtest_test.go`**
- `TestBacktestBuyAndHold` — known data, expected return

Tags: `#tests`

---

## Verification

```
go test ./internal/agents/trading/...
```

- "What is AAPL doing today?" → agent calls get_quote + get_ohlcv + get_signals, summarizes
- "Buy 10 shares of TSLA" → risk check runs → paper order placed → portfolio updated
- Order exceeding MaxOrderValue → risk engine blocks → agent explains the limit
- Backtest SMA crossover on AAPL 2024 → returns BacktestResult with Sharpe ratio
