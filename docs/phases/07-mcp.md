# Phase 07 — MCP Protocol

Tags: `#mcp` `#protocol` `#tools` `#integration` `#external` `#stdio` `#http` `#p2` `#status/planned`

Prerequisites: Phase 01 (tools.Host, registry), Phase 05 (observability spans).

---

## Goal

Implement the Model Context Protocol so the agent can:
1. **Act as MCP client** — connect to external MCP servers (GitHub, Slack, DBs, filesystem, etc.)
   and expose their tools through `tools.Host` as if they were native tools.
2. **Act as MCP server** — expose our own tools over MCP so other MCP hosts (Claude Desktop, IDE)
   can call them.

Reference: MCP spec at https://spec.modelcontextprotocol.io

---

## Protocol overview

```
MCP Host (us)
  └── MCP Client A ──stdio──► MCP Server (github-mcp-server)  ──► GitHub API
  └── MCP Client B ──http──►  MCP Server (db-server)          ──► Postgres
  └── MCP Client C ──stdio──► MCP Server (filesystem-server)  ──► local fs
```

Message types: `initialize`, `tools/list`, `tools/call`, `resources/list`, `resources/read`,
`prompts/list`, `prompts/get`. Transport: JSON-RPC 2.0 over stdio or HTTP+SSE.

---

## Files to create (in order)

### Step 1 — JSON-RPC 2.0 types
**`internal/mcp/jsonrpc.go`**

```go
package mcp

type Request struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      any             `json:"id"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      any             `json:"id"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

type Notification struct {
    JSONRPC string          `json:"jsonrpc"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}
```

Tags: `#mcp/jsonrpc`

---

### Step 2 — MCP tool types
**`internal/mcp/types.go`**

```go
type ToolDef struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`  // JSON Schema
}

type CallToolRequest struct {
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments"`
}

type CallToolResult struct {
    Content []Content `json:"content"`
    IsError bool      `json:"isError,omitempty"`
}

type Content struct {
    Type string `json:"type"`  // "text" | "image" | "resource"
    Text string `json:"text,omitempty"`
}

type InitializeResult struct {
    ProtocolVersion string       `json:"protocolVersion"`
    Capabilities    Capabilities `json:"capabilities"`
    ServerInfo      ServerInfo   `json:"serverInfo"`
}
```

Tags: `#mcp/types`

---

### Step 3 — Transport interface
**`internal/mcp/transport.go`**

```go
type Transport interface {
    Send(ctx context.Context, msg []byte) error
    Recv(ctx context.Context) ([]byte, error)
    Close() error
}

// StdioTransport: spawns a subprocess and communicates over stdin/stdout.
type StdioTransport struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout *bufio.Scanner
}

func NewStdioTransport(command string, args ...string) (*StdioTransport, error)

// HTTPTransport: HTTP+SSE transport for remote MCP servers.
type HTTPTransport struct {
    baseURL string
    client  *http.Client
}

func NewHTTPTransport(baseURL string) *HTTPTransport
```

Tags: `#mcp/transport`

---

### Step 4 — MCP Client
**`internal/mcp/client.go`**

```go
type Client struct {
    transport Transport
    tools     []ToolDef
    name      string
}

func NewClient(transport Transport) *Client

func (c *Client) Initialize(ctx context.Context) error
func (c *Client) ListTools(ctx context.Context) ([]ToolDef, error)
func (c *Client) CallTool(ctx context.Context, req CallToolRequest) (CallToolResult, error)
func (c *Client) Close() error
```

Tags: `#mcp/client`

---

### Step 5 — MCP tool adapter
**`internal/mcp/tool_adapter.go`**

Wraps an MCP `Client` as a `tools.Tool` so it integrates into `tools.Registry`:

```go
type MCPToolAdapter struct {
    client  *Client
    toolDef ToolDef
}

func (a *MCPToolAdapter) Name() string
func (a *MCPToolAdapter) Description() string
func (a *MCPToolAdapter) Execute(ctx context.Context, args map[string]any) (string, error)

// AdaptAll returns one MCPToolAdapter per tool the MCP server exposes.
func AdaptAll(ctx context.Context, client *Client) ([]tools.Tool, error)
```

Tags: `#mcp/adapter` `#tools/registry`

---

### Step 6 — MCP Server (expose our tools)
**`internal/mcp/server.go`**

```go
type Server struct {
    host    *tools.Host
    name    string
    version string
}

func NewServer(host *tools.Host, name, version string) *Server

// ServeStdio reads JSON-RPC requests from stdin and writes responses to stdout.
func (s *Server) ServeStdio(ctx context.Context) error

// ServeHTTP implements http.Handler for HTTP+SSE transport.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)

// handlers
func (s *Server) handleInitialize(req Request) Response
func (s *Server) handleToolsList(req Request) Response
func (s *Server) handleToolsCall(req Request) Response
```

Tags: `#mcp/server`

---

### Step 7 — MCP config + bootstrap
**`internal/config/config.go`** — add MCP server list:

```go
type MCPServer struct {
    Name      string   `json:"name"`
    Transport string   `json:"transport"`  // "stdio" | "http"
    Command   string   `json:"command"`    // for stdio
    Args      []string `json:"args"`
    URL       string   `json:"url"`        // for http
}

type Config struct {
    // ... existing fields ...
    MCPServers []MCPServer `json:"mcpServers"`
    MCPConfigPath string   // path to mcp.json; default $root/mcp.json
}
```

**`internal/app/bootstrap.go`** — after registry creation:

```go
for _, srv := range cfg.MCPServers {
    client, err := mcp.ConnectServer(ctx, srv)
    if err != nil { obs.Error("mcp connect", err, "server", srv.Name); continue }
    tools, _ := mcp.AdaptAll(ctx, client)
    for _, t := range tools { reg.Register(t) }
}
```

**`mcp.json`** (example, not committed — add to .gitignore):

```json
{
  "mcpServers": [
    {"name": "github", "transport": "stdio", "command": "github-mcp-server", "args": []},
    {"name": "filesystem", "transport": "stdio", "command": "mcp-server-filesystem", "args": ["/tmp"]}
  ]
}
```

Tags: `#mcp/config` `#mcp/bootstrap`

---

### Step 8 — CLI flag to expose as MCP server
**`cmd/ollama_agent/main.go`** — add `--mcp-server` flag:

```go
if cfg.MCPServerMode {
    srv := mcp.NewServer(application.Engine.ToolHost, "ollama-agent", "1.0")
    log.Fatal(srv.ServeStdio(ctx))
}
```

Tags: `#mcp/server/cli`

---

## Tests

**`internal/mcp/client_test.go`**
- `TestClientInitialize` — mock stdio transport, verify handshake
- `TestClientListTools` — verify tool defs deserialized correctly
- `TestClientCallTool` — verify args forwarded, result returned as string

**`internal/mcp/server_test.go`**
- `TestServerToolsList` — returns tools registered in host
- `TestServerToolsCall` — dispatches to host.Execute, returns result

**`internal/mcp/tool_adapter_test.go`**
- `TestAdaptAll` — MCP tool defs become tools.Tool with correct Name/Description

Tags: `#tests`

---

## Verification

```
go test ./internal/mcp/...
```

- Start `github-mcp-server` in stdio mode → agent lists GitHub tools (list_repos, create_issue, etc.)
- Agent calls `create_issue` → approval gate fires → issue created on GitHub
- Run `ollama-agent --mcp-server` → Claude Desktop can call our read_file/write_file tools
