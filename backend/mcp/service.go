package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"vectile/backend/services"
)

// MCPStatus is the live state of the MCP server, mirrored to the frontend.
type MCPStatus struct {
	Running bool   `json:"running"`
	Port    int    `json:"port"`
	URL     string `json:"url"`
}

// MCPService exposes the in-process MCP SSE server to the frontend. The
// server binds to 127.0.0.1 only, so only AI clients on this machine can
// reach it. Method names carry a "Server" suffix so they never collide with
// the Wails service lifecycle hooks (Init/Start/Shutdown).
type MCPService struct {
	core *services.Core

	mu        sync.Mutex
	running   bool
	port      int
	sseServer *server.SSEServer
}

// NewMCPService creates an MCPService bound to the shared core.
func NewMCPService(core *services.Core) *MCPService { return &MCPService{core: core} }

// StartServer begins an SSE server on 127.0.0.1:port and returns the
// connection URL clients should use. Starting an already-running server is a
// no-op that returns the current URL.
func (s *MCPService) StartServer(port int) (string, error) {
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid port %d: use a value in 1..65535", port)
	}

	s.mu.Lock()
	if s.running {
		url := s.url()
		s.mu.Unlock()
		return url, nil
	}

	// Fail fast with a friendly message when the port is taken, instead of
	// letting the HTTP server die silently in the background.
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		s.mu.Unlock()
		return "", fmt.Errorf("port %d is in use: %w", port, err)
	}
	ln.Close()

	sse := server.NewSSEServer(
		CreateServer(s.core),
		server.WithKeepAliveInterval(15*time.Second),
	)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	s.sseServer = sse
	s.port = port
	s.running = true
	s.mu.Unlock()

	go func() {
		if err := sse.Start(addr); err != nil {
			slog.Error("MCP SSE server stopped", "err", err)
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			s.emitStatus()
		}
	}()

	s.emitStatus()
	return s.url(), nil
}

// StopServer shuts down the SSE server with a short grace period.
func (s *MCPService) StopServer() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	sse := s.sseServer
	s.running = false
	s.sseServer = nil
	s.mu.Unlock()

	if sse != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := sse.Shutdown(ctx); err != nil {
			slog.Error("MCP shutdown error", "err", err)
		}
	}
	s.emitStatus()
	return nil
}

// GetMCPStatus returns the current server state.
func (s *MCPService) GetMCPStatus() MCPStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *MCPService) url() string {
	return fmt.Sprintf("http://127.0.0.1:%d/sse", s.port)
}

func (s *MCPService) statusLocked() MCPStatus {
	st := MCPStatus{Running: s.running, Port: s.port}
	if s.running && s.port > 0 {
		st.URL = s.url()
	}
	return st
}

// emitStatus pushes the current state to the frontend via the mcp:status
// event so the Settings section stays live without polling.
func (s *MCPService) emitStatus() {
	if s.core.App == nil {
		return
	}
	s.mu.Lock()
	st := s.statusLocked()
	s.mu.Unlock()
	s.core.App.Event.Emit("mcp:status", st)
}
