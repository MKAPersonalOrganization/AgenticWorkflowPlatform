package mcp

import (
	"fmt"
	"sync"
	"time"
)

// Circuit breaker states following the standard pattern:
//
//	closed  → (failures ≥ threshold) → open
//	open    → (recovery timeout elapsed) → half-open
//	half-open → success → closed
//	half-open → failure → open
type circuitState int

const (
	circuitClosed   circuitState = iota // Normal: requests flow through
	circuitOpen                         // Tripped: requests rejected immediately
	circuitHalfOpen                     // Testing: one request allowed to probe
)

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = fmt.Errorf("mcp: circuit breaker open — remote server is unreachable")

// circuitBreaker tracks failures for a single MCP connector and trips open
// after a configurable threshold to prevent thundering-herd against failing
// servers. Modelled after agentstack's probe → disconnect(permanent) pattern
// but expressed as a proper circuit breaker state machine.
type circuitBreaker struct {
	mu               sync.Mutex
	state            circuitState
	failureCount     int
	lastFailureTime  time.Time
	lastSuccessTime  time.Time
	failureThreshold int           // failures before tripping open
	recoveryTimeout  time.Duration // how long to wait before half-open
}

// newCircuitBreaker creates a breaker with the given thresholds.
func newCircuitBreaker(failureThreshold int, recoveryTimeout time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:            circuitClosed,
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
	}
}

// Allow checks whether a request should be permitted.
// Returns true if the circuit is closed or has transitioned to half-open.
func (cb *circuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return true
	case circuitOpen:
		// Check if recovery timeout has elapsed → transition to half-open.
		if time.Since(cb.lastFailureTime) >= cb.recoveryTimeout {
			cb.state = circuitHalfOpen
			return true
		}
		return false
	case circuitHalfOpen:
		// Only one probe at a time — already allowed one.
		return true
	}
	return false
}

// RecordSuccess resets the breaker to closed state.
func (cb *circuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = circuitClosed
	cb.failureCount = 0
	cb.lastSuccessTime = time.Now()
}

// RecordFailure increments the failure counter and may trip the breaker open.
func (cb *circuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case circuitClosed:
		if cb.failureCount >= cb.failureThreshold {
			cb.state = circuitOpen
		}
	case circuitHalfOpen:
		// Probe failed — back to open.
		cb.state = circuitOpen
	}
}

// State returns the current state (for observability / logging).
func (cb *circuitBreaker) State() circuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// String returns the human-readable state name.
func (s circuitState) String() string {
	switch s {
	case circuitClosed:
		return "closed"
	case circuitOpen:
		return "open"
	case circuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// --------------------------------------------------------------------------
// Circuit breaker registry — one breaker per connector URL
// --------------------------------------------------------------------------

// circuitRegistry manages per-URL circuit breakers. Keyed by server URL
// (not connector ID) so multiple connectors pointing at the same server
// share a single breaker — this is the right granularity because the failure
// domain is the remote server, not the user's connector record.
type circuitRegistry struct {
	mu               sync.Mutex
	breakers         map[string]*circuitBreaker
	failureThreshold int
	recoveryTimeout  time.Duration
}

// newCircuitRegistry creates a registry with shared configuration.
func newCircuitRegistry(failureThreshold int, recoveryTimeout time.Duration) *circuitRegistry {
	return &circuitRegistry{
		breakers:         make(map[string]*circuitBreaker),
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
	}
}

// Get returns (or creates) the circuit breaker for a server URL.
func (r *circuitRegistry) Get(serverURL string) *circuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	cb, ok := r.breakers[serverURL]
	if !ok {
		cb = newCircuitBreaker(r.failureThreshold, r.recoveryTimeout)
		r.breakers[serverURL] = cb
	}
	return cb
}

// Reset removes the breaker for a URL (e.g. on connector delete).
func (r *circuitRegistry) Reset(serverURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.breakers, serverURL)
}
