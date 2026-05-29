package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

// Transport is the low-level message-passing layer for MCP.
type Transport interface {
	Send(ctx context.Context, msg []byte) error
	Recv(ctx context.Context) ([]byte, error)
	Close() error
}

// ─── stdio transport ─────────────────────────────────────────────────────────

// StdioTransport spawns a subprocess and communicates over its stdin/stdout
// using newline-delimited JSON (one JSON object per line).
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
}

// NewStdioTransport starts command with args and returns a connected transport.
func NewStdioTransport(command string, args ...string) (*StdioTransport, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdio stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdio stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp stdio start: %w", err)
	}
	return &StdioTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}, nil
}

func (t *StdioTransport) Send(_ context.Context, msg []byte) error {
	line := append(msg, '\n')
	_, err := t.stdin.Write(line)
	return err
}

func (t *StdioTransport) Recv(_ context.Context) ([]byte, error) {
	if !t.stdout.Scan() {
		if err := t.stdout.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	return t.stdout.Bytes(), nil
}

func (t *StdioTransport) Close() error {
	_ = t.stdin.Close()
	return t.cmd.Wait()
}

// ─── HTTP transport ───────────────────────────────────────────────────────────

// HTTPTransport sends MCP requests over HTTP POST and receives JSON responses.
// SSE streams are not yet implemented; this covers request-response only.
type HTTPTransport struct {
	baseURL string
	client  *http.Client
}

// NewHTTPTransport creates an HTTP transport targeting baseURL.
func NewHTTPTransport(baseURL string) *HTTPTransport {
	return &HTTPTransport{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{},
	}
}

func (t *HTTPTransport) Send(ctx context.Context, msg []byte) error {
	// HTTP transport stores the last sent message for Recv to use.
	// We implement a simple send-then-receive cycle here.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/rpc", bytes.NewReader(msg))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (t *HTTPTransport) Recv(_ context.Context) ([]byte, error) {
	return nil, fmt.Errorf("HTTPTransport.Recv: use SendRecv for HTTP")
}

// SendRecv sends msg and returns the parsed response body in a single HTTP round-trip.
func (t *HTTPTransport) SendRecv(ctx context.Context, msg []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/rpc", bytes.NewReader(msg))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (t *HTTPTransport) Close() error { return nil }

// ─── in-process pair transport (for testing) ─────────────────────────────────

// InProcTransport is a pair of connected in-process transports for tests.
type InProcTransport struct {
	send chan []byte
	recv chan []byte
}

// NewInProcPair returns a client-side and server-side transport that are connected.
func NewInProcPair() (*InProcTransport, *InProcTransport) {
	a := make(chan []byte, 16)
	b := make(chan []byte, 16)
	return &InProcTransport{send: a, recv: b}, &InProcTransport{send: b, recv: a}
}

func (t *InProcTransport) Send(_ context.Context, msg []byte) error {
	cp := make([]byte, len(msg))
	copy(cp, msg)
	t.send <- cp
	return nil
}

func (t *InProcTransport) Recv(ctx context.Context) ([]byte, error) {
	select {
	case msg := <-t.recv:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *InProcTransport) Close() error { return nil }

// ─── helpers ─────────────────────────────────────────────────────────────────

func marshalRequest(id any, method string, params any) ([]byte, error) {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	return json.Marshal(Request{JSONRPC: "2.0", ID: id, Method: method, Params: raw})
}
