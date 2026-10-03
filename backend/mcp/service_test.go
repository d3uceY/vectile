package mcp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"vectile/backend/config"
	"vectile/backend/services"
)

// initRequest is a minimal MCP initialize call: the first POST a Streamable
// HTTP client makes, so a test using it exercises the real transport rather
// than just the listener.
const initRequest = `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
	`"params":{"protocolVersion":"2025-03-26","capabilities":{},` +
	`"clientInfo":{"name":"vectile-test","version":"1.0"}}}`

func newTestClient() *http.Client { return &http.Client{Timeout: 3 * time.Second} }

// waitForListener blocks until the HTTP server StartServer launched in a
// goroutine is accepting connections. Without it a test that hits the URL
// immediately can race the listener and see ECONNREFUSED.
func waitForListener(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("nothing listening on %s after 3s", addr)
}

// TestStartServerServesSSE boots the MCPService on the SSE transport and checks
// that the SSE endpoint answers with an event-stream. This validates the
// service lifecycle (Start/Status/Stop) and the transport wiring without
// needing a database or an embedding model.
func TestStartServerServesSSE(t *testing.T) {
	const port = 39123
	svc := NewMCPService(&services.Core{})

	url, err := svc.StartServer(port, config.TransportSSE)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	defer svc.StopServer()

	wantURL := fmt.Sprintf("http://127.0.0.1:%d/sse", port)
	if url != wantURL {
		t.Fatalf("url = %q, want %q", url, wantURL)
	}
	st := svc.GetMCPStatus()
	if !st.Running || st.Port != port || st.URL != wantURL || st.Transport != config.TransportSSE {
		t.Fatalf("status = %+v", st)
	}

	waitForListener(t, fmt.Sprintf("127.0.0.1:%d", port))
	resp, err := newTestClient().Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", ct)
	}

	if err := svc.StopServer(); err != nil {
		t.Fatalf("StopServer: %v", err)
	}
	if st := svc.GetMCPStatus(); st.Running {
		t.Fatal("expected stopped after StopServer")
	}
}

// TestStartServerServesStreamableHTTP covers the default transport: the URL is
// /mcp, an initialize POST returns a JSON-RPC result, and the old /sse path is
// gone, so a client can never accidentally talk to the wrong endpoint.
func TestStartServerServesStreamableHTTP(t *testing.T) {
	const port = 39124
	svc := NewMCPService(&services.Core{})

	url, err := svc.StartServer(port, config.TransportStreamableHTTP)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	defer svc.StopServer()

	wantURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	if url != wantURL {
		t.Fatalf("url = %q, want %q", url, wantURL)
	}
	st := svc.GetMCPStatus()
	if !st.Running || st.URL != wantURL || st.Transport != config.TransportStreamableHTTP {
		t.Fatalf("status = %+v", st)
	}

	waitForListener(t, fmt.Sprintf("127.0.0.1:%d", port))
	resp, err := newTestClient().Get(fmt.Sprintf("http://127.0.0.1:%d/sse", port))
	if err != nil {
		t.Fatalf("GET /sse: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /sse on the streamable transport = %d, want 404", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(initRequest))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = newTestClient().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read initialize response: %v", err)
	}
	if !strings.Contains(string(body), "vectile") {
		t.Fatalf("initialize response has no server name: %s", body)
	}
}

// TestStartServerDefaultsToStreamableHTTP proves the default path: a blank or
// unrecognized transport value must not leave the server unreachable.
func TestStartServerDefaultsToStreamableHTTP(t *testing.T) {
	const port = 39125
	svc := NewMCPService(&services.Core{})
	defer svc.StopServer()

	url, err := svc.StartServer(port, "")
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	if want := fmt.Sprintf("http://127.0.0.1:%d/mcp", port); url != want {
		t.Fatalf("url = %q, want %q", url, want)
	}
	if st := svc.GetMCPStatus(); st.Transport != config.TransportStreamableHTTP {
		t.Fatalf("transport = %q, want %q", st.Transport, config.TransportStreamableHTTP)
	}
}

// TestStartServerRejectsTransportChangeWhileRunning guards the guard: switching
// transport means restarting the server, so a live server must refuse rather
// than hand back a URL for an endpoint it does not serve.
func TestStartServerRejectsTransportChangeWhileRunning(t *testing.T) {
	const port = 39126
	svc := NewMCPService(&services.Core{})
	defer svc.StopServer()

	if _, err := svc.StartServer(port, config.TransportSSE); err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	if _, err := svc.StartServer(port, config.TransportStreamableHTTP); err == nil {
		t.Fatal("expected an error when switching transport on a running server")
	}
	if st := svc.GetMCPStatus(); !st.Running || st.Transport != config.TransportSSE {
		t.Fatalf("SSE server should still be running: %+v", st)
	}
	// Restarting on the same transport stays a no-op, as before.
	if url, err := svc.StartServer(port, config.TransportSSE); err != nil || url == "" {
		t.Fatalf("idempotent restart: url=%q err=%v", url, err)
	}
}

// TestStartServerRejectsBadPort guards against a 0 or oversized port silently
// binding an ephemeral socket while reporting a wrong :0/mcp URL.
func TestStartServerRejectsBadPort(t *testing.T) {
	svc := NewMCPService(&services.Core{})
	for _, port := range []int{0, -1, 65536} {
		if _, err := svc.StartServer(port, config.TransportStreamableHTTP); err == nil {
			t.Errorf("StartServer(%d) should have errored", port)
		}
	}
	if st := svc.GetMCPStatus(); st.Running {
		t.Fatal("server should not be running after rejected starts")
	}
}
