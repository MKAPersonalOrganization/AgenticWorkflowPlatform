# ADR 017: SSE Resilience — Redis Streams for Cross-Pod Event Replay

## Status

Accepted

## Context

The chat module streams AI-generated responses to the browser using Server-Sent Events (SSE). Each SSE connection subscribes to a `RedisBroadcaster` that fans out domain events from a shared Redis Pub/Sub channel. Prior to this ADR, the broadcaster had three related problems that would prevent the system from scaling to hundreds of thousands of concurrent users.

### Problem 1: Silent event drops on slow clients

Each subscriber held a `chan es.Event` with a buffer of 100 entries. When the channel was full the broadcaster executed the `default:` branch of a `select` — logging a warning and discarding the event permanently. The client received no indication that data was lost.

For AI token streaming this is severe: `AssistantContentStreamed` events (one per LLM output token) are **transient** — they are never persisted to the PostgreSQL event store (see ADR 007). A dropped `AssistantContentStreamed` event is gone forever; the user sees a gap in the streamed response with no recovery path.

### Problem 2: No replay on reconnect

The existing `onerror` handler in `chat-streaming.js` reconnected the `EventSource` with the same URL and no position information. The server created a fresh subscription starting from the live position. Any events published while the client was disconnected — or any events dropped due to Problem 1 — were irretrievably lost.

### Problem 3: In-memory state is single-pod

The first candidate fix considered was an **in-memory ring buffer** per thread inside the broadcaster process. This would work on a single machine but fails in a horizontally-scaled deployment: after a disconnect the load balancer may route the reconnecting client to a different pod, which has no ring buffer for that thread. The ring buffer approach was therefore rejected for any deployment targeting hundreds of thousands of users.

### Why not replace Pub/Sub with Redis Streams entirely?

Redis Streams support blocking reads (`XREAD BLOCK`) which can replace Pub/Sub for live delivery. However, Pub/Sub pushes messages to all subscribers simultaneously with a single server-side fan-out. With Streams each subscriber would need its own read cursor and periodic polling or blocking read, which is significantly less efficient for high-fanout workloads. Pub/Sub is retained for live delivery; Streams are added only for the replay path.

### Why not block the broadcaster on a slow client?

A third option was to replace the `default:` drop with a short `time.After` deadline (e.g. 50 ms) to give slow clients a grace window. This was rejected because the broadcaster has a single shared `readLoop` goroutine that fans out to **all** subscribers on all threads. Blocking for 50 ms on one slow subscriber on thread A delays every fast subscriber on threads B, C, and D. At scale this serialises the entire broadcast path.

## Decision

### 1. Redis Streams as a cross-pod replay buffer (Option A)

On every `Broadcast` call the broadcaster now performs two Redis operations:

1. **`XADD thread:{threadID}:stream MAXLEN ~ 2000 * type {eventType} data {payload}`**
   This appends the event to a per-thread Redis Stream capped at approximately 2 000 entries. Redis returns a globally unique, monotonically increasing entry ID in the format `{milliseconds}-{sequence}` (e.g. `1717000000000-0`). This ID is used as the SSE `id:` field.

2. **`PUBLISH thread:{threadID}:events {envelope}`**
   The pub/sub envelope now embeds the Redis Stream entry ID (`stream_id` field) so that live subscribers receive the same identifier without a second round-trip.

The Redis Stream carries a 2-hour TTL refreshed on every `XADD`. After a job completes and all clients disconnect, the stream expires automatically.

On reconnect the server calls `XREAD COUNT 2000 STREAMS thread:{id}:stream {lastEventId}` (non-blocking, 5 s timeout) to fetch missed events before resuming live delivery. Because the stream lives in Redis, any pod can serve the replay.

### 2. Disconnect slow clients instead of dropping events

When a subscriber's channel is full the broadcaster closes the channel and removes the subscriber from the fan-out map. The stream handler detects the closed channel (`ok == false`) and returns, ending the HTTP response. The browser's `onerror` handler fires and schedules a reconnect with exponential backoff.

On reconnect the client passes `?lastEventId={lastSeqId}`, the server replays the missed events from the Redis Stream, and streaming resumes. The user experiences a brief pause but no data loss.

### 3. Subscribe-first, replay, deduplicate

The stream handler follows this ordering to eliminate the race condition between historical replay and live events:

```
1. Subscribe() — start buffering live events in the channel now
2. XREAD from lastEventId — fetch missed events from Redis Stream
3. Send replayed events with id: {seqID}; track maxReplayedSeqID
4. Enter live loop — skip env.SeqID <= maxReplayedSeqID; send rest
```

Subscribing before replaying ensures that any events published between the start of the XREAD and the moment the handler begins reading from the live channel are held in the 100-entry channel buffer and are not lost. The deduplication step (step 4) prevents `TEXT_MESSAGE_CONTENT` delta events from being sent twice.

### 4. `BroadcastEnvelope` in `foundation/eventsourcing`

The new `BroadcastEnvelope{SeqID string; Event Event}` type is defined in `foundation/eventsourcing` alongside `EventEnvelope`. This placement satisfies the dependency rule established in ADR 016:

```
foundation/eventsourcing   (defines BroadcastEnvelope)
      ↑ imports
infra/projection/subscriber  (implements broadcaster)
      ↑ imports (via main.go wiring)
modules/chat/projection/agui (defines AguiBroadcaster interface)
```

Neither `infra/` nor `modules/` import each other; both depend only on `foundation/`. The `AguiBroadcaster` interface uses `es.BroadcastEnvelope` from `foundation/eventsourcing`, which both the interface definition package and the implementation package already import.

### 5. Client tracks `lastSeqId` per job

The `EventSource` specification requires the browser to send a `Last-Event-ID` request header automatically on its built-in reconnect. The existing client code however calls `eventSource.close()` then `new EventSource(url)` in the `onerror` handler — a manual reconnect that creates a fresh object and does not carry forward any `Last-Event-ID` state.

The fix stores the last received `event.lastEventId` (a native `MessageEvent` property set from the SSE `id:` line) in a per-job `Map` on the thread state object. On the next `__connectToJobStream` call the stored value is appended to the URL as `?lastEventId=`. The server reads this query parameter rather than the `Last-Event-ID` request header. This approach is compatible with the existing manual-reconnect pattern without requiring changes to the reconnect scheduling or backoff logic.

The `jobSeqIdMap` is cleared on `RUN_FINISHED` and `RUN_ERROR` so that a fresh job on the same thread does not accidentally resume from a previous job's position.

## Consequences

### Positive

- **No data loss on reconnect.** All events that passed through the broadcaster within the last 2 hours (up to 2 000 per thread) are replayable from any pod.
- **Scales horizontally.** The replay state lives entirely in the existing Redis instance. Adding pods does not require any coordination or shared memory.
- **Bounded memory.** The `MAXLEN ~ 2000` cap on XADD ensures each thread's replay buffer cannot grow beyond ~2 000 entries (~600 KB worst case). Total Redis memory for the replay layer is proportional to the number of active threads, not the number of pods.
- **No change to the live-delivery hot path.** Pub/Sub fan-out is unchanged. The XADD is a single additional Redis command appended to the existing PUBLISH call; latency impact is ~0.1–0.5 ms.
- **Transparent to the user.** On reconnect the user sees a brief pause (governed by the client's backoff schedule: 1 s, 2 s, 4 s, …, max 16 s) and then streaming resumes from the exact point of disconnection.

### Negative

- **One extra Redis command per broadcast.** Every `Broadcast` call now makes two Redis calls (XADD + PUBLISH) instead of one. At 100 tokens/s × 10 000 concurrent active streams this is 1 000 000 Redis ops/s — within normal Redis throughput limits but worth monitoring.
- **XADD failure is non-fatal but silent.** If the XADD fails (Redis overloaded, network partition), the event is still delivered live via Pub/Sub but is absent from the replay buffer. A client that disconnects during such a window will miss that event even after reconnecting. A warning is logged but no alert is raised. Production deployments should alert on sustained XADD error rates.
- **Replay is best-effort for the last 2 000 events.** A client that was disconnected for an entire long-running job producing more than 2 000 events (very unlikely for an LLM response; 2 000 tokens ≈ ~1 500 words) will not receive the full history on reconnect. In practice the 2 000-entry cap covers the entire output of any single AI run many times over.
- **Redis Stream keys do not expire immediately on job completion.** The 2-hour TTL is refreshed on every broadcast. For a thread that is still receiving events (e.g. a follow-up message), the stream for the previous run persists and grows. Because `MAXLEN ~ 2000` caps the total size per thread (not per run), older events are evicted naturally as new ones arrive. The thread key is not segmented by job ID.

## Alternatives Rejected

| Alternative | Reason rejected |
|---|---|
| In-memory ring buffer per thread | Single-pod only; reconnect to a different pod loses all buffered events |
| Blocking broadcaster with timeout | Blocks the single shared `readLoop`; one slow client delays all subscribers on all threads |
| Replace Pub/Sub with Streams entirely | Pub/Sub is more efficient for high-fanout live delivery; Streams per-subscriber polling adds unnecessary latency and load |
| Make `AssistantContentStreamed` durable | Persisting every token to PostgreSQL significantly increases write load and event store size; tokens are streaming artefacts, not business facts (ADR 007 §Transient events) |
| Larger in-memory channel buffer | Does not address the cross-pod replay problem; at 100 K concurrent streams a 2 000-entry buffer per stream would require ~120 GB of heap |

## Files Changed

| File | Change |
|---|---|
| `foundation/eventsourcing/event.go` | Added `BroadcastEnvelope{SeqID string; Event Event}` |
| `modules/chat/projection/agui/agui_projection.go` | `AguiBroadcaster.Subscribe` returns `<-chan es.BroadcastEnvelope`; added `Replay(threadID, afterSeqID string) []es.BroadcastEnvelope` |
| `infra/projection/subscriber/redis_broadcaster.go` | XADD on broadcast; `pubsubEnvelope` embeds `stream_id`; channel type changed to `chan es.BroadcastEnvelope`; disconnect on full instead of drop; added `Replay` via `XREAD` |
| `modules/chat/taskmgmt/stream_handler.go` | Subscribe-first; replay on `?lastEventId`; SeqID deduplication; `id:` field in SSE output |
| `modules/chat/taskmgmt/helpers.go` | `WriteSSEEvent` accepts `seqID string`; added `streamIDLE` comparator for Redis Stream IDs |
| `assets/js/chat/chat-streaming.js` | Tracks `lastSeqId` per job in `jobSeqIdMap`; passes `?lastEventId=` on reconnect |
| `assets/js/chat/chat-event-handlers.js` | Clears `jobSeqIdMap` on `RUN_FINISHED` and `RUN_ERROR` |
