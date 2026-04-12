# ADR 016: Accidental Complexity Retrospective — Lessons from the Modular Monolith Refactoring

## Status

Accepted — all nine problems resolved

## Context

ADRs 012–015 migrated the codebase from a layered architecture to a modular monolith across six months and hundreds of commits. The migration achieved its primary goal — twelve bounded-context modules, no module-to-module imports, a frozen shared kernel — but the journey exposed **accidental complexities** and **structural debt** that were not anticipated in the original ADR plans.

This ADR is a post-mortem. Its goal is to catalogue every unnecessary complication encountered, diagnose the root cause of each, and define targeted fixes that increase **cohesion within modules** and reduce **coupling between layers**.

## Problems Encountered

### Problem 1: The Alias Graveyard in `domain/`

**What happened:** During earlier migration phases (ADR 012–013), types were moved out of `domain/` into modules, but backward-compatible **type aliases** were left behind in `domain/` "in case something still imports them." Over time, every consumer was updated, but nobody deleted the aliases. By the time ADR 015 audited `domain/`, five alias files had **zero consumers** — pure dead weight that confused every new reader of the codebase.

**Root cause:** No automated enforcement. The migration plan said "leave aliases temporarily," but there was no CI check to flag when an alias's consumer count hit zero. Dead code accumulated silently.

**Accidental complexity cost:** Developers had to mentally resolve `domain.UserPreference` → `modules/preferences.UserPreference` → `domain.UserPreference` (alias) to understand that three names all pointed to the same type. The `domain/` package appeared to own 20 files when it really only owned 7.

**Fix applied (ADR 015 WS1):** Deleted all five alias files and the orphaned `infra/rag/` tree. Build + tests passed immediately — proof they were dead.

**Lesson:** *Aliases are technical debt with an expiry date. Add a `// TODO(cleanup): delete after YYYY-MM-DD if no external consumers` comment and a CI grep that flags expired TODOs.*

---

### Problem 2: `domain/` as a Gravity Well

**What happened:** The original layered architecture placed _all_ domain types in `domain/`. During the modular migration, shared-kernel types (thread aggregate, events) legitimately stayed, but **module-private types** (`AgentCatalogEntry`, `AISettings`, `MCPConnector`) also stayed because "moving them is risky." The result: `domain/` became a gravity well — every new developer's first instinct was to add types there because that's where types already lived.

**Root cause:** The migration lacked a bright-line rule about what may live in `domain/`. ADR 012 said "types remain in domain/ for Phase 1" but never defined when Phase 1 ended. Without a forcing function, inertia won.

**Accidental complexity cost:**
- `modules/aisettings/` imported `domain/` for 57 symbols across 3 files — every type it used was its own private domain.
- `modules/mcp/` imported `domain/` for connector types used by no other module.
- Each module appeared tightly coupled to a shared package when in reality the types were private.

**Fix applied (ADR 015 WS2):** Moved `ai_settings.go` → `modules/aisettings/`, `mcp_connector.go` → `modules/mcp/`. Each module now owns its types — zero `domain/` import for private types.

**Remaining debt:** Re-analysed in ADR 016 — both `AgentDefinition` (29 references, 5+ packages) and `StudentProfile` (16 references, 3+ packages across 2+ layers) meet the shared-kernel admission threshold. They stay in `domain/` by design. Comments updated in `domain/student_profile.go` and `modules/identity/types.go` to document this decision.

**Lesson:** *Define a package admission policy on day one: "only types with 3+ consumers across 2+ layers may enter `domain/`." Enforce it in code review.*

---

### Problem 3: Repository Adapters Stranded in `infra/sqldb/`

**What happened:** The layered architecture placed all Postgres repository implementations in `infra/sqldb/`. During the module migration (ADR 012–013), module-internal repos moved into their modules (e.g., `modules/billing/repo.go`), but three repo files serving `services/` packages were left behind because "they're infra, not domain."

**Root cause:** A false distinction between "infrastructure" (where the code physically lives) and "ownership" (who depends on and maintains the code). The `infra/sqldb/` directory was treated as a physical container rather than a semantic boundary.

**Accidental complexity cost:**
- `services/permissions/` defined the `Repository` interface but the implementation lived in `infra/sqldb/permissions_repo.go`. Reading the service required looking in two directories.
- `main.go` had to import both `services/permissions` and `infra/sqldb` and wire them together — a manual dependency injection step that exists only because the impl and interface were separated.
- Test files in `infra/sqldb/` tested `services/permissions` types but belonged to `package sqldb`, requiring qualified type names (`permissions.Permission`) that would be unnecessary if the test lived in the service package.

**Fix applied (ADR 015 WS3):** Moved all three repo files + tests into their owning `services/` packages. `infra/sqldb/` now contains only `sqldb.go` (connection pool) and `migrate/` — genuinely shared infrastructure.

**Lesson:** *A repository implementation belongs next to its interface. "Infra" is not a package — it's a concern. Co-locate implementation with the abstraction it fulfils.*

---

### Problem 4: The `components/` → `domain/` Import Cycle

**What happened:** `components/profile/profile_page_templ.go` imports `domain.StudentProfile` directly, so the type cannot move to `modules/identity/` without creating `modules/identity/` → `components/profile/` → `modules/identity/` (cycle). The `modules/identity/types.go` file contains aliases as a workaround.

**Root cause:** Templ components were designed as "smart" — they accept domain types as props. This creates a downward dependency from the presentation layer into the domain layer, then back up when a module wants to own its types. In a modular monolith, **components should accept plain DTOs or primitives**, not domain types.

**Accidental complexity cost:**
- `StudentProfile` is trapped in `domain/` despite being owned by `modules/identity/`.
- `modules/identity/types.go` exists solely as an alias shim — 16 lines of boilerplate that does nothing but paper over the cycle.
- Any developer adding a new profile field must update `domain/student_profile.go` (wrong package) instead of `modules/identity/` (right package).

**Fix applied:** Refactored `components/profile/` to define a local `StudentProfileDTO` struct as a DTO. The `ProfilePageProps.Profile` field now uses this DTO instead of `domain.StudentProfile`. The handler in `modules/identity/` maps domain → DTO before rendering. This broke the cycle and eliminated the `domain/` import from the component package.

**Remaining debt:** `StudentProfile` stays in `domain/` as shared kernel — it has 16 references across 3+ packages in 2+ layers, meeting the shared-kernel admission threshold. The `modules/identity/types.go` alias shim was updated to document this decision.

**Lesson:** *Templ components are the view layer. They must never import domain types. Define prop structs in the component package and let the handler (inside the module) do the mapping.*

---

### Problem 5: `foundation/agentsdk/` — The God Adapter

**What happened:** `foundation/agentsdk/adapters/custom/runner.go` is a 722-line file that imports:
- `domain/` (shared kernel — acceptable)
- `modules/chat/threadmgmt` (module!)
- `modules/knowledge/edgequake` (module!)
- `modules/mcp` (module!)
- `services/artifact`, `services/memory`, `services/permissions`, `services/toolregistry`, `services/workspacepermissions` (5 services)
- `plugins/memory` (plugin!)
- `shared/`

A package in `foundation/` importing 3 modules, 5 services, and 1 plugin is a **dependency inversion violation**. Foundation is supposed to be a low-level framework layer that modules depend on — not the other way around.

**Root cause:** The runner adapter was written as a single concrete class that orchestrates LLM calls, tool execution, thread management, MCP proxy calls, and permission checks. Instead of depending on abstractions (interfaces), it reaches directly into modules for their concrete types. This made it easy to write but turned it into a coupling bottleneck.

**Accidental complexity cost:**
- `AgentDefinition` cannot move out of `domain/` because `foundation/agentsdk/` consumes it. The type is semantically a domain concept, but physically trapped by the SDK import.
- Any change to `modules/mcp`, `modules/knowledge/edgequake`, or `modules/chat/threadmgmt` can break `foundation/agentsdk/` — the foundation layer is fragile.
- The runner cannot be tested in isolation; it needs the entire module graph.
- This single file is the primary reason the dependency graph has a cycle between `foundation/` and `modules/`.

**Fix applied:**
1. Defined **7 narrow interfaces** in `foundation/agentsdk/adapters/custom/interfaces.go`: `ThreadManager`, `EventProcessor`, `ToolPermissionChecker`, `WorkspacePermissionChecker`, `ToolSetFactory`, `MCPToolResolver`, plus a `NewToolSetFactoryFunc` closure type for per-request factory injection.
2. Rewrote `RunnerAdapter` and `RegistryAgentProvider` to depend on interfaces instead of concrete module/service/plugin types.
3. `main.go` (composition root) wires concrete implementations to the interfaces.
4. Result: **zero** imports from `modules/`, `platform/`, or `plugins/` in the adapter package. Only `domain/`, `foundation/*`, and `shared/` remain.

**Lesson:** *A framework layer must depend only on abstractions it defines. If a foundation package imports a module, the abstraction is in the wrong place.*

---

### Problem 6: `services/interfaces.go` — The Junk Drawer

**What happened:** `services/interfaces.go` contained `AuthService` and `ThreadQueryService` — two interfaces with nothing in common, placed in a shared file because "they're both interfaces used by services."

**Root cause:** Grouping by technical similarity ("interfaces") instead of by domain ownership ("who consumes this?").

**Accidental complexity cost:**
- `AuthService` is a cross-cutting concern (belongs in `shared/`), while `ThreadQueryService` is a chat-module-internal contract (belongs in `modules/chat/`). Placing them together obscured their different scopes.
- Any developer adding a new service interface would continue the pattern — growing the junk drawer.

**Fix applied (ADR 015 WS4):** `AuthService` → `shared/auth_service.go`, `ThreadQueryService` + `ThreadSummary` → `modules/chat/chatcontracts/contracts.go`. The file was deleted.

**Lesson:** *Never create a file named `interfaces.go` at a package root. Interfaces belong next to their consumer (Go convention) or next to their primary implementation.*

---

### Problem 7: `handlers/` — A Layer That Wasn't

**What happened:** After the module migration, `handlers/` contained only `auth/` (middleware) and `health/` (health check). Both are cross-cutting infrastructure, not domain handlers. But the directory name `handlers/` implied a layer in the layered architecture that no longer existed.

**Root cause:** The migration moved domain handlers into modules but left cross-cutting handlers behind, creating a vestigial directory that violated the new architecture's naming conventions.

**Fix applied (ADR 015 WS5):** Moved both to `foundation/auth/` and `foundation/health/`, deleted `handlers/`. The 12 modules that imported `handlers/auth` now import `foundation/auth`.

**Lesson:** *When a migration empties a directory of its semantic purpose, delete the directory immediately — don't wait for a cleanup phase. Empty shells attract new junk.*

---

### Problem 8: The 863-Line `main.go` Composition Root

**What happened:** `main.go` is 863 lines. It constructs every module, every service, every adapter, and wires all routes. It imports 40+ packages. Adding a new module requires editing `main.go` in 3+ places (construct, wire dependencies, register routes).

**Root cause:** The modular monolith migration focused on extracting modules but did not address the composition root. Each module exposes `New()` + `RegisterRoutes()`, but the wiring between modules (e.g., chat module needs the identity module's `StudentProfileRepo()`) happens in `main.go` through manual constructor calls.

**Accidental complexity cost:**
- The file is a merge conflict magnet — every feature branch touches `main.go`.
- The construction order is implicit and fragile (the AI settings module must be created before the runner adapter, which must be created before the chat module).
- Error handling for each module's construction is spread across 863 lines of sequential code.

**Fix applied:** Extracted three helpers from `main.go`: `buildRootRouter()` (creates the root chi router with global middleware and rate-limited sub-routers), `routeModules` struct (bundles all modules that register routes), and `registerRoutes()` (wires all module and page routes onto the app router). The routing block (~120 lines) is now separated from the construction/wiring concerns, reducing merge-conflict surface and improving readability.

---

### Problem 9: Cross-Cutting Services Without Clear Ownership

**What happened:** Five packages live under `services/` — `artifact/`, `memory/`, `permissions/`, `toolregistry/`, `workspacepermissions/`. They are consumed by multiple modules and plugins, which is why they were not absorbed into any single module. But `services/` as a directory name carries no architectural meaning — it's a leftover from the layered architecture.

**Root cause:** The migration needed a place for "multi-consumer utilities that aren't framework-level." Rather than defining what that place should be, the team reused the existing `services/` directory.

**Accidental complexity cost:**
- New developers ask: "What's the difference between `services/` and `foundation/`?" The answer — "services/ has business logic, foundation/ has framework plumbing" — is subjective and hard to enforce.
- `services/permissions/` and `modules/permissions/` coexist, causing confusion about which "permissions" is which.

**Fix applied:** Renamed `services/` → `platform/`. All import paths updated across ~40 files (`.go` and `.templ`). The `platform/` name clearly distinguishes cross-cutting infrastructure from module-internal services.

## Decision

Acknowledge these nine problems as the primary accidental complexities of the modular monolith migration. ADR 015 resolved problems 1, 2 (partially), 3, 6, and 7. This ADR (016) resolved the remaining problems — 2 (remainder), 4, 5, 8, and 9. All nine problems are now closed:

| Problem | Status | Resolution |
|---------|--------|------------|
| 1. Alias graveyard | ✅ Fixed (ADR 015 WS1) | Deleted all alias files |
| 2. `domain/` gravity well | ✅ Resolved | Module-private types moved (ADR 015 WS2). `AgentDefinition` and `StudentProfile` stay as shared kernel — 3+ consumers across 2+ layers justifies placement |
| 3. Stranded repo adapters | ✅ Fixed (ADR 015 WS3) | Moved repos into owning packages |
| 4. `components/` → `domain/` cycle | ✅ Fixed | `components/profile/` now uses local `StudentProfileDTO`; handler maps domain → DTO |
| 5. God Adapter in `foundation/agentsdk/` | ✅ Fixed | Extracted 7 narrow interfaces (`ThreadManager`, `ToolPermissionChecker`, `WorkspacePermissionChecker`, `ToolSetFactory`, `MCPToolResolver`, etc.). Zero module/service/plugin imports in the adapter package |
| 6. `services/interfaces.go` junk drawer | ✅ Fixed (ADR 015 WS4) | Interfaces moved to consumers |
| 7. Vestigial `handlers/` directory | ✅ Fixed (ADR 015 WS5) | Moved to `foundation/` |
| 8. 863-line `main.go` | ✅ Fixed | Extracted `buildRootRouter()`, `routeModules` struct, and `registerRoutes()` helpers — routing logic separated from composition |
| 9. `services/` naming confusion | ✅ Fixed | Renamed to `platform/` — all import paths updated |

## Design Principles Reinforced

These problems all stem from violations of three principles:

### 1. Cohesion: A module owns everything it needs

A module should contain its types, repository interfaces, repository implementations, handlers, service logic, and tests. When any of these live outside the module directory, the module is incomplete and the codebase requires cross-directory navigation to understand a single bounded context.

**Metric:** For any module, the ratio of `internal imports / total imports` should be > 0.5. If a module imports more external packages than internal files, its cohesion is too low.

### 2. Coupling: Dependencies point inward

The dependency rule for a modular monolith:

```
main.go (composition root)
  ↓ imports
modules/*  (bounded contexts)
  ↓ imports
shared/    (cross-cutting kernel: User, context helpers)
foundation/ (framework: SDK, eventsourcing, logger, middleware)
  ↓ imports
stdlib + third-party
```

**Violations found and resolved:**
- `foundation/agentsdk/` imported `modules/chat/`, `modules/mcp/`, `modules/knowledge/` → **Fixed (Problem 5):** replaced with narrow interfaces, dependency arrows flipped.
- `components/profile/` imported `domain/` trapping `StudentProfile` → **Fixed (Problem 4):** replaced with local DTO.

**Rule:** `foundation/` must **never** import `modules/`. If it needs a capability from a module, it must define an interface and let the module (or `main.go`) provide the implementation.

### 3. Locality: No file exists outside its owner's directory

Every `.go` file must be traceable to exactly one owner:
- A module → lives under `modules/<name>/`
- The shared kernel → lives in `domain/` (frozen) or `shared/`
- Framework infrastructure → lives in `foundation/`
- The composition root → `main.go`

If a file's owner is ambiguous ("it's in `infra/sqldb/` but only `services/permissions/` uses it"), the file is in the wrong place.

## Consequences

### Positive

- This retrospective creates a **shared vocabulary** for discussing architectural debt — "that's a Problem 5" is more precise than "the SDK has too many imports."
- The remaining four open problems have clear, scoped fixes that can each be a single ADR.
- The design principles give reviewers concrete criteria for rejecting PRs that introduce new coupling.

### Negative

- The retrospective identified nine problems — all now resolved across ADRs 015 and 016.
- Future maintenance requires vigilance to avoid re-introducing these patterns (especially the gravity-well and god-adapter anti-patterns).

## Priority Order for Remaining Work

All four remaining problems have been resolved:

1. **Problem 5** (God Adapter) — ✅ Extracted 7 narrow interfaces in `foundation/agentsdk/adapters/custom/interfaces.go`. Dependency arrows flipped: the adapter defines interfaces, modules satisfy them via `main.go` wiring. Zero module/service/plugin imports remain.
2. **Problem 4** (component import cycle) — ✅ `components/profile/` now defines a local `StudentProfileDTO` struct. The handler in `modules/identity/` maps `domain.StudentProfile` → DTO before rendering. The `domain/` import is gone from components.
3. **Problem 8** (main.go bloat) — ✅ Extracted `buildRootRouter()`, `routeModules` struct, and `registerRoutes()` helper functions. Routing logic (~120 lines) is now separated from the construction/wiring concerns.
4. **Problem 9** (services/ naming) — ✅ Renamed `services/` → `platform/`. All import paths updated across ~40 files.

**Problem 2** (domain gravity well) was re-analysed: `AgentDefinition` (29 references, 5+ packages) and `StudentProfile` (16 references, 3+ packages) are legitimately shared-kernel types. Moving them to modules would require either `domain/` importing `modules/` (illegal) or duplicating types across consumers. They stay in `domain/` by design, not by accident.
