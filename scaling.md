# Backend Scaling Analysis for Production

## Executive Summary

This document identifies potential scaling issues in the Go backend (SchoolWithAI) in preparation for production deployment. The system uses **event sourcing with KurrentDB**, **Asynq task queue with Redis**, and **query-time projections** for read models.

### Architecture Overview

```mermaid
graph TB
    Client[HTTP Clients] --> Router[Chi Router]
    Router --> Auth[Auth Middleware]
    Auth --> Handlers[HTTP Handlers]
    Handlers --> Asynq[Asynq Server]
    Handlers --> UIProjection[UI Projection Service]
    
    Asynq --> Redis[(Redis)]
    Asynq --> Orchestration[Orchestration Service]
    Orchestration --> ChatWrite[ChatWrite Service]
    Orchestration --> ThreadStore[LLM Context Projection]
    Orchestration --> WhiteboardProj[Whiteboard Projection]
    
    ChatWrite --> EventStore[(KurrentDB)]
    
    UIProjection --> AGUI[AGUI Projection]
    
    AGUI --> EventStore
    ThreadStore --> EventStore
    WhiteboardProj --> EventStore
    
    ChatWrite --> LLM[OpenAI API]
    
    style EventStore fill:#f9f,stroke:#333
    style Redis fill:#f96,stroke:#333
    style LLM fill:#9cf,stroke:#333
```

---

## Critical Issues (High Priority)

### 1. Query-Time Projection Performance

> [!CAUTION]
> **Issue**: All projections use query-time rebuilding, which reads and replays ALL events from the event store on EVERY query.

**Location**: [llm_context_projection.go](file:///home/mabdelrahman/SchoolWithAI/infra/projection/querytime/llm_context_projection.go)

**Impact**:
- `Get()`, `GetAgentThread()`, `Messages()` methods read entire event stream on each call
- `All()` method iterates through ALL threads and rebuilds each one from scratch
- Performance degrades linearly with event count per thread
- No caching or memoization

**Example**:
```go
// This reads ALL events for a thread on EVERY call
func (p *QueryTimeLLMContextProjection) Get(threadID string) (*domain.Thread, bool) {
    events, err := p.eventStore.ReadStream(ctx, streamName, 0) // Reads from version 0
    // ... rebuilds entire state
}
```

**Scaling Impact**:
- Thread with 1,000 messages = 1,000+ events replayed per query
- 100 concurrent users × 10 threads each = 1,000 full rebuilds
- Database load increases exponentially

**Recommendations**:
1. **Immediate**: Implement materialized projections with incremental updates
2. **Short-term**: Add read-through caching (Redis) with TTL
3. **Medium-term**: Use snapshot events every N events (e.g., every 100 events)
4. **Long-term**: Consider CQRS with separate read database

---

### 2. Database Connection Management

> [!WARNING]
> **Issue**: No connection pooling configuration visible for KurrentDB or PostgreSQL (Kratos).

**Locations**:
- [main.go:96](file:///home/mabdelrahman/SchoolWithAI/main.go#L96) - KurrentDB connection
- [docker-compose.yml:61](file:///home/mabdelrahman/SchoolWithAI/docker-compose.yml#L61) - Postgres max_conns=20

**Current State**:
```go
// main.go - No pool configuration visible
baseEventStore, err := kurrentdb.NewKurrentDBEventStore(cfg.KurrentDB.ConnectionString)
```

**Scaling Impact**:
- Default connection limits may be too low for production
- Connection exhaustion under load
- Slow connection establishment on each request

**Recommendations**:
1. Configure KurrentDB client connection pool (min/max connections)
2. Increase Postgres `max_conns` from 20 to at least 100 for production
3. Set `max_idle_conns` appropriately (typically 25% of max)
4. Add connection health checks and retry logic
5. Monitor connection pool metrics

---

### 3. Event Store Read Amplification

> [!WARNING]
> **Issue**: Multiple projections read the same events independently, causing read amplification.

**Location**: [main.go:189-200](file:///home/mabdelrahman/SchoolWithAI/main.go#L189-L200)

**Current Architecture**:
```go
// Three separate projections, each reading from event store independently
aguiProjection := agui.NewQueryTimeAguiProjection(baseEventStore)
whiteboardProjection := whiteboard.NewQueryTimeWhiteboardProjection(baseEventStore, baseEventStore)
llmContextProjection := querytime.NewQueryTimeLLMContextProjection(baseEventStore, aguiProjection)
```

**Scaling Impact**:
- Same events read 3× for different projections
- Network and I/O overhead multiplied
- Cache thrashing if event store has caching

**Recommendations**:
1. Implement event fanout pattern (single read, multiple projections)
2. Use shared event cache layer (Redis)
3. Consider event streaming (subscriptions) instead of polling
4. Batch projection updates

---

### 4. Asynq Concurrency Configuration

> [!IMPORTANT]
> **Issue**: Fixed concurrency of 5 workers may be insufficient for production load.

**Location**: [main.go:102](file:///home/mabdelrahman/SchoolWithAI/main.go#L102)

**Current Configuration**:
```go
Orchestration struct {
    Concurrency         int           `conf:"default:5"`
    ShutdownTimeout     time.Duration `conf:"default:10s"`
    HealthCheckInterval time.Duration `conf:"default:15s"`
}
```

**Scaling Impact**:
- Only 5 concurrent chat processing jobs
- Queue backlog grows under burst traffic
- User-facing latency increases

**Recommendations**:
1. **Immediate**: Increase to at least 20-50 for production
2. Make concurrency auto-scale based on queue depth
3. Implement priority queues for different task types
4. Add queue depth monitoring and alerting
5. Consider horizontal scaling (multiple Asynq servers)

---

### 5. HTTP Server Timeouts

> [!IMPORTANT]
> **Issue**: Conservative timeout settings may cause premature connection closures for long-running AI operations.

**Location**: [main.go:80-84](file:///home/mabdelrahman/SchoolWithAI/main.go#L80-L84)

**Current Configuration**:
```go
Web struct {
    ReadTimeout       time.Duration `conf:"default:15s"`
    WriteTimeout      time.Duration `conf:"default:15s"`
    IdleTimeout       time.Duration `conf:"default:60s"`
    ReadHeaderTimeout time.Duration `conf:"default:10s"`
}
```

**Scaling Impact**:
- LLM API calls can take 10-30+ seconds
- SSE streaming connections need longer write timeouts
- Premature disconnections cause poor UX

**Recommendations**:
1. Increase `WriteTimeout` to 120s for SSE endpoints
2. Use context-based timeouts per handler
3. Implement separate timeout configurations for different endpoint types
4. Add timeout monitoring and metrics

---

## Moderate Issues (Medium Priority)

### 6. Memory Usage - Event Replay

> [!WARNING]
> **Issue**: Loading all events into memory for projection rebuilding.

**Location**: [llm_context_projection.go:43-72](file:///home/mabdelrahman/SchoolWithAI/infra/projection/querytime/llm_context_projection.go#L43-L72)

**Current Implementation**:
```go
// Loads ALL events into memory
events, err := p.eventStore.ReadStream(ctx, streamName, 0)
for _, event := range events {
    _ = applyEventToThread(agentThread, meta, event)
}
```

**Scaling Impact**:
- Large threads (1000+ events) consume significant memory
- Concurrent requests multiply memory usage
- Risk of OOM under load

**Recommendations**:
1. Implement streaming event replay (iterator pattern)
2. Use event snapshots to reduce replay window
3. Add memory limits and monitoring
4. Consider pagination for large event streams

---

### 7. Redis Single Point of Failure

> [!WARNING]
> **Issue**: Single Redis instance for Asynq with no replication or persistence configuration.

**Location**: [docker-compose.yml:3-15](file:///home/mabdelrahman/SchoolWithAI/docker-compose.yml#L3-L15)

**Current Configuration**:
```yaml
redis:
  image: redis:7-alpine
  ports:
    - "6379:6379"
  volumes:
    - redis-data:/data
  # No replication, no sentinel, no cluster
```

**Scaling Impact**:
- Redis failure = complete task queue outage
- No high availability
- Data loss on crash (depending on persistence settings)

**Recommendations**:
1. **Production**: Use Redis Sentinel or Redis Cluster
2. Configure AOF persistence with `appendfsync everysec`
3. Set up Redis replication (master-replica)
4. Add Redis health checks and failover automation
5. Consider managed Redis (AWS ElastiCache, Redis Cloud)

---

### 8. Static File Serving

> [!NOTE]
> **Issue**: Application server serving static assets instead of CDN.

**Location**: [main.go:299-302](file:///home/mabdelrahman/SchoolWithAI/main.go#L299-L302)

**Current Implementation**:
```go
r.Handle("/assets/css/*", http.StripPrefix("/assets/css/", http.FileServer(http.Dir("./assets/css"))))
r.Handle("/assets/js/*", http.StripPrefix("/assets/js/", http.FileServer(http.Dir("./assets/js"))))
r.Handle("/assets/images/*", http.StripPrefix("/assets/images/", http.FileServer(http.Dir("./assets/images"))))
```

**Scaling Impact**:
- Application server bandwidth wasted on static files
- No caching headers visible
- No compression configuration
- Slower global delivery

**Recommendations**:
1. **Production**: Use CDN (CloudFront, Cloudflare, Fastly)
2. Add cache headers for static assets
3. Enable gzip/brotli compression
4. Use asset versioning/fingerprinting
5. Serve from object storage (S3, GCS)

---

### 9. Missing Rate Limiting

> [!CAUTION]
> **Issue**: No rate limiting middleware visible in the application.

**Location**: [main.go:284-296](file:///home/mabdelrahman/SchoolWithAI/main.go#L284-L296)

**Current Middleware Stack**:
```go
r.Use(middleware.Panics())
r.Use(middleware.Logger(log))
r.Use(middleware.Metrics())
// No rate limiting!
```

**Scaling Impact**:
- Vulnerable to DoS attacks
- No protection against abusive users
- Uncontrolled API costs (OpenAI)
- Resource exhaustion

**Recommendations**:
1. **Immediate**: Add rate limiting middleware (per user, per IP)
2. Implement different limits for authenticated vs. anonymous
3. Rate limit expensive operations (LLM calls) more aggressively
4. Add rate limit headers (X-RateLimit-*)
5. Consider distributed rate limiting (Redis-based)

---

### 10. Thread Deletion Performance

> [!WARNING]
> **Issue**: Thread deletion uses soft delete (tombstone) which doesn't reduce event store size.

**Location**: [kurrentdb_eventstore.go:245-256](file:///home/mabdelrahman/SchoolWithAI/infra/eventstore/kurrentdb/kurrentdb_eventstore.go#L245-L256)

**Current Implementation**:
```go
// DeleteStream sets "Truncate Before" but doesn't actually delete events
func (es *KurrentDBEventStore) DeleteStream(ctx context.Context, streamID string) error {
    // After deletion, reads will return "stream not found" but the stream can be written to again
}
```

**Scaling Impact**:
- Deleted threads still consume storage
- Event store grows indefinitely
- Backup and restore times increase

**Recommendations**:
1. Implement hard delete or archival process
2. Move old/deleted threads to cold storage
3. Set up automated cleanup jobs
4. Monitor event store size growth
5. Implement data retention policies

---

## Infrastructure Concerns

### 11. KurrentDB Cluster Configuration

> [!IMPORTANT]
> **Issue**: Single-node KurrentDB cluster in development configuration.

**Location**: [docker-compose.yml:92-117](file:///home/mabdelrahman/SchoolWithAI/docker-compose.yml#L92-L117)

**Current Configuration**:
```yaml
kurrentdb:
  environment:
    - EVENTSTORE_CLUSTER_SIZE=1
    - EVENTSTORE_INSECURE=true # For development only
```

**Recommendations for Production**:
1. Use cluster size of 3+ nodes for high availability
2. Enable TLS/SSL (`EVENTSTORE_INSECURE=false`)
3. Configure proper authentication
4. Set up monitoring and alerting
5. Use persistent volumes with backups
6. Consider managed EventStoreDB service

---

### 12. Observability Gaps

> [!NOTE]
> **Issue**: Limited production observability configuration.

**Current State**:
- Langfuse tracing is optional (`Enabled: false` by default)
- No distributed tracing (OpenTelemetry)
- No structured error tracking (Sentry)
- Basic metrics only

**Recommendations**:
1. Enable Langfuse in production for LLM observability
2. Add OpenTelemetry for distributed tracing
3. Integrate error tracking (Sentry, Rollbar)
4. Add business metrics (messages/sec, threads/user)
5. Set up alerting (PagerDuty, Opsgenie)
6. Implement health check endpoints with detailed status

---

### 13. Graceful Shutdown

> [!NOTE]
> **Issue**: Shutdown timeout may be too short for long-running tasks.

**Location**: [main.go:84](file:///home/mabdelrahman/SchoolWithAI/main.go#L84)

**Current Configuration**:
```go
ShutdownTimeout   time.Duration `conf:"default:20s"`
```

**Scaling Impact**:
- LLM operations can take 30-60+ seconds
- Tasks may be killed mid-execution
- Partial state updates

**Recommendations**:
1. Increase shutdown timeout to 60-90s
2. Implement graceful task cancellation
3. Add task checkpoint/resume capability
4. Drain queue before shutdown
5. Use health check endpoints for load balancer deregistration

---

## Performance Optimization Opportunities

### 14. Concurrent Event Store Reads

**Issue**: Sequential event reads in `All()` method.

**Location**: [llm_context_projection.go:119-144](file:///home/mabdelrahman/SchoolWithAI/infra/projection/querytime/llm_context_projection.go#L119-L144)

**Current Implementation**:
```go
for _, threadID := range threadIDs {
    if thread, exists := p.Get(threadID); exists {
        threads = append(threads, *thread)
    }
}
```

**Recommendation**: Use goroutines with worker pool to parallelize reads.

---

### 15. HTTP Handler Optimization

**Issue**: Creating new inspector instances per request.

**Location**: [asynq_server.go:192-209](file:///home/mabdelrahman/SchoolWithAI/services/orchestration/asynq_server.go#L192-L209)

**Current Implementation**:
```go
func (s *AsynqServer) GetTaskInfo(queue, taskID string) (*asynq.TaskInfo, error) {
    inspector := asynq.NewInspector(...) // New instance per call
    defer inspector.Close()
}
```

**Recommendation**: Reuse inspector instance with connection pooling.

---

## Recommended Action Plan

### Phase 1: Immediate (Pre-Production)

1. ✅ Increase Asynq concurrency to 20-50
2. ✅ Add rate limiting middleware
3. ✅ Configure Redis persistence (AOF)
4. ✅ Increase HTTP write timeout to 120s
5. ✅ Add connection pool configuration for databases
6. ✅ Enable Langfuse tracing

### Phase 2: Short-term (First Month)

1. ✅ Implement materialized projections with caching
2. ✅ Set up Redis Sentinel/Cluster
3. ✅ Move static assets to CDN
4. ✅ Add comprehensive monitoring and alerting
5. ✅ Implement event snapshots
6. ✅ Add distributed tracing (OpenTelemetry)

### Phase 3: Medium-term (3-6 Months)

1. ✅ Horizontal scaling for Asynq workers
2. ✅ KurrentDB cluster setup (3+ nodes)
3. ✅ Implement data archival/retention policies
4. ✅ Add auto-scaling based on metrics
5. ✅ Performance testing and optimization
6. ✅ Consider separate read database (CQRS)

---

## Monitoring Checklist

### Key Metrics to Track

- **Event Store**:
  - Events written/sec
  - Events read/sec
  - Stream sizes (p50, p95, p99)
  - Read latency
  - Connection pool usage

- **Asynq**:
  - Queue depth
  - Task processing time (p50, p95, p99)
  - Task failure rate
  - Worker utilization
  - Redis connection pool

- **HTTP**:
  - Request rate
  - Response time (p50, p95, p99)
  - Error rate (4xx, 5xx)
  - Concurrent connections
  - Timeout rate

- **Application**:
  - Memory usage
  - CPU usage
  - Goroutine count
  - GC pause time
  - LLM API latency

- **Business**:
  - Active users
  - Messages per second
  - Threads per user
  - LLM token usage
  - Cost per request

---

## Load Testing Recommendations

Before production deployment, conduct load tests for:

1. **Baseline**: 100 concurrent users, 10 messages/min each
2. **Peak**: 500 concurrent users, 20 messages/min each
3. **Burst**: 1000 concurrent users for 5 minutes
4. **Sustained**: 200 concurrent users for 24 hours
5. **Failure**: Test with Redis/KurrentDB failures

**Tools**: k6, Gatling, or Locust

---

## Conclusion

The current architecture is well-designed for development but requires significant hardening for production scale. The **most critical issues** are:

1. **Query-time projection performance** - Will not scale beyond 100s of active threads
2. **Database connection management** - Risk of connection exhaustion
3. **Asynq concurrency** - Current limit of 5 workers is too low
4. **Missing rate limiting** - Vulnerable to abuse and cost overruns

Addressing these issues in the recommended phases will ensure a smooth production launch and sustainable scaling path.
