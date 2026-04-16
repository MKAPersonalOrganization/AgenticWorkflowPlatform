# Architecture Misalignment Report

**Date:** 2026-04-16
**Scope:** Full structural and dependency analysis of the SchoolWithAI modular monolith
**Audience:** Senior Go engineers and frontend developers
**Goal:** Identify all visible misalignments between the intended architecture and the current implementation, with actionable fixes

---

## Table of Contents

1. [Intended Architecture](#1-intended-architecture)
2. [Misalignment 1 — Cross-Module Import: `templates` depends on `agentdefinition`](#misalignment-1--cross-module-import-templates-depends-on-agentdefinition)
3. [Misalignment 2 — TenantContext in wrong bounded context](#misalignment-2--tenantcontext-in-wrong-bounded-context)
4. [Misalignment 3 — Infrastructure layer imports UI components](#misalignment-3--infrastructure-layer-imports-ui-components)
5. [Misalignment 4 — Presentation layer imports business logic](#misalignment-4--presentation-layer-imports-business-logic)
6. [Misalignment 5 — Dead `AuthService` interface creates phantom abstraction](#misalignment-5--dead-authservice-interface-creates-phantom-abstraction)
7. [~~Misalignment 6 — God Handler in `chat/taskmgmt`~~](#misalignment-6--god-handler-in-chattaskmgmt) **RESOLVED**
8. [~~Misalignment 7 — Foundation test files import from plugins and infra~~](#misalignment-7--foundation-test-files-import-from-plugins-and-infra) **RESOLVED**
9. [Misalignment 8 — `adapters.go` composition root accumulating domain logic](#misalignment-8--adaptergo-composition-root-accumulating-domain-logic)
10. [Misalignment 9 — Plugin initialization pattern inconsistency](#misalignment-9--plugin-initialization-pattern-inconsistency)
11. [Priority Matrix](#priority-matrix)

---

## 1. Intended Architecture

Based on ADR-012 (Modular Monolith Migration), ADR-014 (Chat Module Extraction), ADR-015 (Zero Leaf Files), and the project's directory structure, the intended layering is:

```
┌─────────────────────────────────────────────────────────────┐
│  Composition Root (main.go, adapters.go)                    │
│  May import everything. Wires modules together.             │
└────────────────────────────┬────────────────────────────────┘
                             │
     ┌───────────────────────┼───────────────────────┐
     ▼                       ▼                       ▼
┌──────────┐          ┌──────────┐            ┌──────────┐
│ modules/ │          │ plugins/ │            │adapters/ │
│ (domains)│          │ (tools)  │            │(LLM glue)│
└────┬─────┘          └────┬─────┘            └────┬─────┘
     │                     │                       │
     │    ┌────────────────┼───────────────────────┘
     ▼    ▼                ▼
┌──────────────┐    ┌─────────────┐    ┌──────────────┐
│ foundation/  │    │   shared/   │    │components/   │
│ (framework)  │    │  (kernel)   │    │(presentation)│
└──────┬───────┘    └─────────────┘    └──────────────┘
       │
       ▼
┌──────────────┐
│    infra/    │
│(external I/O)│
└──────────────┘
```

**Dependency rules (from ADR-012):**

| Layer | May import from | Must NOT import from |
|-------|-----------------|----------------------|
| `shared/` | standard library only | everything in the project |
| `foundation/` | `shared/`, standard lib, third-party | `modules/`, `plugins/`, `infra/`, `adapters/`, `components/` |
| `infra/` | `foundation/`, `shared/` | `modules/`, `plugins/`, `adapters/`, `components/` |
| `modules/` | `foundation/`, `shared/`, `infra/` | other modules, `plugins/`, `adapters/` |
| `plugins/` | `foundation/`, `shared/` | `modules/`, `infra/`, `adapters/`, other plugins |
| `adapters/` | `foundation/`, `shared/`, `modules/` (interfaces) | `infra/`, `components/` |
| `components/` | `foundation/`, `shared/`, other components | `modules/`, `infra/`, `adapters/`, `plugins/` |
| Composition root | everything | N/A |

Every misalignment below is a violation of these rules or of the modular monolith's bounded-context isolation.

---

## Misalignment 1 — Cross-Module Import: `templates` depends on `agentdefinition`

### What is wrong

The `modules/templates` module imports concrete types from `modules/agentdefinition` in 5 files. This is a direct cross-module dependency that violates the "zero cross-module imports" constraint from ADR-012.

### Affected files

| File | Import |
|------|--------|
| `modules/templates/types.go:4` | `aicompass/modules/agentdefinition` |
| `modules/templates/catalog_loader.go:4` | `aicompass/modules/agentdefinition` |
| `modules/templates/catalog_repo.go:4` | `aicompass/modules/agentdefinition` |
| `modules/templates/handler.go:5` | `aicompass/modules/agentdefinition` |
| `modules/templates/module.go:6` | `aicompass/modules/agentdefinition` |

### Types consumed across the boundary

- `agentdefinition.AgentCatalogEntry` (struct) — used in repository interfaces, loader, and handler
- `agentdefinition.CatalogSubAgent` (struct) — used when parsing workflow YAML definitions
- `agentdefinition.AgentDefinitionRepository` (interface) — used for naming-conflict checks and forking

### Why this matters

When `templates` imports concrete types from `agentdefinition`, changes to the `AgentCatalogEntry` struct or `AgentDefinitionRepository` interface in `agentdefinition` will break the `templates` module. This couples the release and testing of two bounded contexts that should evolve independently. If these modules were ever extracted into separate services, this dependency would require shared libraries or API contracts.

### How to fix

**Option A (recommended): Move shared types to `shared/` kernel.**
`AgentCatalogEntry` and `CatalogSubAgent` are data-transfer types used at the boundary between two modules. They belong in the shared kernel:

1. Create `shared/catalog.go` with the `AgentCatalogEntry` and `CatalogSubAgent` structs
2. Update `modules/agentdefinition` to import from `shared/`
3. Update `modules/templates` to import from `shared/`
4. The `AgentDefinitionRepository` interface used by `templates` should be redefined locally inside `modules/templates/types.go` with only the methods it actually needs (Dependency Inversion Principle)

**Option B: Define local interfaces in `templates`.**
Instead of importing the concrete repository, define a narrow `AgentCreator` interface in `templates` that exposes only `Create()` and `GetByName()`. Wire the implementation at the composition root. This leaves the data types still shared but removes the interface coupling.

**Estimated effort:** 2-3 hours

---

## Misalignment 2 — TenantContext in wrong bounded context

### What is wrong

`TenantContext` was recently moved from `shared/` to `modules/knowledge/tenant/` (commit `8821c04a`). The commit message states it was "only used by the knowledge module (99%)." However, 3 other modules still import it, creating cross-module dependencies:

| File | Import |
|------|--------|
| `modules/chat/chatcontracts/knowledge.go:4` | `aicompass/modules/knowledge/tenant` |
| `modules/chat/taskmgmt/document_handler.go:5` | `aicompass/modules/knowledge/tenant` |
| `modules/permissions/provider.go:4` | `aicompass/modules/knowledge/tenant` |
| `modules/permissions/workspace_handler.go:6` | `aicompass/modules/knowledge/tenant` |
| `adapters.go` (composition root) | `aicompass/modules/knowledge/tenant` |

### Why this matters

The `TenantContext` type carries multitenancy state: `TenantID`, `WorkspaceID`, `UserID`, and `Role`. This is a cross-cutting concern, not a knowledge-domain concept. By placing it inside `modules/knowledge/`, we force every module that interacts with knowledge (chat, permissions) to import from knowledge's internal package. This:

- Violates bounded-context isolation — `permissions` defines a public `WorkspaceProvider` interface that depends on `knowledge/tenant`, meaning any implementor must also import from knowledge
- Creates a false ownership signal — the type appears to be owned by knowledge, but chat and permissions instantiate `TenantContext` objects directly
- Makes the knowledge module a mandatory dependency for unrelated concerns

### How to fix

**Move `TenantContext` back to `shared/` (or to `foundation/`).**

The type is dependency-free (only imports `github.com/google/uuid`), small (22 lines), and used across 3+ module boundaries. It fits the shared-kernel definition perfectly.

1. Create `shared/tenant.go` with `TenantContext`, `EdgequakeRole`, and the role constants
2. Update all 47+ import sites from `aicompass/modules/knowledge/tenant` to `aicompass/shared`
3. Delete `modules/knowledge/tenant/` directory

**Estimated effort:** 1 hour (mostly mechanical import updates)

---

## ~~Misalignment 3 — Infrastructure layer imports UI components~~ RESOLVED

Resolved in commit `4cd45612`. Replaced `ErrorPage` (rendered HTML directly) with `ErrorPageData` (returns data struct). Rendering moved to the route-wiring layer in `main.go`. The `infra/googleauth` package no longer imports from `components/`.

---

## ~~Misalignment 4 — Presentation layer imports business logic~~ RESOLVED

Resolved in commit `c45a88b4`. Created local view-model types in `components/knowledge/viewmodels.go` (`WorkspaceViewModel`) and `components/analytics/viewmodels.go` (`StatRow`, `TimeSeriesPoint`, `ToolSummary`). Updated `.templ` source files to use local types. Module handlers now map domain types to view models before rendering. Zero `aicompass/modules/` imports remain in `components/`.

---

## ~~Misalignment 5 — Dead `AuthService` interface~~ RESOLVED

Resolved in commit `357f55fd`. Deleted `shared/auth_service.go`, removed `contextAuthService` from `main.go`, replaced all `h.authService.GetUserFromContext()` calls with `shared.GetUserFromContext()` across 13 files.

---

## ~~Misalignment 6 — God Handler in `chat/taskmgmt`~~ RESOLVED

Resolved in current session. Refactored the 1,848-line God Handler into 8 domain-specific handlers under `modules/chat/handlers/`: `messagehandler`, `threadhandler`, `agenthandler`, `artifacthandler`, `documenthandler`, `requirementhandler`, `pluginhandler`, and `toolsethandler`. Shared utilities and SSE logic were moved to a new `handlers/common` package. Each handler now only carries its necessary dependencies, significantly improving maintainability and testability.

---

## ~~Misalignment 7 — Foundation test files import from plugins and infra~~ RESOLVED

Resolved in current session. Decoupled `foundation/toolregistry` unit tests from `plugins/calculator` by introducing a local `testPlugin` mock in `shared_test.go`. Isolated database-dependent repository tests into `integration_test.go` protected by an `//go:build integration` tag. Standard unit tests now run in isolation without cross-layer imports or database requirements.

---

## Misalignment 8 — `adapters.go` composition root accumulating domain logic

### What is wrong

This misalignment has been addressed.

The domain-heavy adapters were relocated out of the composition root:

| Type | Lines | Concern |
|------|-------|---------|
| `modules/agentdefinition.UserAwareProvider` | ~190 lines | Loads agents from DB, resolves coordinator sub-agents, builds workflow agents |
| `adapters/adkgo.KnowledgeBasePromptInjector` | ~35 lines | Resolves workspace permissions and injects KB instructions into ADK prompts |
| `adapters/filesystembridge.FSPluginAdapter` | ~100 lines | Maps filesystem module types to plugin-facing filesystem DTOs |

### Why this matters

The composition root should contain only wiring — constructing dependencies and passing them to constructors. That is now the case:

- `adapters.go` now contains only thin composition glue
- the moved adapters are in packages where their responsibilities are discoverable
- unit tests now exist for the moved agent provider and prompt injector

### How to fix

Implemented by relocating the domain-heavy adapters to proper packages:

1. Moved `userAwareAgentProvider` to `modules/agentdefinition/provider.go` as `UserAwareProvider`
2. Moved `knowledgeBasePromptInjector` to `adapters/adkgo/prompt_injector.go` as `KnowledgeBasePromptInjector`
3. Moved the filesystem bridge to `adapters/filesystembridge/fsplugin_adapter.go` as `FSPluginAdapter`
4. Kept only the thin Edgequake, profile, and A2A adapters in `adapters.go`

`main.go` now wires these adapters via exported constructors instead of owning their implementations.

**Status:** Fixed

---

## Misalignment 9 — Plugin initialization pattern inconsistency

### What is wrong

This misalignment has been addressed.

All 15 plugins are now registered as empty struct pointers:

```go
// Standard pattern (15 plugins)
toolregistry.RegisterPlugin(&calculator.Provider{})
toolregistry.RegisterPlugin(&artifacts.Provider{})
// ... etc
toolregistry.RegisterPlugin(&websearch.Provider{})
toolregistry.RegisterPlugin(&fsplugin.Provider{})
```

### Why this matters

The `PluginProvider` interface from `foundation/toolregistry` was designed so that plugins receive their dependencies lazily via `FactoryDeps` when `CreateExecutable()` is called. That pattern is now used consistently.

The implementation now does this:

- `websearch.Provider` resolves its client factory from `deps.GroundedSearchClientFactory()` inside `CreateExecutable()`
- `fsplugin.Provider` resolves its filesystem dependency from `deps.AgentFilesystem()` inside `CreateExecutable()`
- `main.go` registers both plugins with the standard `&Provider{}` pattern and wires their concrete dependencies into `ExecutableToolFactory`

### How to fix

Implemented via the `FactoryDeps` lazy-injection pattern.

For `websearch`:
1. Removed the `ClientFactory` field and `NewProvider()` constructor
2. Added `GroundedSearchClientFactory()` to `toolregistry.FactoryDeps`
3. Resolved the factory inside `CreateExecutable()`
4. Registered as `toolregistry.RegisterPlugin(&websearch.Provider{})`

For `fsplugin`:
1. Removed the `FS` field from `Provider`
2. Added `AgentFilesystem()` to `toolregistry.FactoryDeps`
3. Resolved the filesystem inside `CreateExecutable()`
4. Registered as `toolregistry.RegisterPlugin(&fsplugin.Provider{})`

Note: `FactoryDeps` exposes these two dependencies as cycle-safe `any`-typed getters. This keeps `foundation/toolregistry` free of imports from `plugins/websearch` and `plugins/fsplugin`, while preserving lazy injection and a uniform provider lifecycle.

**Status:** Fixed

---

## Priority Matrix

| # | Misalignment | Severity | Effort | Impact on architecture characteristics |
|---|-------------|----------|--------|---------------------------------------|
| 2 | TenantContext in wrong bounded context | **Critical** | 1 hour | Blocks independent module evolution; violates bounded-context isolation across 3 modules |
| ~~3~~ | ~~Infra imports UI components~~ | ~~Critical~~ | ~~30 min~~ | **RESOLVED** (commit `4cd45612`) |
| ~~5~~ | ~~Dead AuthService interface~~ | ~~High~~ | ~~30 min~~ | **RESOLVED** (commit `357f55fd`) |
| 4 | Presentation imports business logic | **High** | 2 hours | Couples UI templates to domain models; blocks independent frontend development |
| 1 | templates depends on agentdefinition | **High** | 2-3 hours | Cross-module coupling; blocks independent module testing and evolution |
| ~~6~~ | ~~God Handler in chat/taskmgmt~~ | ~~High~~ | ~~4-6 hours~~ | **RESOLVED** (refactored into 8 focused handlers) |
| ~~7~~ | ~~Foundation tests import plugins/infra~~ | ~~Medium~~ | ~~1 hour~~ | **RESOLVED** (mocked plugins, isolated integration tests) |
| 8 | adapters.go accumulating domain logic | **Medium** | 3-4 hours | Untestable domain logic in composition root; growing maintenance burden |
| 9 | Plugin init pattern inconsistency | **Low** | 2 hours | Developer confusion; 3 patterns for same concern; minor extensibility friction |

### Recommended execution order

**Phase 1 — Quick wins (Week 1):** Items 2, 3, 5 — all under 1 hour each, highest architectural impact
**Phase 2 — Boundary enforcement (Weeks 2-3):** Items 4, 1, 7 — enforce layer and module isolation
**Phase 3 — Structural refinement (Weeks 4-6):** Items 6, 8, 9 — refactor internal structure for maintainability

---

## How to verify fixes

After addressing each misalignment, run these checks:

```bash
# Check for cross-module imports (should return zero results)
grep -r '"aicompass/modules/' modules/ --include='*.go' | grep -v '_test.go' | \
  awk -F: '{split($1,a,"/"); split($2,b,"\""); if (a[3] != "" && b[2] != "") { \
    mod=a[3]; imp=b[2]; split(imp,c,"/"); if (c[3] != mod) print $0 }}'

# Check infra does not import components
grep -r '"aicompass/components' infra/ --include='*.go'

# Check components do not import modules
grep -r '"aicompass/modules' components/ --include='*.templ'

# Check foundation does not import modules, plugins, or infra (excluding tests)
grep -r '"aicompass/modules\|aicompass/plugins\|aicompass/infra' foundation/ --include='*.go' | grep -v '_test.go'

# Check shared has no project imports
grep -r '"aicompass/' shared/ --include='*.go'
```

All commands should return empty output when the architecture is fully aligned.
