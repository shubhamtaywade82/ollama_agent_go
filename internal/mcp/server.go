package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"ollama_agent_go/internal/tools"
)

// Server exposes tools.Host over MCP so external hosts can call our tools.
type Server struct {
	host    *tools.Host
	name    string
	version string
}

// NewServer creates an MCP server that wraps host.
func NewServer(host *tools.Host, name, version string) *Server {
	return &Server{host: host, name: name, version: version}
}

// ServeStdio reads newline-delimited JSON-RPC from stdin and writes to stdout.
// Blocks until ctx is cancelled or stdin closes.
func (s *Server) ServeStdio(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()

		// Peek: might be a notification (no ID).
		var peek map[string]json.RawMessage
		if err := json.Unmarshal(line, &peek); err != nil {
			continue
		}
		if _, hasID := peek["id"]; !hasID {
			// Notification — no response needed.
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(errResponse(nil, CodeParseError, err.Error()))
			continue
		}
		resp := s.dispatch(ctx, req)
		_ = enc.Encode(resp)
	}
	return scanner.Err()
}

// ServeHTTP implements http.Handler for single-path HTTP transport.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	resp := s.dispatch(r.Context(), req)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) dispatch(ctx context.Context, req Request) Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		return errResponse(req.ID, CodeMethodNotFound, fmt.Sprintf("method %q not found", req.Method))
	}
}

func (s *Server) handleInitialize(req Request) Response {
	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    Capabilities{Tools: &ToolsCapability{}},
		ServerInfo:      ServerInfo{Name: s.name, Version: s.version},
	}
	resp, _ := okResponse(req.ID, result)
	return resp
}

func (s *Server) handleToolsList(req Request) Response {
	specs := s.host.Specs()
	defs := make([]ToolDef, 0, len(specs))
	for _, sp := range specs {
		schema := map[string]any{
			"type":       "object",
			"properties": sp.Function.Parameters,
		}
		b, _ := json.Marshal(schema)
		defs = append(defs, ToolDef{
			Name:        sp.Function.Name,
			Description: sp.Function.Description,
			InputSchema: b,
		})
	}
	resp, _ := okResponse(req.ID, ToolsListResult{Tools: defs})
	return resp
}

func (s *Server) handleToolsCall(ctx context.Context, req Request) Response {
	var params CallToolRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errResponse(req.ID, CodeInvalidParams, err.Error())
	}
	out, err := s.host.Execute(ctx, params.Name, params.Arguments)
	if err != nil {
		resp, _ := okResponse(req.ID, CallToolResult{
			Content: []Content{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
		return resp
	}
	resp, _ := okResponse(req.ID, CallToolResult{
		Content: []Content{{Type: "text", Text: out}},
	})
	return resp
}
