# Agent Abstraction Layer

This package provides framework-agnostic interfaces for building AI agents in the SchoolWithAI application.

## Purpose

The agent abstraction layer decouples the application from specific agent framework implementations, enabling:

1. **Framework Flexibility**: Switch between custom agentsdk, ADK-Go, or Blades without changing application code
2. **Clean Architecture**: Handlers and services depend only on interfaces, not implementations
3. **Testability**: Mock agent behavior without running full LLM workflows
4. **Future-Proofing**: Add new agent frameworks by implementing adapters

## Core Interfaces

### `Agent`
Represents an AI agent that processes user requests.
```go
type Agent interface {
    Name() string
    Description() string
    Run(context.Context, *Invocation) iter.Seq2[Event, error]
    Tools() []Tool
    SubAgents() []Agent
}
```

### `Runner`
Orchestrates agent execution across sessions (main entry point for application code).
```go
type Runner interface {
    Run(context.Context, *RunRequest) iter.Seq2[Event, error]
    GetAgent(name string) (Agent, error)
    ListAgents() []string
}
```

### `Tool`
A function that agents can invoke to perform actions.
```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Execute(context.Context, *ToolContext) (*ToolResult, error)
}
```

### `Model`
An LLM client abstraction (OpenAI, Gemini, Ollama, etc.).
```go
type Model interface {
    Generate(context.Context, *GenerateRequest) iter.Seq2[*GenerateResponse, error]
    Provider() string
}
```

### `Session`
Manages conversation state and history.
```go
type Session interface {
    ID() string
    UserID() string
    State() State
    History() []*Message
    Append(context.Context, *Message) error
}
```

## Design Principles

1. **Streaming-First**: All agent execution returns `iter.Seq2` for real-time event streaming
2. **Context-Rich**: Provide enough context for tools to make intelligent decisions
3. **Error Transparency**: Events can represent errors without breaking the stream
4. **Metadata-Friendly**: All types include metadata fields for extensibility

## Compatibility

The interfaces are designed to align with:
- **Custom Implementation**: `foundation/agentsdk/*`
- **ADK-Go**: Google's Agent Development Kit (`github.com/google/adk-go`)
- **Blades**: Go-Kratos agent framework (`github.com/go-kratos/blades`)

## Usage

Application code should depend only on the `foundation/agent` interfaces:

```go
import "school-with-ai/foundation/agent"

func HandleChatRequest(runner agent.Runner, req *agent.RunRequest) {
    for event, err := range runner.Run(ctx, req) {
        if err != nil {
            // Handle error
            continue
        }
        
        switch event.Type() {
        case agent.EventTypeMessage:
            // Send message to client
        case agent.EventTypeToolCall:
            // Show tool execution status
        }
    }
}
```

Concrete implementations are in `foundation/agent/adapters/`:
- `custom/` - Wraps `foundation/agentsdk`
- `adkgo/` - Wraps Google ADK-Go
- `blades/` - Wraps Go-Kratos Blades

## Next Steps

1. Implement `foundation/agent/adapters/custom/` (Phase 1)
2. Refactor services to use `agent.Runner` instead of `agentorchestrator.Service` (Phase 2)
3. Update handlers to consume `agent.Event` streams (Phase 2)
4. Implement ADK-Go and Blades adapters (Phase 4)
