package permcache

import (
	"strings"
	"sync"
)

const (
	// maxUsers is the maximum number of distinct user entries in a
	// permission cache before random eviction kicks in.
	maxUsers = 5000

	// keySep separates the parts of a composite cache key (agentID, resource IDs).
	keySep = "\x00"
)

// Cache is a thread-safe, generic cache keyed by userID plus a composite
// resource key (agentID + one or more resource IDs).
//
// Eviction is per-user: when the number of distinct user entries reaches
// [maxUsers], half the users are evicted (random order, leveraging Go map
// iteration non-determinism).
//
// All values are stored as pointers so callers can cheaply copy-on-read when
// needed. The cache itself never copies values — that is the caller's
// responsibility.
type Cache[V any] struct {
	mu    sync.RWMutex
	users map[string]map[string]*V // userID → compositeKey → *V
}

// New returns a ready-to-use cache.
func New[V any]() *Cache[V] {
	return &Cache[V]{
		users: make(map[string]map[string]*V),
	}
}

// Get looks up a cached value. Returns (value, true) on hit, (nil, false) on miss.
// The returned pointer is the same one stored in the cache — copy it if you
// intend to expose it to callers.
func (c *Cache[V]) Get(userID string, resourceKeys ...string) (*V, bool) {
	key := joinKeys(resourceKeys)

	c.mu.RLock()
	defer c.mu.RUnlock()

	if userMap, ok := c.users[userID]; ok {
		if v, ok := userMap[key]; ok {
			return v, true
		}
	}
	return nil, false
}

// Set stores a value in the cache, evicting users if the cache is full.
func (c *Cache[V]) Set(userID string, value *V, resourceKeys ...string) {
	key := joinKeys(resourceKeys)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.evictIfNeeded()

	if _, ok := c.users[userID]; !ok {
		c.users[userID] = make(map[string]*V)
	}
	c.users[userID][key] = value
}

// evictIfNeeded removes approximately half the user entries when the cache
// exceeds [maxUsers]. Must be called while holding c.mu (write lock).
func (c *Cache[V]) evictIfNeeded() {
	if len(c.users) < maxUsers {
		return
	}
	half := len(c.users) / 2
	count := 0
	for k := range c.users {
		delete(c.users, k)
		count++
		if count >= half {
			break
		}
	}
}

// joinKeys combines resource keys into a single string using a null-byte
// separator (which is invalid in UUIDs and other typical key values).
func joinKeys(keys []string) string {
	return strings.Join(keys, keySep)
}
