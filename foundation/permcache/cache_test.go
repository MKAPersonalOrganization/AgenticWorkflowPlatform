package permcache

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func testRedis(t *testing.T) (*miniredis.Miniredis, goredis.Cmdable) {
	t.Helper()
	s := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: s.Addr()})
	return s, rdb
}

// ---------------------------------------------------------------------------
// Basic Get / Set
// ---------------------------------------------------------------------------

func TestGetMiss(t *testing.T) {
	t.Parallel()

	_, rdb := testRedis(t)
	c := New[string](rdb, "test")
	v, ok := c.Get("user1", "agent1", "res1")
	if ok || v != nil {
		t.Fatalf("expected miss, got ok=%v val=%v", ok, v)
	}
}

func TestSetAndGet(t *testing.T) {
	t.Parallel()

	_, rdb := testRedis(t)
	c := New[string](rdb, "test")
	val := "hello"
	c.Set("user1", &val, "agent1", "res1")

	got, ok := c.Get("user1", "agent1", "res1")
	if !ok {
		t.Fatal("expected hit")
	}
	if *got != val {
		t.Fatalf("got %v, want %v", *got, val)
	}
}

func TestOverwrite(t *testing.T) {
	t.Parallel()

	_, rdb := testRedis(t)
	c := New[int](rdb, "test")
	v1 := 1
	c.Set("u1", &v1, "a", "r")
	v2 := 2
	c.Set("u1", &v2, "a", "r")

	got, ok := c.Get("u1", "a", "r")
	if !ok || *got != 2 {
		t.Fatalf("expected 2, got ok=%v val=%v", ok, got)
	}
}

func TestMultipleKeysPerUser(t *testing.T) {
	t.Parallel()

	_, rdb := testRedis(t)
	c := New[int](rdb, "test")
	v10 := 10
	v20 := 20
	c.Set("u1", &v10, "a1")
	c.Set("u1", &v20, "a2")

	g1, _ := c.Get("u1", "a1")
	g2, _ := c.Get("u1", "a2")
	if *g1 != 10 || *g2 != 20 {
		t.Fatalf("multi-key: got %d, %d", *g1, *g2)
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	t.Parallel()

	_, rdb := testRedis(t)
	c := New[int](rdb, "test")
	v := 42
	c.Set("u1", &v, "k")

	c.Delete("u1", "k")

	_, ok := c.Get("u1", "k")
	if ok {
		t.Fatal("expected miss after delete")
	}
}

// ---------------------------------------------------------------------------
// TTL expiry
// ---------------------------------------------------------------------------

func TestTTLExpiry(t *testing.T) {
	t.Parallel()

	s, rdb := testRedis(t)
	c := NewWithTTL[int](rdb, "test", 5*time.Second)
	v := 99
	c.Set("u1", &v, "k")

	// Verify it's there.
	got, ok := c.Get("u1", "k")
	if !ok || *got != 99 {
		t.Fatal("expected hit before expiry")
	}

	// Fast-forward past the TTL.
	s.FastForward(6 * time.Second)

	_, ok = c.Get("u1", "k")
	if ok {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestNonPositiveTTLUsesDefault(t *testing.T) {
	t.Parallel()

	_, rdb := testRedis(t)
	c := NewWithTTL[int](rdb, "test", 0)
	if c.ttl != DefaultTTL {
		t.Fatalf("ttl = %v, want %v", c.ttl, DefaultTTL)
	}

	c2 := NewWithTTL[int](rdb, "test", -5*time.Second)
	if c2.ttl != DefaultTTL {
		t.Fatalf("ttl = %v, want %v", c2.ttl, DefaultTTL)
	}
}

// ---------------------------------------------------------------------------
// Key isolation between cache instances
// ---------------------------------------------------------------------------

func TestPrefixIsolation(t *testing.T) {
	t.Parallel()

	_, rdb := testRedis(t)
	c1 := New[int](rdb, "cache1")
	c2 := New[int](rdb, "cache2")

	v1 := 111
	c1.Set("u1", &v1, "k")

	// c2 should not see c1's data even for the same user+key.
	_, ok := c2.Get("u1", "k")
	if ok {
		t.Fatal("expected miss on c2 — different prefix")
	}

	// c1 should still see its own data.
	got, ok := c1.Get("u1", "k")
	if !ok || *got != 111 {
		t.Fatal("expected hit on c1")
	}
}

// ---------------------------------------------------------------------------
// Struct serialisation round-trip (mirrors real permission types)
// ---------------------------------------------------------------------------

type samplePerm struct {
	UserID  string `json:"user_id"`
	Enabled bool   `json:"enabled"`
}

func TestStructRoundTrip(t *testing.T) {
	t.Parallel()

	_, rdb := testRedis(t)
	c := New[samplePerm](rdb, "perm")

	p := &samplePerm{UserID: "u1", Enabled: true}
	c.Set("u1", p, "agent", "res")

	got, ok := c.Get("u1", "agent", "res")
	if !ok {
		t.Fatal("expected hit")
	}
	if got.UserID != "u1" || !got.Enabled {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
