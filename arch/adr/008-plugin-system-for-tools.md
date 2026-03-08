# ADR 008: Plugin System Architecture for Tools

## Status
Accepted

## Context
As the agent orchestration capabilities expanded, new tools and toolsets were frequently introduced. Historically, these tools were tightly coupled with the core application:
- Backend tool logic resided in a shared `agents/tools` package.
- Frontend rendering logic for tools that required custom UI interactions (like document search, or elicitation quizzes) lived in a central `components/` directory.
- Frontend JavaScript (`chat-tool-registry.js`) maintained a monolithic, hardcoded `TOOL_REGISTRY` containing exact string mappings of every supported tool to determine if it required special UI handling or had an elicitation schema.

This monolithic approach created several issues:
- **Tight Coupling & Cyclical Dependencies**: Adding a new tool required modifying core application routing and frontend components, increasing the risk of cyclical imports.
- **Scalability**: The hardcoded frontend registry became a maintenance burden and a frequent source of bugs when backend tools were added or renamed without meticulously updating the JS.
- **Encapsulation**: Tool logic, its UI representation (Templ), and its interaction model were physically separated across the codebase.

## Decision
**Implement a modular Plugin Architecture for all tools to encapsulate their backend logic, frontend representation, and registry metadata into self-contained packages.**

1. **Standalone Plugin Packages:** All toolsets (e.g., `calculator`, `document`, `library`, `knowledgebase`, `elicitation`) are extracted into isolated packages under the `plugins/` directory. Each plugin contains its own backend execution logic (`backend.go`), frontend Templ components (`ui.templ`), and a standard interface provider (`provider.go`).
2. **The `PluginProvider` Interface:** A normalized interface is introduced in `services/toolregistry` that every plugin must implement:
    ```go
    type PluginProvider interface {
        ID() string
        DefaultSets() []ToolSet
        DefaultTools() []Tool
        CreateExecutable(ctx context.Context, toolID string, deps FactoryDeps) (agent.ExecutableTool, error)
        GetUITemplate() templ.Component
        UIHandledTools() []string
    }
    ```
3. **Dynamic Registry Loading:** The central `toolregistry.Service` is refactored to iterate over registered `PluginProvider`s to discover tools, rather than defining a monolithic static switch statement.
4. **Dynamic Frontend Injection:** The hardcoded JS `TOOL_REGISTRY` is eliminated. Instead, the backend aggregates the `UIHandledTools()` from all registered plugins and statically renders them as JSON directly into the DOM's `<head>` within `layout.templ` as a global variable (`window.SPECIAL_TOOLS`). JS files like `chat-tool-registry.js` dynamically rely on this variable.
5. **Registration:** Plugins are wired into the application solely through explicit registration in `main.go` (e.g., `toolregistry.RegisterPlugin(&calculator.Provider{})`).

## Consequences

**Positive:**
- **Encapsulation:** A tool's backend execution logic and frontend visual representation are physically co-located and completely decoupled from the core framework.
- **Plug-and-Play Extensibility:** Adding a new complex tool with a custom UI now requires exactly one touchpoint in the core application: registering the provider once in `main.go`.
- **Elimination of Frontend Drift:** By dynamically computing handled tools and injecting them into the DOM globally at render time, the frontend can never be out of sync with backend tool availability, curing JS race conditions and maintenance drag.

**Negative:**
- **Boilerplate:** Developing even a simple backend-only tool now requires scaffolding an entire `Provider` struct to exclusively implement the `PluginProvider` interface.
- **Dependency Inversion Complexity:** Shared dependencies (like Repositories or external service clients) must be explicitly passed down via `FactoryDeps` at runtime rather than being statically accessible or globally defined.

## Constraints
1. Plugins MUST NOT import from the core application layer (e.g., `services/taskmanagement`) to definitively avoid cyclical dependencies. They must ideally only rely on standardized interfaces broadly scoped inside `foundation/` or `domain/`.
2. All plugin UI components MUST be carefully namespaced or prefixed securely (both JS events and CSS classes) to prevent collision in the global, shared DOM space.
