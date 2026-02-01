# Plan: Abstract Agent Framework for Multi-Implementation Support

Decouple the web application from the custom agent framework (`foundation/agentsdk`) by introducing a clean abstraction layer. This enables switching between the current implementation, ADK-Go, and Blades with minimal changes to the handlers, services, and domain layers.

## Steps

### 1. Create core abstraction interfaces in a new `foundation/agent` package

Define framework-agnostic interfaces for `Agent`, `Tool`, `Session`, `Model`, and `Runner` that align with common patterns from ADK-Go and Blades. Keep them minimal and avoid leaking implementation details.

**Files to create:**
- `foundation/agent/agent.go` - Agent interface
- `foundation/agent/tool.go` - Tool interface
- `foundation/agent/session.go` - Session/State interface
- `foundation/agent/model.go` - Model/LLM interface
- `foundation/agent/runner.go` - Runner interface
- `foundation/agent/event.go` - Event types for streaming

**Proposed interfaces:**

```go
// agent.go
type Agent interface {
    Name() string
    Description() string
    Run(context.Context, *Invocation) iter.Seq2[Event, error]
    Tools() []Tool
    SubAgents() []Agent
}

// tool.go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Execute(context.Context, *ToolContext) (any, error)
}

// session.go
type Session interface {
    ID() string
    State() State
    SetState(key string, value any)
    History() []Message
    Append(context.Context, Message) error
}

// model.go
type Model interface {
    Generate(context.Context, *GenerateRequest) iter.Seq2[*GenerateResponse, error]
    Provider() string
}

// runner.go
type Runner interface {
    Run(context.Context, *RunRequest) iter.Seq2[Event, error]
}
```

---

### 2. Introduce adapter layer for each framework implementation

Create subpackages that implement the core interfaces by wrapping each framework's native types.

**Directory structure:**
```
foundation/agent/
├── agent.go
├── tool.go
├── session.go
├── model.go
├── runner.go
├── event.go
└── adapters/
    ├── custom/      # Wraps foundation/agentsdk
    │   ├── agent.go
    │   ├── tool.go
    │   └── runner.go
    ├── adkgo/       # Wraps ADK-Go
    │   ├── agent.go
    │   ├── tool.go
    │   └── runner.go
    └── blades/      # Wraps Blades
        ├── agent.go
        ├── tool.go
        └── runner.go
```

**Custom adapter example:**
```go
// foundation/agent/adapters/custom/agent.go
type AgentAdapter struct {
    config core.AgentConfig
}

func (a *AgentAdapter) Name() string { return a.config.Name }
func (a *AgentAdapter) Run(ctx context.Context, inv *agent.Invocation) iter.Seq2[agent.Event, error] {
    // Wrap core.AgentConfig execution and emit agent.Event
}
```

---

### 3. Extract LLM/Model abstraction from `types.ChatClient`

Define a `Model` interface that decouples from `foundation/agentsdk/llm` types.

**Current coupling in `types/chat_client.go`:**
```go
type ChatClient interface {
    GetStreamingResponse(ctx context.Context, messages []llm.ChatMessage, options *llm.ChatOptions) iter.Seq2[llm.ChatResponseUpdate, error]
}
```

**Refactored to:**
```go
// foundation/agent/model.go
type GenerateRequest struct {
    Messages    []Message
    Tools       []ToolSchema
    Temperature float64
    MaxTokens   int
}

type GenerateResponse struct {
    Content     []Content
    FinishReason string
    Usage       *Usage
}

type Model interface {
    Generate(context.Context, *GenerateRequest) iter.Seq2[*GenerateResponse, error]
    Provider() string
}
```

**Create model adapters:**
- `foundation/agent/adapters/models/openai.go`
- `foundation/agent/adapters/models/google.go`

---

### 4. Refactor `services/agents/service.go` to depend on abstractions

Replace direct usage of `core.AgentConfig`, `core.RunContext`, and `engine.Engine` with the new interfaces.

**Before:**
```go
type Service struct {
    config       core.AgentConfig
    chatClient   ChatClient
    writeModel   core.EventStore
    toolRegistry *core.ToolRegistry
    engine       *engine.Engine
}
```

**After:**
```go
type Service struct {
    runner      agent.Runner        // Abstract runner
    model       agent.Model         // Abstract model
    sessionRepo agent.SessionStore  // Abstract session storage
}

func (s *Service) Run(ctx context.Context, agentName string, req *agent.RunRequest) iter.Seq2[agent.Event, error] {
    return s.runner.Run(ctx, req)
}
```

---

### 5. Update agent definitions to use factory pattern returning abstract `Agent`

Modify `agents/shared/agentregistry.go` so `AgentFactory` returns an `agent.Agent` interface.

**Before:**
```go
type AgentFactory func(ctx context.Context, notifier core.EventNotifier) core.AgentConfig
```

**After:**
```go
type AgentFactory func(ctx context.Context, opts ...AgentOption) agent.Agent
```

**Each agent package implements:**
```go
// agents/math/agent.go
func NewMathAgent(ctx context.Context, opts ...agent.AgentOption) agent.Agent {
    return &MathAgent{
        tools: []agent.Tool{
            tools.NewCalculateTool(),
            tools.NewPlotTool(),
        },
    }
}
```

---

### 6. Migrate handlers and projections to use session abstraction

Update handlers to use the abstract `Session` interface instead of directly accessing domain aggregates.

**Files to update:**
- `handlers/agui/*.go`
- `handlers/thread/*.go`
- `infra/projection/agui/*.go`
- `infra/projection/subscriber/*.go`

**Pattern:**
```go
// Before
thread, _ := s.threadRepo.Load(ctx, threadID)
messages := thread.GetMessages()

// After
session, _ := s.sessionStore.Get(ctx, sessionID)
messages := session.History()
```

---

## Further Considerations

### A. Event Sourcing vs. Session State

| Approach | Pros | Cons |
|----------|------|------|
| **A: Keep event sourcing** | Audit trail, replay, existing domain model | ADK-Go/Blades use session state natively |
| **B: Migrate to session state** | Simpler, matches external frameworks | Lose audit trail, major refactor |

**Recommendation: Option A** - Keep event sourcing as the canonical store. Create a `SessionAdapter` that:
1. Reads from the event-sourced `ThreadAggregate`
2. Emits domain events when session state changes
3. Provides session-like API for framework compatibility

```go
// foundation/agent/adapters/session/eventsourced.go
type EventSourcedSession struct {
    aggregate *domain.ThreadAggregate
    store     core.EventStore
}

func (s *EventSourcedSession) History() []agent.Message {
    return convertMessages(s.aggregate.GetMessages())
}

func (s *EventSourcedSession) Append(ctx context.Context, msg agent.Message) error {
    event := domain.NewMessageAddedEvent(msg)
    return s.store.Append(ctx, s.aggregate.GetID(), event)
}
```

---

### B. Tool Interface Alignment

| Framework | Tool Signature |
|-----------|---------------|
| Custom | `Invoke(ctx ToolContext, args any) (any, error)` |
| ADK-Go | `Call(ctx tool.Context) (any, error)` |
| Blades | `Handle(ctx context.Context, input string) (string, error)` |

**Solution:** Create a normalized `ToolAdapter`:

```go
// foundation/agent/adapters/custom/tool.go
type CustomToolAdapter struct {
    tool core.Tool
}

func (a *CustomToolAdapter) Execute(ctx context.Context, tc *agent.ToolContext) (any, error) {
    coreCtx := convertToToolContext(ctx, tc)
    return a.tool.Invoke(coreCtx, tc.Arguments)
}
```

---

### C. Multi-Agent Handoffs

| Framework | Pattern |
|-----------|---------|
| Custom | `Command.Goto` field |
| ADK-Go | `SubAgents()` tree |
| Blades | Agent-as-tool wrapper |

**Unified approach:**
```go
type Agent interface {
    // ...
    SubAgents() []Agent           // Hierarchical delegation
    CanHandoff(target string) bool // Explicit handoff support
}

type HandoffEvent struct {
    FromAgent string
    ToAgent   string
    Reason    string
}
```

---

## Migration Order

```
Phase 0: Centralize Custom Agent Framework (Low Risk)
├── Audit all agent-related code scattered across the codebase
├── Move services/agents/engine/ → foundation/agentsdk/engine/
├── Move services/agents/service.go → foundation/agentsdk/service/
├── Move agents/shared/agentregistry.go → foundation/agentsdk/registry/
├── Move types.ChatClient → foundation/agentsdk/llm/client.go
├── Consolidate all tool implementations under foundation/agentsdk/tools/
└── Update all imports to point to centralized locations

Phase 1: Create Abstractions (Low Risk)
├── Create foundation/agent/ interfaces (on top of centralized agentsdk)
├── Create custom adapter (wraps foundation/agentsdk)
└── No changes to existing behavior

Phase 2: Service Layer (Medium Risk)
├── Create new services/agents/service.go that uses abstractions
├── Inject Runner via dependency injection
└── Maintain backward compatibility

Phase 3: Handlers & Projections (Medium Risk)
├── Update handlers to use Session interface
├── Migrate projections
└── Test all API endpoints

Phase 4: External Frameworks (Low Risk per framework)
├── Implement ADK-Go adapter
├── Implement Blades adapter
└── Add configuration to switch frameworks
```

---

## Phase 0: Centralize Custom Agent Framework (Detailed)

Before creating abstractions, consolidate all agent-related code into `foundation/agentsdk/`. This makes the custom implementation self-contained and easier to wrap with adapters.

### Current Scattered Locations

| Current Location | Purpose | Target Location |
|------------------|---------|-----------------|
| `foundation/agentsdk/core/` | Core types (Tool, Thread, Config) | Keep as-is |
| `foundation/agentsdk/llm/` | LLM message types | Keep as-is |
| `foundation/agentsdk/tools/` | Built-in tools | Keep as-is |
| `services/agents/engine/` | Execution engine, state machine | → `foundation/agentsdk/engine/` |
| `services/agents/service.go` | Orchestration service | → `foundation/agentsdk/service/` |
| `services/agents/types.go` | ChatClient interface | → `foundation/agentsdk/llm/client.go` |
| `agents/shared/agentregistry.go` | Agent factory registry | → `foundation/agentsdk/registry/` |
| `agents/shared/interfaces.go` | AgentOrchestrator interface | → `foundation/agentsdk/service/` |
| `agents/*/agent.go` | Agent definitions | Keep, but depend on agentsdk |
| `domain/tool_lifecycle.go` | Tool state machine | → `foundation/agentsdk/tools/lifecycle.go` |

### New Directory Structure

```
foundation/agentsdk/
├── core/                    # Existing - core types
│   ├── tool.go
│   ├── thread.go
│   ├── config.go
│   ├── middleware.go
│   ├── command.go
│   └── ...
├── llm/                     # Existing + ChatClient
│   ├── message.go
│   ├── response.go
│   ├── content.go
│   └── client.go            # ← Move from services/agents/types.go
├── tools/                   # Existing + lifecycle
│   ├── memory.go
│   ├── browse.go
│   └── lifecycle.go         # ← Move from domain/tool_lifecycle.go
├── engine/                  # ← Move from services/agents/engine/
│   ├── engine.go
│   ├── llm_processor.go
│   ├── tool_execution.go
│   ├── thread_mgmt.go
│   └── state_machine.go
├── service/                 # ← Move from services/agents/
│   ├── service.go
│   └── interfaces.go        # ← Move from agents/shared/interfaces.go
└── registry/                # ← Move from agents/shared/
    └── registry.go
```

### Migration Steps

1. **Create new directories** under `foundation/agentsdk/`
2. **Move files** with `git mv` to preserve history
3. **Update package declarations** in moved files
4. **Update imports** across the entire codebase
5. **Run tests** to verify nothing broke
6. **Create thin wrappers** in original locations (optional, for gradual migration)

### Import Update Script

```bash
# Example: Update imports after moving engine
find . -name "*.go" -exec sed -i \
  's|"schoolai/services/agents/engine"|"schoolai/foundation/agentsdk/engine"|g' {} \;
```

### Verification Checklist

- [ ] All agent framework code lives under `foundation/agentsdk/`
- [ ] `services/agents/` only contains thin orchestration layer
- [ ] `agents/*` packages only define agent configurations, not framework logic
- [ ] `handlers/` has no direct imports of `foundation/agentsdk/` internals
- [ ] All tests pass
- [ ] No circular dependencies

---

## Success Criteria

1. **Zero framework imports in handlers/** - No direct imports of `foundation/agentsdk`
2. **Switchable via configuration** - Environment variable or config file selects framework
3. **All tests pass** - Existing behavior preserved through adapters
4. **New framework integration < 1 day** - Adding a new framework only requires adapter code
