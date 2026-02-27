package plugindiscovery

import (
	"context"
	"fmt"
)

// ErrConcurrencyLimit is returned when the per-pod session semaphore is full.
var ErrConcurrencyLimit = fmt.Errorf("mcp: too many concurrent MCP sessions — try again later")

// semaphore is a bounded concurrency limiter backed by a buffered channel.
// It caps the number of simultaneous ephemeral MCP sessions per process,
// preventing a single pod from exhausting file descriptors or memory when
// many users trigger tool calls concurrently.
//
// This mirrors agentstack's approach of bounding DB connections via
// create_async_engine(pool_size=...) — applied here to outbound MCP sessions.
type semaphore struct {
	ch chan struct{}
}

// newSemaphore creates a semaphore with the given capacity.
func newSemaphore(maxConcurrent int) *semaphore {
	return &semaphore{
		ch: make(chan struct{}, maxConcurrent),
	}
}

// Acquire blocks until a slot is available or the context is cancelled.
func (s *semaphore) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", ErrConcurrencyLimit, ctx.Err())
	}
}

// Release returns a slot to the pool. Must be called after Acquire.
func (s *semaphore) Release() {
	<-s.ch
}

// Available returns the number of free slots (for observability).
func (s *semaphore) Available() int {
	return cap(s.ch) - len(s.ch)
}

// Capacity returns the total slot count.
func (s *semaphore) Capacity() int {
	return cap(s.ch)
}
