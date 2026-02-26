package plugindiscovery

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// sessionConnectTimeout limits how long we wait for the MCP handshake
	// to complete when opening a new session.
	sessionConnectTimeout = 30 * time.Second

	// defaultHTTPTimeout is applied to the HTTP client used by the
	// StreamableClientTransport when the caller does not supply one.
	defaultHTTPTimeout = 30 * time.Second
)

// SessionEntry holds a live MCP session along with its lifecycle controls.
type SessionEntry struct {
	Session   *mcp.ClientSession
	Cancel    context.CancelFunc
	ServerURL string
}

// SessionManager maintains a pool of live MCP client sessions, one per connector.
type SessionManager struct {
	mu       sync.RWMutex
	client   *mcp.Client
	sessions map[string]*SessionEntry // connectorID → session
}

// NewSessionManager creates a new session manager with a shared MCP client.
func NewSessionManager() *SessionManager {
	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "schoolwithai",
			Version: "1.0.0",
		},
		&mcp.ClientOptions{},
	)

	return &SessionManager{
		client:   client,
		sessions: make(map[string]*SessionEntry),
	}
}

// Connect establishes a new MCP session for the given connector.
// If an existing session exists, it is closed first.
// httpClient can inject OAuth Bearer tokens via a custom RoundTripper.
func (sm *SessionManager) Connect(ctx context.Context, connectorID, serverURL string, httpClient *http.Client) (*mcp.ClientSession, error) {
	// Close existing session if any.
	sm.Disconnect(connectorID)

	transport := &mcp.StreamableClientTransport{
		Endpoint: serverURL,
	}
	if httpClient != nil {
		transport.HTTPClient = httpClient
	} else {
		transport.HTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	// Create a long-lived context for the session.
	sessionCtx, cancel := context.WithCancel(context.Background())

	// Use a bounded timeout for the initial handshake so we don't block
	// forever if the remote server stalls during initialization.
	connectCtx, connectCancel := context.WithTimeout(sessionCtx, sessionConnectTimeout)
	defer connectCancel()

	session, err := sm.client.Connect(connectCtx, transport, nil)
	if err != nil {
		cancel()
		return nil, err
	}

	sm.mu.Lock()
	sm.sessions[connectorID] = &SessionEntry{
		Session:   session,
		Cancel:    cancel,
		ServerURL: serverURL,
	}
	sm.mu.Unlock()

	return session, nil
}

// Disconnect gracefully closes the MCP session for a connector.
func (sm *SessionManager) Disconnect(connectorID string) {
	sm.mu.Lock()
	entry, ok := sm.sessions[connectorID]
	if ok {
		delete(sm.sessions, connectorID)
	}
	sm.mu.Unlock()

	if ok && entry != nil {
		entry.Session.Close()
		entry.Cancel()
	}
}

// GetSession returns the live session for a connector, or nil if not connected.
func (sm *SessionManager) GetSession(connectorID string) *mcp.ClientSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if entry, ok := sm.sessions[connectorID]; ok {
		return entry.Session
	}
	return nil
}

// IsConnected checks whether a connector has an active session.
func (sm *SessionManager) IsConnected(connectorID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	_, ok := sm.sessions[connectorID]
	return ok
}

// CloseAll gracefully disconnects all active sessions.
func (sm *SessionManager) CloseAll() {
	sm.mu.Lock()
	entries := make(map[string]*SessionEntry, len(sm.sessions))
	for k, v := range sm.sessions {
		entries[k] = v
	}
	sm.sessions = make(map[string]*SessionEntry)
	sm.mu.Unlock()

	for _, entry := range entries {
		entry.Session.Close()
		entry.Cancel()
	}
}
