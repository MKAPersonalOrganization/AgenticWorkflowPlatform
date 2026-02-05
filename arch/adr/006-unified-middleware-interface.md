# ADR 006: Unified Middleware Architecture

## Status
Accepted

## Context

After studying Microsoft Agent Framework, Blades, Chi, and Echo middleware designs, we identified:

1. **Three operation types** need interception: Agent runs, LLM calls, Tool calls
2. **Echo's pattern** is cleanest: single `Context` struct, single `Handler`, single `Middleware`
3. **Avoid two contexts**: Embed `context.Context` in our `Context`

## Operation Semantics

### Agent Operation

An **Agent operation** represents the entire execution of an agent from receiving user input to producing a final response. This is the outermost operation that may contain multiple Chat and Function operations as the agent reasons through a task.

**Triggered when**: A user or system invokes an agent with a message or task.

**Contains**: The agent instance, input messages, session for state persistence, and streaming configuration.

**Typical middleware concerns**: Retry on failure, overall timeout, distributed tracing, circuit breaker, audit logging.

### Chat Operation

A **Chat operation** represents a single request-response cycle with a Large Language Model (LLM). During one agent run, there may be multiple chat operations as the agent iterates through reasoning steps.

**Triggered when**: The agent loop sends messages to the LLM for completion.

**Contains**: The model to call, the conversation messages, and model configuration options (temperature, max tokens, etc.).

**Typical middleware concerns**: Token counting and budgeting, rate limiting API calls, injecting system prompts, caching identical requests, logging prompts and completions.

### Function Operation

A **Function operation** represents the execution of a single tool or function call requested by the LLM. During one chat operation, the LLM may request zero or more function calls.

**Triggered when**: The LLM response includes a tool call that must be executed.

**Contains**: The tool to execute and the arguments extracted from the LLM's request.

**Typical middleware concerns**: Argument validation, result caching, mocking in tests, per-tool timeouts, permission checks.

## Decision

Adopt a **unified middleware architecture** following Echo's pattern:
- One `Context` struct with grouped sub-fields
- One `Handler` interface with single argument
- One `Middleware` type

### Core Types

```go
package middleware

import "context"

// Handler processes operations.
type Handler interface {
    Handle(*Context) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(*Context) error

func (f HandlerFunc) Handle(c *Context) error { return f(c) }

// Middleware wraps a Handler.
type Middleware func(Handler) Handler

// Chain combines middleware (first is outermost).
func Chain(mws ...Middleware) Middleware {
    return func(final Handler) Handler {
        for i := len(mws) - 1; i >= 0; i-- {
            final = mws[i](final)
        }
        return final
    }
}
```

### Context with Grouped Fields

```go
// Context carries all operation data.
// Embeds context.Context for cancellation/timeout.
type Context struct {
    context.Context
    
    // Operation type (agent, chat, function)
    Kind OperationKind
    
    // Exactly one of these is non-nil
    Agent    *AgentOp
    Chat     *ChatOp
    Function *FunctionOp
    
    // Shared across middleware
    Metadata map[string]any
    
    // Result (populated after handler)
    Result any
}

// OperationKind identifies the operation type.
type OperationKind int

const (
    OperationAgent OperationKind = iota
    OperationChat
    OperationFunction
)

// Helpers
func (c *Context) IsAgent() bool    { return c.Kind == OperationAgent }
func (c *Context) IsChat() bool     { return c.Kind == OperationChat }
func (c *Context) IsFunction() bool { return c.Kind == OperationFunction }
```

### Operation Structs

```go
// AgentOp groups agent-level data.
type AgentOp struct {
    Agent     Agent
    Messages  []Message
    Session   Session
    Streaming bool
}

// ChatOp groups LLM call data.
type ChatOp struct {
    Model    Model
    Messages []Message
    Options  *ChatOptions
}

// FunctionOp groups tool call data.
type FunctionOp struct {
    Tool      Tool
    Arguments map[string]any
}
```

### Example Middleware

```go
// Retry - applies to agent operations only
func Retry(attempts int) Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(c *Context) error {
            if c.Agent == nil {
                return next.Handle(c)  // Pass through non-agent ops
            }
            var err error
            for i := 0; i < attempts; i++ {
                if err = next.Handle(c); err == nil {
                    return nil
                }
            }
            return err
        })
    }
}

// TokenCounter - applies to chat operations only
func TokenCounter(counter Counter) Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(c *Context) error {
            if c.Chat == nil {
                return next.Handle(c)  // Pass through non-chat ops
            }
            counter.CountInput(c.Chat.Messages)
            err := next.Handle(c)
            counter.CountOutput(c.Result)
            return err
        })
    }
}

// FunctionCache - applies to function operations only
func FunctionCache(cache Cache) Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(c *Context) error {
            if c.Function == nil {
                return next.Handle(c)  // Pass through non-function ops
            }
            key := cacheKey(c.Function.Tool, c.Function.Arguments)
            if cached, ok := cache.Get(key); ok {
                c.Result = cached
                return nil
            }
            if err := next.Handle(c); err != nil {
                return err
            }
            cache.Set(key, c.Result)
            return nil
        })
    }
}
```

## Comparison

| Framework | Handler | Context | Middleware |
|-----------|---------|---------|------------|
| net/http | `ServeHTTP(w, r)` | `*http.Request` | `func(http.Handler) http.Handler` |
| Chi | `ServeHTTP(w, r)` | `*http.Request` + chi context | `func(http.Handler) http.Handler` |
| Echo | `func(c) error` | `echo.Context` (struct) | `func(next) handler` |
| **Ours** | `Handle(*Context) error` | `*Context` (struct) | `func(Handler) Handler` |

## Consequences

### Positive
- **One type**: Single `Middleware` for all operations
- **Echo-style**: Proven pattern from popular framework
- **Grouped data**: Related fields in sub-structs
- **Composable**: Single `Chain()` function
- **No two contexts**: Embedded `context.Context`

### Negative
- **Nil checks**: Middleware must check operation type
- **Larger Context**: All fields present, most nil

## Related Decisions
- ADR 001: Middleware operates within adapter layer
- ADR 002: Middleware does NOT emit lifecycle events
