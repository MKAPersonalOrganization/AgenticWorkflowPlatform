package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

// newTestRedis starts a miniredis server and returns a connected client.
// The server is stopped automatically when the test ends.
func newTestRedis(t *testing.T) *goredis.Client {
	t.Helper()
	s := miniredis.RunT(t)
	return goredis.NewClient(&goredis.Options{Addr: s.Addr()})
}

func TestRateLimiterAllowsWithinLimit(t *testing.T) {
	t.Parallel()

	rdb := newTestRedis(t)
	// 10 req/s with burst of 5
	handler := RateLimiter(rdb, 10, 5)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 5 requests should succeed (burst).
	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

func TestRateLimiterBlocks(t *testing.T) {
	t.Parallel()

	rdb := newTestRedis(t)
	// 1 req/s with burst of 1 — second request should be blocked.
	handler := RateLimiter(rdb, 1, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "5.6.7.8:5678"

	// First request consumes the single token.
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec1.Code)
	}

	// Second request should be rate-limited.
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", rec2.Code)
	}
}

func TestRateLimiterDifferentIPsGetOwnBuckets(t *testing.T) {
	t.Parallel()

	rdb := newTestRedis(t)
	handler := RateLimiter(rdb, 1, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First IP uses its token.
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "10.0.0.1:1111"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("IP1 first request: expected 200, got %d", rec1.Code)
	}

	// Second IP should still have its own token.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.2:2222"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("IP2 first request: expected 200, got %d", rec2.Code)
	}
}

func TestRateLimiterUsesXForwardedFor(t *testing.T) {
	t.Parallel()

	rdb := newTestRedis(t)
	handler := RateLimiter(rdb, 1, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Two requests from same X-Forwarded-For, different RemoteAddr.
	makeReq := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "127.0.0.1:9999" // proxy address
		r.Header.Set("X-Forwarded-For", "203.0.113.42, 10.0.0.1")
		return r
	}

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, makeReq())
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, makeReq())
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request from same X-Forwarded-For: expected 429, got %d", rec2.Code)
	}
}
