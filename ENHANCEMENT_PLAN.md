# Agent SDK Enhancement Plan
**Date**: 2026-02-01
**Version**: 1.0
**Goal**: Abstract our agent framework to enable framework portability while improving custom SDK reusability and isolating web layer from implementation details.

## Strategic Objectives

1. **Framework Portability**: Make our agent interface abstract enough to potentially adopt ADK-go or Blades without rewriting application logic
2. **Custom SDK Excellence**: Improve our custom SDK's reusability, type safety, and interface clarity while maintaining zero external dependencies
3. **Web Layer Isolation**: Decouple HTTP/SSE layer from agent framework implementation details
4. **Immediate Value**: Each step provides standalone benefit without requiring full plan completion

---

## Analysis Summary

### Current State Assessment
- **Strengths**: Clean event sourcing, good middleware pattern, streaming architecture, tool state machine
- **Weaknesses**: Domain coupling in services, minimal thread abstraction, global agent registry, weak event routing
- **Critical Gap**: Services import `domain/` package, breaking SDK standalone nature

### Framework Comparison Insights

| Feature | ADK-go | Blades | Our SDK | Target |
|---------|--------|--------|---------|--------|
| Agent Interface | Sealed, constructor-based | Open, composable | Config-based | Hybrid (composable + validated) |
| Tool System | Dynamic resolution, HITL | Dynamic + MCP, middleware | Static registry | Dynamic + validation |
| Execution Model | Iterator streaming | Generator streaming | Iterator streaming | ✓ Keep current |
| Session Management | Pluggable service | In-memory + extensible | Thread abstraction | Expand abstraction |
| Event System | Event-driven with actions | Message-based with parts | Event sourcing | ✓ Keep event sourcing |
| State Management | State scopes (app/user/temp) | Session state map | Command.Update map | Add typed state |
| Middleware | Multi-layer callbacks | Onion model | Chain pattern | ✓ Keep + enhance |
| Agent Composition | SubAgents, workflows | Sequential/Parallel/Router | No composition | **Add composition** |

### Key Learnings
1. **ADK-go**: Sealed interfaces prevent misuse, dynamic tool resolution, sophisticated callback layers, state scoping
2. **Blades**: Generator-based elegance, unified Agent interface for all components, middleware ecosystem, graph-based workflows
3. **Both**: Separate session storage interface, pluggable model providers, context injection patterns

---

## 20 Enhancement Steps

### Phase 1: Break Domain Coupling (Steps 1-4)

#### **Step 1: Create Standalone Event Package**
**File**: `foundation/agentsdk/events/` (new package)
**Goal**: Remove `domain/` dependency from SDK by creating SDK-native event types
**Changes**:
- Create `events/types.go` with SDK event types (MessageAdded, ToolExecuted, AgentHandoff, etc.)
- Move event serialization logic from `domain/` to SDK
- Add `__typename` metadata field for polymorphic deserialization
- Create event builder functions for type safety

**Immediate Benefit**: SDK can emit events without knowing consumer domain model
**Dependencies**: None
**Estimated Impact**: Enables true SDK reusability across projects

**Example**:
```go
// foundation/agentsdk/events/types.go
package events

type Event interface {
    TypeName() string
    Timestamp() time.Time
}

type MessageAdded struct {
    MessageID string
    ThreadID  string
    Role      string
    Content   string
    Timestamp time.Time
}

func (e *MessageAdded) TypeName() string { return "message.added" }
```

---

#### **Step 2: Extract EventRouter Interface**
**File**: `foundation/agentsdk/core/event_router.go` (new)
**Goal**: Separate event emission from event handling
**Changes**:
- Create `EventRouter` interface with `Route(ctx, event Event) error`
- Create `EventTracker` interface for observability
- Refactor `EventNotifier` to compose Router + Tracker
- Update `AgentConfig` to use `EventRouter` instead of `EventNotifier`

**Immediate Benefit**: Services can provide custom routing without SDK knowing implementation
**Dependencies**: Step 1
**Estimated Impact**: Clean separation between SDK (emits) and services (handle)

**Example**:
```go
// foundation/agentsdk/core/event_router.go
type EventRouter interface {
    Route(ctx context.Context, event events.Event) error
}

type EventTracker interface {
    Track(ctx context.Context, event events.Event) error
}

type CompositeEventHandler struct {
    Router  EventRouter
    Tracker EventTracker
}

func (c *CompositeEventHandler) HandleEvent(ctx context.Context, event events.Event) error {
    if err := c.Router.Route(ctx, event); err != nil {
        return err
    }
    return c.Tracker.Track(ctx, event)
}
```

---

#### **Step 3: Decouple ToolContext from RequestContext**
**File**: `foundation/agentsdk/core/tool_context.go`
**Goal**: Prevent tools from accessing full request state
**Changes**:
- Create `IsolatedToolContext` with only thread ID, message ID, observability hooks
- Replace `RequestCtx *RequestContext` with minimal context fields
- Add `StateReader` interface for read-only access to conversation state
- Tools emit events via `EventEmitter` instead of direct RequestContext access

**Immediate Benefit**: Tools become portable, testable in isolation
**Dependencies**: Step 2
**Estimated Impact**: Tools can be shared across different agent frameworks

**Example**:
```go
// foundation/agentsdk/core/tool_context.go
type ToolContext struct {
    context.Context
    ThreadID    string
    MessageID   string
    EventRouter EventRouter
    StateReader StateReader // Read-only access
}

type StateReader interface {
    GetState(key string) (any, bool)
    GetConversationSummary(ctx context.Context) (string, error)
}
```

---

#### **Step 4: Remove Domain Imports from AgentOrchestrator Service**
**File**: `services/agentorchestrator/service.go`
**Goal**: Break explicit dependency on `domain/` package
**Changes**:
- Service depends only on `foundation/agentsdk/core` and `foundation/agentsdk/events`
- Domain event types converted to SDK events at service boundary
- Service constructor accepts `EventRouter` interface instead of concrete domain implementations
- Update tests to use SDK event types

**Immediate Benefit**: Service becomes portable, can be used in different projects
**Dependencies**: Steps 1-3
**Estimated Impact**: True isolation between SDK and application domain

---

### Phase 2: Expand Thread Abstraction (Steps 5-7)

#### **Step 5: Extend AgentThread Interface with History Management**
**File**: `foundation/agentsdk/core/thread.go`
**Goal**: Add sophisticated history management capabilities
**Changes**:
- Add `GetMessageSlice(ctx, start, limit int) ([]Message, error)` for pagination
- Add `GetRecentMessages(ctx, count int) ([]Message, error)` for context window
- Add `GetMessagesByRole(ctx, role string) ([]Message, error)` for filtering
- Add `GetMessageCount(ctx) (int, error)` for UI display
- Update InMemoryThread implementation with these methods

**Immediate Benefit**: Better control over LLM context window size
**Dependencies**: None (can run in parallel with Phase 1)
**Estimated Impact**: Reduces token costs, improves response quality

**Example**:
```go
// foundation/agentsdk/core/thread.go
type AgentThread interface {
    GetID() string
    RetrieveFullConversationHistory(ctx context.Context) ([]Message, error)
    AddMessages(messages []Message) error

    // New methods
    GetMessageSlice(ctx context.Context, start, limit int) ([]Message, error)
    GetRecentMessages(ctx context.Context, count int) ([]Message, error)
    GetMessagesByRole(ctx context.Context, role string) ([]Message, error)
    GetMessageCount(ctx context.Context) (int, error)
}
```

---

#### **Step 6: Add ConversationSummarizer Interface**
**File**: `foundation/agentsdk/core/summarizer.go` (new)
**Goal**: Enable automatic history compression for long conversations
**Changes**:
- Create `ConversationSummarizer` interface with `Summarize(ctx, messages, maxTokens) (summary string, err error)`
- Create `LLMSummarizer` implementation using model provider
- Add `WithSummarizer(summarizer)` option to engine configuration
- Middleware automatically triggers summarization when token count exceeds threshold

**Immediate Benefit**: Conversations can run indefinitely without token overflow
**Dependencies**: Step 5
**Estimated Impact**: Enables long-running tutoring sessions

**Example**:
```go
// foundation/agentsdk/core/summarizer.go
type ConversationSummarizer interface {
    Summarize(ctx context.Context, messages []Message, maxTokens int) (summary string, err error)
}

type LLMSummarizer struct {
    client ChatClient
    model  string
}

func (s *LLMSummarizer) Summarize(ctx context.Context, messages []Message, maxTokens int) (string, error) {
    prompt := "Summarize the following conversation, preserving key educational context..."
    // Use LLM to generate concise summary
}
```

---

#### **Step 7: Implement SessionState with Type Safety**
**File**: `foundation/agentsdk/core/session_state.go` (new)
**Goal**: Replace untyped `Command.Update` map with typed state management
**Changes**:
- Create `SessionState` interface with `Get[T](key string) (T, error)` generic method
- Add state scopes: `AppScope`, `UserScope`, `TempScope` (inspired by ADK-go)
- Implement validation when setting state (type checking)
- Replace `Command.Update map[string]any` with `Command.StateChanges []StateChange`

**Immediate Benefit**: Compile-time type safety for state operations
**Dependencies**: None
**Estimated Impact**: Eliminates runtime type assertion bugs

**Example**:
```go
// foundation/agentsdk/core/session_state.go
type SessionState interface {
    Get[T any](ctx context.Context, key string) (T, error)
    Set(ctx context.Context, key string, value any) error
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, scope Scope) (map[string]any, error)
}

type Scope string
const (
    AppScope  Scope = "app:"
    UserScope Scope = "user:"
    TempScope Scope = "temp:"
)

type StateChange struct {
    Scope Scope
    Key   string
    Value any
    Op    StateOp // Set, Delete
}
```

---

### Phase 3: Abstract Agent Interface (Steps 8-11)

#### **Step 8: Create Unified Agent Interface**
**File**: `foundation/agentsdk/core/agent.go` (new)
**Goal**: Define framework-agnostic agent interface
**Changes**:
- Create `Agent` interface with `Name()`, `Description()`, `Run(ctx, invocation) Seq2[Message, error]`
- Create `AgentInvocation` struct (replaces current adhoc parameters)
- Agents implement interface rather than being configuration structs
- Existing agents wrapped in adapters implementing new interface

**Immediate Benefit**: Enables swapping agent implementations (ours, ADK, Blades) via adapter pattern
**Dependencies**: None
**Estimated Impact**: Foundation for framework portability

**Example**:
```go
// foundation/agentsdk/core/agent.go
type Agent interface {
    Name() string
    Description() string
    Run(ctx context.Context, inv *AgentInvocation) iter.Seq2[Message, error]
    Capabilities() *AgentCapabilities
}

type AgentInvocation struct {
    InvocationID string
    Thread       AgentThread
    Input        Message
    State        SessionState
    Tools        []Tool
}

type AgentCapabilities struct {
    SupportedTools []string
    Handoffs       []string
    MaxIterations  int
}
```

---

#### **Step 9: Implement AgentRegistry Interface**
**File**: `foundation/agentsdk/registry/registry.go`
**Goal**: Replace global singleton with dependency-injected registry
**Changes**:
- Create `AgentRegistry` interface with `Register()`, `Get()`, `List()`, `GetCapabilities()`
- Remove `GlobalAgents` variable
- Services receive registry via constructor injection
- Add validation at registration time (duplicate names, circular handoffs)

**Immediate Benefit**: Testable agent selection, no global state
**Dependencies**: Step 8
**Estimated Impact**: Easier testing, better isolation

**Example**:
```go
// foundation/agentsdk/registry/registry.go
type AgentRegistry interface {
    Register(agent Agent) error
    Get(name string) (Agent, error)
    List() []string
    GetCapabilities(name string) (*AgentCapabilities, error)
    ValidateHandoff(from, to string) error
}

type inMemoryRegistry struct {
    agents map[string]Agent
    mu     sync.RWMutex
}

func (r *inMemoryRegistry) Register(agent Agent) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.agents[agent.Name()]; exists {
        return fmt.Errorf("agent already registered: %s", agent.Name())
    }

    r.agents[agent.Name()] = agent
    return nil
}
```

---

#### **Step 10: Add Agent Composition Patterns**
**File**: `foundation/agentsdk/composition/` (new package)
**Goal**: Enable sequential, parallel, and routing agent workflows
**Changes**:
- Create `SequentialAgent` - executes sub-agents in order
- Create `ParallelAgent` - executes sub-agents concurrently, merges results
- Create `RoutingAgent` - uses LLM to select appropriate sub-agent
- All implement `Agent` interface for composability

**Immediate Benefit**: Complex multi-agent workflows without custom orchestration code
**Dependencies**: Step 8
**Estimated Impact**: Simplifies student-facing-agent routing logic

**Example**:
```go
// foundation/agentsdk/composition/sequential.go
type SequentialAgent struct {
    name      string
    subAgents []Agent
}

func (s *SequentialAgent) Run(ctx context.Context, inv *AgentInvocation) iter.Seq2[Message, error] {
    return func(yield func(Message, error) bool) {
        currentInput := inv.Input
        for _, agent := range s.subAgents {
            childInv := &AgentInvocation{
                InvocationID: inv.InvocationID,
                Thread:       inv.Thread,
                Input:        currentInput,
                State:        inv.State,
                Tools:        inv.Tools,
            }

            var lastMessage Message
            for msg, err := range agent.Run(ctx, childInv) {
                if err != nil {
                    yield(Message{}, err)
                    return
                }
                lastMessage = msg
                if !yield(msg, nil) {
                    return
                }
            }
            currentInput = lastMessage
        }
    }
}
```

---

#### **Step 11: Create ModelProvider Abstraction**
**File**: `foundation/agentsdk/llm/model_provider.go` (new)
**Goal**: Standardize interface for different LLM providers
**Changes**:
- Create `ModelProvider` interface abstracting `ChatClient`
- Move OpenAI, Gemini, Ollama adapters to implement `ModelProvider`
- Add `StreamingMode` configuration (SSE vs batch)
- AgentConfig accepts `ModelProvider` instead of `ChatClient`

**Immediate Benefit**: Easier to add new models (Claude, DeepSeek, etc.)
**Dependencies**: None
**Estimated Impact**: Future-proof for model changes

**Example**:
```go
// foundation/agentsdk/llm/model_provider.go
type ModelProvider interface {
    Name() string
    Generate(ctx context.Context, req *ModelRequest) (*ModelResponse, error)
    GenerateStream(ctx context.Context, req *ModelRequest) iter.Seq2[*ModelResponse, error]
}

type ModelRequest struct {
    Model         string
    Messages      []Message
    Tools         []Tool
    Temperature   float64
    MaxTokens     int
    StreamingMode StreamingMode
}

type StreamingMode string
const (
    StreamingModeNone StreamingMode = "none"
    StreamingModeSSE  StreamingMode = "sse"
)
```

---

### Phase 4: Isolate Web Layer (Steps 12-15)

#### **Step 12: Create AgentExecutionResult Interface**
**File**: `foundation/agentsdk/core/result.go` (new)
**Goal**: Decouple agent execution output from HTTP/SSE serialization
**Changes**:
- Create `ExecutionResult` interface with `Messages()`, `State()`, `Events()`, `Status()`
- Engine returns `ExecutionResult` instead of directly streaming events
- HTTP handlers convert `ExecutionResult` to AG-UI protocol
- Result can be serialized to different formats (JSON, SSE, protobuf)

**Immediate Benefit**: Agent SDK doesn't know about HTTP/SSE
**Dependencies**: None
**Estimated Impact**: Enables CLI, gRPC, or other non-HTTP interfaces

**Example**:
```go
// foundation/agentsdk/core/result.go
type ExecutionResult interface {
    InvocationID() string
    Messages() []Message
    StateChanges() []StateChange
    Events() []events.Event
    Status() ExecutionStatus
    Error() error
}

type ExecutionStatus string
const (
    StatusInProgress ExecutionStatus = "in_progress"
    StatusCompleted  ExecutionStatus = "completed"
    StatusFailed     ExecutionStatus = "failed"
)
```

---

#### **Step 13: Extract Protocol Adapter Layer**
**File**: `infra/protocol/` (new package)
**Goal**: Isolate AG-UI protocol from agent framework
**Changes**:
- Create `ProtocolAdapter` interface for converting `ExecutionResult` to wire format
- Implement `AGUIProtocolAdapter` for current AG-UI SSE format
- HTTP handlers use adapter instead of direct event serialization
- Add `JSONProtocolAdapter` for REST API responses

**Immediate Benefit**: Can support multiple client protocols simultaneously
**Dependencies**: Step 12
**Estimated Impact**: Enables mobile apps, different frontend frameworks

**Example**:
```go
// infra/protocol/adapter.go
type ProtocolAdapter interface {
    SerializeResult(result ExecutionResult) ([]byte, error)
    StreamResult(ctx context.Context, result ExecutionResult) iter.Seq2[[]byte, error]
    ContentType() string
}

type AGUIProtocolAdapter struct {
    eventSerializer EventSerializer
}

func (a *AGUIProtocolAdapter) StreamResult(ctx context.Context, result ExecutionResult) iter.Seq2[[]byte, error] {
    return func(yield func([]byte, error) bool) {
        for _, event := range result.Events() {
            data := a.eventSerializer.Serialize(event)
            sseData := fmt.Sprintf("data: %s\n\n", data)
            if !yield([]byte(sseData), nil) {
                return
            }
        }
    }
}

func (a *AGUIProtocolAdapter) ContentType() string {
    return "text/event-stream"
}
```

---

#### **Step 14: Create HTTP Handler Facade**
**File**: `app/api/handlers/agent_handler.go`
**Goal**: Thin HTTP layer that delegates to service
**Changes**:
- Handler only knows about HTTP concerns (routing, auth, serialization)
- Accepts `AgentService` interface (not concrete implementation)
- Service returns `ExecutionResult`, handler converts to HTTP response
- All agent-specific logic lives in service layer

**Immediate Benefit**: HTTP layer can be replaced (gRPC, GraphQL) without touching business logic
**Dependencies**: Steps 12-13
**Estimated Impact**: Clean architecture boundaries

**Example**:
```go
// app/api/handlers/agent_handler.go
type AgentHandler struct {
    agentService    AgentService
    protocolAdapter protocol.ProtocolAdapter
}

func (h *AgentHandler) HandleSendMessage(w http.ResponseWriter, r *http.Request) {
    // Parse HTTP request
    var req SendMessageRequest
    json.NewDecoder(r.Body).Decode(&req)

    // Call service
    result, err := h.agentService.ProcessMessage(r.Context(), &req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Convert to protocol format
    w.Header().Set("Content-Type", h.protocolAdapter.ContentType())
    for data, err := range h.protocolAdapter.StreamResult(r.Context(), result) {
        if err != nil {
            return
        }
        w.Write(data)
        w.(http.Flusher).Flush()
    }
}
```

---

#### **Step 15: Implement AgentService Interface**
**File**: `services/agentorchestrator/interface.go` (new)
**Goal**: Define service contract independent of implementation
**Changes**:
- Create `AgentService` interface with `ProcessMessage()`, `GetThread()`, `DeleteThread()`
- Current orchestrator implements interface
- Interface uses only SDK types, no domain imports
- Enables multiple implementations (cached, mocked, remote)

**Immediate Benefit**: Easy to mock for API tests
**Dependencies**: Step 12
**Estimated Impact**: Better testability, swap implementations

**Example**:
```go
// services/agentorchestrator/interface.go
type AgentService interface {
    ProcessMessage(ctx context.Context, req *ProcessMessageRequest) (core.ExecutionResult, error)
    GetThread(ctx context.Context, threadID string) (core.AgentThread, error)
    DeleteThread(ctx context.Context, threadID string) error
}

type ProcessMessageRequest struct {
    ThreadID  string
    Message   core.Message
    UserID    string
    SessionID string
}
```

---

### Phase 5: Enhance Custom SDK (Steps 16-18)

#### **Step 16: Add SDK Error Types**
**File**: `foundation/agentsdk/errors/` (new package)
**Goal**: Standardized error handling with context
**Changes**:
- Create error types: `ToolExecutionError`, `AgentNotFoundError`, `InvalidStateError`, etc.
- Implement `errors.Is()` and `errors.As()` support
- Add error codes for programmatic handling
- Include stack traces in development mode

**Immediate Benefit**: Clients can handle errors programmatically
**Dependencies**: None
**Estimated Impact**: Better error debugging, clearer API contracts

**Example**:
```go
// foundation/agentsdk/errors/errors.go
type SDKError struct {
    Code       ErrorCode
    Message    string
    Cause      error
    StackTrace []string
}

type ErrorCode string
const (
    ErrCodeToolNotFound      ErrorCode = "TOOL_NOT_FOUND"
    ErrCodeAgentNotFound     ErrorCode = "AGENT_NOT_FOUND"
    ErrCodeInvalidState      ErrorCode = "INVALID_STATE"
    ErrCodeExecutionTimeout  ErrorCode = "EXECUTION_TIMEOUT"
)

func NewToolExecutionError(toolName string, cause error) *SDKError {
    return &SDKError{
        Code:    ErrCodeToolNotFound,
        Message: fmt.Sprintf("tool execution failed: %s", toolName),
        Cause:   cause,
    }
}
```

---

#### **Step 17: Implement Tool Middleware Support**
**File**: `foundation/agentsdk/tools/middleware.go` (new)
**Goal**: Apply cross-cutting concerns to tool execution
**Changes**:
- Create `ToolMiddleware` type (similar to agent middleware)
- Add `WithMiddleware()` option to tool registration
- Built-in middleware: `RetryMiddleware`, `TimeoutMiddleware`, `ValidationMiddleware`
- Middleware can short-circuit tool execution

**Immediate Benefit**: Consistent retry/timeout logic across all tools
**Dependencies**: None
**Estimated Impact**: More robust tool execution

**Example**:
```go
// foundation/agentsdk/tools/middleware.go
type ToolMiddleware func(ToolHandler) ToolHandler

type ToolHandler func(ctx context.Context, args any) (string, error)

func RetryMiddleware(maxRetries int, backoff time.Duration) ToolMiddleware {
    return func(next ToolHandler) ToolHandler {
        return func(ctx context.Context, args any) (string, error) {
            var result string
            var err error

            for i := 0; i < maxRetries; i++ {
                result, err = next(ctx, args)
                if err == nil {
                    return result, nil
                }

                if i < maxRetries-1 {
                    time.Sleep(backoff * time.Duration(i+1))
                }
            }

            return "", fmt.Errorf("tool failed after %d retries: %w", maxRetries, err)
        }
    }
}

// Usage in tool registration
tool := tools.NewTool("flaky_api",
    tools.WithHandler(apiHandler),
    tools.WithMiddleware(
        RetryMiddleware(3, time.Second),
        TimeoutMiddleware(10*time.Second),
    ),
)
```

---

#### **Step 18: Add Dynamic Tool Resolution**
**File**: `foundation/agentsdk/tools/resolver.go` (new)
**Goal**: Enable runtime tool discovery (MCP, plugins)
**Changes**:
- Create `ToolResolver` interface with `Resolve(ctx, agentName) ([]Tool, error)`
- Implement `StaticResolver` (current registry)
- Implement `MCPResolver` for Model Context Protocol servers
- AgentConfig accepts `ToolResolver` instead of fixed tool list

**Immediate Benefit**: Can integrate external tool providers (MCP servers, API plugins)
**Dependencies**: None
**Estimated Impact**: Extensible tool ecosystem

**Example**:
```go
// foundation/agentsdk/tools/resolver.go
type ToolResolver interface {
    Resolve(ctx context.Context, agentName string) ([]Tool, error)
    Refresh(ctx context.Context) error
}

type CompositeResolver struct {
    resolvers []ToolResolver
}

func (c *CompositeResolver) Resolve(ctx context.Context, agentName string) ([]Tool, error) {
    var allTools []Tool
    for _, resolver := range c.resolvers {
        tools, err := resolver.Resolve(ctx, agentName)
        if err != nil {
            return nil, err
        }
        allTools = append(allTools, tools...)
    }
    return allTools, nil
}

// Usage
resolver := tools.NewCompositeResolver(
    tools.NewStaticResolver(builtinTools),
    tools.NewMCPResolver(mcpServerURL),
)
```

---

### Phase 6: Observability & Testing (Steps 19-20)

#### **Step 19: Extract Observability Interface**
**File**: `foundation/agentsdk/observability/` (new package)
**Goal**: Separate tracing/metrics from core execution
**Changes**:
- Create `ObservabilityProvider` interface with `TraceExecution()`, `TraceToolCall()`, `RecordMetric()`
- Remove `ToolTracer` from `ToolExecutor`, inject via context
- Implement `LangfuseProvider`, `OpenTelemetryProvider`, `NoOpProvider`
- Middleware uses observability provider instead of direct Langfuse calls

**Immediate Benefit**: Swap observability backends without code changes
**Dependencies**: None
**Estimated Impact**: Flexibility for different deployment environments

**Example**:
```go
// foundation/agentsdk/observability/provider.go
type ObservabilityProvider interface {
    TraceExecution(ctx context.Context, trace *ExecutionTrace) error
    TraceToolCall(ctx context.Context, call *ToolCallTrace) error
    RecordMetric(ctx context.Context, metric *Metric) error
}

type ExecutionTrace struct {
    InvocationID string
    AgentName    string
    StartTime    time.Time
    EndTime      time.Time
    Status       string
    Error        error
    Metadata     map[string]any
}

type ToolCallTrace struct {
    ToolName    string
    Args        any
    Result      string
    Duration    time.Duration
    Error       error
}

// Context injection
ctx = observability.WithProvider(ctx, langfuseProvider)
provider, _ := observability.FromContext(ctx)
```

---

#### **Step 20: Create Agent Test Harness**
**File**: `foundation/agentsdk/testing/` (new package)
**Goal**: Standardized testing utilities for agents
**Changes**:
- Create `MockAgent` implementing `Agent` interface
- Create `MockModelProvider` with canned responses
- Create `MockToolResolver` for test tools
- Create `AgentTestSuite` with common test scenarios
- Add `AssertConversationFlow()` helper for multi-turn validation

**Immediate Benefit**: Faster, more reliable agent tests
**Dependencies**: Steps 8, 11
**Estimated Impact**: Better test coverage, easier debugging

**Example**:
```go
// foundation/agentsdk/testing/harness.go
type AgentTestHarness struct {
    agent         Agent
    mockProvider  *MockModelProvider
    mockResolver  *MockToolResolver
    thread        AgentThread
}

func NewTestHarness(agent Agent) *AgentTestHarness {
    return &AgentTestHarness{
        agent:        agent,
        mockProvider: NewMockModelProvider(),
        mockResolver: NewMockToolResolver(),
        thread:       core.NewInMemoryThread("test-thread"),
    }
}

func (h *AgentTestHarness) SetResponse(response string) {
    h.mockProvider.SetNextResponse(response)
}

func (h *AgentTestHarness) RegisterTool(tool Tool) {
    h.mockResolver.Register(tool)
}

func (h *AgentTestHarness) Run(t *testing.T, input string) []Message {
    inv := &AgentInvocation{
        InvocationID: "test-inv",
        Thread:       h.thread,
        Input:        Message{Content: input},
    }

    var messages []Message
    for msg, err := range h.agent.Run(context.Background(), inv) {
        require.NoError(t, err)
        messages = append(messages, msg)
    }

    return messages
}

// Usage in tests
func TestMathInstructor(t *testing.T) {
    agent := agents.NewMathInstructor(config)
    harness := testing.NewTestHarness(agent)

    harness.SetResponse("Let me solve that equation...")
    messages := harness.Run(t, "Solve x^2 = 4")

    assert.Contains(t, messages[0].Content, "x = 2 or x = -2")
}
```

---

## Implementation Strategy

### Recommended Execution Order

**Week 1-2: Foundation (Steps 1-4)**
Break domain coupling to establish SDK independence. This is the highest priority as it unblocks everything else.

**Week 3-4: Thread Enhancement (Steps 5-7)**
Improve core abstractions for better conversation management. Provides immediate user-facing benefits.

**Week 5-6: Agent Abstraction (Steps 8-11)**
Create framework-agnostic interfaces. Enables potential migration to ADK/Blades in future.

**Week 7-8: Web Isolation (Steps 12-15)**
Decouple HTTP layer from business logic. Improves architecture quality.

**Week 9-10: SDK Enhancement (Steps 16-18)**
Polish SDK with error handling, tool middleware, dynamic resolution.

**Week 11-12: Observability & Testing (Steps 19-20)**
Add testing and observability infrastructure for long-term maintainability.

### Parallel Execution Opportunities

Can run in parallel:
- **Phase 1** (Steps 1-4) + **Phase 2** (Steps 5-7) - Different subsystems
- **Phase 4** (Steps 12-15) + **Phase 5** (Steps 16-18) - Different layers

Must run sequentially:
- Phase 1 before Phase 3 (Agent abstraction depends on event decoupling)
- Phase 3 before Phase 6 (Testing harness needs Agent interface)

---

## Success Metrics

### Technical Metrics
- **Zero domain imports in SDK**: `go list -f '{{.Imports}}' ./foundation/agentsdk/... | grep domain` returns empty
- **Agent interface compliance**: All agents implement unified `Agent` interface
- **Test coverage**: >80% coverage in SDK packages
- **Performance**: No regression in message processing latency (<100ms overhead)

### Architectural Metrics
- **Coupling**: SDK package has zero imports from `domain/`, `services/`, `app/`
- **Abstraction count**: At least 5 interface-based injection points (EventRouter, ModelProvider, ToolResolver, ObservabilityProvider, AgentRegistry)
- **Reusability**: SDK can be imported into new project without modification

### User-Facing Metrics
- **No breaking changes**: Existing conversations continue working
- **Feature parity**: All current features work after refactoring
- **Migration path**: Clear adapter pattern for old->new agents

---

## Risk Mitigation

### Risk: Breaking Changes to Production
**Mitigation**:
- Adapter pattern for backward compatibility
- Feature flags for gradual rollout
- Keep old code paths until new ones proven

### Risk: Performance Degradation
**Mitigation**:
- Benchmark before/after each phase
- Profile streaming performance
- Monitor production latency

### Risk: Scope Creep
**Mitigation**:
- Each step is independently valuable
- Can stop at any phase and still gain benefits
- No dependencies on external framework adoption

### Risk: Team Learning Curve
**Mitigation**:
- Documentation for each new interface
- Code examples in `testing/` package
- Gradual rollout allows learning in stages

---

## Framework Migration Path (Optional)

If after completion we decide to adopt ADK-go or Blades:

### Migration to ADK-go
1. Implement `agent.Agent` adapter wrapping our `core.Agent`
2. Replace our `ModelProvider` with ADK's `model.LLM`
3. Map our `Tool` to ADK's `tool.Tool`
4. Keep our event sourcing (ADK supports custom state storage)
5. Gradual agent-by-agent migration

**Estimated effort**: 2-3 weeks for full migration after plan completion

### Migration to Blades
1. Implement Blades `Agent` interface adapter
2. Replace `ChatClient` with Blades `ModelProvider`
3. Map our middleware to Blades middleware
4. Keep our event sourcing projection system
5. Leverage Blades flow compositions

**Estimated effort**: 2-3 weeks for full migration after plan completion

### Stay with Custom SDK
If we decide to keep our custom SDK, we'll have:
- Framework-quality abstractions
- Clean architecture boundaries
- Reusable components
- Well-tested infrastructure
- No external dependencies

---

## Appendix: File Changes Summary

| Step | New Files | Modified Files | Deleted Files |
|------|-----------|----------------|---------------|
| 1 | `foundation/agentsdk/events/*.go` | None | None |
| 2 | `foundation/agentsdk/core/event_router.go` | `core/config.go` | None |
| 3 | None | `core/tool_context.go` | None |
| 4 | None | `services/agentorchestrator/service.go` | None |
| 5 | None | `core/thread.go`, `core/inmemory_thread.go` | None |
| 6 | `core/summarizer.go` | None | None |
| 7 | `core/session_state.go` | `core/command.go` | None |
| 8 | `core/agent.go`, `core/invocation.go` | `engine/*.go` | None |
| 9 | `registry/interface.go` | `registry/agentregistry.go` | None |
| 10 | `composition/*.go` | None | None |
| 11 | `llm/model_provider.go` | `llm/client.go` | None |
| 12 | `core/result.go` | `engine/engine.go` | None |
| 13 | `infra/protocol/*.go` | None | None |
| 14 | None | `app/api/handlers/*.go` | None |
| 15 | `services/agentorchestrator/interface.go` | None | None |
| 16 | `errors/*.go` | All packages (error handling) | None |
| 17 | `tools/middleware.go` | `tools/*.go` | None |
| 18 | `tools/resolver.go` | `core/config.go` | None |
| 19 | `observability/*.go` | `core/tool_executor.go` | None |
| 20 | `testing/*.go` | None | None |

**Total**: ~40 new files, ~25 modified files, 0 deleted files

---

## Conclusion

This enhancement plan provides a **systematic path to framework-quality abstractions** while maintaining backward compatibility and delivering immediate value at each step. The plan achieves both goals:

1. **Framework Portability**: Agent interface abstract enough to adopt ADK-go or Blades if needed
2. **Custom SDK Excellence**: Improved reusability, type safety, and clean architecture

Each step is **atomic, explicit, and independently valuable**, ensuring the project can benefit even if only partially completed.
