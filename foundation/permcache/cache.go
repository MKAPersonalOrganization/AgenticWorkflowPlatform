package permcache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// DefaultTTL is the default time-to-live for cache entries in Redis.
	// Entries expire automatically; no manual eviction is needed.
	DefaultTTL = 15 * time.Minute

	// keyPrefix namespaces all permission cache keys in Redis.
	keyPrefix = "permcache:"

	// keySep separates the parts of a composite cache key (agentID, resource IDs).
	keySep = "\x00"
)

// Cache is a distributed, generic cache backed by Redis, keyed by userID plus
// a composite resource key (agentID + one or more resource IDs).
//
// Eviction is handled by Redis TTL — entries expire after [DefaultTTL] (or a
// custom TTL) without any application-side bookkeeping. Because the cache is
// shared across all application instances, a write on one pod is immediately
// visible to every other pod.
//
// Values are serialised as JSON. The type parameter V must be
// JSON-marshallable (all current permission types already carry JSON tags).
type Cache[V any] struct {
	rdb    redis.Cmdable
	prefix string // per-cache key namespace, e.g. "permcache:toolperm:"
	ttl    time.Duration
}

// New returns a Redis-backed cache with [DefaultTTL] and the given key prefix.
// The prefix is used to namespace keys so that different cache instances
// (e.g. tool permissions vs workspace permissions) don't collide.
func New[V any](rdb redis.Cmdable, prefix string) *Cache[V] {
	return NewWithTTL[V](rdb, prefix, DefaultTTL)
}

// NewWithTTL returns a Redis-backed cache with a custom TTL.
// If ttl ≤ 0, [DefaultTTL] is used.
func NewWithTTL[V any](rdb redis.Cmdable, prefix string, ttl time.Duration) *Cache[V] {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Cache[V]{
		rdb:    rdb,
		prefix: keyPrefix + prefix + ":",
		ttl:    ttl,
	}
}

// Get looks up a cached value. Returns (value, true) on hit, (nil, false) on miss.
// The returned pointer is freshly allocated — callers may mutate it freely.
func (c *Cache[V]) Get(userID string, resourceKeys ...string) (*V, bool) {
	ctx := context.Background()
	rk := c.redisKey(userID, resourceKeys)

	data, err := c.rdb.Get(ctx, rk).Bytes()
	if err != nil {
		// redis.Nil (miss) or transient error — treat as miss.
		return nil, false
	}

	var v V
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, false
	}

	return &v, true
}

// Set stores a value in the cache with the configured TTL.
func (c *Cache[V]) Set(userID string, value *V, resourceKeys ...string) {
	ctx := context.Background()
	rk := c.redisKey(userID, resourceKeys)

	data, err := json.Marshal(value)
	if err != nil {
		return // best-effort; callers still hit the DB on miss
	}

	_ = c.rdb.Set(ctx, rk, data, c.ttl).Err()
}

// Delete removes a specific entry from the cache.
func (c *Cache[V]) Delete(userID string, resourceKeys ...string) {
	ctx := context.Background()
	rk := c.redisKey(userID, resourceKeys)
	_ = c.rdb.Del(ctx, rk).Err()
}

// redisKey builds the full Redis key for a given user + resource combination.
func (c *Cache[V]) redisKey(userID string, resourceKeys []string) string {
	return fmt.Sprintf("%s%s:%s", c.prefix, userID, joinKeys(resourceKeys))
}

// joinKeys combines resource keys into a single string using a null-byte
// separator (which is invalid in UUIDs and other typical key values).
func joinKeys(keys []string) string {
	return strings.Join(keys, keySep)
}
