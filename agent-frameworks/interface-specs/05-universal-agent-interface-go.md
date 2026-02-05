# Universal Agent Framework Interface - Go Specification

## Document Purpose

This document defines a **universal, vendor-agnostic Go interface specification** for AI agent frameworks. It synthesizes the best patterns from **Google ADK**, **Microsoft Agent Framework**, and **AWS Strands** into a cohesive, idiomatic Go API.

### Design Philosophy

1. **Interface Segregation**: Small, focused interfaces that can be composed
2. **Vendor Agnostic**: No dependencies on specific LLM providers
3. **Context-First**: All operations accept `context.Context` for cancellation/timeout
4. **Explicit Over Implicit**: Clear contracts, no hidden behavior
5. **Composition Over Configuration**: Build complex behavior from simple parts
6. **Error As Values**: Rich error types with context for debugging

---

## Table of Contents

1. [Core Design Principles](#1-core-design-principles)
2. [Foundational Types](#2-foundational-types)
3. [Agent Interface](#3-agent-interface)
4. [Tool Interface](#4-tool-interface)
5. [Model Interface](#5-model-interface)
6. [Message Types](#6-message-types)
7. [Session Management](#7-session-management)
8. [Event System](#8-event-system)
9. [Hook System](#9-hook-system)
10. [Middleware](#10-middleware)
11. [Multi-Agent Orchestration](#11-multi-agent-orchestration)
12. [Services](#12-services)
13. [Error Handling](#13-error-handling)
14. [Package Structure](#14-package-structure)

---

## 1. Core Design Principles

### 1.1 Interface Design Rules

```go
// Rule 1: Accept interfaces, return structs
func ProcessAgent(a Agent) *Result    // Good
func ProcessAgent(a Agent) Agent      // Avoid (unless factory)

// Rule 2: Keep interfaces small (1-3 methods ideal)
type Runner interface {
    Run(ctx context.Context, input Input) (Output, error)
}

// Rule 3: Define interfaces where they're used, not where implemented
// consumer package defines what it needs

// Rule 4: Use context.Context as first parameter
func (a *agent) Run(ctx context.Context, ...) error

// Rule 5: Return concrete errors, accept error interface
func (a *agent) Run(...) error         // Returns error interface
type AgentError struct { ... }          // Concrete error type for inspection
```

### 1.2 Naming Conventions

```go
// Interfaces: capability-focused names
type Runner interface { Run(...) }      // -er suffix for single-method
type Agent interface { ... }            // Noun for role/entity

// No "I" prefix or "Interface" suffix
type IAgent interface { ... }           // Bad (C# style)
type AgentInterface interface { ... }   // Bad

// Methods: verb for actions, noun for getters
func (a *agent) Execute() error         // Action
func (a *agent) Name() string           // Getter (no "Get" prefix)
func (a *agent) IsRunning() bool        // Boolean getter

// Constructors
func New(opts ...Option) (*Agent, error)           // Primary
func NewWithConfig(cfg Config) (*Agent, error)    // Alternative

// Options pattern
type Option func(*options)
func WithName(name string) Option
func WithTimeout(d time.Duration) Option
```

### 1.3 Error Handling Patterns

```go
// Sentinel errors for type checking
var (
    ErrNotFound     = errors.New("not found")
    ErrInvalidInput = errors.New("invalid input")
)

// Structured errors for rich context
type Error struct {
    Op      string         // Operation that failed
    Kind    ErrorKind      // Category of error
    Err     error          // Underlying error
    Context map[string]any // Additional context
}

func (e *Error) Error() string { ... }
func (e *Error) Unwrap() error { return e.Err }

// Error checking
if errors.Is(err, ErrNotFound) { ... }
var agentErr *AgentError
if errors.As(err, &agentErr) { ... }
```

---

## 2. Foundational Types

### 2.1 Core Value Types

```go
package agent

import (
    "context"
    "encoding/json"
    "time"
)

// ID is a unique identifier. Use type alias for documentation.
type ID = string

// Metadata holds arbitrary key-value pairs.
type Metadata map[string]any

// Content represents any content that can be part of a message.
type Content interface {
    ContentType() string
    isContent()  // Marker method to restrict implementations
}

// TextContent is plain text content.
type TextContent struct {
    Text string
}

func (TextContent) ContentType() string { return "text" }
func (TextContent) isContent()          {}

// BinaryContent is binary data with MIME type.
type BinaryContent struct {
    Data     []byte
    MimeType string
    Name     string // Optional filename
}

func (BinaryContent) ContentType() string { return "binary" }
func (BinaryContent) isContent()          {}

// JSONContent is structured JSON data.
type JSONContent struct {
    Data json.RawMessage
}

func (JSONContent) ContentType() string { return "json" }
func (JSONContent) isContent()          {}

// FunctionCall represents a request to invoke a function.
type FunctionCall struct {
    ID        string
    Name      string
    Arguments json.RawMessage
}

func (FunctionCall) ContentType() string { return "function_call" }
func (FunctionCall) isContent()          {}

// FunctionResult is the result of a function invocation.
type FunctionResult struct {
    CallID  string
    Name    string
    Result  json.RawMessage
    IsError bool
}

func (FunctionResult) ContentType() string { return "function_result" }
func (FunctionResult) isContent()          {}
```

### 2.2 Schema Types (Vendor-Agnostic)

```go
package schema

// Schema describes the structure of data for tool parameters or structured output.
type Schema struct {
    Type        SchemaType         `json:"type"`
    Description string             `json:"description,omitempty"`
    Properties  map[string]*Schema `json:"properties,omitempty"`
    Required    []string           `json:"required,omitempty"`
    Items       *Schema            `json:"items,omitempty"`       // For arrays
    Enum        []string           `json:"enum,omitempty"`        // For enums
    Format      string             `json:"format,omitempty"`      // date-time, email, etc.
    Default     any                `json:"default,omitempty"`
    Nullable    bool               `json:"nullable,omitempty"`
}

type SchemaType string

const (
    TypeString  SchemaType = "string"
    TypeNumber  SchemaType = "number"
    TypeInteger SchemaType = "integer"
    TypeBoolean SchemaType = "boolean"
    TypeArray   SchemaType = "array"
    TypeObject  SchemaType = "object"
)

// FromType generates a Schema from a Go type using reflection.
func FromType[T any]() (*Schema, error)

// Validate checks if data conforms to the schema.
func (s *Schema) Validate(data any) error
```

---

## 3. Agent Interface

### 3.1 Core Interfaces (Interface Segregation)

```go
package agent

import (
    "context"
    "iter"
)

// Identifiable provides identity information.
type Identifiable interface {
    ID() ID
    Name() string
    Description() string
}

// Runner executes agent logic and produces events.
type Runner interface {
    // Run executes the agent with the given context and input.
    // Returns an iterator of events for streaming support.
    Run(ctx context.Context, input *Input) iter.Seq2[Event, error]
}

// Agent is the primary interface combining identity and execution.
// Keep it minimal - additional capabilities via type assertions.
type Agent interface {
    Identifiable
    Runner
}

// Input represents the input to an agent invocation.
type Input struct {
    Messages []Message      // Conversation messages
    State    State          // Shared state
    Config   *InvokeConfig  // Per-invocation configuration
}

// InvokeConfig holds per-invocation settings.
type InvokeConfig struct {
    MaxIterations int
    Timeout       time.Duration
    Streaming     bool
    Metadata      Metadata
}
```

### 3.2 Extended Agent Capabilities (Via Type Assertion)

```go
package agent

// Hierarchical agents support parent/child relationships.
type Hierarchical interface {
    Agent
    Parent() Agent
    Children() []Agent
}

// Toolable agents can be converted to tools for use by other agents.
type Toolable interface {
    Agent
    AsTool(opts ...AsToolOption) (Tool, error)
}

type AsToolOption func(*asToolConfig)

func WithToolName(name string) AsToolOption
func WithToolDescription(desc string) AsToolOption

// Configurable agents expose their configuration.
type Configurable interface {
    Agent
    Config() Config
    Reconfigure(opts ...Option) error
}

// Stateful agents maintain internal state across invocations.
type Stateful interface {
    Agent
    State() State
    ResetState() error
}

// Serializable agents can be persisted and restored.
type Serializable interface {
    Agent
    MarshalState() ([]byte, error)
    UnmarshalState(data []byte) error
}

// ServiceProvider exposes internal services for advanced use.
type ServiceProvider interface {
    Agent
    // GetService returns a service by type, or nil if not available.
    // Use type assertion on the result.
    GetService(serviceType any) any
}
```

### 3.3 Agent Factory

```go
package agent

// Factory creates agents from configuration.
type Factory interface {
    // Create builds an agent from the given configuration.
    Create(ctx context.Context, cfg Config) (Agent, error)
    
    // SupportedTypes returns the agent types this factory can create.
    SupportedTypes() []string
}

// Config is a vendor-agnostic agent configuration.
type Config struct {
    // Identity
    ID          ID
    Name        string
    Description string
    
    // Model (provider-specific configuration passed through)
    ModelConfig any
    
    // Instructions
    SystemPrompt    string
    PromptTemplates map[string]string
    
    // Capabilities
    Tools     []Tool
    SubAgents []Agent
    
    // Behavior
    MaxIterations int
    Hooks         []HookProvider
    Middleware    []Middleware
    
    // Extensions (provider-specific)
    Extensions Metadata
}
```

---

## 4. Tool Interface

### 4.1 Core Tool Interface

```go
package tool

import (
    "context"
)

// Tool represents a capability that an agent can invoke.
// This is the minimal interface - implementations add behavior.
type Tool interface {
    // Name returns the unique identifier for this tool.
    Name() string
    
    // Description returns a human-readable description for the LLM.
    Description() string
    
    // Schema returns the JSON Schema for the tool's parameters.
    Schema() *schema.Schema
}

// Executor can execute tool invocations.
type Executor interface {
    // Execute runs the tool with the given arguments.
    Execute(ctx context.Context, args Arguments) (Result, error)
}

// ExecutableTool combines Tool metadata with execution capability.
type ExecutableTool interface {
    Tool
    Executor
}

// Arguments holds the decoded arguments for a tool invocation.
type Arguments struct {
    Raw    json.RawMessage // Original JSON
    CallID string          // Unique ID for this invocation
}

// Decode unmarshals arguments into the target type.
func (a Arguments) Decode(target any) error

// Result represents the outcome of a tool execution.
type Result struct {
    Content  []Content      // Output content
    Error    error          // Execution error, if any
    Metadata Metadata       // Additional result metadata
}

// Success creates a successful result with text content.
func Success(text string) Result

// SuccessJSON creates a successful result with JSON content.
func SuccessJSON(data any) Result

// Failure creates a failed result.
func Failure(err error) Result
```

### 4.2 Tool Behavioral Traits (Via Type Assertion)

```go
package tool

// LongRunning tools return immediately with a handle for later completion.
type LongRunning interface {
    Tool
    // IsLongRunning returns true if this tool runs asynchronously.
    IsLongRunning() bool
}

// Confirmable tools require user confirmation before execution.
type Confirmable interface {
    Tool
    // RequiresConfirmation returns true if user must confirm.
    RequiresConfirmation() bool
    // ConfirmationMessage returns the message to show the user.
    ConfirmationMessage(args Arguments) string
}

// Stateful tools maintain state across invocations.
type Stateful interface {
    Tool
    // Reset clears any internal state.
    Reset() error
}

// Closable tools hold resources that need cleanup.
type Closable interface {
    Tool
    Close() error
}
```

### 4.3 Function Tool (Type-Safe Generic)

```go
package tool

// Func is the signature for type-safe tool handlers.
type Func[TArgs, TResult any] func(ctx context.Context, args TArgs) (TResult, error)

// FunctionConfig configures a function-based tool.
type FunctionConfig[TArgs, TResult any] struct {
    Name        string
    Description string
    Handler     Func[TArgs, TResult]
    
    // Optional overrides
    InputSchema  *schema.Schema // Override auto-generated
    OutputSchema *schema.Schema
    
    // Behavior flags
    LongRunning   bool
    Confirmable   bool
    ConfirmPrompt string
}

// NewFunction creates a type-safe tool from a function.
// Schema is generated from TArgs via reflection.
func NewFunction[TArgs, TResult any](cfg FunctionConfig[TArgs, TResult]) (ExecutableTool, error)

// Example usage:
//
//   type SearchArgs struct {
//       Query string `json:"query" description:"Search query"`
//       Limit int    `json:"limit,omitempty" description:"Max results"`
//   }
//
//   searchTool, _ := tool.NewFunction(tool.FunctionConfig[SearchArgs, []SearchResult]{
//       Name:        "search",
//       Description: "Search the knowledge base",
//       Handler:     searchHandler,
//   })
```

### 4.4 Tool Provider and Registry

```go
package tool

import "context"

// Provider dynamically supplies tools, supporting lazy loading and MCP.
type Provider interface {
    // Load retrieves all tools from this provider.
    Load(ctx context.Context) ([]Tool, error)
    
    // Close releases any resources held by the provider.
    Close() error
}

// Registry manages tool registration and lookup.
type Registry interface {
    // Register adds a tool to the registry.
    Register(t Tool) error
    
    // Get retrieves a tool by name.
    Get(name string) (Tool, bool)
    
    // All returns all registered tools.
    All() []Tool
    
    // LoadFrom adds tools from a provider.
    LoadFrom(ctx context.Context, p Provider) error
}

// NewRegistry creates a new tool registry.
func NewRegistry() Registry
```

---

## 5. Model Interface

### 5.1 Core LLM Interface (Vendor-Agnostic)

```go
package model

import (
    "context"
    "iter"
)

// LLM represents any language model backend.
// This is the minimal interface - all provider-specific features
// are accessed via type assertions on the implementation.
type LLM interface {
    // Generate produces a response for the given request.
    // The iterator yields streaming chunks if supported.
    Generate(ctx context.Context, req *Request) iter.Seq2[*Response, error]
}

// Request contains all parameters for a model call.
// Designed to be provider-agnostic.
type Request struct {
    // Messages is the conversation history.
    Messages []Message
    
    // System is the system prompt/instructions.
    System string
    
    // Tools available for this request.
    Tools []ToolDeclaration
    
    // Parameters for generation (temperature, etc.)
    Parameters *Parameters
    
    // ResponseFormat for structured output.
    ResponseFormat *ResponseFormat
    
    // Extensions for provider-specific features.
    Extensions Metadata
}

// Parameters controls generation behavior.
type Parameters struct {
    MaxTokens        int
    Temperature      *float64 // Pointer allows distinguishing unset from 0
    TopP             *float64
    TopK             *int
    StopSequences    []string
    PresencePenalty  *float64
    FrequencyPenalty *float64
}

// Response contains the model's output.
type Response struct {
    // Content is the generated content.
    Content []Content
    
    // FinishReason indicates why generation stopped.
    FinishReason FinishReason
    
    // Usage tracks token consumption.
    Usage *Usage
    
    // Partial indicates this is a streaming chunk.
    Partial bool
    
    // Extensions for provider-specific data.
    Extensions Metadata
}

// FinishReason indicates why generation stopped.
type FinishReason string

const (
    FinishReasonStop          FinishReason = "stop"
    FinishReasonLength        FinishReason = "length"
    FinishReasonToolUse       FinishReason = "tool_use"
    FinishReasonContentFilter FinishReason = "content_filter"
)

// Usage tracks token consumption.
type Usage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
}

// ToolDeclaration describes a tool for the model.
type ToolDeclaration struct {
    Name        string
    Description string
    Parameters  *schema.Schema
}

// ResponseFormat specifies structured output requirements.
type ResponseFormat struct {
    Type   ResponseFormatType
    Schema *schema.Schema // For JSON mode
}

type ResponseFormatType string

const (
    ResponseFormatText ResponseFormatType = "text"
    ResponseFormatJSON ResponseFormatType = "json"
)
```

### 5.2 Extended Model Capabilities

```go
package model

import "context"

// Streamable models support streaming generation.
type Streamable interface {
    LLM
    SupportsStreaming() bool
}

// StructuredOutput models support structured output.
type StructuredOutput interface {
    LLM
    SupportsStructuredOutput() bool
}

// ToolUser models can use tools.
type ToolUser interface {
    LLM
    SupportsTools() bool
}

// Cacheable models support context caching.
type Cacheable interface {
    LLM
    Cache(ctx context.Context, messages []Message) (CacheHandle, error)
}

type CacheHandle interface {
    ID() string
    Invalidate(ctx context.Context) error
}

// Configurable models expose their configuration.
type Configurable interface {
    LLM
    Config() any
    WithConfig(cfg any) (LLM, error)
}
```

### 5.3 Model Registry

```go
package model

import "context"

// Registry manages model instantiation by name.
type Registry interface {
    // Register adds a model factory for a provider.
    Register(provider string, factory Factory) error
    
    // Resolve creates a model instance.
    // modelID format: "provider/model-name" (e.g., "openai/gpt-4")
    Resolve(ctx context.Context, modelID string, opts ...Option) (LLM, error)
    
    // Providers returns registered provider names.
    Providers() []string
}

// Factory creates model instances for a provider.
type Factory func(ctx context.Context, modelName string, opts map[string]any) (LLM, error)

// DefaultRegistry is the global model registry.
var DefaultRegistry Registry = NewRegistry()
```

---

## 6. Message Types

### 6.1 Message Structure

```go
package message

import "time"

// Role indicates the author of a message.
type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleSystem    Role = "system"
    RoleTool      Role = "tool"
)

// Message represents a single message in a conversation.
type Message struct {
    // Role indicates who authored this message.
    Role Role
    
    // Content contains the message content.
    Content []Content
    
    // Name is an optional author identifier for multi-agent scenarios.
    Name string
    
    // Metadata holds additional message data.
    Metadata Metadata
    
    // Timestamps
    CreatedAt time.Time
}

// Builder provides fluent message construction.
type Builder struct {
    msg Message
}

func New(role Role) *Builder
func (b *Builder) Text(text string) *Builder
func (b *Builder) Image(data []byte, mimeType string) *Builder
func (b *Builder) FunctionCall(id, name string, args any) *Builder
func (b *Builder) FunctionResult(callID, name string, result any) *Builder
func (b *Builder) WithName(name string) *Builder
func (b *Builder) WithMetadata(meta Metadata) *Builder
func (b *Builder) Build() Message

// Convenience constructors
func User(content ...Content) Message
func Assistant(content ...Content) Message
func System(text string) Message
func ToolResult(callID, name string, result any) Message
```

---

## 7. Session Management

### 7.1 Core Session Interface

```go
package session

import (
    "context"
    "iter"
    "time"
)

// Session represents a conversation with state and history.
type Session interface {
    // ID returns the unique session identifier.
    ID() ID
    
    // State provides access to session state.
    State() State
    
    // Messages returns the conversation history.
    Messages() []Message
    
    // AddMessage appends a message to history.
    AddMessage(msg Message) error
    
    // Metadata returns session metadata.
    Metadata() Metadata
    
    // CreatedAt returns when the session was created.
    CreatedAt() time.Time
    
    // UpdatedAt returns when the session was last modified.
    UpdatedAt() time.Time
}
```

### 7.2 State Management

```go
package session

import "iter"

// State provides key-value storage with change tracking.
type State interface {
    StateReader
    StateWriter
    
    // Delta returns changes since last clear.
    Delta() map[string]any
    
    // ClearDelta resets the change tracker.
    ClearDelta()
    
    // HasChanges returns true if there are uncommitted changes.
    HasChanges() bool
}

// StateReader provides read-only state access.
type StateReader interface {
    // Get retrieves a value by key.
    Get(key string) (any, bool)
    
    // GetTyped retrieves and type-asserts a value.
    GetTyped(key string, target any) error
    
    // Has checks if a key exists.
    Has(key string) bool
    
    // All iterates over all key-value pairs.
    All() iter.Seq2[string, any]
    
    // Snapshot returns a copy of all state.
    Snapshot() map[string]any
}

// StateWriter provides state mutation.
type StateWriter interface {
    // Set stores a value.
    Set(key string, value any) error
    
    // Delete removes a key.
    Delete(key string) error
    
    // Merge combines multiple values.
    Merge(values map[string]any) error
}

// Key prefixes for state scoping.
const (
    PrefixApp     = "app:"     // Application-scoped
    PrefixUser    = "user:"    // User-scoped
    PrefixAgent   = "agent:"   // Agent-scoped
    PrefixTemp    = "_temp:"   // Temporary (not persisted)
)

// Helper functions for scoped keys.
func AppKey(key string) string    { return PrefixApp + key }
func UserKey(key string) string   { return PrefixUser + key }
func AgentKey(agent, key string) string { return PrefixAgent + agent + ":" + key }
func TempKey(key string) string   { return PrefixTemp + key }
```

### 7.3 Session Service

```go
package session

import "context"

// Service manages session persistence.
type Service interface {
    // Create creates a new session.
    Create(ctx context.Context, opts ...CreateOption) (Session, error)
    
    // Get retrieves an existing session.
    Get(ctx context.Context, id ID) (Session, error)
    
    // Save persists session changes.
    Save(ctx context.Context, sess Session) error
    
    // Delete removes a session.
    Delete(ctx context.Context, id ID) error
    
    // List returns sessions matching the filter.
    List(ctx context.Context, filter *ListFilter) ([]Session, error)
}

type CreateOption func(*createConfig)

func WithID(id ID) CreateOption
func WithInitialState(state map[string]any) CreateOption
func WithMetadata(meta Metadata) CreateOption

type ListFilter struct {
    UserID    string
    AppID     string
    Limit     int
    Offset    int
    OrderBy   string
    OrderDesc bool
}
```

---

## 8. Event System

### 8.1 Event Types

```go
package event

import "time"

// Event represents something that happened during agent execution.
// Use type switches to handle specific event types.
type Event interface {
    // EventType returns the type identifier.
    EventType() EventType
    
    // Timestamp returns when the event occurred.
    Timestamp() time.Time
    
    // sealed prevents external implementations.
    sealed()
}

// EventType identifies the kind of event.
type EventType string

const (
    TypeTextChunk      EventType = "text_chunk"
    TypeTextComplete   EventType = "text_complete"
    TypeToolCall       EventType = "tool_call"
    TypeToolResult     EventType = "tool_result"
    TypeStateChange    EventType = "state_change"
    TypeAgentTransfer  EventType = "agent_transfer"
    TypeError          EventType = "error"
    TypeDone           EventType = "done"
)

// Base provides common event fields.
type Base struct {
    Type EventType
    Time time.Time
    Meta Metadata
}

func (b Base) EventType() EventType { return b.Type }
func (b Base) Timestamp() time.Time { return b.Time }
func (Base) sealed()                {}
```

### 8.2 Concrete Event Types

```go
package event

// TextChunk is a streaming text fragment.
type TextChunk struct {
    Base
    Text  string
    Index int // Chunk index for ordering
}

// TextComplete signals the full text is available.
type TextComplete struct {
    Base
    Text         string
    FinishReason FinishReason
}

// ToolCall indicates the model wants to invoke a tool.
type ToolCall struct {
    Base
    ID        string
    Name      string
    Arguments json.RawMessage
}

// ToolResult contains the outcome of a tool invocation.
type ToolResult struct {
    Base
    CallID   string
    Name     string
    Result   json.RawMessage
    Error    string
    Duration time.Duration
}

// StateChange indicates state was modified.
type StateChange struct {
    Base
    Key      string
    OldValue any
    NewValue any
}

// AgentTransfer indicates control is passing to another agent.
type AgentTransfer struct {
    Base
    FromAgent string
    ToAgent   string
    Reason    string
}

// Error indicates something went wrong.
type Error struct {
    Base
    Err       error
    Retryable bool
}

// Done signals the agent has finished.
type Done struct {
    Base
    Reason string
    Usage  *Usage
}
```

### 8.3 Event Handler

```go
package event

import "context"

// Handler processes events from an agent.
type Handler interface {
    Handle(ctx context.Context, event Event) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, event Event) error

func (f HandlerFunc) Handle(ctx context.Context, event Event) error {
    return f(ctx, event)
}

// Multiplex fans out events to multiple handlers.
func Multiplex(handlers ...Handler) Handler
```

---

## 9. Hook System

### 9.1 Hook Registry (Type-Safe Event System)

```go
package hook

import "context"

// Registry manages typed event callbacks.
type Registry interface {
    // Register adds a callback for an event type.
    Register(eventType EventType, callback Callback) Registration
    
    // RegisterProvider adds all hooks from a provider.
    RegisterProvider(provider Provider) []Registration
    
    // Invoke calls all callbacks for an event.
    Invoke(ctx context.Context, event HookEvent) error
}

// Registration allows unregistering a callback.
type Registration interface {
    Unregister()
}

// Callback handles a hook event.
type Callback func(ctx context.Context, event HookEvent) error

// Provider registers multiple hooks.
type Provider interface {
    RegisterHooks(registry Registry)
}

// HookEvent is the base interface for hook events.
type HookEvent interface {
    HookEventType() EventType
}

// NewRegistry creates a new hook registry.
func NewRegistry() Registry
```

### 9.2 Lifecycle Hook Events

```go
package hook

// BeforeInvoke fires before agent processing begins.
type BeforeInvoke struct {
    Agent    Agent
    Input    *Input
    Session  Session
    Messages []Message // Mutable - modify to inject context
}

func (BeforeInvoke) HookEventType() EventType { return "before_invoke" }

// AfterInvoke fires after agent processing completes.
type AfterInvoke struct {
    Agent    Agent
    Input    *Input
    Session  Session
    Result   any
    Error    error
    Duration time.Duration
}

func (AfterInvoke) HookEventType() EventType { return "after_invoke" }

// BeforeModelCall fires before each model request.
type BeforeModelCall struct {
    Agent   Agent
    Request *model.Request // Mutable
}

func (BeforeModelCall) HookEventType() EventType { return "before_model" }

// AfterModelCall fires after each model response.
type AfterModelCall struct {
    Agent    Agent
    Request  *model.Request
    Response *model.Response
    Error    error
}

func (AfterModelCall) HookEventType() EventType { return "after_model" }

// BeforeToolCall fires before tool execution.
type BeforeToolCall struct {
    Agent     Agent
    Tool      Tool
    Arguments Arguments // Mutable
}

func (BeforeToolCall) HookEventType() EventType { return "before_tool" }

// AfterToolCall fires after tool execution.
type AfterToolCall struct {
    Agent     Agent
    Tool      Tool
    Arguments Arguments
    Result    Result
    Error     error
    Duration  time.Duration
}

func (AfterToolCall) HookEventType() EventType { return "after_tool" }

// MessageAdded fires when a message is added to history.
type MessageAdded struct {
    Agent   Agent
    Session Session
    Message Message
}

func (MessageAdded) HookEventType() EventType { return "message_added" }
```

---

## 10. Middleware

### 10.1 Middleware Interface

```go
package middleware

import "context"

// Middleware wraps agent execution with additional behavior.
type Middleware interface {
    // Wrap returns a wrapped runner.
    Wrap(next Runner) Runner
}

// Runner executes agent logic.
type Runner interface {
    Run(ctx context.Context, input *Input) iter.Seq2[Event, error]
}

// Func adapts a function to Middleware.
type Func func(next Runner) Runner

func (f Func) Wrap(next Runner) Runner { return f(next) }

// Chain combines multiple middleware in order.
// First middleware is outermost (executed first on request, last on response).
func Chain(middleware ...Middleware) Middleware
```

### 10.2 Common Middleware

```go
package middleware

import (
    "context"
    "time"
)

// Logging logs requests and responses.
func Logging(logger Logger) Middleware

// Metrics collects execution metrics.
func Metrics(collector MetricsCollector) Middleware

// Timeout enforces a maximum execution time.
func Timeout(d time.Duration) Middleware

// Retry automatically retries on transient errors.
func Retry(strategy RetryStrategy) Middleware

// Recovery catches panics and converts to errors.
func Recovery() Middleware

// RateLimit enforces request rate limits.
func RateLimit(limiter RateLimiter) Middleware

// Tracing adds distributed tracing spans.
func Tracing(tracer Tracer) Middleware

// Caching caches responses based on input.
func Caching(cache Cache) Middleware

// Validation validates input before execution.
func Validation(validator Validator) Middleware

// ContextProvider injects additional context.
func ContextProvider(provider ContextProvider) Middleware

// Logger interface for logging middleware.
type Logger interface {
    Log(ctx context.Context, level Level, msg string, fields ...Field)
}

// MetricsCollector interface for metrics middleware.
type MetricsCollector interface {
    Record(ctx context.Context, name string, value float64, tags ...Tag)
}

// RetryStrategy determines retry behavior.
type RetryStrategy interface {
    ShouldRetry(err error, attempt int) bool
    Delay(attempt int) time.Duration
}
```

---

## 11. Multi-Agent Orchestration

### 11.1 Orchestrator Interface

```go
package orchestrator

import (
    "context"
    "iter"
)

// Orchestrator coordinates multiple agents.
type Orchestrator interface {
    Agent // Orchestrators are also agents
    
    // Agents returns the managed agents.
    Agents() []Agent
}

// Router selects which agent handles a request.
type Router interface {
    // Route returns the agent to handle the input.
    Route(ctx context.Context, input *Input, agents []Agent) (Agent, error)
}

// RouterFunc adapts a function to Router.
type RouterFunc func(ctx context.Context, input *Input, agents []Agent) (Agent, error)

func (f RouterFunc) Route(ctx context.Context, input *Input, agents []Agent) (Agent, error) {
    return f(ctx, input, agents)
}
```

### 11.2 Built-in Orchestration Patterns

```go
package orchestrator

// Sequential runs agents one after another.
type Sequential struct {
    agents []Agent
}

func NewSequential(agents ...Agent) *Sequential

// Parallel runs agents concurrently and merges results.
type Parallel struct {
    agents []Agent
    merger Merger
}

func NewParallel(agents []Agent, merger Merger) *Parallel

// Merger combines results from parallel agents.
type Merger interface {
    Merge(results []AgentResult) (AgentResult, error)
}

// Loop runs agents repeatedly until a condition is met.
type Loop struct {
    agents    []Agent
    condition StopCondition
    maxIter   int
}

func NewLoop(agents []Agent, condition StopCondition, maxIter int) *Loop

// StopCondition determines when to stop looping.
type StopCondition interface {
    ShouldStop(iteration int, lastResult AgentResult) bool
}

// Router dynamically selects an agent based on input.
type RouterOrchestrator struct {
    agents map[string]Agent
    router Router
}

func NewRouter(agents map[string]Agent, router Router) *RouterOrchestrator

// Graph executes agents as a directed acyclic graph.
type Graph struct {
    nodes map[string]Agent
    edges map[string][]Edge
    entry string
}

func NewGraph() *Graph
func (g *Graph) AddNode(name string, agent Agent) error
func (g *Graph) AddEdge(from, to string, condition EdgeCondition) error
func (g *Graph) SetEntry(name string) error

type Edge struct {
    Target    string
    Condition EdgeCondition
}

type EdgeCondition func(ctx context.Context, result AgentResult) bool
```

---

## 12. Services

### 12.1 Artifact Service

```go
package artifact

import (
    "context"
    "io"
)

// Service manages file/artifact storage.
type Service interface {
    Reader
    Writer
}

// Reader provides read-only artifact access.
type Reader interface {
    // Get retrieves an artifact.
    Get(ctx context.Context, id ID) (*Artifact, error)
    
    // List returns artifacts matching the filter.
    List(ctx context.Context, filter *Filter) ([]*Artifact, error)
    
    // Open returns a reader for artifact content.
    Open(ctx context.Context, id ID) (io.ReadCloser, error)
}

// Writer provides artifact mutation.
type Writer interface {
    // Create stores a new artifact.
    Create(ctx context.Context, artifact *Artifact, content io.Reader) (*Artifact, error)
    
    // Delete removes an artifact.
    Delete(ctx context.Context, id ID) error
}

// Artifact represents a stored file or data blob.
type Artifact struct {
    ID        ID
    Name      string
    MimeType  string
    Size      int64
    Metadata  Metadata
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Filter struct {
    SessionID string
    MimeType  string
    Prefix    string
    Limit     int
}
```

### 12.2 Memory Service (RAG)

```go
package memory

import "context"

// Service manages long-term memory via retrieval-augmented generation.
type Service interface {
    // Store adds content to memory.
    Store(ctx context.Context, entry *Entry) error
    
    // Search finds relevant memories.
    Search(ctx context.Context, query *Query) ([]*SearchResult, error)
    
    // Delete removes memories.
    Delete(ctx context.Context, filter *Filter) error
}

// Entry represents a memory to store.
type Entry struct {
    ID       ID
    Content  string
    Metadata Metadata
    Tags     []string
}

// Query specifies search parameters.
type Query struct {
    Text      string
    Filters   map[string]any
    Limit     int
    Threshold float64 // Minimum similarity score
}

// SearchResult is a matching memory.
type SearchResult struct {
    Entry *Entry
    Score float64
}

type Filter struct {
    IDs   []ID
    Tags  []string
    Since time.Time
}
```

---

## 13. Error Handling

### 13.1 Error Types

```go
package errors

import (
    "errors"
    "fmt"
)

// Sentinel errors for type checking with errors.Is().
var (
    ErrNotFound       = errors.New("not found")
    ErrAlreadyExists  = errors.New("already exists")
    ErrInvalidInput   = errors.New("invalid input")
    ErrUnauthorized   = errors.New("unauthorized")
    ErrRateLimited    = errors.New("rate limited")
    ErrTimeout        = errors.New("timeout")
    ErrCanceled       = errors.New("canceled")
    ErrModelError     = errors.New("model error")
    ErrToolError      = errors.New("tool error")
    ErrContextOverflow = errors.New("context window overflow")
    ErrContentFiltered = errors.New("content filtered")
)

// Kind categorizes errors.
type Kind int

const (
    KindUnknown Kind = iota
    KindNotFound
    KindInvalidInput
    KindUnauthorized
    KindRateLimited
    KindTimeout
    KindInternal
    KindExternal
)

// Error is a rich error type with context.
type Error struct {
    Kind    Kind
    Op      string         // Operation that failed
    Err     error          // Underlying error
    Context map[string]any // Additional context
}

func (e *Error) Error() string {
    if e.Op != "" {
        return fmt.Sprintf("%s: %v", e.Op, e.Err)
    }
    return e.Err.Error()
}

func (e *Error) Unwrap() error {
    return e.Err
}

func (e *Error) Is(target error) bool {
    if e.Err == nil {
        return false
    }
    return errors.Is(e.Err, target)
}

// E creates a new Error.
func E(op string, err error, ctx ...map[string]any) *Error {
    e := &Error{Op: op, Err: err}
    if len(ctx) > 0 {
        e.Context = ctx[0]
    }
    return e
}

// AgentError wraps errors with agent context.
type AgentError struct {
    AgentID   string
    AgentName string
    Err       error
}

func (e *AgentError) Error() string {
    return fmt.Sprintf("agent %s (%s): %v", e.AgentName, e.AgentID, e.Err)
}

func (e *AgentError) Unwrap() error { return e.Err }

// ToolError wraps tool execution errors.
type ToolError struct {
    ToolName  string
    CallID    string
    Arguments json.RawMessage
    Err       error
}

func (e *ToolError) Error() string {
    return fmt.Sprintf("tool %s (call %s): %v", e.ToolName, e.CallID, e.Err)
}

func (e *ToolError) Unwrap() error { return e.Err }

// ModelError wraps model/LLM errors.
type ModelError struct {
    Provider string
    Model    string
    Code     string
    Message  string
}

func (e *ModelError) Error() string {
    return fmt.Sprintf("model %s/%s [%s]: %s", e.Provider, e.Model, e.Code, e.Message)
}
```

### 13.2 Error Helpers

```go
package errors

// Is reports whether err matches target.
func Is(err, target error) bool {
    return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target.
func As(err error, target any) bool {
    return errors.As(err, target)
}

// Wrap wraps an error with an operation name.
func Wrap(op string, err error) error {
    if err == nil {
        return nil
    }
    return E(op, err)
}

// IsRetryable returns true if the error is transient.
func IsRetryable(err error) bool {
    if errors.Is(err, ErrRateLimited) || errors.Is(err, ErrTimeout) {
        return true
    }
    var e *Error
    if errors.As(err, &e) {
        return e.Kind == KindRateLimited || e.Kind == KindTimeout
    }
    return false
}
```

---

## 14. Package Structure

```
github.com/your-org/agentkit/
├── agent/                    # Core agent interfaces
│   ├── agent.go             # Agent, Runner, Identifiable
│   ├── config.go            # Config, Factory
│   ├── input.go             # Input, InvokeConfig
│   └── traits.go            # Hierarchical, Toolable, etc.
│
├── tool/                     # Tool interfaces
│   ├── tool.go              # Tool, Executor, ExecutableTool
│   ├── function.go          # NewFunction, FunctionConfig
│   ├── provider.go          # Provider, Registry
│   ├── arguments.go         # Arguments, Result
│   └── traits.go            # LongRunning, Confirmable, etc.
│
├── model/                    # LLM interfaces
│   ├── model.go             # LLM interface
│   ├── request.go           # Request, Parameters
│   ├── response.go          # Response, Usage
│   ├── registry.go          # Registry, Factory
│   └── traits.go            # Streamable, ToolUser, etc.
│
├── message/                  # Message types
│   ├── message.go           # Message, Role
│   ├── content.go           # Content types
│   └── builder.go           # Builder pattern
│
├── session/                  # Session management
│   ├── session.go           # Session interface
│   ├── state.go             # State, StateReader, StateWriter
│   ├── service.go           # Service interface
│   └── memory/              # In-memory implementation
│
├── event/                    # Event system
│   ├── event.go             # Event interface
│   ├── types.go             # Concrete event types
│   └── handler.go           # Handler interface
│
├── hook/                     # Hook system
│   ├── registry.go          # Registry interface
│   ├── events.go            # Hook event types
│   └── provider.go          # Provider interface
│
├── middleware/               # Middleware
│   ├── middleware.go        # Middleware interface
│   ├── chain.go             # Chain helper
│   ├── logging.go           # Logging middleware
│   ├── metrics.go           # Metrics middleware
│   ├── retry.go             # Retry middleware
│   └── tracing.go           # Tracing middleware
│
├── orchestrator/             # Multi-agent
│   ├── orchestrator.go      # Orchestrator interface
│   ├── sequential.go        # Sequential execution
│   ├── parallel.go          # Parallel execution
│   ├── router.go            # Dynamic routing
│   ├── loop.go              # Iterative execution
│   └── graph.go             # DAG execution
│
├── artifact/                 # Artifact storage
│   ├── service.go           # Service interface
│   └── memory/              # In-memory implementation
│
├── memory/                   # Long-term memory (RAG)
│   ├── service.go           # Service interface
│   └── memory/              # In-memory implementation
│
├── schema/                   # JSON Schema
│   ├── schema.go            # Schema type
│   ├── reflect.go           # FromType helper
│   └── validate.go          # Validation
│
├── errors/                   # Error types
│   ├── errors.go            # Error, Kind
│   ├── sentinel.go          # Sentinel errors
│   └── helpers.go           # Is, As, Wrap
│
├── internal/                 # Internal packages
│   ├── typeutil/            # Type conversion utilities
│   └── jsonutil/            # JSON utilities
│
└── providers/                # Provider implementations (separate modules)
    ├── openai/              # OpenAI provider
    ├── anthropic/           # Anthropic provider
    ├── bedrock/             # AWS Bedrock provider
    └── gemini/              # Google Gemini provider
```

---

## Appendix A: Interface Summary

| Interface | Package | Methods | Purpose |
|-----------|---------|---------|---------|
| `Agent` | agent | 4 | Core agent identity + execution |
| `Runner` | agent | 1 | Execution capability |
| `Identifiable` | agent | 3 | Identity information |
| `Tool` | tool | 3 | Tool metadata |
| `Executor` | tool | 1 | Tool execution |
| `Provider` | tool | 2 | Dynamic tool loading |
| `Registry` | tool | 4 | Tool management |
| `LLM` | model | 1 | Model generation |
| `Session` | session | 6 | Conversation state |
| `State` | session | 5+ | Key-value storage |
| `Service` | session | 5 | Session persistence |
| `Event` | event | 3 | Event interface |
| `Handler` | event | 1 | Event processing |
| `Registry` | hook | 3 | Hook management |
| `Middleware` | middleware | 1 | Execution wrapping |
| `Orchestrator` | orchestrator | 1+ | Multi-agent coordination |

---

## Appendix B: Design Decisions

### Why Small Interfaces?

1. **Composability**: Small interfaces combine easily
2. **Testability**: Easy to mock single-method interfaces
3. **Flexibility**: Implementations can satisfy multiple interfaces
4. **Go Idiom**: Follows `io.Reader`, `io.Writer` patterns

### Why No Generics in Core Interfaces?

1. **Simplicity**: Easier to understand and use
2. **Compatibility**: Works with existing reflection-based tools
3. **Flexibility**: Runtime type handling via `any` where needed
4. **Generics Where Useful**: Used in `tool.NewFunction` for type safety

### Why Vendor-Agnostic?

1. **Portability**: Switch providers without code changes
2. **Testing**: Mock implementations without provider SDKs
3. **Future-Proof**: New providers don't require interface changes
4. **Clean Dependencies**: Core has no external dependencies

### Why Events + Hooks + Middleware?

- **Events**: Read-only notification stream (consumers)
- **Hooks**: Typed interception points (modifiers)
- **Middleware**: Request/response wrapping (cross-cutting)

Each serves a distinct purpose with minimal overlap.

---

## Appendix C: Migration from Framework-Specific Code

| Framework Concept | Universal Equivalent |
|-------------------|---------------------|
| ADK `BaseAgent` | `agent.Agent` interface |
| ADK `InvocationContext` | `context.Context` + `*Input` |
| ADK `BaseTool` | `tool.Tool` + `tool.Executor` |
| Strands `Agent` | `agent.Agent` interface |
| Strands `HookProvider` | `hook.Provider` interface |
| Strands `ConversationManager` | `middleware.Middleware` |
| Microsoft `AIAgent` | `agent.Agent` interface |
| Microsoft `AIContextProvider` | `middleware.ContextProvider` |
| Microsoft `AgentSession` | `session.Session` interface |

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2024-02-04 | Initial specification |
| 2.0.0 | 2024-02-04 | Complete redesign for universality and Go best practices |

