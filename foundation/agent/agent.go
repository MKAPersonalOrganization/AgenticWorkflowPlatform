package agent

import (
	"context"
	"iter"
)

// ID is a unique identifier for an agent.
type ID = string

// Agent represents an AI agent that can process user requests and produce responses.
// Per ADR 001: This interface is the sole access point for agent interactions.
//
// Agents form a tree: each agent can have SubAgents for delegation/transfer.
// The runner walks the tree to find the appropriate agent for each invocation.
type Agent interface {
	// ID returns the unique identifier for this agent.
	ID() ID

	// Name returns the agent's human-readable name (e.g., "math-instructor").
	Name() string

	// Description returns a human-readable description of the agent's capabilities.
	Description() string

	// Run executes the agent with the given invocation.
	// Returns an iterator that yields events as the agent processes the request.
	Run(ctx context.Context, inv *Invocation) iter.Seq2[Event, error]

	// SubAgents returns child agents available for delegation.
	// Returns nil for leaf agents.
	SubAgents() []Agent
}

// Configurable is an optional interface for agents that expose their configuration.
// This allows the adapter layer to extract agent configuration without importing
// concrete agent types (e.g., CustomAgent).
type Configurable interface {
	Agent
	Config() Config
}

// HookProvider is an optional interface for agents that expose lifecycle hooks.
// Adapters use this to wire Before/After interception at each execution level.
type HookProvider interface {
	BeforeToolHooks() []BeforeToolHook
	AfterToolHooks() []AfterToolHook
	BeforeModelHooks() []BeforeModelHook
	AfterModelHooks() []AfterModelHook
	BeforeRunHooks() []BeforeRunHook
	AfterRunHooks() []AfterRunHook
}

// MiddlewareProvider is an optional interface for agents that expose middleware chains.
type MiddlewareProvider interface {
	AgentMiddlewares() []Middleware
	ChatMiddlewares() []Middleware
	FunctionMiddlewares() []Middleware
}

// AgentMetadata represents metadata for an agent exposed via the API.
type AgentMetadata struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// AgentListResponse represents the response containing a list of agents.
type AgentListResponse struct {
	Agents []AgentMetadata `json:"agents"`
}

// AgentProvider provides discovery and lookup for available agents.
// It replaces the previous AgentRegistry interface with a read-only contract,
// promoting direct instantiation and dependency injection over global state.
type AgentProvider interface {
	// Get retrieves an agent by name.
	Get(name string) (Agent, error)

	// List returns all available agents.
	List() []Agent
}

// Invocation contains all the information needed to execute an agent.
// It captures the full execution context: user input, history, tools,
// and configuration for a single agent run.
type Invocation struct {
	// ID is a unique identifier for this invocation (for resumability).
	ID string

	// SessionID identifies the conversation session.
	SessionID string

	// UserID identifies the user making the request.
	UserID string

	// Message is the user's input message.
	Message *Message

	// History contains previous messages in the conversation.
	History []*Message

	// Instruction is an optional system instruction override.
	Instruction *Message

	// Tools are additional tools available for this invocation.
	// These are merged with the agent's default tools.
	Tools []Tool

	// Context is custom context data (e.g., user context, memory).
	Context map[string]any

	// Config contains agent execution configuration.
	Config *RunConfig

	// Resumable indicates if the invocation can be resumed after interruption.
	Resumable bool

	// Streamable indicates if the response should be streamed.
	Streamable bool

	// Branch isolates conversation history for concurrent sub-agents.
	// Format: "agent_1.agent_2.agent_3" (parent chain).
	Branch string

	// Effects accumulates side-effects for the current step.
	// State deltas, artifact changes, transfer requests, etc.
	Effects *StepEffects

	// EndInvocation is set to true to stop the current invocation.
	// This halts any further agent calls in the invocation.
	EndInvocation bool

	// Step tracks the current step number within an agent call.
	// Used by Requirements to make step-dependent decisions.
	Step int
}

// Clone creates a deep copy of the Invocation.
func (inv *Invocation) Clone() *Invocation {
	if inv == nil {
		return nil
	}
	clone := *inv
	if inv.Message != nil {
		clone.Message = inv.Message.Clone()
	}
	if inv.Instruction != nil {
		clone.Instruction = inv.Instruction.Clone()
	}
	if inv.History != nil {
		clone.History = make([]*Message, len(inv.History))
		for i, m := range inv.History {
			clone.History[i] = m.Clone()
		}
	}
	if inv.Tools != nil {
		clone.Tools = make([]Tool, len(inv.Tools))
		copy(clone.Tools, inv.Tools)
	}
	if inv.Context != nil {
		clone.Context = make(map[string]any, len(inv.Context))
		for k, v := range inv.Context {
			clone.Context[k] = v
		}
	}
	return &clone
}

// RunConfig controls agent execution behavior.
type RunConfig struct {
	// Temperature controls randomness (0.0-2.0).
	Temperature float64

	// MaxTokens limits output length.
	MaxTokens int

	// MaxIterations limits tool calling loops.
	MaxIterations int

	// StreamingMode controls how events are streamed.
	StreamingMode StreamingMode

	// ModelID optionally overrides the default model.
	ModelID string

	// StopSequences are strings that stop generation when encountered.
	StopSequences []string
}

// DefaultRunConfig returns sensible defaults for agent execution.
func DefaultRunConfig() *RunConfig {
	return &RunConfig{
		Temperature:   0.7,
		MaxTokens:     4096,
		MaxIterations: 10,
		StreamingMode: StreamingModeSSE,
	}
}
