package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// maxProxyRequestBytes limits the size of incoming MCP protocol messages
	// forwarded through the proxy to prevent memory exhaustion.
	maxProxyRequestBytes = 4 << 20 // 4 MiB

	// maxProxyResponseBytes limits the size of MCP server responses read
	// through the proxy. Responses exceeding this are truncated.
	maxProxyResponseBytes = 16 << 20 // 16 MiB
)

// Hop-by-hop headers that must not be forwarded (RFC 7230 §6.1).
var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

// mcpForwardHeaders are the only request headers forwarded to the upstream MCP
// server. This whitelist aligns with agentstack's proxy and prevents leaking
// sensitive headers (cookies, internal auth tokens) to external servers.
var mcpForwardHeaders = []string{
	"Accept",
	"Content-Type",
	"Mcp-Protocol-Version",
	"Mcp-Session-Id",
	"Last-Event-ID",
}

// MCPProxy forwards raw MCP protocol messages to the connector's server.
// This is agentstack's core scaling pattern: the platform acts as an
// authenticated reverse proxy. The client manages its own MCP session;
// the server just injects auth headers and forwards bytes.
//
// This enables:
//   - External MCP clients (Inspector, SDKs) to use connectors directly
//   - Browser-based MCP clients to connect through the proxy
//   - No server-side session state for proxy traffic
func (s *Service) MCPProxy(ctx context.Context, connectorID, userID string, w http.ResponseWriter, r *http.Request) error {
	connector, cb, release, err := s.acquireProxySlot(ctx, connectorID, userID)
	if err != nil {
		return err
	}
	defer release()

	upstreamReq, err := s.buildUpstreamRequest(ctx, w, r, connector.ServerURL)
	if err != nil {
		return err
	}

	resp, err := s.buildHTTPClient(connector).Do(upstreamReq)
	if err != nil {
		cb.RecordFailure()
		return fmt.Errorf("proxying to MCP server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 500 {
		cb.RecordFailure()
	} else {
		cb.RecordSuccess()
	}

	copyResponseHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)
	// Mark that we've started writing the response — prevents the handler
	// from attempting to write a JSON error after streaming has begun.
	w.Header().Set("X-MCP-Proxy-Streamed", "1")

	streamResponseBody(w, resp)
	return nil
}

// acquireProxySlot performs the pre-flight checks (ownership, connector
// status, circuit breaker, per-user + global concurrency) and returns the
// connector, its circuit breaker, and a release func that must be called
// to free the acquired concurrency slots.
func (s *Service) acquireProxySlot(ctx context.Context, connectorID, userID string) (*MCPConnector, *circuitBreaker, func(), error) {
	connector, err := s.getOwnedConnector(ctx, connectorID, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	if connector.Status != ConnectorStatusConnected {
		return nil, nil, nil, ErrConnectorNotConnected
	}

	cb := s.circuits.Get(connector.ServerURL)
	if !cb.Allow() {
		return nil, nil, nil, ErrCircuitOpen
	}

	if err := s.userSessions.Acquire(ctx, userID); err != nil {
		return nil, nil, nil, err
	}
	if err := s.sessions.Acquire(ctx); err != nil {
		s.userSessions.Release(userID)
		return nil, nil, nil, err
	}

	release := func() {
		s.sessions.Release()
		s.userSessions.Release(userID)
	}
	return connector, cb, release, nil
}

// buildUpstreamRequest materialises the outbound request for the upstream MCP
// server. The body is wrapped with MaxBytesReader and only whitelisted headers
// are forwarded (see mcpForwardHeaders) so cookies / internal auth tokens
// don't leak.
func (s *Service) buildUpstreamRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, upstreamURL string) (*http.Request, error) {
	body := http.MaxBytesReader(w, r.Body, maxProxyRequestBytes)
	safeUpstream, err := safeOutboundURL(upstreamURL)
	if err != nil {
		return nil, fmt.Errorf("validating upstream URL: %w", err)
	}
	// Preserve any sub-path after the proxy mount point:
	//   /v1/mcp/connectors/{id}/mcp/some/path → serverURL/some/path
	// The handler strips the prefix before calling this method.
	req, err := http.NewRequestWithContext(ctx, r.Method, safeUpstream.String(), body)
	if err != nil {
		return nil, fmt.Errorf("creating upstream request: %w", err)
	}
	for _, key := range mcpForwardHeaders {
		for _, v := range r.Header.Values(key) {
			req.Header.Add(key, v)
		}
	}
	return req, nil
}

// copyResponseHeaders copies upstream response headers onto the downstream
// ResponseWriter, dropping hop-by-hop headers (RFC 7230 §6.1).
func copyResponseHeaders(w http.ResponseWriter, resp *http.Response) {
	for key, values := range resp.Header {
		if hopByHopHeaders[key] {
			continue
		}
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
}

// streamResponseBody copies the upstream response body. SSE / streaming
// responses are relayed through a flush-aware chunked copy; non-streaming
// bodies are capped at maxProxyResponseBytes to prevent OOM.
func streamResponseBody(w http.ResponseWriter, resp *http.Response) {
	if !isStreamingResponse(resp) {
		_, _ = io.Copy(w, io.LimitReader(resp.Body, maxProxyResponseBytes))
		return
	}
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32*1024) // 32 KiB buffer
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			return
		}
	}
}

// isStreamingResponse checks if the response uses Server-Sent Events,
// which is the streaming format for MCP Streamable HTTP.
func isStreamingResponse(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	return strings.Contains(ct, "text/event-stream")
}
