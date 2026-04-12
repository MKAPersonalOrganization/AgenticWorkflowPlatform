# ADR 013: Modular Monolith Migration — Phase 2: Remaining Modules

## Status

Accepted (continues ADR 012)

## Context

ADR 012 completed the first wave of module extractions (Phase 1) and the shared kernel work (Phases 2–3). Six modules now live under `modules/`:

| Module | Status |
|--------|--------|
| `billing` | ✅ Extracted |
| `aisettings` | ✅ Extracted |
| `resources` | ✅ Extracted |
| `mcp` | ✅ Extracted |
| `knowledge` | ✅ Extracted |

The following code still lives in the old layered directories (`handlers/`, `services/`, `infra/`):

| Handler | Service | Infra/Domain |
|---------|---------|--------------|
| `handlers/preferences/` | — | `infra/sqldb/user_preference_repo.go`, `domain/user_preference.go` |
| `handlers/auth/`, `handlers/profile/` | `services/auth/` | `infra/sqldb/student_profile_repo.go`, `domain/student_profile.go`, `infra/googleauth/` |
| `handlers/statistics/` | — | `infra/projection/statistics/` |
| `handlers/banner/` | — | — |
| `handlers/resourcepermissions/`, `handlers/workspacepermissions/` | `services/resourcepermissions/`, `services/workspacepermissions/`, `services/permissions/` | — |
| `handlers/agui/` | `services/chatread/`, `services/agentorchestrator/`, `services/taskmanagement/`, `services/threadmgmt/`, `services/memory/`, `services/artifact/` | `infra/projection/agui/`, `infra/projection/llmcontext/`, `domain/thread_*` |
| `handlers/taskmgmt/` | (same as above) | (same as above) |
| `handlers/health/` | — | — |

Additionally, `main.go` (925 lines) manually constructs handlers and registers routes inline for all un-modularized code, making it hard to reason about module boundaries.

## Decision

Continue the modular-monolith migration with three additional phases, extracting modules in order of ascending dependency complexity.

### Phase 4: Extract supporting modules (low cross-domain coupling)

| Order | Module | Source Files | Cross-Domain Deps | Notes | Status |
|-------|--------|-------------|-------------------|-------|--------|
| 7 | **preferences** | `handlers/preferences/`, `infra/sqldb/user_preference_repo.go`, `domain/user_preference.go` | None | Move domain type + repo into module. Simplest leaf. | ✅ Done |
| 8 | **identity** | `handlers/profile/`, `infra/sqldb/student_profile_repo.go`, `domain/student_profile.go` | `infra/googleauth/` (shared infra — stays) | Profile page + student profile CRUD. Auth middleware stays shared in `handlers/auth/`. `StudentProfile` stays in `domain/` (import cycle via `components/profile`). | ✅ Done |
| 9 | **analytics** | `handlers/statistics/`, `infra/projection/statistics/` | Statistics projection (stays in `infra/projection/statistics/` — depends on domain events) | Self-contained; owns its own basic auth middleware. | ✅ Done |
| 10 | **banner** | `handlers/banner/` | None | Trivial — one handler, one type. | ✅ Done |
| 11 | **permissions** | `handlers/workspacepermissions/`, `services/workspacepermissions/` | `knowledge` module (edgequake), `agent.AgentProvider` | Workspace permissions. Resource permissions handler not yet wired in `main.go` — will be added when resource perm routes go live. | ✅ Done |

#### Module Structure Convention

Each new module follows the established pattern from Phase 1:

```go
// modules/<name>/<name>.go
type Module struct { ... }
func New(log *logger.Logger, deps ...) *Module
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware)
```

### Phase 5: Extract core chat domain (future — not in this ADR's scope)

The chat/taskmgmt/agui bounded context is the most interconnected piece of the system. It depends on nearly every other module and contains the core event-sourced thread aggregate. Extracting it requires:

- Moving `handlers/taskmgmt/`, `handlers/agui/`, `services/agentorchestrator/`, `services/taskmanagement/`, `services/threadmgmt/`, `services/memory/`, `services/artifact/`, `services/chatread/` into `modules/chat/`
- Moving `infra/projection/agui/`, `infra/projection/llmcontext/` into the module
- Moving `domain/thread_aggregate.go`, `domain/thread_repository.go`, and thread events

This will be tracked separately as it requires careful dependency-cycle analysis.

### Phase 6: Domain type localisation (parallel with Phase 4)

As modules are extracted, their domain types move from `domain/` into the module package. Backward-compatible aliases remain in `domain/` during the transition:

| Type | Current Location | Target Module |
|------|-----------------|---------------|
| `UserPreference`, `UserPreferenceRepository` | `domain/user_preference.go` | `modules/preferences/` |
| `StudentProfile`, `StudentProfileRepository` | `domain/student_profile.go` | `modules/identity/` |
| `MCPConnector` | `domain/mcp_connector.go` | `modules/mcp/` (already done) |
| `AISettings` | `domain/ai_settings.go` | `modules/aisettings/` (already done) |

### Constraints

All constraints from ADR 012 remain in effect:

1. **Modules must NOT import other modules** — cross-module communication uses interfaces or events.
2. **One module at a time** — extract, update `main.go`, run tests, commit.
3. **Backward-compatible aliases** — `domain/` keeps type aliases during transition so external consumers (e.g., `foundation/agentsdk`) don't break.
4. **`infra/googleauth/` stays shared** — it's a cross-cutting authentication adapter, similar to `infra/sqldb/`.
5. **`handlers/health/` stays in `handlers/`** — health checks are infrastructure-level, not a domain module. They may optionally move to a `modules/health/` later.

### Execution Order

Phase 4 modules are extracted in dependency order:

1. `preferences` — zero deps, simplest leaf
2. `identity` — depends only on shared infra (`googleauth`)
3. `analytics` — self-contained with own projection
4. `banner` — trivial, zero deps
5. `permissions` — depends on `resources` + `knowledge` modules (via interfaces)

After each extraction:
- Update `main.go` to use `module.RegisterRoutes()`
- Run `go build ./...` and `go test ./...`
- Commit

## Consequences

### Positive

- **`handlers/` directory empties out** — only `health/` remains (infrastructure-level).
- **`services/` directory shrinks** — only chat-related services remain (Phase 5 scope).
- **`domain/` continues shrinking** — only thread aggregate + events remain.
- **`main.go` simplifies** — inline handler construction replaced by `module.New()` + `module.RegisterRoutes()`.
- **Clear ownership** — every feature has a single directory home.

### Negative

- **Transition period complexity** — During migration, some code is modularised and some isn't.
- **Type alias maintenance** — Backward-compatible aliases in `domain/` add a small maintenance burden until all consumers migrate.
- **Permissions module has cross-module deps** — Even with interfaces, the permissions module needs careful dependency management.

### Risk Mitigation

- Extract one module at a time with full test coverage between each step.
- Keep backward-compatible aliases until all imports are migrated.
- Use architecture tests (import linting) to enforce module boundaries.
