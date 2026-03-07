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
	connector, err := s.getOwnedConnector(ctx, connectorID, userID)
	if err != nil {
		return err
	}
	if connector.Status != ConnectorStatusConnected {
		return ErrConnectorNotConnected
	}

	// Check circuit breaker before proxying.
	cb := s.circuits.Get(connector.ServerURL)
	if !cb.Allow() {
		return ErrCircuitOpen
	}

	// Per-user concurrency cap.
	if err := s.userSessions.Acquire(ctx, userID); err != nil {
		return err
	}
	defer s.userSessions.Release(userID)

	// Acquire global concurrency slot.
	if err := s.sessions.Acquire(ctx); err != nil {
		return err
	}
	defer s.sessions.Release()

	// Build upstream request.
	upstreamURL := connector.ServerURL
	// Preserve any sub-path after the proxy mount point.
	// e.g., /v1/mcp/connectors/{id}/mcp/some/path → serverURL/some/path
	// The handler strips the prefix before calling this method.

	body := http.MaxBytesReader(w, r.Body, maxProxyRequestBytes)
	upstreamReq, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL, body)
	if err != nil {
		return fmt.Errorf("creating upstream request: %w", err)
	}

	// Forward only MCP-relevant headers (whitelist pattern from agentstack).
	// This prevents leaking cookies, internal auth tokens, and other
	// request headers to external MCP servers.
	for _, key := range mcpForwardHeaders {
		if values := r.Header.Values(key); len(values) > 0 {
			for _, v := range values {
				upstreamReq.Header.Add(key, v)
			}
		}
	}

	// Inject the connector's OAuth token (if available).
	httpClient := s.buildHTTPClient(connector)

	resp, err := httpClient.Do(upstreamReq)
	if err != nil {
		cb.RecordFailure()
		return fmt.Errorf("proxying to MCP server: %w", err)
	}
	defer resp.Body.Close()

	// Record success/failure for circuit breaker.
	if resp.StatusCode >= 500 {
		cb.RecordFailure()
	} else {
		cb.RecordSuccess()
	}

	// Copy response headers.
	for key, values := range resp.Header {
		if hopByHopHeaders[key] {
			continue
		}
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}

	w.WriteHeader(resp.StatusCode)

	// Mark that we've started writing the response — prevents the handler
	// from attempting to write a JSON error after streaming has begun.
	w.Header().Set("X-MCP-Proxy-Streamed", "1")

	// Stream the response body.
	// For SSE/streaming responses, we copy without buffering.
	if isStreamingResponse(resp) {
		// Flush-aware copy for SSE streams.
		flusher, ok := w.(http.Flusher)
		buf := make([]byte, 32*1024) // 32 KiB buffer
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				_, writeErr := w.Write(buf[:n])
				if writeErr != nil {
					break
				}
				if ok {
					flusher.Flush()
				}
			}
			if readErr != nil {
				break
			}
		}
	} else {
		// Non-streaming: enforce size limit to prevent OOM.
		io.Copy(w, io.LimitReader(resp.Body, maxProxyResponseBytes))
	}

	return nil
}

// isStreamingResponse checks if the response uses Server-Sent Events,
// which is the streaming format for MCP Streamable HTTP.
func isStreamingResponse(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	return strings.Contains(ct, "text/event-stream")
}
