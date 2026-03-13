# Production Scalability Bottleneck Report

**Date:** 2026-03-13
**Codebase:** SchoolWithAI
**Scope:** Bottlenecks that will limit scalability to hundreds of thousands of concurrent users

---

## Summary Table

| # | Category | Issue | Severity | Location |
|---|----------|-------|----------|----------|
| 1 | In-memory State | Permission cache with random eviction at 5 000-user threshold | **CRITICAL** | `foundation/permcache/cache.go:73-86` |
| 2 | Concurrency | Unbounded goroutine spawning — one goroutine per SSE subscription | **CRITICAL** | `infra/projection/subscriber/redis_broadcaster.go:86-123` |
| 3 | Streaming | SSE event channel silently drops events when slow clients stall | **HIGH** | `infra/projection/subscriber/redis_broadcaster.go:68, 116-119` |
| 4 | Streaming | Write deadline disabled on SSE connections — slow clients hold slots indefinitely | **HIGH** | `modules/chat/taskmgmt/stream_handler.go:30-35` |
| 5 | In-memory State | Subscriber maps in Redis broadcaster grow unbounded | **HIGH** | `infra/projection/subscriber/redis_broadcaster.go:24-26` |
| 6 | Database | Connection pool size hard-coded to 100 / 100 | **HIGH** | `main.go:154-155` |
| 7 | Database | No statement timeout — single slow query can exhaust all connections | **HIGH** | `infra/sqldb/sqldb.go:71-103` |
| 8 | HTTP Server | No global request-body size limit | **HIGH** | `main.go:739` |
| 9 | In-memory State | Event router handler slices grow without limit; copies on every dispatch | **MEDIUM** | `foundation/event/router.go:18-20, 68-73` |
| 10 | Concurrency | Asynq server started without lifecycle coordination | **MEDIUM** | `modules/chat/taskmanagement/asynq_task_manager.go:134-139` |
| 11 | Database | Projection updater lacks indexed cursor pagination | **MEDIUM** | `infra/projection/subscriber/projection_updater_new.go:96` |
| 12 | Database | No circuit breaker for PostgreSQL | **MEDIUM** | `infra/sqldb/sqldb.go` |
| 13 | External Calls | HTTP clients created per MCP connector with unbounded transport pools | **MEDIUM** | `modules/mcp/service.go:98` |
| 14 | External Calls | No circuit breaker for external calls (MCP, Edgequake) | **MEDIUM** | `modules/mcp/service.go:296-320` |
| 15 | Auth / Session | Cookie-based sessions — no server-side revocation, 4 KB size cap | **MEDIUM** | `infra/googleauth/session.go` |
| 16 | File I/O | Static assets served from disk — no CDN, no compression, no cache headers | **MEDIUM** | `main.go:746-748` |
| 17 | Configuration | Limits hard-coded throughout — cannot be tuned without a code change | **MEDIUM** | Multiple files |
| 18 | HTTP Server | `MaxHeaderBytes` not set — vulnerable to header-bomb exhaustion | **LOW** | `main.go:670-680` |
| 19 | External Calls | MCP probe retries use fixed 2 s delay — thundering-herd on bulk failure | **LOW** | `modules/mcp/service.go:36` |
| 20 | Concurrency | MCP global/per-user session semaphore too conservative | **LOW** | `modules/mcp/service.go:41, 46` |
| 21 | Database | Projection polling interval fixed at 100 ms — cascades under backlog | **LOW** | `infra/projection/subscriber/projection_updater_new.go` |

---

## Detailed Findings

---

### 1. Permission Cache — Random Eviction at Hard-Coded 5 000-User Threshold

**Severity: CRITICAL**
**File:** `foundation/permcache/cache.go:11, 73-86`

```go
const maxUsers = 5000   // line 11

// eviction (line 73-86): when len(cache) >= 5000,
// iterate map and delete half the entries randomly
```

**Problem:**
The cache stores `map[userID]map[resourceID]*V`. When the live user count reaches 5 000, the code iterates the map and deletes half the entries at random. At 100 000 concurrent users, this threshold is hit continuously, causing constant cache invalidation. Because eviction is random, the hottest users are evicted as often as cold ones, so permission lookups fall back to the database on nearly every request.

**Impact at scale:**
- Cache hit rate approaches 0 % beyond 10 000 users.
- Permission lookups hammer PostgreSQL, multiplying DB load by the number of permission checks per request.
- GC pressure from constant map reallocation.

**Fix:**
Replace with an LRU cache (e.g., `github.com/hashicorp/golang-lru`) with a TTL, or offload to Redis.

---

### 2. Unbounded Goroutine Spawning — One Goroutine per SSE Subscription

**Severity: CRITICAL**
**File:** `infra/projection/subscriber/redis_broadcaster.go:86-123`

```go
func (b *RedisBroadcaster) Subscribe(threadID string) (<-chan es.Event, func()) {
    ch := make(chan es.Event, 100)
    go func() {           // ← new goroutine for every subscriber
        pubsub := b.client.Subscribe(ctx, threadID)
        for msg := range pubsub.Channel() { ... }
    }()
    ...
}
```

**Problem:**
Every SSE client connection spawns an independent goroutine that opens a dedicated Redis Pub/Sub connection. There is no goroutine pool or connection multiplexing.

**Impact at scale:**
- 100 000 SSE connections = 100 000 goroutines + 100 000 Redis TCP connections.
- Default goroutine stack is 8 KB (grows to 2–8 MB each under load) → memory exhaustion.
- Redis `maxclients` (default 10 000) is exceeded, causing connection-refused errors for all clients.
- OS file-descriptor limit (default 1 024 per process) breached immediately.

**Fix:**
Use a single Redis Pub/Sub connection per channel multiplexed across all local subscribers. A fan-out broker pattern (one Redis goroutine → N local channels) eliminates the 1:1 relationship.

---

### 3. SSE Event Channel Drops Messages Silently

**Severity: HIGH**
**File:** `infra/projection/subscriber/redis_broadcaster.go:68, 116-119`

```go
ch := make(chan es.Event, 100)   // line 68

// in dispatch loop (lines 116-119):
select {
case sub.ch <- event:
default:
    log.Warn("channel full, dropping event")   // silent drop
}
```

**Problem:**
Each SSE subscriber has a 100-event buffer. If the HTTP write to the browser is slower than the event arrival rate (e.g., on a slow mobile connection), the buffer fills and events are dropped with no recovery mechanism. The client has no way to detect this loss without a sequence number check.

**Impact at scale:**
- A deployment with millions of active streams has a guaranteed percentage of clients losing messages.
- Users see truncated AI responses.
- No replay mechanism means lost events are gone permanently.

**Fix:**
(a) Implement event sequence numbers and a server-side replay buffer; (b) use a larger adaptive buffer; (c) disconnect and let the client reconnect with a `Last-Event-ID` for catch-up.

---

### 4. SSE Write Deadline Disabled — Slow Clients Hold Connection Slots Indefinitely

**Severity: HIGH**
**File:** `modules/chat/taskmgmt/stream_handler.go:30-35`

```go
// Disable write deadline for SSE streaming
if err := conn.SetWriteDeadline(time.Time{}); err != nil { ... }
```

**Problem:**
The server's `WriteTimeout` (set to 60 s in `main.go:673`) is the only guard against hanging connections, and it is explicitly disabled per-connection here. The only hard cap is a 30-minute TTL (`stream_handler.go:92`). A client on a flaky network that has TCP keep-alive disabled will hold a connection slot, goroutine, and Redis subscription for up to 30 minutes while consuming no data.

**Impact at scale:**
- 1 % of 100 000 users on flaky connections = 1 000 zombie connections pinning resources.
- Each zombie holds a goroutine, a Redis pub/sub connection, and a file descriptor.

**Fix:**
Introduce an idle-write timeout (e.g., 90 s). Send a heartbeat comment every 15 s (`data: \n\n`); if the write fails, close the connection immediately.

---

### 5. Redis Broadcaster Subscriber Maps Grow Unbounded

**Severity: HIGH**
**File:** `infra/projection/subscriber/redis_broadcaster.go:24-26`

```go
type RedisBroadcaster struct {
    subscribers map[string][]*redisSubscription   // keyed by threadID
    mu          sync.RWMutex
}
```

**Problem:**
Subscribers are added on connect and removed only when the cancel function returned from `Subscribe()` is called. If the HTTP handler exits abnormally (panic, context cancel without deferred cleanup), the subscription entry leaks permanently.

**Impact at scale:**
- Memory grows monotonically for every leaked connection.
- Map lock contention under 100 000 concurrent subscribe/unsubscribe operations becomes a serialization bottleneck.

**Fix:**
(a) Enforce cleanup via `defer cancel()` consistently in all call sites; (b) add a watchdog goroutine that prunes entries whose context is done; (c) use a concurrent map (e.g., `sync.Map`) to reduce lock contention.

---

### 6. Database Connection Pool Hard-Coded at 100/100

**Severity: HIGH**
**File:** `main.go:154-155`

```go
cfg.DB.MaxIdleConns = 100
cfg.DB.MaxOpenConns = 100
```

**Problem:**
100 connections is an arbitrary constant. Under burst traffic it is the ceiling on parallel query throughput. Under low traffic, 100 idle connections waste PostgreSQL server memory (~10 MB each). Neither extreme is appropriate.

**Impact at scale:**
- Peak of 10 000 RPS with avg query time of 10 ms → needs 100 connections to sustain, which is all of the pool. Any query latency spike causes queue buildup → cascading timeout.
- PostgreSQL `max_connections` (default 100) means a second replica of this service will cause connection-refused errors.

**Fix:**
Make both values configurable via env vars. Set `MaxIdleConns = 2 * runtime.NumCPU()`, `MaxOpenConns = 8 * runtime.NumCPU()` as defaults. Use PgBouncer in transaction-pooling mode for very high connection counts.

---

### 7. No Statement Timeout on Database Queries

**Severity: HIGH**
**File:** `infra/sqldb/sqldb.go:71-103`

**Problem:**
No `statement_timeout` is set either on the connection string or via `SET LOCAL`. A single runaway query (e.g., a missing index on a large table) can hold a connection for minutes, blocking all other requests waiting for a connection from the pool.

**Impact at scale:**
- One bad query at 100 000 users → all 100 connections occupied → all new requests queue → cascade failure.

**Fix:**
Append `statement_timeout=5000` (5 s) to the DSN, or execute `SET LOCAL statement_timeout = 5000` in a query middleware wrapper. Adjust per query type.

---

### 8. No Global Request-Body Size Limit

**Severity: HIGH**
**File:** `main.go:739`

```go
// Only one endpoint gets a body limit, and it is 0 (unlimited):
r.Use(middleware.BodyLimit(0, "/v1/message:send"))
```

**Problem:**
A `BodyLimit` of `0` means unlimited. All other endpoints also have no limit. Any client can POST a multi-gigabyte body, which Go will buffer in memory before the handler sees it.

**Impact at scale:**
- A single large upload exhausts server heap.
- Simultaneous large uploads cause OOM kills.

**Fix:**
Apply a default global limit (e.g., 10 MB) and selectively increase for endpoints that need it. Use `http.MaxBytesReader` in the handler for streaming control.

---

### 9. Event Router Copies Handler Slices on Every Dispatch

**Severity: MEDIUM**
**File:** `foundation/event/router.go:68-73`

```go
anyHandlers := make([]HandlerFunc, len(r.anyHandlers))
copy(anyHandlers, r.anyHandlers)
handlers := make([]HandlerFunc, len(r.handlers[t]))
copy(handlers, r.handlers[t])
nameHandlers := make([]HandlerFunc, len(r.nameHandlers[eventName]))
copy(nameHandlers, r.nameHandlers[eventName])
```

**Problem:**
Three heap allocations per event dispatch to avoid lock-while-calling. At 10 000 events/sec across all active streams this is 30 000 allocs/sec, which shortens GC cycle intervals.

**Impact at scale:**
- GC pauses become measurable (1–10 ms) under high event throughput.
- Latency percentile (p99) degrades significantly.

**Fix:**
Hold an `sync.RWMutex` read lock during iteration (handlers do not mutate the slice), eliminating the copy. Alternatively, use an immutable slice swap pattern on registration only.

---

### 10. Asynq Server Started Without Lifecycle Coordination

**Severity: MEDIUM**
**File:** `modules/chat/taskmanagement/asynq_task_manager.go:134-139`

```go
go func() {
    if err := srv.Run(mux); err != nil { ... }
}()
```

**Problem:**
The Asynq server is launched as a bare goroutine. There is no `errgroup` or `WaitGroup` to block shutdown until in-flight tasks drain. If the process receives `SIGTERM`, the main goroutine exits before Asynq has a chance to complete running tasks.

**Impact at scale:**
- In-flight tasks are silently abandoned on rolling deploys.
- Partial writes to the database leave records in intermediate states.

**Fix:**
Use `errgroup.WithContext` and call `srv.Shutdown()` in the shutdown sequence before the process exits.

---

### 11. Projection Updater Lacks Indexed Cursor Pagination

**Severity: MEDIUM**
**File:** `infra/projection/subscriber/projection_updater_new.go:96`

```go
batchSize: 500
```

**Problem:**
The updater polls for the next 500 events from the event store using a checkpoint offset. If the query does not use a covering index on `(aggregate_id, sequence_number)` or `(global_position)`, it will perform a sequential scan on a table with millions of rows.

**Impact at scale:**
- After a restart (projection rebuild) against a large event store, the catch-up scan can take hours and saturate I/O.
- Polling at 100 ms intervals while behind creates sustained high DB load.

**Fix:**
Ensure a `BTREE` index on the checkpoint column. Consider using PostgreSQL `LISTEN/NOTIFY` instead of polling to eliminate the 100 ms delay.

---

### 12. No Circuit Breaker for PostgreSQL

**Severity: MEDIUM**
**File:** `infra/sqldb/sqldb.go`

**Problem:**
Every request attempts a database connection directly. If PostgreSQL is slow or down, all goroutines block waiting for a connection or timeout, exhausting the pool and blocking the HTTP server.

**Fix:**
Wrap database access with a circuit breaker (e.g., `sony/gobreaker`). When the breaker is open, return a `503` immediately rather than tying up a goroutine for the full timeout duration.

---

### 13. HTTP Client Created per MCP Connector with Unbounded Transport Pools

**Severity: MEDIUM**
**File:** `modules/mcp/service.go:98`

**Problem:**
Each MCP connector creates its own `*http.Client` (cached for 5 minutes with TTL cache). The default `http.Transport` pools up to 100 idle connections per host. With 1 000 MCP connectors registered by users, up to 100 000 idle TCP connections can be held.

**Fix:**
Share a single `*http.Transport` (or a small pool of them) across all MCP HTTP clients. Tune `MaxIdleConnsPerHost` to a low value (e.g., 5).

---

### 14. No Circuit Breaker for External Service Calls

**Severity: MEDIUM**
**File:** `modules/mcp/service.go:296-320`

**Problem:**
Calls to MCP tools and Edgequake proceed synchronously without a circuit breaker. If an external service is slow (e.g., 10 s latency), goroutines pile up waiting, depleting the server's ability to handle other requests.

**Fix:**
Add `sony/gobreaker` or `afex/hystrix-go` around all external HTTP calls. Fast-fail when the breaker is open.

---

### 15. Cookie-Based Session Storage — No Server-Side Revocation

**Severity: MEDIUM**
**File:** `infra/googleauth/session.go`

**Problem:**
Sessions are stored entirely in encrypted cookies. There is no server-side session store, so:
- Revoking a session (e.g., after a security incident) is impossible until the cookie expires.
- Cookies have a hard size limit (~4 KB). Storing OAuth tokens + user data may exceed this, causing silent truncation.
- At 100 000 users, every authenticated request re-validates the cookie HMAC, adding CPU overhead without a cache benefit.

**Fix:**
Use Redis as a session store (e.g., `rbcerveira/gorilla-sessions-redis`). Store only a session ID in the cookie; keep the session data server-side with a configurable TTL.

---

### 16. Static Assets Served from Disk — No CDN, No Compression, No Cache Headers

**Severity: MEDIUM**
**File:** `main.go:746-748`

```go
r.Mount("/assets/", http.StripPrefix("/assets/",
    http.FileServer(http.Dir("./assets"))))
```

**Problem:**
- Every asset request hits disk I/O and network bandwidth of the application server.
- No `Cache-Control` headers: browsers re-fetch assets on every page load.
- No gzip/brotli: JS and CSS files are sent uncompressed.

**Impact at scale:**
- At 100 000 users, static-asset requests dominate and saturate network egress.
- Application server CPU is wasted on file serving instead of business logic.

**Fix:**
(a) Add `Cache-Control: public, immutable, max-age=31536000` for content-hashed assets.
(b) Enable `middleware.Compress()` in Chi router.
(c) Serve assets through a CDN (CloudFront, Cloudflare) in production.

---

### 17. Limits Hard-Coded Throughout — No Runtime Tuning

**Severity: MEDIUM**
**Locations:**

| Constant | Value | File |
|----------|-------|------|
| `maxUsers` | 5 000 | `foundation/permcache/cache.go:11` |
| `maxConcurrentSessions` | 100 | `modules/mcp/service.go:41` |
| `maxSessionsPerUser` | 10 | `modules/mcp/service.go:46` |
| SSE channel buffer | 100 | `infra/projection/subscriber/redis_broadcaster.go:68` |
| Projection batch size | 500 | `infra/projection/subscriber/projection_updater_new.go:96` |
| SSE max TTL | 30 min | `modules/chat/taskmgmt/stream_handler.go:92` |

**Problem:**
Tuning any of these requires a code change, rebuild, and redeployment.

**Fix:**
Read all limits from environment variables with the current values as defaults.

---

### 18. `MaxHeaderBytes` Not Set

**Severity: LOW**
**File:** `main.go:670-680`

**Problem:**
Go's default `MaxHeaderBytes` is 1 MB. A header-bomb attack (sending thousands of large headers) can exhaust memory before the handler is reached.

**Fix:**
Set `MaxHeaderBytes: 8 << 10` (8 KB) explicitly on the `http.Server`.

---

### 19. MCP Probe Retries Use Fixed 2 s Delay — Thundering Herd on Bulk Failure

**Severity: LOW**
**File:** `modules/mcp/service.go:36`

```go
probeRetryDelay = 2 * time.Second
```

**Problem:**
If a batch of MCP connectors all fail simultaneously (e.g., upstream restart), all probes retry at exactly t=2 s, t=4 s, etc., creating synchronized bursts.

**Fix:**
Use exponential backoff with full jitter: `delay = baseDelay * 2^attempt * rand.Float64()`.

---

### 20. MCP Session Semaphore Too Conservative

**Severity: LOW**
**File:** `modules/mcp/service.go:41, 46`

```go
maxConcurrentSessions = 100   // global
maxSessionsPerUser    = 10    // per user
```

**Problem:**
100 global concurrent MCP sessions across all users is extremely limiting at scale. A single power user with 20 active connectors and parallel tool calls is capped at 10, serializing what should be parallel work.

**Fix:**
Increase defaults to `1 000` global and `50` per-user, or make configurable.

---

### 21. Projection Polling at Fixed 100 ms — Cascades Under Backlog

**Severity: LOW**
**File:** `infra/projection/subscriber/projection_updater_new.go`

**Problem:**
The poll interval is fixed. When the projection is behind by many events, each 100 ms poll processes 500 events but the backlog grows faster than it is consumed (if event ingestion rate > 5 000 events/s). Meanwhile, poll overhead is constant regardless of load.

**Fix:**
Implement adaptive polling: if the last batch was full (500 events), poll immediately; if the batch was empty, back off exponentially up to a max interval (e.g., 5 s). This is effectively a busy-wait drain loop.

---

## Immediate Action Plan

Priority order based on impact at 100 000 concurrent users:

1. **[CRITICAL]** Implement goroutine pool for Redis broadcaster — replace 1:1 goroutine-per-subscriber with a fan-out broker pattern.
2. **[CRITICAL]** Replace permission cache eviction with LRU + TTL or Redis.
3. **[HIGH]** Add SSE event sequence numbers and a replay buffer — eliminate silent drops.
4. **[HIGH]** Enforce idle-write timeout on SSE connections with heartbeat pings.
5. **[HIGH]** Add PostgreSQL statement timeout and a circuit breaker.
6. **[HIGH]** Tune and externalize database connection pool settings.
7. **[HIGH]** Set a global HTTP request-body size limit.
8. **[MEDIUM]** Move all hard-coded constants to environment variables.
9. **[MEDIUM]** Add circuit breaker for all external HTTP calls.
10. **[MEDIUM]** Migrate session storage to Redis.
11. **[MEDIUM]** Serve static assets with cache headers, compression, and a CDN.
12. **[MEDIUM]** Coordinate Asynq server lifecycle with graceful shutdown.
