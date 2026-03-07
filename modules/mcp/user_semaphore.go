package mcp

import (
	"context"
	"fmt"
	"sync"
)

// ErrUserConcurrencyLimit is returned when a single user has exhausted their
// per-user session allowance. This prevents one user from monopolizing the
// global semaphore — a critical isolation property at 100K+ users.
var ErrUserConcurrencyLimit = fmt.Errorf("mcp: too many concurrent MCP sessions for this user — try again later")

// userSemaphore tracks per-user concurrency to prevent a single user from
// saturating the global session pool. At 100K users × 10 connectors, without
// per-user caps one user running 10 parallel tool calls could consume 10% of
// the global 100-slot semaphore, starving other users.
//
// Design: map[userID]→count protected by a mutex, checked BEFORE the global
// semaphore. This is O(1) per acquire/release with minimal lock contention
// (the critical section is just a map lookup + increment).
type userSemaphore struct {
	mu         sync.Mutex
	counts     map[string]int
	maxPerUser int
}

// newUserSemaphore creates a per-user concurrency limiter.
func newUserSemaphore(maxPerUser int) *userSemaphore {
	return &userSemaphore{
		counts:     make(map[string]int),
		maxPerUser: maxPerUser,
	}
}

// Acquire reserves a slot for the given user. Returns an error if the user
// has already reached their per-user cap. Context is accepted for interface
// consistency but is not currently used (the check is non-blocking).
func (us *userSemaphore) Acquire(_ context.Context, userID string) error {
	us.mu.Lock()
	defer us.mu.Unlock()

	if us.counts[userID] >= us.maxPerUser {
		return fmt.Errorf("%w (limit: %d)", ErrUserConcurrencyLimit, us.maxPerUser)
	}
	us.counts[userID]++
	return nil
}

// Release returns a slot for the given user. Must be called after Acquire.
func (us *userSemaphore) Release(userID string) {
	us.mu.Lock()
	defer us.mu.Unlock()

	us.counts[userID]--
	if us.counts[userID] <= 0 {
		delete(us.counts, userID) // prevent unbounded map growth
	}
}

// InUse returns the number of slots currently held by a user (for observability).
func (us *userSemaphore) InUse(userID string) int {
	us.mu.Lock()
	defer us.mu.Unlock()
	return us.counts[userID]
}

// ActiveUsers returns the number of users currently holding slots.
func (us *userSemaphore) ActiveUsers() int {
	us.mu.Lock()
	defer us.mu.Unlock()
	return len(us.counts)
}
