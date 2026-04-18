# ADR 020: Agent Memory Service

**Status:** Partially superseded. The "Per-Agent Toggle via Plugin System"
section below described the original design where memory was exposed through
a `plugins/agentmemory` PluginProvider stub in the tools picker. That package
has been removed — memory is an orchestration-engine service, not a plugin,
and surfacing it as a tool misled non-programmers. The toolset-permission
gate (`memory-set`) is kept as an interim so already-configured agents keep
working; a per-agent `Config.MemoryPolicy { Enabled; Preload }` will replace
it in a follow-up (MISALIGNMENTS §M8). Everything else in this ADR — the
service interface, the tools, the runner wiring — still stands.

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

The runner accepts a `MemoryService` field. When set, it wraps the service in a
scoped `Memory` adapter and passes it to the `InvocationContext`, making it
available to tools via `toolCtx.SearchMemory()`.

## Decision

Wire ADK-Go's memory service end-to-end with a **per-agent toggle**. Use a
custom PostgreSQL FTS backend (`memdb.Service`) that implements ADK-Go's
`memory.Service` interface.

### Backend: `memdb.Service` (PostgreSQL FTS)

ADK-Go's only built-in implementation is `memory.InMemoryService()` — an
in-memory singleton that resets on process restart and cannot scale horizontally.
We replace it with `infra/memdb.Service`, which satisfies the same
`memory.Service` interface but persists to PostgreSQL:

- **`AddSessionToMemory`**: Iterates session events, extracts text from
  `*genai.Content` parts, stores each event in a `memory_entries` table within
  a transaction. Idempotent: deletes existing entries for the session before
  inserting (same behaviour as ADK-Go's in-memory replacement).
- **`SearchMemory`**: Uses `to_tsvector('english', content_text) @@
  plainto_tsquery('english', $query)` — PostgreSQL full-text search with
  stemming and stop-word removal. Scoped by `(app_name, user_id)`.
  Returns up to 20 results ordered by `event_timestamp DESC`.

Schema (migration v1.71):

```sql
CREATE TABLE memory_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_name        TEXT NOT NULL DEFAULT '',
    user_id         TEXT NOT NULL,
    session_id      TEXT NOT NULL,
    event_id        TEXT NOT NULL,
    author          TEXT NOT NULL DEFAULT '',
    content         JSONB,
    content_text    TEXT NOT NULL DEFAULT '',
    custom_metadata JSONB,
    event_timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Indexes:

| Index | Purpose |
|-------|---------|
| `(app_name, user_id)` | Per-user scoped lookups |
| `GIN(to_tsvector('english', content_text))` | Full-text search |
| `UNIQUE(app_name, user_id, session_id, event_id)` | Deduplication |

### Per-Agent Toggle via Plugin System

Memory is controlled through the standard plugin permission system
(`plugins/agentmemory`) rather than a standalone `memory_enabled` flag. The
plugin registers a `"memory-set"` tool set containing `"preload_memory"`. Users
enable/disable it per agent in the agent designer like any other plugin.

When the `memory-set` tool set is enabled for an agent:

1. The `preloadmemorytool` is injected into the agent's tool list (both regular
   and workflow leaf agents).
2. The `MemoryService` is passed to the ADK runner config so tools can search.
3. After the run completes, the session is persisted via `AddSessionToMemory()`.

The legacy `memory_enabled` boolean column on `user_agents` is superseded by
the plugin permission (`user_tool_set_permissions` table). The runner checks
`toolReqs.HasToolSetPermission(ctx, userID, agentName, "memory-set")` instead
of reading `agentCfg.MemoryEnabled`.

### Dependency Injection

`RunnerAdapterConfig.MemoryService` is a required field (constructor rejects nil).
In production, `main.go` wires `memdb.New(db)` using the shared `*sqlx.DB`.

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
        │  extracts user query text
        │  calls SearchMemory(query) → PostgreSQL FTS
        │  injects matching past conversations into system instructions
        ▼
  LLM generates response (with memory context)
        │
        ▼
  Run completes
        │
        ▼
  AddSessionToMemory(session)
        │  iterates events, extracts text, stores in memory_entries
        │  DELETE + INSERT in transaction (idempotent)
        ▼
  Memory available for next run
```

### Scoping

Memory is scoped by `(appName, userID)`. The `appName` is a constant
(`"ai-chat-platform"`). Since each user has their own agents and sessions,
memories are naturally isolated per user.

## Consequences

### Positive

- Agents can recall past conversations with zero configuration beyond a toggle.
- Persistent across process restarts and horizontally scalable (shared PostgreSQL).
- PostgreSQL FTS provides better recall than ADK-Go's simple word intersection
  (stemming handles "running"/"runs"/"run" variations).
- The `memory.Service` interface allows swapping backends without changing
  wiring (e.g., pgvector for semantic search).
- Scoped by `(app_name, user_id)` — no cross-user data leakage.

### Negative

- No user-facing memory management (view/search/delete). Users cannot inspect
  or remove stored memories. Future work is needed for a Memory management UI
  or explicit LLM tools backed by `memdb.Service`.
- PostgreSQL FTS search behavior differs from ADK-Go's in-memory word
  intersection — results may vary for the same query (FTS is more
  permissive due to stemming).
- No memory eviction or TTL. Entries accumulate unbounded per user.

## Future Work

- Add a "Memory" tab in the agent settings UI for browsing/searching/deleting
  stored memories (REST API + viewer dialog).
- Re-add explicit LLM tools (`save_memory`, `forget_memory`, `list_memories`,
  `clear_memories`) backed by `memdb.Service` instead of the removed
  artifact-based `ArtifactMemoryService`.
- Consider per-session memory opt-out (e.g., sensitive conversations).
- Add memory eviction policy (TTL, max entries per user, LRU pruning).

## Files Changed

| File | Change |
|------|--------|
| `infra/memdb/memdb.go` | PostgreSQL FTS `memory.Service` implementation |
| `infra/sqldb/migrate/sql/migrate.sql` | v1.70: `memory_enabled` column. v1.71: `memory_entries` table + indexes |
| `adapters/adkgo/runner.go` | `MemoryService` required field, tool injection, post-run persistence, error logging |
| `adapters/adkgo/workflow.go` | `preloadmemorytool` injection for workflow leaf agents |
| Agent definition types + repo | `MemoryEnabled` field on `AgentDefinition`, SELECT/INSERT/UPDATE queries |
| Agent handlers + service | Request/response types + handlers, fork handler copies flag |
| `foundation/agent/config.go` | `MemoryEnabled` on `Config` |
| `adapters.go` | `agentFromDefinition()` maps flag |
| `components/agenteditor/agenteditor.templ` | Toggle switch UI |
| `components/landing/features.templ` | Updated feature card copy |
| `main.go` | Wires `memdb.New(db)` into `RunnerAdapterConfig.MemoryService` |

### Removed

| File | Reason |
|------|--------|
| `plugins/memory/` (entire package) | Replaced by ADK-Go's `preloadmemorytool` |
| `modules/chat/memory/adapter.go` | `ArtifactMemoryService` (S3-backed) removed; replaced by `memdb.Service` |
| `platform/toolregistry/plugin.go` | `MemoryService` interface removed |
| `platform/toolregistry/factory.go` | `memoryFactory` + `SetMemoryFactory` removed |
| `modules/chat/taskmgmt/artifact_handler.go` | `GetMemory` handler removed |
| `modules/chat/chat.go` | `/v1/memory` route removed |
| `components/nav/settings.templ` | Memory dialog button removed |
