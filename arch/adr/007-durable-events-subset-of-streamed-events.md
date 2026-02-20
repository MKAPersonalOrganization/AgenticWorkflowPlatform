# ADR 007: Durable Events as a Subset of Streamed Events (Outbox Bypassing)

## Status
Accepted

## Context
The system distinguishes between Ephemeral Events (transient signals like `RunFinished`) and Persistent Events (durable business facts like `ToolNeedsInput`, per ADR 003). 
Previously, Ephemeral events were dispatched immediately to the live Server-Sent Events (SSE) stream via Redis, while Persistent events were recorded in the PostgreSQL Event Store.

A background `ProjectionUpdater` polls the Event Store to update Read Models (Projections) like `agui_projection` and `llm_context_projection` for fast querying. As a side effect, the `ProjectionUpdater` was inadvertently acting as the *only* broadcaster of Persistent events to the Redis SSE stream, creating an unintentional "Outbox" delay.

Because Ephemeral events skipped the database, they consistently completed their journey to the user interface in ~1ms. Durable events, however, were subject to database write latency, projection polling interval delays, and worker queueing (~100ms+ latency). 

This disparity fundamentally broke the live streaming experience for end users. For example, when a `ToolNeedsInput` (Durable) halted an agent loop, the orchestrator immediately fired a `RunFinished` (Ephemeral). The `RunFinished` event would arrive at the frontend instantly, causing the frontend to terminate the SSE connection. The slower `ToolNeedsInput` event would be processed by the Projection Updater much later, entirely missing the live user session. As a result, the user would not see the `ToolNeedsInput` prompt in real-time; they were forced to manually refresh their browser to fetch the updated state from the completed Projection Read Models.

## Decision
**Durable Events are now conceptually treated as a subset of all Streamed Events during initial broadcast.**

1. The Orchestrator will now **synchronously broadcast** ALL events (both Ephemeral and Persistent) to the live streaming bus (Redis -> SSE) immediately upon generation/transaction boundary.
2. Persistent events are still saved to the Event Store synchronously.
3. The background `ProjectionUpdater` continues its primary duty: polling the Event Store to update PostgreSQL Read Models (Projections). The shared router configuration was explicitly removed so it **no longer re-broadcasts** these events to Redis.
4. The client-side architecture (frontend Javascript) relies entirely on the Orchestrator's instantaneous stream for fast, live updates, keeping architecture layers correctly isolated.

## Consequences

**Positive:**
- Complete elimination of the "missing live update" bug reliably delivering durable events within the active SSE stream.
- Zero latency rendering for the UI. The frontend receives every state change instantaneously, regardless of durability requirements. Users no longer need to manually refresh the page to see `ToolNeedsInput` prompts or persistent workflow state changes.
- Clean separation of concerns: The `ProjectionUpdater` builds read models without being simultaneously burdened as a critical-path messaging bottleneck.

**Negative:**
- The Orchestrator's instantaneous stream is now the sole source of truth for the live SSE connection. If the connection drops or the redis message is lost, the client must depend on traditional reconnection polling to fetch the up-to-date Projection Read Models.

## Constraints
1. Frontend event handlers MUST be strictly idempotent.
2. The Orchestrator MUST properly broadcast to Redis upon transaction boundaries. Missed broadcasts rely solely on the frontend recovering state via its native retry/refresh policies against Read Models.

## Related ADRs
- ADR 003: Event Classification - Ephemeral vs. Persistent
