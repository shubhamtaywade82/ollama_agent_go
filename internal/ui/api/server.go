// Package api provides an HTTP API server as an alternative to the TUI.
// It exposes the runtime.Engine over HTTP with SSE streaming for agent events.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ollama_agent_go/internal/agent"
	"ollama_agent_go/internal/runtime"
)

// Server is a thin HTTP handler that delegates to the Engine.
type Server struct {
	engine *runtime.Engine
	mux    *http.ServeMux
}

// New creates an API Server and registers all routes.
func New(eng *runtime.Engine) *Server {
	s := &Server{engine: eng}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("POST /v1/sessions", s.createSession)
	s.mux.HandleFunc("POST /v1/sessions/{id}/submit", s.submit)
	s.mux.HandleFunc("DELETE /v1/sessions/{id}", s.clearSession)
	s.mux.HandleFunc("GET /v1/sessions/{id}/audit", s.auditLog)
	s.mux.HandleFunc("GET /v1/health", s.health)
	return s
}

// ServeHTTP satisfies http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server on addr (e.g. ":8080").
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s)
}

// ─── Handlers ────────────────────────────────────────────────────────────────

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "model": s.engine.Model()})
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	if err := s.engine.Init(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"session_id": s.engine.SessionID()})
}

type submitRequest struct {
	Input string `json:"input"`
}

// submit streams agent events as Server-Sent Events (SSE).
func (s *Server) submit(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Input == "" {
		writeError(w, http.StatusBadRequest, "input required")
		return
	}

	// SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, canFlush := w.(http.Flusher)

	emit := func(ev agent.Event) {
		data, _ := json.Marshal(map[string]any{
			"kind": int(ev.Kind),
			"text": ev.Text,
			"tool": ev.Tool,
		})
		fmt.Fprintf(w, "data: %s\n\n", data)
		if canFlush {
			flusher.Flush()
		}
	}

	if err := s.engine.Submit(r.Context(), req.Input, emit); err != nil {
		fmt.Fprintf(w, "data: %s\n\n", jsonError(err.Error()))
		if canFlush {
			flusher.Flush()
		}
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}
}

func (s *Server) clearSession(w http.ResponseWriter, r *http.Request) {
	if err := s.engine.ClearHistory(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"session_id": s.engine.SessionID()})
}

func (s *Server) auditLog(w http.ResponseWriter, r *http.Request) {
	events, err := s.engine.ExportAudit(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func jsonError(msg string) string {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return string(b)
}

// startupBanner returns a startup log line for the API server.
func startupBanner(addr string) string {
	return fmt.Sprintf("[api] listening on %s  started=%s", addr, time.Now().Format(time.RFC3339))
}

func init() {
	_ = strings.ToLower // keep import used
}
