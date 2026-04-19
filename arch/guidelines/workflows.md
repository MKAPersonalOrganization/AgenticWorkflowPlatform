# Workflows — Repeated Recipes for Feature Work

Step-by-step recipes I've re-derived more than once. Consult these before starting so the order is right the first time; each shortcuts a few hours of backtracking.

Each workflow lists the exact files touched and the *order* they must be touched in. Skipping a step (especially the registry / migration / wiring steps) usually surfaces as a silent runtime failure, not a compile error — see `pitfalls.md` for the common ones.

---

## Workflow A — Add a new agent-config field (e.g. `MemoryPolicy`)

Goal: a new policy/config field ends up readable by the adapter, writable by the designer UI, persisted through migrations, and exposed by the REST API.

1. **Foundation type** — `foundation/agent/config.go`. Define the framework-agnostic struct. This is what the `adapters/adkgo/runner.go` resolver reads — keep it free of DB tags.
2. **DB type + JSONB column plumbing** — `modules/agentdefinition/types.go`. Mirror the foundation type with `json:"..."` tags (mixed casing — see `pitfalls.md` #11). Add a `<Name>JSON` string field. Write `Marshal<Name>` / `Unmarshal<Name>` helpers. Add a field to `AgentDefinition` struct.
3. **Migration** — new file in `infra/sqldb/migrate/sql/` (next version number, JSONB column, `ADD COLUMN IF NOT EXISTS`). Bump the Darwin migration version in the name. **Also append the same `-- Version: / -- Description: / ALTER TABLE …` block to `sql/migrate.sql`** — that is the only file Darwin actually reads (`migrate.go` does `//go:embed sql/migrate.sql`; the numbered files are source references). Keep the `-- Description:` on a single line and never let continuation comments start with "-- ver" or "-- des" — see `pitfalls.md` #13.
4. **Repository** — `modules/agentdefinition/agent_definition_repo.go`. Thread the new JSON column through `SELECT`, `INSERT`, `UPDATE`, and the `hydrate` helper.
5. **Provider → foundation** — `modules/agentdefinition/provider.go`. Add a `toFoundation<Name>Policy` helper and wire it into the `Config{...}` struct built from the DB row.
6. **Adapter** — `adapters/adkgo/runner.go`. Read the field from the resolved agent config in `Run(...)` and apply it (inject tool, set session flag, whatever). **If the field toggles behaviour that varies per provider**, branch by `modelCfg.Provider` here — don't assume one backend.
7. **REST API** — `modules/agentdefinition/handler.go`. Add the field to both `createRequest` and `updateRequest`, wire it into the `AgentDefinition` instantiation in `create()` / `update()`. Also add it to `agentResponse` and `toAgentResponse` so reads return it.
8. **UI** — `frontend/ui/agenteditor/agenteditor.templ`. Add a card or toggle. If the feature is provider-gated, add a one-line warning when the selected provider doesn't support it (pattern exists for code-execution).
9. **Integration test** — add a spec under `userrequirements/`. Start from `code_execution_test.go` — it covers auth, settings, agent creation, SSE assertion, and projection-based persistence assertion.

Order matters for step 3 — if you run migrations after the repo query changes, the app crashes at startup. Migrations are idempotent; keep them so.

---

## Workflow B — Add a new domain event (e.g. `CodeExecutionRan`)

Goal: a new event flows from the adapter through persistence into the SSE stream and survives a page refresh.

1. **Event type** — `modules/chat/thread/events.go`. Define the struct with `EventType()`, `AggregateID()`, `Timestamp` fields (see neighbours for style).
2. **Aggregate apply** — `modules/chat/thread/thread_aggregate.go`. Add a `case *YourEvent:` to `Apply(...)`. If the event changes visible state (e.g. a message appended), compute it here.
3. **Deserializer registry** — `modules/chat/thread/events_registry.go`. Add `r.Register("YourEvent", func() es.Event { return &YourEvent{} })`. **Easy to forget — and costs hours when you do.** See `pitfalls.md` #1.
4. **Adapter translation** — `adapters/adkgo/events.go`. Translate the underlying ADK session event (or provider-specific part) into your domain event. Yield it in `translateSessionEvent` or the relevant specialised translator.
5. **Orchestrator classification** — `modules/chat/agentorchestrator/service.go`:
   - Add the event type to `isDurable(...)` if it should persist.
   - Add a `case` in `persistDurableEvent(...)` that calls the right `threadMgmt` method (usually `RecordToolEvent` for generic tool lifecycle events).
6. **SSE converter** — `modules/chat/handlers/common/converter.go`. Add a `case *thread.YourEvent:` that returns an `aguievents.NewCustomEvent(...)` with the fields the frontend needs. **Custom events carry their name under `name`, payload under `value`** (see `pitfalls.md` #3).
7. **Projection** — `modules/chat/projection/agui/postgres_projection.go`:
   - Register the handler.
   - Write an `apply<YourEvent>` that inserts (or updates) rows in `agui_messages` / `agui_threads` / a sidecar map.
   - If sidecar: add the field to `AguiThreadState` in `agui_projection.go`.
8. **AG-UI handler** — `modules/chat/agui/handlers.go`. If you added a sidecar, include it in the response payload map.
9. **Frontend** — `assets/js/chat/chat-event-handlers.js` (live stream) + `assets/js/chat/chat-messaging.js` (history). Both paths must render the same UI; the live path gets custom events, the history path gets the sidecar.
10. **Templ card** — `frontend/ui/messagelist/messagelist.templ`. Add a rendering component.
11. **Integration test** — assert the event on the SSE stream AND that `/api/agui/threads/{id}/messages` returns the sidecar after the projection catches up.

If refresh survival is the spec you care about, step 3 + step 5 + step 7 + step 8 are the critical path. Miss step 3 and everything after breaks at persistence time with a concurrency conflict (see `pitfalls.md` #1).

---

## Workflow C — Write an integration test

Goal: a user-facing spec expressed as a REST-only test that's identical in behaviour to what the browser UI does.

1. **Pick a file** — under `userrequirements/`. One spec per test function; test names read as sentences (`TestAn<AgentType><DoesWhat>`).
2. **Start from the canary** — copy `code_execution_test.go` as a template. It covers every surface the harness exposes.
3. **Auth + settings**: `client := authedClient(t); upsertAISettings(t, client, "openai", apiKey)` — settings reset is inside the helper.
4. **Build the agent(s)**: use `agentBody(name, systemPrompt, provider, model)` for defaults, then merge the spec-specific fields (`outputSchema`, `memoryPolicy`, `agentTools`, `graph`, etc.).
5. **Workflow specs**: create leaves first, capture their IDs, then build the parent's `graph.children[].agent_id` referencing those UUIDs. See `pitfalls.md` #9.
6. **Send a message**: `threadID, events := sendMessage(t, client, "", agentID, "...")` — empty contextID opens a fresh thread.
7. **Assert on the SSE stream**: route CUSTOM events by `Name()` (see `pitfalls.md` #3). Extract payloads via `Value(&dst)`. `assistantFinalText(events)` for text, `findToolStarts(events)` for tool lifecycle, `runErrorMessage(events)` for run failure.
8. **Refresh-survival assertions**: `waitForCodeExecutionSidecar` pattern — poll `readThreadMessages` until the expected sidecar appears.
9. **Cleanup**: `createAgent` and `sendMessage` already register `t.Cleanup`s. Don't add extra DELETEs unless testing specifically.
10. **Run it**: `task up:cluster` first (if code changed), then `set -a && . .env && set +a && go test -tags integration -count=1 -run 'TestName' -v ./userrequirements/...`.

If the test hits the dev-login endpoint and gets a skip, `APP_AUTH_DEV_LOGIN_TOKEN` isn't set in the cluster — re-check `.env` and `docker-compose.cluster.yml`.

---

## Workflow D — Debug a concurrency conflict or "exceeded max retries" error

Goal: find the root cause without burning an hour guessing at advisory locks.

1. **What event is the retry on?** Grep app logs for `Failed to persist durable event eventType=...` — note the type.
2. **Is the event registered?** Open `modules/chat/thread/events_registry.go`. If the event *before* the failing one in the sequence (not the failing one itself) isn't registered, that's your bug. See `pitfalls.md` #1.
3. **Are there two `threadmgmt.Manager` instances?** Check the code path — if a new handler built its own `agentorchestrator.Service`, the orchestrator and the adk runner might not share a manager. See `pitfalls.md` #2.
4. **Query the DB directly** (only when the above don't match):
   ```
   docker exec app-postgres-cluster psql -U appuser -d projections -c \
     "SELECT stream_id, stream_version FROM event_streams WHERE stream_id LIKE '%<thread-id>%';"
   docker exec app-postgres-cluster psql -U appuser -d projections -c \
     "SELECT stream_version, event_type FROM events WHERE stream_id LIKE '%<thread-id>%' ORDER BY stream_version;"
   ```
   If the `event_streams.stream_version` disagrees with the count of rows in `events`, that's registry-silent-skip. If they agree and retries still fail, look for a concurrent writer.
5. **Instrument `retryUpdate`** — log `thread.GetVersion()` on each retry in `modules/chat/threadmgmt/manager.go`. If it's stuck at the same value, the aggregate loaded stale state (registry issue). If it climbs, someone is actively writing.

Do not restart the cluster hoping it'll fix itself. It won't.

---

## Workflow E — Ship a backend change to the cluster

Goal: the 10 app pods are running the new binary without losing DB state.

1. `go build ./...` — compile check locally.
2. `task up:cluster` — rebuilds the Docker image and recreates containers in place. **Never** `task down:cluster` unless you mean to wipe Postgres (see `pitfalls.md` #7).
3. Wait for all pods to be healthy (a few seconds). Task prints the status table.
4. Smoke-test the changed endpoint with `curl` before running the full integration suite — cheaper iteration.
5. `task integration` for the full suite, or `go test -tags integration -count=1 -run 'TestName' -v ./userrequirements/...` for a single spec.

If a test fails with "context deadline exceeded" it's usually an LLM call timeout, not your code. Re-run once before investigating.

---

## Workflow F — Add a new public REST endpoint

Goal: new endpoint follows existing conventions for auth, rate limiting, and response shape.

1. **Pick the module** — where does the endpoint's resource live? Its handler goes in `modules/<module>/handler.go` (or a sub-file).
2. **Auth** — decide RequireAuth vs public. Auth endpoints (dev-login, login, callback, logout) are the only public writes; everything else should require `school_session`.
3. **Request / response types** — use camelCase for top-level fields to match the rest of the API. Only go snake_case if nesting into a type that already uses it (SubAgentRef, AgentToolRef, CompositionNode — historical, don't extend the pattern).
4. **Wire the route** — the module's `RegisterRoutes(r chi.Router, mw *auth.Middleware)`. Rate-limiting middleware is applied at the route group level in `main.go:registerRoutes` — don't re-apply per endpoint.
5. **Test via `task up:cluster` + curl first** — skip the integration test until the handler is known-good.
6. **Add an integration test** (Workflow C) once the endpoint works end-to-end.

---

## Quick reference — files you always touch for X

| Feature X | Files in order |
| :--- | :--- |
| New agent-config field | `foundation/agent/config.go` → `modules/agentdefinition/types.go` → new migration → `agent_definition_repo.go` → `provider.go` → `adapters/adkgo/runner.go` → `modules/agentdefinition/handler.go` → `frontend/ui/agenteditor/agenteditor.templ` |
| New domain event | `thread/events.go` → `thread/thread_aggregate.go` (Apply) → `thread/events_registry.go` → `adapters/adkgo/events.go` → `agentorchestrator/service.go` (isDurable + persistDurableEvent) → `handlers/common/converter.go` → `projection/agui/postgres_projection.go` → `agui/handlers.go` → `assets/js/chat/*` → `frontend/ui/messagelist/messagelist.templ` |
| New REST endpoint | `modules/<mod>/handler.go` → module's `RegisterRoutes` → integration test |
| New integration test | `userrequirements/<name>_test.go` (copy from `code_execution_test.go`) |
