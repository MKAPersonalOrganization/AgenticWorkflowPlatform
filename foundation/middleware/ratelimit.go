package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/go-redis/redis_rate/v10"
	goredis "github.com/redis/go-redis/v9"
)

// RateLimiter returns middleware that applies per-IP sliding-window rate
// limiting backed by Redis. All pods behind a load balancer share the
// same counters, so an attacker cannot multiply the limit by the number
// of replicas.
//
// rps is the sustained requests-per-second allowed per IP.
// burst is the maximum number of requests allowed in a single burst.
//
// If Redis is unreachable the middleware fails open (allows the request)
// and logs a warning so that a Redis hiccup does not take down the site.
func RateLimiter(rdb *goredis.Client, rps int, burst int) func(http.Handler) http.Handler {
	if rps <= 0 {
		rps = 10
	}
	if burst <= 0 {
		burst = 20
	}

	limiter := redis_rate.NewLimiter(rdb)
	limit := redis_rate.Limit{
		Rate:   rps,
		Burst:  burst,
		Period: 1_000_000_000, // 1 second in nanoseconds (time.Second)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			key := "ratelimit:" + ip

			res, err := limiter.Allow(r.Context(), key, limit)
			if err != nil {
				// Fail open — allow the request but log the error.
				slog.Warn("rate limiter redis error, failing open",
					"ip", ip, "error", err)
				next.ServeHTTP(w, r)
				return
			}

			if res.Allowed == 0 {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiterHealthCheck pings Redis and returns an error if unreachable.
// Useful for wiring into /readiness probes.
func RateLimiterHealthCheck(ctx context.Context, rdb *goredis.Client) error {
	return rdb.Ping(ctx).Err()
}

// clientIP extracts the client IP from the request, preferring
// X-Forwarded-For when present (common behind reverse proxies).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First entry is the original client.
		if comma := indexOf(xff, ','); comma > 0 {
			xff = xff[:comma]
		}
		return xff
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func indexOf(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
