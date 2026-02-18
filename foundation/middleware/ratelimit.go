package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter returns middleware that applies per-IP token-bucket rate
// limiting. Each unique client IP gets its own limiter that permits
// rps requests per second with a burst of burst. Pass 0 for rps to use
// the default of 10 req/s and 0 for burst to use 20.
//
// Stale limiters are evicted every 3 minutes to bound memory.
func RateLimiter(rps float64, burst int) func(http.Handler) http.Handler {
	if rps <= 0 {
		rps = 10
	}
	if burst <= 0 {
		burst = 20
	}

	type entry struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*entry)
	)

	// Background cleanup of stale entries.
	go func() {
		for {
			time.Sleep(3 * time.Minute)
			mu.Lock()
			for ip, e := range clients {
				if time.Since(e.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)

			mu.Lock()
			e, ok := clients[ip]
			if !ok {
				e = &entry{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
				clients[ip] = e
			}
			e.lastSeen = time.Now()
			lim := e.limiter
			mu.Unlock()

			if !lim.Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
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
