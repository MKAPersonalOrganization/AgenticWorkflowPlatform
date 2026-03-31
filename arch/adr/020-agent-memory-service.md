# ADR 020: Agent Memory Service

**Status:** Accepted

## Context

Agents have no cross-session knowledge. Each conversation starts from scratch,
which is a significant limitation for tutoring agents, support agents, and
personal assistants that need to recall past interactions.

ADK-Go ships a `memory.Service` interface with two methods:

| Method | Purpose |
|--------|---------|
| `AddSessionToMemory(ctx, session)` | Index a completed session for future recall |
| `SearchMemory(ctx, request)` | Keyword-based search across indexed sessions |

It also provides two companion tools:

| Tool | Behaviour |
|------|-----------|
| `preloadmemorytool` | Auto-searches memory each LLM turn; injects matches into system instructions |
| `loadmemorytool` | Explicit tool the LLM can call to search memory on demand |

The runner already accepts an optional `MemoryService` field. When set, it wraps
the service in a scoped `Memory` adapter and passes it to the `InvocationContext`,
making it available to tools via `toolCtx.SearchMemory()`.

## Decision

Wire ADK-Go's memory service end-to-end with a **per-agent toggle**. Use the
in-memory implementation (`memory.InMemoryService()`) as the initial backend.

### Per-Agent Flag

A `memory_enabled` boolean column on `user_agents` (default false). When true:

1. The `preloadmemorytool` is injected into the agent's tool list (both regular
   and workflow leaf agents).
2. The `MemoryService` is passed to the ADK runner config so tools can search.
3. After the run completes, the session is persisted via `AddSessionToMemory()`.

### Memory Service Lifecycle

The `memory.InMemoryService()` is instantiated once as a **singleton** on
`RunnerAdapter` (not per-run). This ensures memories accumulate across runs
for the lifetime of the process.

### Tool Choice: preloadmemorytool

We use `preloadmemorytool` rather than `loadmemorytool` because:

- It runs automatically on each LLM request (no explicit tool call needed).
- Better UX for no-code users — memory "just works" when enabled.
- Injects context via system instructions, so the LLM sees relevant history
  without needing to decide when to search.

### Data Flow

```
User message arrives
        │
        ▼
  preloadmemorytool.ProcessRequest()
        │  searches memory for keywords matching the user's query
        │  injects matching past conversations into system instructions
        ▼
  LLM generates response (with memory context)
        │
        ▼
  Run completes
        │
        ▼
  AddSessionToMemory(session)
        │  indexes all events from this session
        │  for future keyword-based recall
        ▼
  Memory available for next run
```

### Scoping

Memory is scoped by `(appName, userID)` in ADK-Go's in-memory implementation.
Since each user has their own agents and sessions, memories are naturally
isolated per user.

## Consequences

### Positive

- Agents can recall past conversations with zero configuration beyond a toggle.
- Uses ADK-Go's native memory infrastructure — no custom implementation needed.
- The `memory.Service` interface allows swapping backends without changing the
  wiring (e.g., pgvector for semantic search, PostgreSQL FTS for persistence).

### Negative

- In-memory backend resets on process restart. Acceptable for v1; future work
  will add a persistent backend.
- Keyword-based search (word intersection) is simple but effective for most
  conversational recall patterns.

## Future Work

- Implement a persistent `memory.Service` backed by PostgreSQL FTS or pgvector.
- Add a "Memory" tab in the agent settings UI for browsing/searching/deleting
  stored memories.
- Consider per-session memory opt-out (e.g., sensitive conversations).

## Files Changed

| File | Change |
|------|--------|
| `infra/sqldb/migrate/sql/migrate.sql` | Version 1.70: `memory_enabled` column |
| `modules/agentstore/types.go` | `MemoryEnabled` field on `AgentDefinition` |
| `modules/agentstore/agent_definition_repo.go` | SELECT/INSERT/UPDATE queries |
| `modules/agentstore/store_handler.go` | Request/response types + handlers |
| `modules/agentstore/service.go` | Fork handler copies flag |
| `foundation/agent/config.go` | `MemoryEnabled` on `Config` |
| `adapters.go` | `agentFromDefinition()` maps flag |
| `adapters/adkgo/runner.go` | Singleton memory service, tool injection, post-run persistence |
| `adapters/adkgo/workflow.go` | Memory tool injection for workflow leaf agents |
| `assets/js/components/agent-designer-dialog.js` | Form state + save payload |
| `components/agenteditor/agenteditor.templ` | Toggle switch UI |
| `components/landing/features.templ` | Updated feature card copy |
| `vendor/...preloadmemorytool/` | Vendored ADK-Go package |
