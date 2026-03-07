# ADR 014: Chat Module Extraction — Phase 5 of Modular Monolith Migration

## Status

Accepted (continues ADR 012, ADR 013)

## Context

ADRs 012 and 013 completed Phases 1–4 of the modular monolith migration. Eleven modules now live under `modules/`:

| Module | Status |
|--------|--------|
| `billing` | ✅ Extracted |
| `aisettings` | ✅ Extracted |
| `resources` | ✅ Extracted |
| `agentstore` | ✅ Extracted |
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

### Sub-phase 5b: Full code relocation (future)

After Sub-phase 5a stabilises, relocate the actual source files into the module:

| Source | Target | Status |
|--------|--------|--------|
| `handlers/agui/` | `modules/chat/agui/` | ✅ Done |
| `handlers/taskmgmt/` | `modules/chat/taskmgmt/` | ✅ Done |
| `services/agentorchestrator/` | `modules/chat/orchestrator/` | Pending |
| `services/taskmanagement/` | `modules/chat/taskmanager/` | Pending |
| `services/threadmgmt/` | `modules/chat/threadmgmt/` | Pending |
| `services/chatread/` | `modules/chat/chatread/` | Pending |
| `services/memory/` | `modules/chat/memory/` | Pending |
| `infra/projection/agui/` | `modules/chat/projection/agui/` | Pending |
| `infra/projection/llmcontext/` | `modules/chat/projection/llmcontext/` | Pending |
| `infra/projections/` | `modules/chat/projection/` | Pending |
| `domain/thread_aggregate.go` | `modules/chat/` (backward alias in `domain/`) | Pending |
| `domain/thread_repository.go` | `modules/chat/` (backward alias in `domain/`) | Pending |
| `domain/events.go` | `modules/chat/` (backward alias in `domain/`) | Pending |
| `domain/events_registry.go` | `modules/chat/` (backward alias in `domain/`) |

### Sub-phase 5c: Remaining `services/` and `handlers/` cleanup (future)

After chat extraction, the old directories will contain only:

| Directory | Remaining | Disposition | Status |
|-----------|-----------|-------------|--------|
| `handlers/auth/` | Auth middleware + handlers | Stays (shared infrastructure) | — |
| `handlers/health/` | Health check endpoints | Stays (infrastructure) | — |
| `handlers/workspacepermissions/` | Delegate for permissions module | Inlined into `modules/permissions/` | ✅ Done |
| `services/artifact/` | S3 adapter | Move to `infra/artifact/` | Pending |
| `services/auth/` | Simple auth service | Move to `shared/` or inline | Pending |
| `services/interfaces.go` | Shared interfaces | Move to `shared/` | Pending |
| `services/permissions/` | Tool permissions | Keep or move to `infra/` | Pending |
| `services/toolregistry/` | Plugin registry | Keep or move to `infra/` | Pending |
| `services/workspacepermissions/` | Workspace permissions | Keep | — |

### Constraints

All constraints from ADRs 012–013 remain in effect, plus:

1. **Two-phase construction** — `SetRunner()` must be called before `RegisterRoutes()`. The module panics if routes are registered without a runner.
2. **Orchestrator exposure** — The module exposes `Orchestrator()` so `main.go` can create the runner adapter with cross-module dependencies (agent provider, AI settings, knowledge service, etc.).
3. **Shared services stay shared** — `artifact`, `permissions`, `toolregistry`, `workspacepermissions` are NOT absorbed into the chat module because they serve the runner adapter and other modules.
4. **Statistics projection stays in `infra/`** — It depends on domain events but is owned by the analytics module's read path, not the chat module.

## Consequences

### Positive

- **`main.go` shrinks by ~100 lines** — all chat construction and route registration moves into `modules/chat/`.
- **Single directory for chat** — developers working on chat features have a clear entry point (`modules/chat/chat.go`).
- **`handlers/` nearly empties** — only `auth/` (shared) and `health/` (infrastructure) remain.
- **Clear extraction path** — Sub-phase 5b can relocate files incrementally without changing the public API.
- **Testability** — The module's `Dependencies` struct makes it easy to create a test instance with mocked dependencies.

### Negative

- **Large `Dependencies` struct** — The chat module has 15 injected dependencies, reflecting the domain's inherent complexity. This is a code smell but accurately represents the bounded context's needs.
- **Two-phase construction** — `SetRunner()` adds a temporal coupling that must be documented and enforced.
- **Facade indirection** — Until Sub-phase 5b, the module is a thin wrapper over existing packages, adding one layer of indirection.

### Risk Mitigation

- Run `go build ./...` and `go test ./...` after each change.
- Keep the old packages intact during Sub-phase 5a — the facade only wraps them.
- Document the `New() → SetRunner() → RegisterRoutes()` sequence in the module's godoc.
