# Migration Plan: Event-Sourced Agent Framework

This plan migrates the Go project to an architecture inspired by the .NET Agent Framework while **keeping event sourcing** as a core differentiator.

## Architecture Comparison

| Aspect | Current Go | Target Architecture |
|--------|-----------|---------------------|
| Agent Definition | `AgentContract` struct | `AgentType` interface |
| Configuration | Mixed in contract/agent | `AgentBuilder` fluent API |
| Middleware | None | `DelegatingAgent` decorator chain |
| Discovery | Static `ContractRegistry` | Service-based `AgentRegistry` |
| Tool Mgmt | Per-contract tools array | `ToolRegistry` with dynamic registration |
| Event Storage | `EventLog` interface | `EventStore` interface (source of truth) |
| Thread State | `InMemoryThreadState` | `ThreadProjection` (read model from events) |
| Event Sourcing | In `Agent.Run()` | `EventSourcingMiddleware` decorator |
| Multi-Agent | EventBus routing | `GroupChatManager` orchestrator |

---

## Step 1: Create AgentType Interface and Core Abstractions

**Goal**: Define the foundational `AgentType` interface that all agents implement.

**Files to create**:
- `services/sdk/agent_type.go`

**Changes**:

```go
// services/sdk/agent_type.go

// AgentType is the core interface for all agents (similar to .NET AIAgent)
type AgentType interface {
    // Name returns the unique identifier for this agent type
    Name() string

    // Description returns a human-readable description of what this agent does
    Description() string

    // Run executes the agent and returns a response
    Run(ctx context.Context, thread AgentThread, opts *RunOptions) (*AgentResponse, error)

    // RunStreaming executes the agent with streaming response
    RunStreaming(ctx context.Context, thread AgentThread, opts *RunOptions) (<-chan AgentResponseUpdate, error)

    // GetService retrieves a typed service from the agent (for metadata, etc.)
    GetService(serviceType string) (any, bool)
}

// AgentResponse contains the result of an agent execution
type AgentResponse struct {
    Messages  []domain.Message
    Metadata  map[string]any
    EventsEmitted []Event  // Events produced during execution
}

// AgentResponseUpdate is a streaming update during execution
type AgentResponseUpdate struct {
    Type     UpdateType
    Content  string        // For text streaming
    ToolCall *ToolCallUpdate
    Event    *Event        // For event streaming
}

type UpdateType string
const (
    UpdateTypeTextStart   UpdateType = "text_start"
    UpdateTypeTextContent UpdateType = "text_content"
    UpdateTypeTextEnd     UpdateType = "text_end"
    UpdateTypeToolStart   UpdateType = "tool_start"
    UpdateTypeToolArgs    UpdateType = "tool_args"
    UpdateTypeToolEnd     UpdateType = "tool_end"
    UpdateTypeEvent       UpdateType = "event"
)

// RunOptions configures a single agent run
type RunOptions struct {
    MaxIterations int
    Temperature   *float64
    MaxTokens     *int
    StopSequences []string
    Metadata      map[string]any
}
```

**Verification**: Build passes, existing agents still work (no changes to existing code yet).

---

## Step 2: Implement AgentBuilder Pattern for Fluent Configuration

**Goal**: Create a builder that allows fluent, composable agent configuration.

**Files to create**:
- `services/sdk/agent_builder.go`

**Changes**:

```go
// services/sdk/agent_builder.go

// AgentBuilder provides fluent API for constructing agents
type AgentBuilder struct {
    name          string
    description   string
    systemPrompt  string
    tools         []Tool
    chatClient    ChatClient
    middleware    []AgentMiddleware
    eventStore    EventStore      // Source of truth for events
    projection    ThreadProjection // Read model for thread state
    config        map[string]any
}

// NewAgentBuilder creates a new builder
func NewAgentBuilder(name string) *AgentBuilder {
    return &AgentBuilder{
        name:   name,
        tools:  make([]Tool, 0),
        config: make(map[string]any),
    }
}

// WithDescription sets the agent description
func (b *AgentBuilder) WithDescription(desc string) *AgentBuilder {
    b.description = desc
    return b
}

// WithSystemPrompt sets the system prompt
func (b *AgentBuilder) WithSystemPrompt(prompt string) *AgentBuilder {
    b.systemPrompt = prompt
    return b
}

// WithTool adds a tool to the agent
func (b *AgentBuilder) WithTool(tool Tool) *AgentBuilder {
    b.tools = append(b.tools, tool)
    return b
}

// WithTools adds multiple tools
func (b *AgentBuilder) WithTools(tools ...Tool) *AgentBuilder {
    b.tools = append(b.tools, tools...)
    return b
}

// WithChatClient sets the LLM client
func (b *AgentBuilder) WithChatClient(client ChatClient) *AgentBuilder {
    b.chatClient = client
    return b
}

// Use adds middleware to the agent pipeline (applied in reverse order)
func (b *AgentBuilder) Use(middleware AgentMiddleware) *AgentBuilder {
    b.middleware = append(b.middleware, middleware)
    return b
}

// WithEventStore sets the event store (source of truth)
func (b *AgentBuilder) WithEventStore(store EventStore) *AgentBuilder {
    b.eventStore = store
    return b
}

// WithProjection sets the thread projection (read model)
func (b *AgentBuilder) WithProjection(proj ThreadProjection) *AgentBuilder {
    b.projection = proj
    return b
}

// Build creates the final AgentType
func (b *AgentBuilder) Build() (AgentType, error) {
    if b.name == "" {
        return nil, errors.New("agent name is required")
    }
    if b.chatClient == nil {
        return nil, errors.New("chat client is required")
    }

    // Create base agent
    base := &ChatClientAgent{
        name:         b.name,
        description:  b.description,
        systemPrompt: b.systemPrompt,
        tools:        b.tools,
        chatClient:   b.chatClient,
    }

    // Apply middleware in reverse order (first added = outermost)
    var agent AgentType = base
    for i := len(b.middleware) - 1; i >= 0; i-- {
        agent = b.middleware[i](agent)
    }

    return agent, nil
}
```

**Verification**: Can create agents using builder, existing contract-based agents still work.

---

## Step 3: Create DelegatingAgent Middleware/Decorator Pattern

**Goal**: Implement middleware pattern for cross-cutting concerns (logging, telemetry, validation).

**Files to create**:
- `services/sdk/delegating_agent.go`

**Changes**:

```go
// services/sdk/delegating_agent.go

// AgentMiddleware is a function that wraps an agent
type AgentMiddleware func(AgentType) AgentType

// DelegatingAgent wraps an inner agent, enabling decorator pattern
type DelegatingAgent struct {
    inner AgentType
}

// NewDelegatingAgent creates a delegating wrapper
func NewDelegatingAgent(inner AgentType) *DelegatingAgent {
    return &DelegatingAgent{inner: inner}
}

func (d *DelegatingAgent) Name() string        { return d.inner.Name() }
func (d *DelegatingAgent) Description() string { return d.inner.Description() }
func (d *DelegatingAgent) GetService(t string) (any, bool) { return d.inner.GetService(t) }

func (d *DelegatingAgent) Run(ctx context.Context, thread AgentThread, opts *RunOptions) (*AgentResponse, error) {
    return d.inner.Run(ctx, thread, opts)
}

func (d *DelegatingAgent) RunStreaming(ctx context.Context, thread AgentThread, opts *RunOptions) (<-chan AgentResponseUpdate, error) {
    return d.inner.RunStreaming(ctx, thread, opts)
}

// --- Built-in Middleware ---

// LoggingMiddleware logs agent executions
func LoggingMiddleware(logger *slog.Logger) AgentMiddleware {
    return func(inner AgentType) AgentType {
        return &loggingAgent{
            DelegatingAgent: DelegatingAgent{inner: inner},
            logger:          logger,
        }
    }
}

type loggingAgent struct {
    DelegatingAgent
    logger *slog.Logger
}

func (a *loggingAgent) Run(ctx context.Context, thread AgentThread, opts *RunOptions) (*AgentResponse, error) {
    a.logger.Info("agent run started", "agent", a.Name())
    start := time.Now()

    resp, err := a.inner.Run(ctx, thread, opts)

    a.logger.Info("agent run completed",
        "agent", a.Name(),
        "duration", time.Since(start),
        "error", err)
    return resp, err
}

// ValidationMiddleware validates inputs before execution
func ValidationMiddleware() AgentMiddleware {
    return func(inner AgentType) AgentType {
        return &validationAgent{DelegatingAgent{inner: inner}}
    }
}

type validationAgent struct {
    DelegatingAgent
}

func (a *validationAgent) Run(ctx context.Context, thread AgentThread, opts *RunOptions) (*AgentResponse, error) {
    if thread == nil {
        return nil, errors.New("thread is required")
    }
    return a.inner.Run(ctx, thread, opts)
}
```

**Verification**: Middleware wraps agents correctly, logging/validation works.

---

## Step 4: Build AgentRegistry with Keyed Service Discovery

**Goal**: Replace static `ContractRegistry` with dynamic service-based discovery.

**Files to create**:
- `services/sdk/agent_registry.go`

**Changes**:

```go
// services/sdk/agent_registry.go

// AgentRegistry manages agent registration and discovery
type AgentRegistry struct {
    mu       sync.RWMutex
    agents   map[string]AgentFactory  // name -> factory
    metadata map[string]AgentMetadata // name -> metadata
}

// AgentFactory creates an agent instance
type AgentFactory func() (AgentType, error)

// AgentMetadata describes an agent's capabilities
type AgentMetadata struct {
    Name        string
    Description string
    Skills      []string           // What this agent can do
    Listens     []string           // Event types this agent handles
    Emits       []string           // Event types this agent can emit
    Tags        map[string]string  // Arbitrary metadata
}

// NewAgentRegistry creates an empty registry
func NewAgentRegistry() *AgentRegistry {
    return &AgentRegistry{
        agents:   make(map[string]AgentFactory),
        metadata: make(map[string]AgentMetadata),
    }
}

// Register adds an agent factory with metadata
func (r *AgentRegistry) Register(name string, factory AgentFactory, meta AgentMetadata) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.agents[name]; exists {
        return fmt.Errorf("agent %q already registered", name)
    }

    meta.Name = name
    r.agents[name] = factory
    r.metadata[name] = meta
    return nil
}

// Get retrieves an agent by name
func (r *AgentRegistry) Get(name string) (AgentType, error) {
    r.mu.RLock()
    factory, ok := r.agents[name]
    r.mu.RUnlock()

    if !ok {
        return nil, fmt.Errorf("agent %q not found", name)
    }
    return factory()
}

// GetMetadata retrieves agent metadata
func (r *AgentRegistry) GetMetadata(name string) (AgentMetadata, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    meta, ok := r.metadata[name]
    return meta, ok
}

// List returns all registered agent names
func (r *AgentRegistry) List() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()

    names := make([]string, 0, len(r.agents))
    for name := range r.agents {
        names = append(names, name)
    }
    return names
}

// FindByListener returns agents that listen to an event type
func (r *AgentRegistry) FindByListener(eventType string) []string {
    r.mu.RLock()
    defer r.mu.RUnlock()

    var result []string
    for name, meta := range r.metadata {
        for _, listens := range meta.Listens {
            if listens == eventType {
                result = append(result, name)
                break
            }
        }
    }
    return result
}

// All returns all agent metadata (for discovery endpoints)
func (r *AgentRegistry) All() []AgentMetadata {
    r.mu.RLock()
    defer r.mu.RUnlock()

    result := make([]AgentMetadata, 0, len(r.metadata))
    for _, meta := range r.metadata {
        result = append(result, meta)
    }
    return result
}
```

**Verification**: Agents can be registered and discovered via registry.

---

## Step 5: Create ToolRegistry with Per-Agent Tool Management

**Goal**: Centralize tool registration with per-agent tool sets.

**Files to create**:
- `services/sdk/tool_registry.go`

**Changes**:

```go
// services/sdk/tool_registry.go

// ToolRegistry manages tool registration per agent
type ToolRegistry struct {
    mu           sync.RWMutex
    globalTools  []Tool                    // Tools available to all agents
    agentTools   map[string][]Tool         // agent name -> tools
    toolByName   map[string]Tool           // tool name -> tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
    return &ToolRegistry{
        globalTools: make([]Tool, 0),
        agentTools:  make(map[string][]Tool),
        toolByName:  make(map[string]Tool),
    }
}

// RegisterGlobal adds a tool available to all agents
func (r *ToolRegistry) RegisterGlobal(tool Tool) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.toolByName[tool.Name()]; exists {
        return fmt.Errorf("tool %q already registered", tool.Name())
    }

    r.globalTools = append(r.globalTools, tool)
    r.toolByName[tool.Name()] = tool
    return nil
}

// RegisterForAgent adds a tool specific to an agent
func (r *ToolRegistry) RegisterForAgent(agentName string, tool Tool) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    key := agentName + ":" + tool.Name()
    if _, exists := r.toolByName[key]; exists {
        return fmt.Errorf("tool %q already registered for agent %q", tool.Name(), agentName)
    }

    r.agentTools[agentName] = append(r.agentTools[agentName], tool)
    r.toolByName[key] = tool
    return nil
}

// GetToolsFor returns all tools available to an agent
func (r *ToolRegistry) GetToolsFor(agentName string) []Tool {
    r.mu.RLock()
    defer r.mu.RUnlock()

    // Combine global + agent-specific tools
    result := make([]Tool, 0, len(r.globalTools)+len(r.agentTools[agentName]))
    result = append(result, r.globalTools...)
    result = append(result, r.agentTools[agentName]...)
    return result
}

// GetTool retrieves a tool by name (optionally scoped to agent)
func (r *ToolRegistry) GetTool(name string, agentName string) (Tool, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    // Check agent-specific first
    if agentName != "" {
        if tool, ok := r.toolByName[agentName+":"+name]; ok {
            return tool, true
        }
    }

    // Fall back to global
    tool, ok := r.toolByName[name]
    return tool, ok
}

// CreateEmissionTool creates a tool for emitting events
func (r *ToolRegistry) CreateEmissionTool(eventType string) Tool {
    return &EmissionTool{
        eventType: eventType,
    }
}

// EmissionTool is a tool that emits events for agent collaboration
type EmissionTool struct {
    eventType string
}

func (t *EmissionTool) Name() string {
    // "question.math" -> "emit_math_question"
    parts := strings.Split(t.eventType, ".")
    if len(parts) == 2 {
        return "emit_" + parts[1] + "_" + parts[0]
    }
    return "emit_" + strings.ReplaceAll(t.eventType, ".", "_")
}

func (t *EmissionTool) Description() string {
    return fmt.Sprintf("Emit a %s event to delegate work to a specialist agent", t.eventType)
}

func (t *EmissionTool) Parameters() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "question": map[string]any{
                "type":        "string",
                "description": "The question to pass to the specialist",
            },
            "context": map[string]any{
                "type":        "string",
                "description": "Optional context from current conversation",
            },
        },
        "required": []string{"question"},
    }
}

func (t *EmissionTool) Invoke(ctx context.Context, args string) (string, error) {
    // Actual emission handled by EventSourcingMiddleware
    return fmt.Sprintf("Event %s will be emitted", t.eventType), nil
}
```

**Verification**: Tools can be registered globally and per-agent.

---

## Step 6: Event Sourcing Architecture - EventStore and Projections ✅ COMPLETED

**Goal**: Establish proper event sourcing architecture where EventStore is the source of truth and ThreadState is a projection.

**Key Insight**: In event sourcing, there is NO "ThreadStore" - threads are projections built from events.

```
EventStore (source of truth - append-only)
    ↓ events flow
Projection (read model - rebuilt from events)
    ↑ agents READ from here

Agents WRITE by emitting events → EventStore
Agents READ from projections → ThreadState
```

**Files created**:
- `services/sdk/event_store.go` - EventStore interface
- `services/sdk/in_memory_event_store.go` - In-memory implementation
- `services/sdk/projection.go` - Projection interface

**Files updated**:
- `services/sdk/thread_state.go` - Clarified as projection, added `ThreadStateWithAgentAccess`
- `infra/in_memory_thread_state.go` - Added `Rebuild()` methods, removed ThreadStore dependency
- All agent contracts - Use `sdk.ThreadStateWithAgentAccess` interface

**Key Interfaces**:

```go
// services/sdk/event_store.go

// EventStore is the source of truth (append-only)
type EventStore interface {
    Append(ctx, streamID string, events ...Event) (int64, error)
    ReadStream(ctx, streamID string, fromVersion int64) ([]Event, error)
    ReadAll(ctx, streamID string) ([]Event, error)
    StreamVersion(ctx, streamID string) (int64, error)
    StreamExists(ctx, streamID string) (bool, error)
    ListStreams(ctx) ([]string, error)
    DeleteStream(ctx, streamID string) error  // GDPR compliance
}

// EventStoreWithSubscription for real-time projection updates
type EventStoreWithSubscription interface {
    EventStore
    Subscribe(ctx, streamID string, handler EventHandler) (unsubscribe func(), error)
    SubscribeAll(ctx, handler EventHandler) (unsubscribe func(), error)
}
```

```go
// services/sdk/thread_state.go

// ThreadState is a PROJECTION (read model) built from events
type ThreadState interface {
    Get(threadID string) (*domain.Thread, bool)
    Messages(threadID string) []domain.Message
    All() []domain.Thread
    Update(event Event) error  // Apply event to update projection
}

// ThreadStateWithAgentAccess adds agent-specific access methods
type ThreadStateWithAgentAccess interface {
    ThreadState
    GetAgentThread(threadID string) (AgentThread, bool)
    GetOrCreateAgentThread(ctx, threadID string) (AgentThread, error)
    Rebuild(ctx, store EventStore, threadID string) error
    RebuildAll(ctx, store EventStore) error
}
```

**Agent contracts updated** to use interface instead of type assertion:
```go
// Before (wrong - depends on infra package)
state, ok := handlerCtx.State.(*infra.InMemoryThreadState)

// After (correct - uses sdk interface)
stateWithAccess, ok := handlerCtx.State.(sdk.ThreadStateWithAgentAccess)
agentThread, exists := stateWithAccess.GetAgentThread(handlerCtx.ThreadID)
```

**Benefits**:
1. **Proper event sourcing** - EventStore is source of truth, projections are derived
2. **Rebuildable projections** - Can reconstruct state from events at any time
3. **Clean separation** - Agents don't depend on storage implementation
4. **Pluggable backends** - Can swap InMemoryEventStore for PostgresEventStore

**Verification**: All tests pass, build succeeds.

---

## Step 7: Create EventSourcingMiddleware Decorator

**Goal**: Extract event sourcing from `Agent.Run()` into a composable middleware.

**Files to create**:
- `services/sdk/event_sourcing_middleware.go`

**Changes**:

```go
// services/sdk/event_sourcing_middleware.go

// EventSourcingMiddleware wraps an agent with event sourcing capabilities
func EventSourcingMiddleware(eventLog EventLog, eventBus *EventBus) AgentMiddleware {
    return func(inner AgentType) AgentType {
        return &eventSourcingAgent{
            DelegatingAgent: DelegatingAgent{inner: inner},
            eventLog:        eventLog,
            eventBus:        eventBus,
        }
    }
}

type eventSourcingAgent struct {
    DelegatingAgent
    eventLog EventLog
    eventBus *EventBus
}

func (a *eventSourcingAgent) Run(ctx context.Context, thread AgentThread, opts *RunOptions) (*AgentResponse, error) {
    threadID := thread.ID()

    // Emit run started event
    a.emit(ctx, threadID, Event{
        Type:      "agent.run.started",
        ThreadID:  threadID,
        AgentName: a.Name(),
        Timestamp: time.Now(),
    })

    // Execute inner agent
    resp, err := a.inner.Run(ctx, thread, opts)

    if err != nil {
        a.emit(ctx, threadID, Event{
            Type:      "agent.run.failed",
            ThreadID:  threadID,
            AgentName: a.Name(),
            Data:      map[string]any{"error": err.Error()},
            Timestamp: time.Now(),
        })
        return nil, err
    }

    // Emit message events
    for _, msg := range resp.Messages {
        a.emit(ctx, threadID, Event{
            Type:      "message.added",
            ThreadID:  threadID,
            AgentName: a.Name(),
            Data:      map[string]any{"role": msg.Role, "content": msg.Content},
            Timestamp: time.Now(),
        })
    }

    // Emit agent-produced events
    for _, evt := range resp.EventsEmitted {
        a.emit(ctx, threadID, evt)
    }

    // Emit run completed event
    a.emit(ctx, threadID, Event{
        Type:      "agent.run.completed",
        ThreadID:  threadID,
        AgentName: a.Name(),
        Timestamp: time.Now(),
    })

    return resp, nil
}

func (a *eventSourcingAgent) RunStreaming(ctx context.Context, thread AgentThread, opts *RunOptions) (<-chan AgentResponseUpdate, error) {
    threadID := thread.ID()

    // Get inner stream
    innerCh, err := a.inner.RunStreaming(ctx, thread, opts)
    if err != nil {
        return nil, err
    }

    // Wrap with event emission
    outCh := make(chan AgentResponseUpdate)

    go func() {
        defer close(outCh)

        a.emit(ctx, threadID, Event{
            Type:      "agent.run.started",
            ThreadID:  threadID,
            AgentName: a.Name(),
            Timestamp: time.Now(),
        })

        for update := range innerCh {
            // Emit corresponding event for each update type
            switch update.Type {
            case UpdateTypeTextStart:
                a.emit(ctx, threadID, Event{
                    Type:      "message.start",
                    ThreadID:  threadID,
                    AgentName: a.Name(),
                    Timestamp: time.Now(),
                })
            case UpdateTypeTextContent:
                a.emit(ctx, threadID, Event{
                    Type:      "message.content",
                    ThreadID:  threadID,
                    AgentName: a.Name(),
                    Data:      map[string]any{"content": update.Content},
                    Timestamp: time.Now(),
                })
            case UpdateTypeToolStart:
                a.emit(ctx, threadID, Event{
                    Type:      "tool.call.start",
                    ThreadID:  threadID,
                    AgentName: a.Name(),
                    Data:      map[string]any{"tool": update.ToolCall.Name},
                    Timestamp: time.Now(),
                })
            }

            // Forward to output channel
            outCh <- update

            // Handle event emissions from tool calls
            if update.Event != nil {
                a.emit(ctx, threadID, *update.Event)
            }
        }

        a.emit(ctx, threadID, Event{
            Type:      "agent.run.completed",
            ThreadID:  threadID,
            AgentName: a.Name(),
            Timestamp: time.Now(),
        })
    }()

    return outCh, nil
}

func (a *eventSourcingAgent) emit(ctx context.Context, threadID string, event Event) {
    // Append to event log
    a.eventLog.Append(threadID, event)

    // Notify event bus for routing
    if a.eventBus != nil {
        a.eventBus.Publish(event)
    }
}

// EventBus handles event routing between agents
type EventBus struct {
    mu          sync.RWMutex
    subscribers map[string][]EventHandler
}

type EventHandler func(ctx context.Context, event Event) error

func NewEventBus() *EventBus {
    return &EventBus{
        subscribers: make(map[string][]EventHandler),
    }
}

func (b *EventBus) Subscribe(eventType string, handler EventHandler) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.subscribers[eventType] = append(b.subscribers[eventType], handler)
}

func (b *EventBus) Publish(event Event) {
    b.mu.RLock()
    handlers := b.subscribers[event.Type]
    b.mu.RUnlock()

    for _, handler := range handlers {
        go handler(context.Background(), event)
    }
}
```

**Verification**: Event sourcing works through middleware, events are logged.

---

## Step 8: Build GroupChatManager for Multi-Agent Orchestration

**Goal**: Create orchestrator for coordinating multiple agents (replacing EventBus routing).

**Files to create**:
- `services/sdk/group_chat.go`

**Changes**:

```go
// services/sdk/group_chat.go

// GroupChatManager orchestrates multi-agent conversations
type GroupChatManager struct {
    registry       *AgentRegistry
    eventStore     EventStore       // Source of truth for events
    projection     ThreadProjection // Read model for thread state
    selector       AgentSelector
    maxIterations  int
    terminationFn  TerminationFunc
}

// AgentSelector chooses which agent should respond next
type AgentSelector interface {
    SelectNext(ctx context.Context, thread AgentThread, availableAgents []string) (string, error)
}

// TerminationFunc determines if conversation should end
type TerminationFunc func(thread AgentThread, lastResponse *AgentResponse) bool

// GroupChatConfig configures the manager
type GroupChatConfig struct {
    Registry      *AgentRegistry
    EventStore    EventStore       // Source of truth
    Projection    ThreadProjection // Read model
    Selector      AgentSelector
    MaxIterations int
    Termination   TerminationFunc
}

func NewGroupChatManager(config GroupChatConfig) *GroupChatManager {
    if config.MaxIterations == 0 {
        config.MaxIterations = 10
    }
    return &GroupChatManager{
        registry:      config.Registry,
        eventStore:    config.EventStore,
        projection:    config.Projection,
        selector:      config.Selector,
        maxIterations: config.MaxIterations,
        terminationFn: config.Termination,
    }
}

// Run executes the group chat until termination
func (m *GroupChatManager) Run(ctx context.Context, threadID string, initialAgent string) (*AgentResponse, error) {
    // Read from projection (not store)
    thread, ok := m.projection.GetAgentThread(threadID)
    if !ok {
        return nil, fmt.Errorf("thread not found: %s", threadID)
    }

    currentAgent := initialAgent
    var lastResponse *AgentResponse

    for i := 0; i < m.maxIterations; i++ {
        // Get agent
        agent, err := m.registry.Get(currentAgent)
        if err != nil {
            return nil, fmt.Errorf("failed to get agent %s: %w", currentAgent, err)
        }

        // Run agent
        resp, err := agent.Run(ctx, thread, nil)
        if err != nil {
            return nil, fmt.Errorf("agent %s failed: %w", currentAgent, err)
        }

        // Add response to thread
        thread.AddMessages(resp.Messages)
        lastResponse = resp

        // Check termination
        if m.terminationFn != nil && m.terminationFn(thread, resp) {
            break
        }

        // Check for routing events
        nextAgent := m.findNextAgent(resp.EventsEmitted)
        if nextAgent == "" {
            // No routing event, use selector
            if m.selector != nil {
                availableAgents := m.registry.List()
                nextAgent, err = m.selector.SelectNext(ctx, thread, availableAgents)
                if err != nil {
                    return nil, fmt.Errorf("selector failed: %w", err)
                }
            }
        }

        if nextAgent == "" || nextAgent == currentAgent {
            // No next agent or same agent, terminate
            break
        }

        currentAgent = nextAgent
    }

    // Events are already persisted via EventSourcingMiddleware
    // Projection is updated via event subscription
    return lastResponse, nil
}

// findNextAgent looks for routing events in response
func (m *GroupChatManager) findNextAgent(events []Event) string {
    for _, evt := range events {
        // Check if any agent listens to this event
        listeners := m.registry.FindByListener(evt.Type)
        if len(listeners) > 0 {
            return listeners[0] // Return first listener
        }
    }
    return ""
}

// --- Built-in Selectors ---

// RoundRobinSelector cycles through agents
type RoundRobinSelector struct {
    current int
}

func (s *RoundRobinSelector) SelectNext(ctx context.Context, thread AgentThread, agents []string) (string, error) {
    if len(agents) == 0 {
        return "", nil
    }
    s.current = (s.current + 1) % len(agents)
    return agents[s.current], nil
}

// EventBasedSelector routes based on emitted events
type EventBasedSelector struct {
    registry *AgentRegistry
}

func NewEventBasedSelector(registry *AgentRegistry) *EventBasedSelector {
    return &EventBasedSelector{registry: registry}
}

func (s *EventBasedSelector) SelectNext(ctx context.Context, thread AgentThread, agents []string) (string, error) {
    // Check last message for routing hints
    msgs := thread.Messages()
    if len(msgs) == 0 {
        return "", nil
    }

    // Could analyze last message content or metadata for routing
    // For now, return empty (no automatic selection)
    return "", nil
}
```

**Verification**: GroupChatManager can orchestrate multi-agent conversations.

---

## Step 9: Migrate Existing Agents to New AgentType Pattern

**Goal**: Convert all existing agents from `AgentContract` to `AgentType`.

**Changes per agent** (example for math-instructor):

```go
// services/agents/math/agent.go

// MathInstructor implements AgentType
type MathInstructor struct {
    chatClient   sdk.ChatClient
    toolRegistry *sdk.ToolRegistry
    config       *MathConfig
}

type MathConfig struct {
    EnableGraphing    bool
    EnableLinearAlgebra bool
}

// NewMathInstructor creates the math instructor agent
func NewMathInstructor(client sdk.ChatClient, toolReg *sdk.ToolRegistry, config *MathConfig) *MathInstructor {
    return &MathInstructor{
        chatClient:   client,
        toolRegistry: toolReg,
        config:       config,
    }
}

func (m *MathInstructor) Name() string        { return "math-instructor" }
func (m *MathInstructor) Description() string { return "Prof. Ada - Mathematics specialist" }

func (m *MathInstructor) Run(ctx context.Context, thread sdk.AgentThread, opts *sdk.RunOptions) (*sdk.AgentResponse, error) {
    tools := m.toolRegistry.GetToolsFor(m.Name())

    // Build messages with system prompt
    messages := []domain.Message{
        {Role: "system", Content: mathSystemPrompt},
    }
    messages = append(messages, thread.Messages()...)

    // Call LLM
    resp, err := m.chatClient.Chat(ctx, messages, tools, opts)
    if err != nil {
        return nil, err
    }

    return &sdk.AgentResponse{
        Messages:      resp.Messages,
        EventsEmitted: resp.Events,
    }, nil
}

func (m *MathInstructor) RunStreaming(ctx context.Context, thread sdk.AgentThread, opts *sdk.RunOptions) (<-chan sdk.AgentResponseUpdate, error) {
    // Streaming implementation
    // ...
}

func (m *MathInstructor) GetService(serviceType string) (any, bool) {
    switch serviceType {
    case "config":
        return m.config, true
    default:
        return nil, false
    }
}

// --- Registration ---

// RegisterMathInstructor registers the math instructor with the agent registry
func RegisterMathInstructor(registry *sdk.AgentRegistry, toolReg *sdk.ToolRegistry, client sdk.ChatClient) error {
    // Register tools
    toolReg.RegisterForAgent("math-instructor", tools.NewCalculator())
    toolReg.RegisterForAgent("math-instructor", tools.NewEquationSolver())
    toolReg.RegisterForAgent("math-instructor", tools.NewPlotTool())
    toolReg.RegisterForAgent("math-instructor", toolReg.CreateEmissionTool("question.linearalgebra"))

    // Register agent
    return registry.Register("math-instructor",
        func() (sdk.AgentType, error) {
            return NewMathInstructor(client, toolReg, &MathConfig{
                EnableGraphing:      true,
                EnableLinearAlgebra: true,
            }), nil
        },
        sdk.AgentMetadata{
            Description: "Mathematics instructor specializing in algebra, calculus, and equations",
            Skills:      []string{"algebra", "calculus", "graphing", "equations"},
            Listens:     []string{"question.math"},
            Emits:       []string{"question.linearalgebra", "question.answered"},
        },
    )
}
```

**Migration order**:
1. `student-facing-agent` (entry point)
2. `math-instructor`
3. `physics-instructor`
4. `chemistry-instructor`
5. `statistics-instructor`
6. `linearalgebra-professor`
7. `diagramming-agent`

**Verification**: Each migrated agent passes existing tests, system continues working.

---

## Step 10: Remove Legacy EventBus/Contract System

**Goal**: Clean up old code after all agents are migrated.

**Files to delete**:
- `services/agents/shared/tools/eventbus/contract.go`
- `services/agents/shared/tools/eventbus/registry.go`
- `services/agents/shared/tools/eventbus/eventbus.go`
- `services/agents/shared/tools/eventbus/emission.go`

**Files to modify**:
- `main.go` - Remove `GlobalContracts.Register()` calls, use new registry
- `infra/worker_pool.go` - Replace `ContractRegistry` with `AgentRegistry`

**New main.go structure**:

```go
func main() {
    // Initialize event sourcing infrastructure
    eventStore := sdk.NewInMemoryEventStore()  // Source of truth
    projection := infra.NewInMemoryThreadState() // Read model (projection)

    // Subscribe projection to events for real-time updates
    eventStore.SubscribeAll(ctx, func(ctx context.Context, streamID string, event sdk.Event) error {
        return projection.Update(event)
    })

    // Initialize registries
    agentRegistry := sdk.NewAgentRegistry()
    toolRegistry := sdk.NewToolRegistry()

    // Initialize chat client
    chatClient := openai.NewChatClient(os.Getenv("OPENAI_API_KEY"))

    // Register global tools
    toolRegistry.RegisterGlobal(tools.NewCalculator())
    toolRegistry.RegisterGlobal(tools.NewWhiteboard())

    // Register agents (each registers its own tools)
    studentfacing.Register(agentRegistry, toolRegistry, chatClient)
    math.Register(agentRegistry, toolRegistry, chatClient)
    physics.Register(agentRegistry, toolRegistry, chatClient)
    chemistry.Register(agentRegistry, toolRegistry, chatClient)
    statistics.Register(agentRegistry, toolRegistry, chatClient)
    linearalgebra.Register(agentRegistry, toolRegistry, chatClient)
    diagramming.Register(agentRegistry, toolRegistry, chatClient)

    // Create event bus for routing between agents
    eventBus := sdk.NewEventBus()

    // Subscribe agents to their events
    for _, meta := range agentRegistry.All() {
        for _, eventType := range meta.Listens {
            agentName := meta.Name
            eventBus.Subscribe(eventType, func(ctx context.Context, evt sdk.Event) error {
                agent, _ := agentRegistry.Get(agentName)
                // Agents READ from projection
                thread, _ := projection.GetAgentThread(evt.AggregateID())
                _, err := agent.Run(ctx, thread, nil)
                return err
            })
        }
    }

    // Create group chat manager
    groupChat := sdk.NewGroupChatManager(sdk.GroupChatConfig{
        Registry:      agentRegistry,
        EventStore:    eventStore,   // Source of truth
        Projection:    projection,   // Read model
        Selector:      sdk.NewEventBasedSelector(agentRegistry),
        MaxIterations: 10,
        Termination: func(thread sdk.AgentThread, resp *sdk.AgentResponse) bool {
            for _, evt := range resp.EventsEmitted {
                if evt.Type == "question.answered" {
                    return true
                }
            }
            return false
        },
    })

    // Setup HTTP handlers (using groupChat instead of WorkerPool)
    // ...
}
```

**Verification**: Full system works with new architecture, all tests pass.

---

## Summary: Architecture After Migration

```
┌─────────────────────────────────────────────────────────────────┐
│                         HTTP Layer                               │
│  POST /send → GET /api/jobs/{id}/stream → DELETE /api/threads   │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                    GroupChatManager                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │AgentRegistry│  │ EventBus    │  │ EventStore + Projection │  │
│  │(discovery)  │  │ (routing)   │  │ (source of truth + read)│  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                    Agent Pipeline (per agent)                    │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ LoggingMiddleware                                         │   │
│  │  └─► ValidationMiddleware                                 │   │
│  │       └─► EventSourcingMiddleware                         │   │
│  │            └─► ChatClientAgent (core implementation)      │   │
│  └──────────────────────────────────────────────────────────┘   │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                    Supporting Infrastructure                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ ToolRegistry│  │ EventLog    │  │ ChatClient (OpenAI)     │  │
│  │ (per-agent) │  │ (sourcing)  │  │                         │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Key Improvements**:
1. **Clean `AgentType` interface** - All agents implement same contract
2. **Builder pattern** - Fluent, composable agent configuration
3. **Middleware chain** - Cross-cutting concerns as decorators
4. **Service discovery** - Agents registered/discovered dynamically
5. **Per-agent tools** - Clean tool isolation via ToolRegistry
6. **Proper event sourcing** - EventStore (source of truth) + Projection (read model)
7. **Pluggable backends** - Swap InMemoryEventStore for PostgresEventStore, etc.
8. **GroupChatManager** - Clean multi-agent orchestration
