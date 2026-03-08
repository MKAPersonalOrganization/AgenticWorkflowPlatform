# Agent Abstraction Layer

This package provides framework-agnostic interfaces for building AI agents in the SchoolWithAI application.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
│  (Handlers, Services, Orchestration)                        │
│                         │                                   │
│                         ▼                                   │
│              ┌─────────────────────┐                        │
│              │   agent.Runner      │  ← Main Entry Point    │
│              │   agent.Agent       │                        │
│              │   agent.Event       │                        │
│              └─────────────────────┘                        │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                     Adapter Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   custom/   │  │   adkgo/    │  │   blades/   │          │
│  │  (agentsdk) │  │  (ADK-Go)   │  │  (Blades)   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   Framework Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │  agentsdk   │  │  Google     │  │  Go-Kratos  │          │
│  │   (custom)  │  │  ADK-Go     │  │  Blades     │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

## Design Principles

Following ADRs 001-004:

1. **Agent Interface as Sole Access Point** (ADR 001)
   - Application code depends only on `foundation/agent` interfaces
   - No direct imports from framework packages

2. **Service Layer Orchestration** (ADR 002)
   - Run lifecycle events (RunStarted, RunFinished) are emitted by services, not adapters
   - Adapters remain stateless event translators

3. **Event Classification** (ADR 003)
   - Ephemeral events: Streaming deltas, progress, lifecycle signals
   - Persistent events: Messages, tool results, state changes

4. **Pointer-based Events** (ADR 004)
   - All event types use pointer receivers
   - Consistent type handling throughout the system

## Core Interfaces

### Agent

```go
type Agent interface {
    Name() string
    Description() string
    Run(context.Context, *Invocation) iter.Seq2[Event, error]
    Tools() []Tool
    SubAgents() []Agent
}
```

### Runner

```go
type Runner interface {
    Run(context.Context, *RunRequest) iter.Seq2[Event, error]
    RunSync(context.Context, *RunRequest) (*RunResult, error)
    GetAgent(name string) (Agent, error)
    ListAgents() []string
}
```

### Event

```go
type Event interface {
    Type() EventType
    Timestamp() time.Time
    Data() any
}

// Marker interfaces for event classification
type EphemeralEvent interface { Event; ephemeral() }
type PersistentEvent interface { Event; persistent() }
```

### Tool

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Execute(context.Context, *ToolContext) (*ToolResult, error)
}
```

### Session

```go
type Session interface {
    ID() string
    UserID() string
    AppName() string
    State() State
    History() []*Message
    Append(context.Context, *Message) error
    LastUpdateTime() int64
}
```

## Usage

### Application Code

```go
import "school-with-ai/foundation/agent"

func HandleChatRequest(runner agent.Runner, req *agent.RunRequest) {
    for event, err := range runner.Run(ctx, req) {
        if err != nil {
            log.Error(err)
            continue
        }
        
        switch e := event.(type) {
        case *agent.MessageEvent:
            sendToClient(e.Message)
        case *agent.MessageDeltaEvent:
            streamToClient(e.Delta)
        case *agent.ToolCallEvent:
            showToolStatus(e.ToolCall)
        case *agent.UsageEvent:
            trackUsage(e.Usage)
        }
    }
}
```

### Creating an Agent

```go
import "school-with-ai/foundation/agent"

config := agent.Config{
    Name:        "math-tutor",
    Description: "A helpful math tutor",
    SystemPrompt: "You are a patient math tutor...",
    Tools:       []agent.Tool{calculator, grapher},

    // Declarative requirements: evaluated before each LLM turn.
    // ToolProvider requirements auto-contribute tools.
    // PromptProvider requirements auto-contribute to the system prompt.
    Requirements: []agent.Requirement{
        agent.NewConditionalRequirement("think", agent.ForceAtStep(1)),    // force think first
        agent.Require("limit-search", func(ctx context.Context, inv *agent.Invocation) ([]agent.Rule, error) {
            return []agent.Rule{{TargetTool: "search", Blocked: true, Reason: "max reached"}}, nil
        }),
    },
}

// Use an adapter to create the agent
adapter := custom.NewAgentAdapter(config)
```

## Adapters

### custom/ (agentsdk)
Wraps the legacy `foundation/agentsdk` implementation.

### adkgo/ (ADK-Go)
Wraps Google's Agent Development Kit for Go.

### blades/ (Go-Kratos)
Wraps Go-Kratos Blades framework.

## Event Types

| Event Type | Classification | Description |
|------------|----------------|-------------|
| MessageEvent | Persistent | Complete message |
| MessageDeltaEvent | Ephemeral | Streaming content |
| ToolCallEvent | Ephemeral | Tool execution started |
| ToolResultEvent | Persistent | Tool execution completed |
| UsageEvent | Persistent | Token consumption |
| TitleEvent | Persistent | Thread title generated |
| ToolInteractionEvent | Persistent | Tool needs user input |
| ErrorEvent | Ephemeral | Execution error |
| HandoffEvent | Ephemeral | Agent transfer |
| CompleteEvent | Ephemeral | Invocation finished |
| StateChangeEvent | Persistent | Session state modified |

## See Also

- [ADR 001: Agent Framework Architectural Boundary](../../arch/adr/001-agent-sdk-architectural-boundary.md)
- [ADR 002: Run Lifecycle Events](../../arch/adr/002-run-lifecycle-events-service-orchestration.md)
- [ADR 003: Event Classification](../../arch/adr/003-event-classification-ephemeral-vs-persistent.md)
- [ADR 004: Pointer-based Domain Events](../../arch/adr/004-standardization-of-pointer-based-domain-events.md)
