# ADR 012: Migration from Layered Architecture to Modular Monolith

## Status

Accepted

## Context

The codebase is organised by **technical layer** — `handlers/`, `services/`, `infra/`, `domain/`, `plugins/`, `foundation/`. Every bounded context is scattered across these directories. For example, billing lives in four locations:

```
handlers/billing/handler.go
services/billing/service.go
infra/projection/payment/postgres_projection.go
domain/payment_aggregate.go, payment_events.go, payment_types.go, payment_repository.go
```

This layered layout has served us well during early development when the system was small and one team owned everything. However, as the system grows toward potential multi-team ownership, the current structure creates problems:

1. **Cognitive overhead** — Understanding the billing feature requires jumping across four top-level directories and mentally reconstructing the bounded context.
2. **Coupling via proximity** — All domain types share one package (`domain/`), making it easy to accidentally couple bounded contexts (e.g., billing events referencing thread types).
3. **Ownership ambiguity** — When billing is spread across `handlers/`, `services/`, `infra/`, and `domain/`, assigning code ownership to a team requires a complex CODEOWNERS file with path-level rules scattered everywhere.
4. **Extraction friction** — Extracting a bounded context into a separate microservice requires gathering files from four directories and untangling shared imports.

A **modular monolith** organises code by **domain boundary** instead of technical layer. Each module is a self-contained vertical slice that owns its handlers, services, domain logic, and infrastructure adapters. Modules communicate through well-defined interfaces, making future extraction into microservices straightforward.

## Decision

Migrate incrementally from layered architecture to a modular monolith. Each module is a Go package under `modules/` that owns its full vertical slice.

### Target Structure

```
modules/
  billing/                   # Billing bounded context
    module.go                # Public API: NewModule, RegisterRoutes, RegisterProjection
    handler.go               # HTTP handlers
    service.go               # Application service
    projection.go            # Read model projection
    types.go                 # Module-local types (AccountStatus, etc.)
  
  knowledge/                 # Knowledge base / RAG bounded context
    module.go
    handler.go
    service.go               # EdgeQuake REST facade
    ...
  
  identity/                  # Auth + profile bounded context
    module.go
    ...
  
  agentstore/                # Agent definitions + catalog
    module.go
    ...
  
  mcp/                       # MCP connector management
    module.go
    ...
  
  aisettings/                # Per-user AI configuration
    module.go
    ...
  
  resources/                 # User-uploaded resources
    module.go
    ...

shared/                      # Shared kernel (extracted from domain/)
  user.go                    # User type + context helpers
  event.go                   # Event interface + BaseAggregate + Repository
```

### Migration Phases

#### Phase 1: Extract leaf modules (zero cross-domain coupling) ✅
Order based on dependency analysis — fewest inbound dependencies first:

| Order | Module | Files to Move | Cross-Domain Deps |
|-------|--------|---------------|-------------------|
| 1 | **billing** | `handlers/billing/`, `services/billing/`, `infra/projection/payment/` | None |
| 2 | **aisettings** | `handlers/aisettings/`, `services/aisettings/` | None (but knowledge depends on it — extract before knowledge) |
| 3 | **resources** | `handlers/resources/`, `services/resources/` | None |
| 4 | **agentstore** | `handlers/agentstore/`, `handlers/agentcatalog/`, `services/agentstore/` | `services.AuthService` interface (trivially abstractable) |
| 5 | **mcp** | `handlers/mcp/`, `services/plugindiscovery/` | `toolregistry` (shared infra — use interface) |
| 6 | **knowledge** | `handlers/knowledge/`, `services/edgequake/`, `services/rag/` | `aisettings` (replace with interface) |

#### Phase 2: Extract core domain events ✅
Remove the `sealed()` constraint on `domain.Event` and allow modules to define their own events. Introduce a shared `eventsourcing` package in `foundation/` with the generic `Event` interface, `BaseAggregate`, `Repository[T]`, and `EventStore`. Each module defines its events locally.

#### Phase 3: Extract shared kernel ✅
Move `domain.User`, `domain.GetUserFromContext`, and other truly cross-cutting types into a `shared/` package. The `domain/` package shrinks to contain only the chat/thread bounded context (the core domain).

**Completed:** Created `shared/` package with `User`, `GetUserFromContext`, `SetUserInContext`, `IsAuthenticated`, `UserContextKey`, `TenantContext`, `EdgequakeRole`, and role constants (`RoleAdmin`, `RoleUser`, `RoleReadonly`). Backward-compatible type aliases remain in `domain/user.go` and `domain/edgequake_permissions.go`. All 110 consumer files migrated to import `shared/` directly. The 48 files that still import `domain/` do so only for thread/aggregate types or category-C module-specific types (AgentDefinition, AISettings, MCPConnector, StudentProfile, UserPreference) — zero shared-kernel references via `domain.` remain outside `domain/`.

### Module Public API Convention

Each module exposes a `Module` struct with a standard interface:

```go
// modules/billing/module.go
type Module struct { ... }

func New(log *logger.Logger, deps Dependencies) *Module

func (m *Module) RegisterRoutes(r chi.Router, authMiddleware AuthMiddleware)
func (m *Module) RegisterProjection(router ProjectionRouter)
```

The `Dependencies` struct contains only the module's required infrastructure — no cross-module service references. Cross-module communication uses interfaces defined by the consuming module (Dependency Inversion Principle).

### Shared Infrastructure

These stay outside modules as they're genuinely cross-cutting:

- `foundation/` — Agent SDK, event router, middleware, metrics, logging, OTel
- `infra/eventstore/` — PostgreSQL event store (shared by all event-sourced modules)
- `infra/sqldb/` — Database connection + migrations (shared)
- `infra/projection/subscriber/` — Projection updater + Redis broadcaster (shared)
- `infra/googleauth/` — Authentication (shared)

### Constraints

1. **Modules must NOT import other modules** — cross-module communication uses interfaces or events.
2. **Modules CAN import** `foundation/`, `domain/` (transitionally), and external libraries.
3. **`domain/` payment types remain temporarily** — the `sealed()` constraint on `domain.Event` prevents moving them to the billing module until Phase 2.
4. **Route registration stays in `main.go`** — modules provide `RegisterRoutes()` but `main.go` decides the URL prefix and middleware stack.
5. **Projection registration stays in `main.go`** — modules provide `RegisterProjection()` but `main.go` manages the projection updater lifecycle.

## Consequences

### Positive

- **Team ownership** — Each module can be owned by a team. CODEOWNERS becomes trivial: `modules/billing/ @payments-team`.
- **Cognitive locality** — Understanding billing means reading one directory.
- **Extraction path** — A module can be extracted to a microservice by replacing in-process calls with HTTP/gRPC.
- **Independent testing** — Module tests are self-contained.
- **Build caching** — Go's build cache works better with isolated packages.

### Negative

- **Transitional duplication** — During migration, some code lives in `modules/` and some in `handlers/`+`services/`, requiring careful coordination.
- **Shared kernel coupling** — `domain/` remains a shared dependency until Phase 2–3, creating a temporary coupling point.
- **Event deserializer complexity** — The central event deserializer must know about events from all modules until Phase 2 introduces module-level deserializers.
- **Migration effort** — Each module extraction requires updating imports in `main.go` and verifying all tests pass.

### Risk Mitigation

- Extract one module at a time, run full CI after each extraction.
- Keep old import paths as aliases temporarily if needed for smooth transition.
- Architecture tests can enforce "no cross-module imports" as modules are extracted.
