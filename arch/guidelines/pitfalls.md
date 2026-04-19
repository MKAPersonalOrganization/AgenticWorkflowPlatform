# Pitfalls — Traps That Cost Real Time

Traps I've walked into before. Each entry describes the symptom, why it's easy to miss, and the fix. **Read this before debugging anything that smells architectural** — there's a good chance the answer is here.

---

## 1. Event deserializer registry silently drops unknown types

**Symptom.** After adding a new domain event, the orchestrator's retry loop for `TokenUsageReported` (or any event immediately following the new one) exhausts its 10 retries with `concurrency conflict: stream version mismatch` — even though no other writer exists. `event_streams.stream_version` in the DB disagrees with the aggregate's `GetVersion()`.

**Why it's invisible.** `PostgresEventStore.ReadStream` in `infra/eventstore/postgres/postgres_eventstore.go` does this in its scan loop:

```go
event, err := DeserializeDomainEvent(s.registry, eventType, data)
if err != nil {
    // Skip unknown event types (forward compatibility)
    continue
}
```

The "forward compatibility" comment masks a trap — any event whose type name isn't registered is silently omitted from the replay. `LoadFromHistory` then increments `Version` only for the events it *sees*, so the aggregate ends up at N-1 when the DB is at N. Every subsequent `Save(expected=N-1)` fails on the `stream_version = $expected` check.

**Fix.** Add the new event to `modules/chat/thread/events_registry.go` in the same commit that introduces the event type:

```go
r.Register("YourNewEvent", func() es.Event { return &YourNewEvent{} })
```

**Future-proofing.** If you add an event and tests start failing with concurrency conflicts **only** on the event that comes right after yours in the persistence sequence — check the registry first. Don't look at advisory locks, retry loops, or concurrent writers; it's the registry.

---

## 2. Two `threadmgmt.Manager` instances on the same stream → phantom conflicts

**Symptom.** A new codepath that builds its own `agentorchestrator.Service` hits stream-version mismatches on events that should serialize cleanly. `retryUpdate` reloads fresh each iteration and still conflicts.

**Why it's invisible.** `agentorchestrator.New` constructs its *own* `threadmgmt.Manager` backed by its *own* `ThreadRepository` unless you pass one in. If the runner (the `adkgo` adapter) was constructed with a different `threadmgmt.Manager`, you now have two `ThreadRepository` instances. PostgreSQL advisory locks serialize the *appends*, but each repository caches an aggregate version snapshot in memory. Between runner-persisted events and orchestrator-persisted events, the two caches diverge and one of them saves with a stale expected version.

**Fix.** Inject the runner's manager into the orchestrator via `WithThreadManagement(mgr)` (added to `modules/chat/agentorchestrator/service.go`). Never let a caller hold two `threadmgmt.Manager` instances over the same stream.

**Related.** `chat.go` builds the production orchestrator and `main.go` wires the runner using `chatModule.Orchestrator().GetThreadManagement()` — respecting that pattern keeps you out of this trap.

---

## 3. Custom AG-UI events hide their name in `value`, not `type`

**Symptom.** SSE assertions that filter on `type == "CODE_EXECUTION_RAN"` (or `TOOL_START`, `TOKEN_USAGE_REPORTED`, `THREAD_TITLE_UPDATED`) find nothing. Events are clearly arriving.

**Why it's invisible.** AG-UI's `CustomEvent` serializes as `{"type": "CUSTOM", "name": "CODE_EXECUTION_RAN", "value": {...}}`. The routable identifier is the `name` field. Our `modules/chat/handlers/common/converter.go` uses `NewCustomEvent` for every domain event that doesn't have a first-class AG-UI type — so virtually all of our domain events ride under `type: "CUSTOM"`.

**Fix.** Route by `sseEvent.Name()` (returns `name` when `type == "CUSTOM"`, else `type`). Extract the payload via `sseEvent.Value(&dst)` — the fields you want are nested under `value`, not at the top level. Both helpers live in `userrequirements/shared_test.go`.

**Events that DO use their own top-level type** (not CUSTOM): `TEXT_MESSAGE_START`, `TEXT_MESSAGE_CONTENT`, `TEXT_MESSAGE_END`, `RUN_STARTED`, `RUN_FINISHED`, `RUN_ERROR`, `THINKING_*`, `STATE_DELTA`. Everything else the chat stream emits is CUSTOM — `TOOL_START`, `TOKEN_USAGE_REPORTED`, `AGENT_TRANSFER_STARTED`, `CODE_EXECUTION_RAN`, `THREAD_TITLE_UPDATED`, `STATE_CHANGE_REPORTED`, etc.

---

## 4. `AssistantMessageAdded` / `UserMessageAdded` / `ThreadCreated` do NOT cross the SSE wire

**Symptom.** You assert the SSE stream contains `AssistantMessageAdded` after a run — the frame never arrives. But `/api/agui/threads/{id}/messages` shows the assistant message fine.

**Why it's invisible.** `modules/chat/handlers/common/converter.go` explicitly returns an empty slice for these three events:

```go
case *thread.UserMessageAdded, *thread.AssistantMessageAdded, *thread.ThreadCreated:
    // These are handled by the frontend locally or via snapshots, no need to stream
    return []any{}
```

The frontend tracks assistant text via `TEXT_MESSAGE_START / TEXT_MESSAGE_CONTENT / TEXT_MESSAGE_END` deltas; the "added" event is implicit in message completion. For history reads, the projection-backed API serves them.

**Fix.** For text assertions, accumulate `TEXT_MESSAGE_CONTENT.delta` (see `assistantFinalText` helper). For history-survival assertions, `GET /api/agui/threads/{id}/messages`. Never assert on `AssistantMessageAdded` via SSE — it's not there by design.

---

## 5. AI-settings `PUT` returns a *generated* UUID even on `ON CONFLICT`

**Symptom.** Test writes settings via `PUT /api/ai-settings`, grabs the `id` from the response, calls `PUT /api/ai-settings/{id}/default` — gets 404 "ai settings not found".

**Why it's invisible.** `postgresAISettingsRepository.Upsert` in `modules/aisettings/repo.go` generates `settings.ID = uuid.New().String()` at the *start* of the function. The `INSERT ... ON CONFLICT (user_id, provider) DO UPDATE ...` runs either path, but the UPDATE branch ignores the newly-generated ID and keeps the row's original UUID. The handler then echoes back the (useless) generated UUID.

**Fix.** Clear all settings first, then create. On the clean path, the auto-"first-row-is-default" logic in `service.saveSettings` activates, and no explicit default call is needed. See `upsertAISettings` + `deleteAllAISettings` in `userrequirements/shared_test.go`.

**Better fix** (not yet done): rewrite `Upsert` to `RETURNING id` and have the handler use that. Filed as future cleanup.

---

## 6. The AG-UI projection is eventually consistent — don't assert the sidecar immediately after SSE closes

**Symptom.** A test completes a run, sees `CODE_EXECUTION_RAN` on the SSE stream, immediately calls `GET /api/agui/threads/{id}/messages`, and the `codeExecutions` sidecar is empty or missing.

**Why it's invisible.** The AG-UI projection is updated by a background `ProjectionSubscriber` loop that polls the event store. There's a small gap between "durable event persisted" and "projection catches up". The SSE stream is driven separately (synchronous broadcast from the orchestrator, not via the projection), so closing the SSE doesn't guarantee projection freshness.

**Fix.** Poll with a short timeout. `waitForCodeExecutionSidecar(t, client, threadID, 5*time.Second)` in `code_execution_test.go` is the canonical pattern — checks every 200ms until the sidecar appears or the deadline passes.

---

## 7. `task down:cluster` wipes volumes — never do it casually

**Symptom.** After `task down:cluster && task up:cluster`, migrations fail with checksum mismatches, tests fail with missing fixtures, and the debug user has to be re-upserted.

**Why it's invisible.** Compose removes named volumes on `down -v`, which clears Postgres, MinIO, and Redis state.

**Fix.** To ship a new binary: just `task up:cluster`. It does `docker compose build --no-cache app` then `up -d`, which recreates the app containers in place without touching volumes. `down:cluster` is only for cases where you genuinely want a clean slate (migration checksum is borked, for example).

---

## 8. Debug user lives forever under a shared ID across test runs

**Symptom.** Memory-policy tests behave oddly on repeated runs — false positives on recall, or the "disabled memory" agent surprisingly finds old facts.

**Why it's invisible.** `/auth/dev-login` upserts a user row with a stable ID (`debug-integration-user`). The memory service indexes sessions per-user. Every test run piles new sessions into that user's memory. Old markers from previous runs are still retrievable.

**Fix.** Use a UUID suffix on every memory marker: `fact := "papaya-violet-" + uuid.NewString()[:8]`. Never assert on generic markers like "teal" that could collide with a previous run.

---

## 9. Workflow composition requires creating sub-agents FIRST

**Symptom.** `POST /v1/agent-definitions` for a workflow root succeeds, but the run fails because the leaves can't be resolved.

**Why it's invisible.** The `graph.children[].agent_id` fields must be real `user_agents.id` UUIDs, not display names. If you create the parent first and reference names, the ID lookup at runtime fails silently (or not so silently).

**Fix.** Create leaves first, capture the returned `id` from each response, then create the root referencing those IDs:

```go
producerID := createAgent(t, client, producerBody)
consumerID := createAgent(t, client, consumerBody)
workflowBody["graph"] = map[string]any{
    "id":   "root",
    "type": "sequential",
    "children": []map[string]any{
        {"id": "n1", "type": "agent", "agent_id": producerID},
        {"id": "n2", "type": "agent", "agent_id": consumerID},
    },
}
```

---

## 10. `subAgents[]` vs `agentTools[]` — different patterns, not interchangeable

**Symptom.** You want a "parent agent that delegates to a specialist" and write it as `subAgents`, but the specialist is never invoked — the parent just answers directly.

**Why it's invisible.** Two distinct ADK patterns, both expressed as JSON fields on the parent:

- `subAgents[]` → **Coordinator pattern**. The parent can *transfer* control to a sub-agent (ADK's `transfer_to_agent` tool). Children are siblings; the parent's own LLM decides when to hand off.
- `agentTools[]` → **Agent-as-tool pattern**. The sub-agent is exposed as a callable tool with a declared input schema. The parent's LLM calls it like any other function tool.

For most "LLM → call this specialist → get answer back" cases, `agentTools` is what you want. For "agent transfers conversation to a specialist and doesn't come back", `subAgents`.

**Fix.** Think about whether you want control transfer (subAgents) or a function-style callable (agentTools). The integration test `TestACoordinatorCanCallASubAgentAsATool` uses the latter despite the name — it asserts a `TOOL_START` frame with the sub-agent's name, which is what agentTools produces.

---

## 11. JSON field casing is mixed — top-level camelCase, nested snake_case

**Symptom.** You send `{"agentId": ...}` inside a `subAgents` entry, the server accepts the request, but at runtime the sub-agent isn't found.

**Why it's invisible.** `createRequest` at the top level uses camelCase tags (`subAgents`, `agentTools`, `modelConfig`). The nested ref types (`SubAgentRef`, `AgentToolRef`, `CompositionNode`) were originally designed for YAML ingestion and use snake_case tags (`agent_id`, `input_schema`, `output_key`). JSON unmarshals silently skip unrecognized fields — a `agentId` at the nested level is a no-op.

**Fix.** Match the tag exactly. The integration tests have correct examples for every shape (see `createAgent` calls in `userrequirements/*_test.go`).

---

## 12. SSE default scanner buffer chokes on large CUSTOM event payloads

**Symptom.** An SSE consumer using `bufio.Scanner` with defaults stops receiving frames midway through a run; no error, just silence.

**Why it's invisible.** `bufio.Scanner`'s default 64KB line buffer is hit by large `CODE_EXECUTION_RAN` payloads (code + output can easily exceed that). The scanner returns `ErrTooLong` internally but the loop just exits.

**Fix.** Bump the buffer at construction:

```go
scanner := bufio.NewScanner(body)
scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
```

This is already done in `parseSSE` in `shared_test.go` — use that helper rather than rolling your own.
