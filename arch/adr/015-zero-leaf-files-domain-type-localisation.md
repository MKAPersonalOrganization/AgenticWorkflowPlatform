# ADR 015: Zero Leaf Files — Domain Type Localisation & Shared Infrastructure Consolidation

## Status

**Accepted — Implemented** (continues ADR 012 → 013 → 014)

## Context

ADRs 012–014 completed the modular monolith migration through five phases. Twelve modules now live under `modules/`, all handlers and services have been modularised, and `main.go` serves purely as a composition root. However, a post-migration audit reveals **residual violations** — files that remain in legacy layered directories despite belonging semantically to a single module. These "leaf files" break the promise that every bounded context has a single directory home.

### Current state of `domain/`

| File | Contents | Real code or alias? | Sole consumer |
|------|----------|--------------------:|---------------|
| `user_preference.go` | `UserPreference`, `UserPreferenceRepository` | **Alias** → `modules/preferences` | Zero external consumers |
| `edgequake_permissions.go` | `TenantContext`, `EdgequakeRole`, role constants | **Alias** → `shared/` | Zero external consumers |
| `user.go` | `User`, `GetUserFromContext`, `IsAuthenticated` | **Alias** → `shared/` | Zero external consumers |
| `aggregate.go` | `EventEnvelope`, `Aggregate`, `BaseAggregate`, `StreamName` | **Alias** → `foundation/eventsourcing` | Zero external consumers |
| `repository.go` | `EventStore`, `Repository[T]`, `NewRepository` | **Alias** → `foundation/eventsourcing` | Zero external consumers |
| `ai_settings.go` | `AIProvider`, `AISettings`, `AICapability`, repos, helpers (265 lines) | **Real code** | Only `modules/aisettings/` |
| `mcp_connector.go` | `ConnectorStatus`, `MCPConnector`, `MCPDiscoveredTool`, `MCPConnectorRepository` | **Real code** | Only `modules/mcp/` |
| `student_profile.go` | `StudentProfile`, `StudentProfileRepository` | **Real code** | `modules/identity/`, `modules/chat/`, `foundation/agentsdk/`, `components/profile/` |
| `rag.go` | `EmbeddingProvider` interface | **Real code** | Only `infra/rag/embeddingproviders/` (dead — zero importers) |
| `doc.go` | Package documentation | Docs only | N/A |
| `domain.go` | `Event` alias, `ToolCall`, `Message`, `Thread`, chat-related types | **Real code** (chat kernel) | Widely used — chat, analytics, SDK |
| `events.go` | Thread domain events | **Real code** (chat kernel) | Widely used — chat, analytics, SDK |
| `events_registry.go` | Event deserialiser registry | **Real code** (chat kernel) | `infra/eventstore/`, `modules/analytics/` |
| `thread_aggregate.go` | `ThreadAggregate` + decider | **Real code** (chat kernel) | `modules/chat/`, `foundation/agentsdk/` |
| `thread_repository.go` | `ThreadRepository` | **Real code** (chat kernel) | `modules/chat/` |

**Summary:** 5 alias files with zero consumers (dead weight), 3 real-code files consumed by only one module (violating locality), 1 dead interface (`rag.go`), and 5 shared-kernel files that legitimately stay.

### Current state of `infra/sqldb/` (non-migration files)

| File | Implements | Sole consumer module |
|------|-----------|---------------------|
| `permissions_repo.go` | `permissions.Repository` | `services/permissions/` |
| `tool_registry_repo.go` | `toolregistry.Repository` | `services/toolregistry/` |
| `workspace_permissions_repo.go` | `workspacepermissions.Repository` | `services/workspacepermissions/` |

These repository adapters sit in `infra/sqldb/` but exist solely to serve a single `services/` package. They should co-locate with their service.

### Current state of `infra/rag/`

| Directory | Contents | Importers |
|-----------|----------|-----------|
| `embeddingproviders/` | `embedding_openai.go` — OpenAI embedding adapter | **Zero** (dead code) |
| `embeddingstore/` | Empty | N/A |
| `services/` | Empty | N/A |

The entire `infra/rag/` tree is orphaned. The knowledge module handles embeddings through EdgeQuake, not through this adapter.

### Current state of `services/interfaces.go`

Contains `AuthService` and `ThreadQueryService` interfaces (42 lines). `AuthService` is implemented inline in `main.go`. `ThreadQueryService` + `ThreadSummary` are consumed only by `modules/chat/` sub-packages.

### Current state of `handlers/`

| Package | Purpose | Consumer count |
|---------|---------|---------------|
| `auth/` | Auth middleware + Google OAuth handlers | 12 modules + `main.go` |
| `health/` | Health check endpoint | Only `main.go` |

Both are genuinely cross-cutting infrastructure, not domain modules.

## Decision

Complete the modular monolith migration by eliminating all leaf-file violations through six work streams, executed in dependency order.

### Work Stream 1: Delete dead aliases and dead code

Files with zero external consumers provide no backward compatibility value and add confusion.

| Action | File(s) |
|--------|---------|
| **Delete** | `domain/user_preference.go` — alias to `modules/preferences`, zero consumers |
| **Delete** | `domain/edgequake_permissions.go` — alias to `shared/`, zero consumers |
| **Delete** | `domain/user.go` — alias to `shared/`, zero consumers |
| **Delete** | `domain/aggregate.go` — alias to `foundation/eventsourcing`, zero consumers |
| **Delete** | `domain/repository.go` — alias to `foundation/eventsourcing`, zero consumers |
| **Delete** | `domain/rag.go` — `EmbeddingProvider` interface, only consumer is dead code |
| **Delete** | `infra/rag/` — entire directory tree (dead code, zero importers) |

**Validation:** `go build ./...` + `go test ./...` must pass after each deletion.

### Work Stream 2: Localise single-consumer domain types into their owning modules

Each of these type files is consumed by exactly one module. Moving the canonical type into the module eliminates the `domain/` dependency and makes the module self-contained.

| Type(s) | Current location | Target module | Migration steps |
|---------|-----------------|---------------|-----------------|
| `AIProvider`, `AISettings`, `AICapability`, `AICapabilityConfig`, repos, helpers | `domain/ai_settings.go` | `modules/aisettings/types.go` | 1. Move all 265 lines. 2. Update internal refs. 3. If `foundation/agentsdk/` references `AISettings`, use an interface or import from module. 4. Delete original. |
| `ConnectorStatus`, `MCPConnector`, `MCPDiscoveredTool`, `MCPConnectorRepository` | `domain/mcp_connector.go` | `modules/mcp/types.go` | 1. Move types. 2. Update internal refs. 3. Delete original. |
| `StudentProfile`, `StudentProfileRepository` | `domain/student_profile.go` | `modules/identity/types.go` | **Blocked** by `components/profile/` importing `domain.StudentProfile`. Unblock by having `components/profile/` accept a generic template struct or import from `modules/identity/`. Then move type and delete alias in `modules/identity/types.go`. |

**Convention:** Each module that owns domain types places them in a `types.go` file within the module package.

### Work Stream 3: Co-locate `infra/sqldb/` repository adapters with their services

Repository adapters in `infra/sqldb/` that serve a single `services/` package should move next to their service, following the pattern already established by module-internal repos (e.g., `modules/billing/repo.go`).

| File | Current location | Target | Rationale |
|------|-----------------|--------|-----------|
| `permissions_repo.go` + test | `infra/sqldb/` | `services/permissions/repo.go` | Sole consumer is `services/permissions/` |
| `tool_registry_repo.go` + test | `infra/sqldb/` | `services/toolregistry/repo.go` | Sole consumer is `services/toolregistry/` |
| `workspace_permissions_repo.go` + test | `infra/sqldb/` | `services/workspacepermissions/repo.go` | Sole consumer is `services/workspacepermissions/` |

After this, `infra/sqldb/` contains only `sqldb.go` (connection pool), `sqldb_test.go`, and `migrate/` (schema migrations) — all genuinely shared infrastructure.

### Work Stream 4: Dissolve `services/interfaces.go`

| Interface | Target | Rationale |
|-----------|--------|-----------|
| `AuthService` | `shared/auth_service.go` or inline in `handlers/auth/` | Cross-cutting contract used by chat module and `main.go`. Belongs in shared kernel or auth package. |
| `ThreadQueryService` + `ThreadSummary` | `modules/chat/interfaces.go` | Only consumed by `modules/chat/` sub-packages. |

Delete `services/interfaces.go` after migration. If the `services/` directory becomes empty of `.go` files at the package root, it becomes a pure directory namespace for the remaining cross-cutting sub-packages.

### Work Stream 5: Relocate `handlers/auth/` to `foundation/auth/`

`handlers/auth/` contains auth middleware and Google OAuth handlers used by 12 modules. It is **not a handler** in the modular-monolith sense — it is cross-cutting infrastructure. Rename to `foundation/auth/` to align with the convention that `foundation/` holds framework-level infrastructure.

| File | Current | Target |
|------|---------|--------|
| `middleware.go` | `handlers/auth/` | `foundation/auth/` |
| `auth_handlers.go` | `handlers/auth/` | `foundation/auth/` |

After this move, `handlers/health/` is the only remaining package in `handlers/`. Move it to `foundation/health/` and delete the `handlers/` directory entirely, eliminating the last vestige of the layered architecture.

### Work Stream 6: Shrink `domain/` to pure shared kernel

After Work Streams 1–2, `domain/` contains only:

| File | Purpose | Why it stays |
|------|---------|-------------|
| `doc.go` | Package documentation | Documentation |
| `domain.go` | `Event` alias, `ToolCall`, `Message`, `Thread`, `GenerateMessageID` | Shared event contracts consumed by chat, analytics, SDK, projections |
| `events.go` | Thread domain events (`UserMessageAdded`, `AssistantMessageCompleted`, etc.) | Integration contracts — consumed by `foundation/agentsdk/`, `modules/analytics/statistics/`, `infra/projection/subscriber/` |
| `events_registry.go` | Event deserialiser registry | Required by event store infrastructure |
| `thread_aggregate.go` | `ThreadAggregate` + decider logic | Shared kernel — consumed by chat module and agent SDK |
| `thread_repository.go` | `ThreadRepository` constructor | Used by chat module |
| `thread_*_test.go` | Tests for aggregate + decider | Test coverage |

This is the **irreducible shared kernel**: the event-sourced thread aggregate and its event contracts. Moving these into `modules/chat/` would create forbidden module→module and foundation→module dependencies. The `domain/` package is explicitly the shared-kernel namespace for cross-boundary event contracts.

**Rename consideration:** `domain/` → `shared/thread/` or `shared/events/` could be considered in a future ADR if the team wants to eliminate the `domain/` package name entirely. This ADR does not mandate a rename — the content reduction is the priority.

## Execution Order

Execute work streams in this order to minimise risk:

| Step | Work Stream | Risk | Validation |
|------|------------|------|------------|
| 1 | WS1: Delete dead aliases + dead code | **Zero** — no consumers | `go build ./... && go test ./...` |
| 2 | WS3: Co-locate `infra/sqldb/` repos | **Low** — move + update imports | `go build ./... && go test ./...` |
| 3 | WS4: Dissolve `services/interfaces.go` | **Low** — 2 interfaces, clear targets | `go build ./... && go test ./...` |
| 4 | WS2: Localise domain types (except `StudentProfile`) | **Medium** — requires import updates in modules | `go build ./... && go test ./...` per type |
| 5 | WS2 (StudentProfile): Unblock + localise | **Medium** — requires `components/profile/` refactor | `go build ./... && go test ./...` |
| 6 | WS5: Relocate `handlers/` → `foundation/` | **Medium** — 12 import paths change | `go build ./... && go test ./...` |

Each step is a separate commit. CI must pass between steps.

## Constraints

All constraints from ADRs 012–014 remain in effect, plus:

1. **No leaf files** — every `.go` file outside `main.go`, `shared/`, `foundation/`, and `domain/` (shared kernel) must live inside a `modules/` directory or a declared shared-infrastructure package.
2. **`domain/` is frozen** — after this ADR, no new types may be added to `domain/`. New domain types belong in their module. The only exception is new thread event types (shared kernel).
3. **`services/` sub-packages are shared infrastructure** — `artifact/`, `memory/`, `permissions/`, `toolregistry/`, `workspacepermissions/` stay under `services/` because each has 3+ consumers across modules, plugins, and foundation. A future ADR may relocate these to `foundation/` if the `services/` namespace becomes confusing.
4. **`plugins/` stays top-level** — plugins are tool implementations registered with the tool registry. They are not domain modules. They follow the `toolregistry.PluginProvider` interface and remain under `plugins/`.
5. **Import linting** — after completion, add a CI check (e.g., `go-import-lint` or a custom script) that enforces:
   - No module imports another module.
   - No new files in `domain/` (frozen).
   - No files in `handlers/` (deleted directory).
   - `infra/sqldb/` contains only `sqldb.go`, tests, and `migrate/`.

## Consequences

### Positive

- **Zero leaf files** — every domain type lives in its owning module. No more hunting across `domain/`, `infra/`, and `modules/` to understand a bounded context.
- **`domain/` shrinks from 20 files to 7** — only the irreducible shared kernel remains.
- **`handlers/` directory eliminated** — the last artefact of the layered architecture is gone.
- **`infra/sqldb/` is clean** — only shared database infrastructure remains (connection + migrations).
- **Dead code removed** — `infra/rag/` tree, dead aliases, and the orphaned `EmbeddingProvider` interface are deleted.
- **Import graph simplifies** — modules import `shared/` and `foundation/`, not `domain/` (except for the frozen shared kernel).
- **Enforceable via CI** — import linting prevents regression.

### Negative

- **Import churn** — Work Streams 2 and 5 require updating import paths across multiple files. Risk is mitigated by doing one type/package at a time with full CI between each.
- **`StudentProfile` blocked by template import cycle** — requires a refactor of `components/profile/` before the type can move. This is the only item that cannot be completed in pure isolation.
- **`services/` still exists** — five shared-infrastructure packages remain under `services/`. This is a conscious trade-off: moving them to `foundation/` would be a rename without architectural benefit, and `foundation/` already has a clear role (framework-level SDK, eventsourcing, middleware).

### Final Target Architecture

```
modules/                        ← 11 bounded-context modules
  aisettings/                   ← owns AISettings, AIProvider, AICapability types
  analytics/
    statistics/
  banner/
  billing/
  chat/
    agentorchestrator/
    agui/
    chatread/
    projection/
    projections/
    taskmanagement/
    taskmgmt/
    threadmgmt/
  identity/                     ← owns StudentProfile type (after unblocking)
  knowledge/
  mcp/                          ← owns MCPConnector, ConnectorStatus types
  permissions/
  preferences/
  resources/

domain/                         ← FROZEN shared kernel (thread events + aggregate)
  doc.go
  domain.go                     ← Event, ToolCall, Message, Thread
  events.go                     ← Thread domain events
  events_registry.go            ← Event deserialiser
  thread_aggregate.go           ← ThreadAggregate
  thread_repository.go          ← ThreadRepository
  thread_*_test.go

shared/                         ← Cross-cutting kernel (User, context helpers)
foundation/                     ← Framework (SDK, eventsourcing, logger, middleware, auth)
  auth/                         ← (moved from handlers/auth/)
  health/                       ← (moved from handlers/health/)
  ...

services/                       ← Shared infrastructure services (multi-consumer)
  artifact/
  memory/
  permissions/
  toolregistry/
  workspacepermissions/

infra/                          ← Infrastructure adapters
  eventstore/
  googleauth/
  projection/subscriber/
  sqldb/                        ← Only sqldb.go + migrate/ (repos co-located with services)

plugins/                        ← Tool plugin implementations
  calculator/
  codebase/
  cognitive/
  document/
  elicitation/
  knowledgebase/
  library/
  memory/
  tts/
```
