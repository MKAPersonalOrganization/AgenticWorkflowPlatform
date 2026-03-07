# ADR 016: Accidental Complexity Retrospective — Lessons from the Modular Monolith Refactoring

## Status

Proposed (retrospective on ADR 012 → 013 → 014 → 015)

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
- `modules/agentstore/` imported `domain/` for catalog types used by no other module.
- Each module appeared tightly coupled to a shared package when in reality the types were private.

**Fix applied (ADR 015 WS2):** Moved `agent_catalog.go` → `modules/agentstore/`, `ai_settings.go` → `modules/aisettings/`, `mcp_connector.go` → `modules/mcp/`. Each module now owns its types — zero `domain/` import for private types.

**Remaining debt:** Two types could not be moved:
1. **`AgentDefinition`** — consumed by `foundation/agentsdk/adapters/custom/` (see Problem 5).
2. **`StudentProfile`** — consumed by `components/profile/`, creating an import cycle if moved to `modules/identity/` (see Problem 4).

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

**Fix (future):** Refactor `components/profile/` to accept a `ProfilePageProps` struct (a DTO defined within the component package) rather than `domain.StudentProfile`. The module handler maps domain → DTO before rendering. This breaks the cycle and allows `StudentProfile` to move into `modules/identity/`.

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
- `AgentDefinition` cannot move out of `domain/` because `foundation/agentsdk/` consumes it. The type is semantically owned by `agentstore`, but physically trapped by the SDK import.
- Any change to `modules/mcp`, `modules/knowledge/edgequake`, or `modules/chat/threadmgmt` can break `foundation/agentsdk/` — the foundation layer is fragile.
- The runner cannot be tested in isolation; it needs the entire module graph.
- This single file is the primary reason the dependency graph has a cycle between `foundation/` and `modules/`.

**Fix (proposed):**
1. Define **interfaces** in `foundation/agentsdk/` for each capability it needs (tool execution, thread management, MCP discovery, permission checking, knowledge query).
2. Each module implements the interface — the dependency arrow flips from `foundation → module` to `module → foundation`.
3. `main.go` (the composition root) wires concrete implementations to the SDK interfaces.
4. `AgentDefinition` moves to `modules/agentstore/` and the SDK depends on a `AgentProvider` interface (which it already has — the concrete import is redundant).

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

**Fix (proposed):** Introduce a lightweight **module registry** pattern. Each module registers itself and declares its dependencies. The composition root iterates the registry, resolves dependencies, and calls `Init()` → `RegisterRoutes()` in topological order. This reduces `main.go` to ~200 lines and makes the construction order explicit and self-documenting.

---

### Problem 9: Cross-Cutting Services Without Clear Ownership

**What happened:** Five packages live under `services/` — `artifact/`, `memory/`, `permissions/`, `toolregistry/`, `workspacepermissions/`. They are consumed by multiple modules and plugins, which is why they were not absorbed into any single module. But `services/` as a directory name carries no architectural meaning — it's a leftover from the layered architecture.

**Root cause:** The migration needed a place for "multi-consumer utilities that aren't framework-level." Rather than defining what that place should be, the team reused the existing `services/` directory.

**Accidental complexity cost:**
- New developers ask: "What's the difference between `services/` and `foundation/`?" The answer — "services/ has business logic, foundation/ has framework plumbing" — is subjective and hard to enforce.
- `services/permissions/` and `modules/permissions/` coexist, causing confusion about which "permissions" is which.

**Fix (proposed):** Rename `services/` → `platform/` to distinguish it from module-internal services. Alternatively, absorb each into the module that most naturally owns it, exposing a contract interface for external consumers. For example, `services/permissions/` could become `modules/permissions/enforcement/` and export an interface consumed by `foundation/agentsdk/`.

## Decision

Acknowledge these nine problems as the primary accidental complexities of the modular monolith migration. ADR 015 resolved problems 1, 2 (partially), 3, 6, and 7. The remaining problems (2-remainder, 4, 5, 8, 9) are the **next horizon of work** and should be addressed in future ADRs:

| Problem | Status | Next action |
|---------|--------|-------------|
| 1. Alias graveyard | ✅ Fixed | Add CI lint for zero-consumer aliases |
| 2. `domain/` gravity well | 🟡 Partial | Move `AgentDefinition` after fixing Problem 5; move `StudentProfile` after fixing Problem 4 |
| 3. Stranded repo adapters | ✅ Fixed | — |
| 4. `components/` → `domain/` cycle | ❌ Open | Refactor templ components to accept DTOs, not domain types |
| 5. God Adapter in `foundation/agentsdk/` | ❌ Open | **Highest priority** — extract interfaces, flip dependency arrows |
| 6. `services/interfaces.go` junk drawer | ✅ Fixed | — |
| 7. Vestigial `handlers/` directory | ✅ Fixed | — |
| 8. 863-line `main.go` | ❌ Open | Introduce module registry pattern |
| 9. `services/` naming confusion | ❌ Open | Rename to `platform/` or absorb into modules |

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

**Violations found:**
- `foundation/agentsdk/` imports `modules/chat/`, `modules/mcp/`, `modules/knowledge/` (arrow points outward)
- `components/profile/` imports `domain/` which traps `StudentProfile` (presentation imports domain)

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

- Problems 4, 5, 8, and 9 require non-trivial refactoring. Problem 5 (the God Adapter) is the largest — the 722-line runner must be decomposed into interface-driven collaborators.
- This ADR does not prescribe implementation details for the remaining fixes — those belong in dedicated ADRs to keep scope manageable.

## Priority Order for Remaining Work

1. **Problem 5** (God Adapter) — unblocks `AgentDefinition` localisation, fixes the dependency inversion, makes the SDK testable in isolation.
2. **Problem 4** (component import cycle) — unblocks `StudentProfile` localisation, establishes the DTO-props pattern for all templ components.
3. **Problem 8** (main.go bloat) — reduces merge conflicts, makes module wiring self-documenting.
4. **Problem 9** (services/ naming) — cosmetic but reduces cognitive load for new developers.
