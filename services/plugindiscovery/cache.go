package plugindiscovery

import (
	"sync"
	"time"
)

// ttlCache is a simple in-memory cache with per-entry TTL expiry.
// It is safe for concurrent use. Modelled after agentstack's @alru_cache(ttl=...)
// pattern — purely TTL-based, no explicit invalidation needed.
type ttlCache[T any] struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry[T]
	ttl     time.Duration
}

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

// newTTLCache creates a cache where entries expire after the given duration.
func newTTLCache[T any](ttl time.Duration) *ttlCache[T] {
	return &ttlCache[T]{
		entries: make(map[string]cacheEntry[T]),
		ttl:     ttl,
	}
}

// Get retrieves a cached value. Returns the value and true if found and not
// expired, or the zero value and false otherwise.
func (c *ttlCache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		var zero T
		if ok {
			// Lazy eviction of expired entries.
			c.mu.Lock()
			if e, still := c.entries[key]; still && time.Now().After(e.expiresAt) {
				delete(c.entries, key)
			}
			c.mu.Unlock()
		}
		return zero, false
	}
	return entry.value, true
}

// Set stores a value in the cache with the configured TTL.
func (c *ttlCache[T]) Set(key string, value T) {
	c.mu.Lock()
	c.entries[key] = cacheEntry[T]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

// Len returns the number of entries (including expired ones not yet evicted).
func (c *ttlCache[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
