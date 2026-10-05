package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"vectile/backend/config"
	"vectile/backend/services"
)

// MCPStatus is the live state of the MCP server, mirrored to the frontend.
type MCPStatus struct {
	Running   bool   `json:"running"`
	Port      int    `json:"port"`
	URL       string `json:"url"`
	Transport string `json:"transport"`
}

// transportServer is the shared lifecycle of mcp-go's two HTTP transports.
// Both *server.SSEServer and *server.StreamableHTTPServer satisfy it, so the
// service can hold whichever one the settings asked for.
type transportServer interface {
	Start(addr string) error
	Shutdown(ctx context.Context) error
}

// MCPService exposes the in-process MCP server to the frontend. The server
// binds to 127.0.0.1 only, so only AI clients on this machine can reach it.
// Method names carry a "Server" suffix so they never collide with the Wails
// service lifecycle hooks (Init/Start/Shutdown).
type MCPService struct {
	core *services.Core

	mu        sync.Mutex
	running   bool
	port      int
	transport string
	srv       transportServer
}

// NewMCPService creates an MCPService bound to the shared core.
func NewMCPService(core *services.Core) *MCPService { return &MCPService{core: core} }

// endpointPath is where clients reach each transport.
func endpointPath(transport string) string {
	if transport == config.TransportSSE {
		return "/sse"
	}
	return "/mcp"
}

// StartServer begins an MCP server on 127.0.0.1:port and returns the connection
// URL clients should use. The transport is "streamable-http" (the default) or
// "sse"; any other value falls back to streamable HTTP. Starting an
// already-running server is a no-op that returns the current URL, unless the
// requested transport differs from the live one.
func (s *MCPService) StartServer(port int, transport string) (string, error) {
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid port %d: use a value in 1..65535", port)
	}
	transport = config.NormalizeTransport(transport)

	s.mu.Lock()
	if s.running {
		if s.transport != transport {
			live := s.transport
			s.mu.Unlock()
			return "", fmt.Errorf("MCP server is already running on the %s transport; stop it first", live)
		}
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

	mcpServer := CreateServer(s.core)
	var srv transportServer
	if transport == config.TransportSSE {
		srv = server.NewSSEServer(
			mcpServer,
			server.WithKeepAliveInterval(15*time.Second),
		)
	} else {
		// The streamable server serves /mcp by default, matching endpointPath.
		//
		// Do not set a heartbeat interval here. mcp-go raises heartbeat pings as
		// JSON-RPC *requests* with empty result objects, and VS Code's MCP client
		// only routes replies that carry the same id as a request it sent. Every
		// ping therefore surfaces as "Unexpected 200 response for request:",
		// once per interval, forever. Clients keep the connection alive on their
		// own, so the heartbeat buys nothing worth that log spam.
		srv = server.NewStreamableHTTPServer(mcpServer)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	s.srv = srv
	s.transport = transport
	s.port = port
	s.running = true
	s.mu.Unlock()

	go func() {
		if err := srv.Start(addr); err != nil {
			slog.Error("MCP server stopped", "transport", transport, "err", err)
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			s.emitStatus()
		}
	}()

	s.emitStatus()
	return s.url(), nil
}

// StopServer shuts down whichever transport is running, with a short grace
// period.
func (s *MCPService) StopServer() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	srv := s.srv
	s.running = false
	s.srv = nil
	s.mu.Unlock()

	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
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
	return fmt.Sprintf("http://127.0.0.1:%d%s", s.port, endpointPath(s.transport))
}

func (s *MCPService) statusLocked() MCPStatus {
	st := MCPStatus{Running: s.running, Port: s.port, Transport: s.transport}
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
