# ADR 014: Chat Module Extraction — Phase 5 of Modular Monolith Migration

## Status

**Completed** (continues ADR 012, ADR 013)

## Context

ADRs 012 and 013 completed Phases 1–4 of the modular monolith migration. Eleven modules now live under `modules/`:

| Module | Status |
|--------|--------|
| `billing` | ✅ Extracted |
| `aisettings` | ✅ Extracted |
| `resources` | ✅ Extracted |
| `mcp` | ✅ Extracted |
| `knowledge` | ✅ Extracted |
| `preferences` | ✅ Extracted |
| `identity` | ✅ Extracted |
| `analytics` | ✅ Extracted |
| `banner` | ✅ Extracted |
| `permissions` | ✅ Extracted |

The **core chat bounded context** — the most interconnected piece of the system — still lives across the old layered directories:

| Layer | Packages |
|-------|----------|
| **Handlers** | `handlers/agui/` (AGUI projection query endpoints), `handlers/taskmgmt/` (chat API: send message, SSE streaming, artifacts, memory, plugins, documents) |
| **Services** | `services/agentorchestrator/` (agent run orchestration + event sourcing), `services/taskmanagement/` (Asynq job processing), `services/threadmgmt/` (thread lifecycle), `services/chatread/` (read service), `services/memory/` (long-term memory via S3) |
| **Projections** | `infra/projection/agui/` (AGUI read model), `infra/projection/llmcontext/` (LLM context read model), `infra/projections/` (UI projection service adapter) |
| **Domain** | `domain/thread_aggregate.go`, `domain/thread_repository.go`, `domain/events.go`, `domain/events_registry.go` |

Additionally, `main.go` (915 lines) constructs these services inline and registers ~25 chat routes manually, making it the single largest source of coupling in the codebase.

### Why the chat module was deferred

ADR 013 explicitly deferred Phase 5:

> "The chat/taskmgmt/agui bounded context is the most interconnected piece of the system. It depends on nearly every other module and contains the core event-sourced thread aggregate. Extracting it requires careful dependency-cycle analysis."

The key challenge is a **circular dependency** between the orchestrator and the runner:

```
agentOrchestrator → Runner (via SetRunner, after construction)
runner → agentOrchestrator (via ThreadManager interface, at construction)
```

This cycle is currently resolved in `main.go` via two-phase construction. The module must preserve this pattern.

### Cross-cutting services

Several services used by the chat handlers are also consumed by the agent runner adapter (`foundation/agentsdk/adapters/custom/`) and other modules. These remain **shared infrastructure**:

| Service | Consumers | Decision |
|---------|-----------|----------|
| `services/artifact/` | Chat module, runner adapter, resources module | Stays in `services/` (S3 infrastructure) |
| `services/permissions/` | Chat handlers, runner adapter | Stays in `services/` (cross-cutting) |
| `services/toolregistry/` | Chat handlers, runner adapter, MCP module | Stays in `services/` (cross-cutting) |
| `services/workspacepermissions/` | Permissions module, runner adapter | Stays in `services/` (cross-cutting) |
| `services/auth/` | Chat handlers | Stays in `services/` (trivial shared interface) |

## Decision

Extract the core chat bounded context into `modules/chat/` using a **two-sub-phase approach**: first a facade module (Sub-phase 5a), then a full code relocation (Sub-phase 5b).

### Sub-phase 5a: Chat module facade (this commit)

Create `modules/chat/chat.go` as a module facade that:

1. **Encapsulates construction** of all chat-domain services (orchestrator, task manager, thread management, projections, handlers, memory service)
2. **Exposes a standard module API** (`New`, `SetRunner`, `RegisterRoutes`, `RegisterProjection`, `Shutdown`)
3. **Eliminates ~100 lines of inline construction and route registration from `main.go`**

#### Module Public API

```go
package chat

// Dependencies holds all external dependencies injected into the chat module.
type Dependencies struct {
    Log                *logger.Logger
    DB                 *sqlx.DB
    EventStore         domain.EventStore
    ArtifactService    artifact.Service
    StudentProfileRepo domain.StudentProfileRepository
    Emitter            agentorchestrator.EventEmitter
    AgentProvider      agent.AgentProvider
    AuthService        services.AuthService
    ToolRegistry       *toolregistry.Registry
    PermissionsService *permissions.Service
    EdgequakeService   *edgequake.Service
    TenantID           string
    AllowedOrigin      string
    Broadcaster        agui.AguiBroadcaster
    AsynqConfig        taskmanagement.AsynqConfig
}

// Module is the chat bounded context.
type Module struct { ... }

func New(deps Dependencies) *Module

// Orchestrator returns the orchestrator for the runner adapter.
func (m *Module) Orchestrator() *agentorchestrator.Service

// SetRunner completes two-phase construction: sets the runner on the
// orchestrator and creates the task manager + HTTP handlers.
func (m *Module) SetRunner(runner agentorchestrator.Runner)

// RegisterProjection wires AGUI + LLM context projections into the router.
func (m *Module) RegisterProjection(router ProjectionRouter)

// RegisterRoutes mounts all chat HTTP routes.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware)

// Shutdown gracefully stops the Asynq task manager.
func (m *Module) Shutdown()
```

#### Two-phase construction in main.go

```go
// 1. Create module (constructs orchestrator without runner)
chatModule := chat.New(chat.Dependencies{...})

// 2. Create runner (needs orchestrator)
runner, err := custom.NewRunnerAdapter(ctx, log, chatModule.Orchestrator(), ...)

// 3. Complete initialisation (sets runner, creates task manager + handlers)
chatModule.SetRunner(runner)

// 4. Wire into application
chatModule.RegisterProjection(persistenceRouter)
chatModule.RegisterRoutes(r, authMiddleware)
defer chatModule.Shutdown()
```

#### What moves into the module's construction

| Currently in `main.go` | Absorbed by `chat.New()` or `chat.SetRunner()` |
|------------------------|-----------------------------------------------|
| `agentorchestrator.New(...)` | `New()` |
| `domain.NewThreadRepository(...)` | `New()` |
| `threadmgmt.New(...)` | `New()` |
| `memory.NewArtifactMemoryService(...)` | `New()` |
| `agui.NewPostgresAguiProjection(...)` | `New()` |
| `llmcontext.NewPostgresLLMContextProjection(...)` | `New()` |
| `projections.NewUIProjectionService(...)` | `New()` |
| `taskmanagement.NewAsynqTaskManager(...)` | `SetRunner()` |
| `taskmgmthandlers.NewTaskManagementHandler(...)` | `SetRunner()` |
| `aguihandlers.NewAguiProjectionHandler(...)` | `SetRunner()` |
| 25 inline route registrations | `RegisterRoutes()` |
| `aguiProjection.RegisterHandlers(persistenceRouter)` | `RegisterProjection()` |
| `llmContextProjection.RegisterHandlers(persistenceRouter)` | `RegisterProjection()` |

### Sub-phase 5b: Full code relocation

Relocate the actual source files into the module:

| Source | Target | Status |
|--------|--------|--------|
| `handlers/agui/` | `modules/chat/agui/` | ✅ Done |
| `handlers/taskmgmt/` | `modules/chat/taskmgmt/` | ✅ Done |
| `services/agentorchestrator/` | `modules/chat/agentorchestrator/` | ✅ Done |
| `services/taskmanagement/` | `modules/chat/taskmanagement/` | ✅ Done |
| `services/threadmgmt/` | `modules/chat/threadmgmt/` | ✅ Done |
| `services/chatread/` | `modules/chat/chatread/` | ✅ Done |
| `services/memory/` | — | ⏭ Skipped (cross-cutting: plugins, SDK, toolregistry) |
| `infra/projection/agui/` | `modules/chat/projection/agui/` | ✅ Done |
| `infra/projection/llmcontext/` | `modules/chat/projection/llmcontext/` | ✅ Done |
| `infra/projections/` | `modules/chat/projections/` | ✅ Done |
| `domain/thread_*.go`, `events*.go` | — | ⏭ Kept in `domain/` (shared kernel — see below) |

> **Note:** Directory names preserve the original package names to avoid
> renaming all `package` declarations (e.g. `agentorchestrator/` not
> `orchestrator/`). `services/memory/` stays in `services/` because it is
> consumed by `plugins/memory`, `foundation/agentsdk/adapters/custom/runner.go`,
> and `services/toolregistry/factory.go` — all outside the chat bounded context.

#### Domain files — shared kernel decision

`domain/thread_aggregate.go`, `domain/thread_repository.go`, `domain/events.go`,
and `domain/events_registry.go` remain in the `domain/` package. Although these
types belong semantically to the chat bounded context, they serve as **integration
contracts** (shared kernel) consumed by:

- `foundation/agentsdk/` — the agent SDK adapter uses `ThreadAggregate`, events,
  and the `ThreadRepository` to drive agent runs.
- `modules/analytics/statistics/` — the statistics projection subscribes to
  `ToolResultAdded` and `AssistantMessageCompleted` events.
- `infra/projection/subscriber/` — the projection updater deserialises events
  via the registry.

Moving them into `modules/chat/` would force `foundation/` and `analytics/` to
import from the chat module, creating an undesirable dependency direction
(foundation → module, module → module). The `domain/` package is the
established shared kernel for cross-module event contracts.

### Sub-phase 5c: Remaining `services/` and `handlers/` cleanup

| Directory | Remaining | Disposition | Status |
|-----------|-----------|-------------|--------|
| `handlers/auth/` | Auth middleware + handlers | Stays (shared infrastructure) | — |
| `handlers/health/` | Health check endpoints | Stays (infrastructure) | — |
| `handlers/workspacepermissions/` | Delegate for permissions module | Inlined into `modules/permissions/` | ✅ Done |
| `services/auth/` | Simple auth service | Inlined into `main.go` as `contextAuthService` | ✅ Done |
| `services/interfaces.go` | Shared contracts (`AuthService`, `ThreadQueryService`) | Stays (consumed by chat sub-packages; shared contracts layer) | ✅ Kept |
| `services/artifact/` | S3 adapter | Stays (cross-cutting: chat, resources, plugins, SDK) | ✅ Kept |
| `services/memory/` | Long-term memory | Stays (cross-cutting: chat, plugins, SDK, toolregistry) | ✅ Kept |
| `services/permissions/` | Tool permissions | Stays (cross-cutting: chat, SDK, infra) | ✅ Kept |
| `services/toolregistry/` | Plugin registry | Stays (cross-cutting: chat, MCP, all plugins, SDK) | ✅ Kept |
| `services/workspacepermissions/` | Workspace permissions | Stays (cross-cutting: permissions module, SDK) | ✅ Kept |
| `services/studentprofile/` | Student profile middleware | Moved to `foundation/agentsdk/middleware/` | ✅ Done |
| `services/docstore/` | Empty | Deleted | ✅ Done |
| `services/embedding/` | Empty | Deleted | ✅ Done |
| `services/titlegenerator/` | Empty | Deleted | ✅ Done |
| `services/rating/` | Dead (zero importers) | Deleted | ✅ Done |
| `services/resourcepermissions/` | Dead (zero importers) | Deleted | ✅ Done |
| `infra/projection/statistics/` | Analytics projection | Moved to `modules/analytics/statistics/` | ✅ Done |

### Constraints

All constraints from ADRs 012–013 remain in effect, plus:

1. **Two-phase construction** — `SetRunner()` must be called before `RegisterRoutes()`. The module panics if routes are registered without a runner.
2. **Orchestrator exposure** — The module exposes `Orchestrator()` so `main.go` can create the runner adapter with cross-module dependencies (agent provider, AI settings, knowledge service, etc.).
3. **Shared services stay shared** — `artifact`, `memory`, `permissions`, `toolregistry`, `workspacepermissions` are NOT absorbed into the chat module because they serve the runner adapter and other modules.
4. **Domain is a shared kernel** — Thread aggregate, events, and repository stay in `domain/` as integration contracts consumed across module boundaries.

## Consequences

### Achieved

- **`main.go` shrank from 915 → ~850 lines** — all chat construction and route registration lives in `modules/chat/`.
- **11 modules under `modules/`** — `aisettings`, `analytics`, `banner`, `billing`, `chat`, `identity`, `knowledge`, `mcp`, `permissions`, `preferences`, `resources`.
- **`handlers/` contains only 2 shared packages** — `auth/` (middleware) and `health/` (infrastructure).
- **`services/` contains only 6 shared packages** — `artifact/`, `interfaces.go`, `memory/`, `permissions/`, `toolregistry/`, `workspacepermissions/` — all genuinely cross-cutting.
- **`infra/projection/` contains only `subscriber/`** — the projection polling engine, a true infrastructure concern.
- **5 dead packages deleted** — `docstore`, `embedding`, `titlegenerator`, `rating`, `resourcepermissions`.
- **All 32 test packages pass** — build and tests verified at every step.

### Remaining design trade-offs

- **Large `Dependencies` struct** — The chat module has 15 injected dependencies, reflecting the domain's inherent complexity.
- **Two-phase construction** — `SetRunner()` adds a temporal coupling that must be documented and enforced.
- **`services/interfaces.go` in shared layer** — `AuthService` and `ThreadQueryService` are consumed only by chat sub-packages but live in `services/` to avoid circular imports between `modules/chat/` and its sub-packages.

## Final Architecture

```
modules/
  aisettings/       — Per-user AI model configuration (DB)
  analytics/        — Dashboard + statistics projection (DB)
    statistics/     — PostgreSQL projection (moved from infra/)
  banner/           — Admin banners
  billing/          — Event-sourced Stripe payment (ES+DB)
  chat/             — Core chat bounded context (ES+DB+Redis)
    agentorchestrator/  — Agent run orchestration
    agui/               — AGUI projection HTTP handlers
    chatread/           — Thread read service
    projection/
      agui/             — AGUI read model (PostgreSQL)
      llmcontext/       — LLM context read model (PostgreSQL)
    projections/        — UI projection service adapter
    taskmanagement/     — Asynq job processing
    taskmgmt/           — Chat API HTTP handlers (SSE, artifacts, plugins)
    threadmgmt/         — Thread lifecycle (event sourcing)
  identity/         — User profiles (DB)
  knowledge/        — Edgequake / RAG services
  mcp/              — Model Context Protocol connectors (DB)
  permissions/      — Workspace permissions (DB)
  preferences/      — User key-value preferences (DB)
  resources/        — File resources (S3+DB)

domain/             — Shared kernel (event contracts, aggregates, repository interfaces)
foundation/         — Framework layer (agent SDK, eventsourcing, logger, middleware)
handlers/           — Shared HTTP infrastructure (auth middleware, health checks)
infra/              — Infrastructure adapters (event store, Google OAuth, SQL, RAG, projection subscriber)
services/           — Cross-cutting shared services (artifact, memory, permissions, toolregistry)
plugins/            — Agent tool plugins
```
