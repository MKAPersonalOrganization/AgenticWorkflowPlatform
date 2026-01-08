# KurrentDB Event Store Implementation

This package implements the Event Store interface using [KurrentDB](https://kurrent.io), a PostgreSQL-backed event store.

## Architecture

The implementation uses **query-time projections** instead of maintaining in-memory state:
- **No memory overhead** - Thread state is rebuilt from events on each query
- **Always consistent** - No projection lag since we read directly from the event store
- **Simpler architecture** - No need to manage projection updates or state synchronization

## Prerequisites

### 1. Enable the $streams System Projection

The `$streams` projection is required to discover all threads. Enable it via the KurrentDB HTTP API:

```bash
curl -X POST http://localhost:2113/projection/$streams/command/enable \
  -H "Content-Type: application/json" \
  -u admin:changeit
```

### 2. Verify the Projection is Running

```bash
curl http://localhost:2113/projection/$streams \
  -u admin:changeit
```

You should see a response with `"status": "Running"`.

## How It Works

### Thread Discovery

The `ReadAllThreadIDs()` method discovers threads by reading the `$streams` system projection:

1. **Read $streams projection** - Contains metadata for all streams in the database
2. **Parse event data** - Each event has format: `"{version}@{stream-name}"`
   - Example: `"0@thread_1234567890"`
3. **Filter for threads** - Extract stream names starting with "thread"
4. **Return unique IDs** - Deduplicate and return the list

### Why This Approach?

KurrentDB does not provide a "list all streams" API. The only way to discover streams is via:
- **$streams system projection** (used here) - Metadata for all streams
- **$ce-{category} category projections** - Events grouped by stream category
- **Custom index** - Maintain your own stream list (adds complexity)

The `$streams` projection is the most reliable approach since it's maintained by KurrentDB itself.

### Event Data Format

The `$streams` projection stores stream metadata in a specific format:

```
{version}@{stream-name}
```

Examples:
- `0@thread_abc123` - First event (version 0) for thread_abc123
- `5@thread_xyz789` - Stream at version 5

We parse this format using string operations:
```go
dataStr := string(resolvedEvent.Event.Data)
if atIndex := strings.Index(dataStr, "@"); atIndex != -1 {
    streamName := dataStr[atIndex+1:]  // Extract stream name after "@"
}
```

## Limitations

### SDK Limitations

The KurrentDB Go SDK does not expose:
- **Projection management APIs** - Cannot enable/disable projections programmatically
- **Stream listing APIs** - No built-in method to list all streams

These must be done via:
- HTTP API (as shown above for enabling projections)
- Reading system projections (as implemented here)

### Performance Considerations

**Query-time projections** rebuild state on every read. This is suitable when:
- ✅ Event streams are small (hundreds of events, not millions)
- ✅ Read-heavy workloads (caching can be added at HTTP layer)
- ✅ Simplicity is valued over raw performance

For large event streams or high-throughput scenarios, consider:
- In-memory projections with periodic rebuilds
- Caching layer (Redis, etc.)
- Snapshots (store periodic state checkpoints)

## Testing

### Manual Testing

1. Start KurrentDB:
```bash
docker-compose up kurrentdb
```

2. Enable $streams projection (see above)

3. Create a thread and verify persistence:
```bash
# Create a thread via API
curl -X POST http://localhost:8080/api/chat/threads \
  -H "Content-Type: application/json" \
  -d '{"title": "Test Thread"}'

# Restart the server
# Thread should still exist when you query the API
```

### Integration Tests

See [arch_test.go](/arch/arch_test.go) for architecture tests that verify:
- Event sourcing patterns
- Dependency boundaries
- Projection implementations

## References

- [KurrentDB Documentation](https://kurrent.io/docs)
- [KurrentDB Client Go SDK](https://github.com/kurrent-io/KurrentDB-Client-Go)
- [System Projections](https://kurrent.io/docs/projections/system-projections)
