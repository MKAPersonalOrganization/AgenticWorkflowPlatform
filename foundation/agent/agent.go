package agent

// ID is a unique identifier for an agent.
type ID = string

// Agent represents an AI agent definition — its identity and capability tree.
//
// Execution is delegated entirely to the framework adapter (ADK-Go).
// Agents form a tree: each agent can have SubAgents for delegation/transfer.
type Agent interface {
	// ID returns the unique identifier for this agent.
	ID() ID

	// Name returns the agent's human-readable name (e.g., "math-instructor").
	Name() string

	// Description returns a human-readable description of the agent's capabilities.
	Description() string

	// SubAgents returns child agents available for delegation.
	// Returns nil for leaf agents.
	SubAgents() []Agent
}

// Configurable is an optional interface for agents that expose their configuration.
// The adapter layer uses this to extract agent configuration without importing
// concrete agent types.
type Configurable interface {
	Agent
	Config() Config
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
type AgentProvider interface {
	// Get retrieves an agent by name.
	Get(name string) (Agent, error)

	// List returns all available agents.
	List() []Agent
}

// Invocation carries the context for a single agent run.
// It is passed to Requirements and InstructionFunc for per-turn customisation.
//
// History is populated from the thread aggregate and used by Requirements
// (e.g. ConditionalRequirement) to inspect tool-call history.
type Invocation struct {
	// ID is a unique identifier for this invocation.
	ID string

	// SessionID identifies the conversation session.
	SessionID string

	// UserID identifies the user making the request.
	UserID string

	// Message is the user's input message.
	Message *Message

	// History contains previous messages in the conversation.
	// Used by Requirements to evaluate tool-call history.
	History []*Message

	// Context is custom context data (e.g., attachment refs).
	Context map[string]any

	// Config contains agent execution configuration.
	Config *RunConfig
}

// Clone creates a shallow copy of the Invocation.
func (inv *Invocation) Clone() *Invocation {
	if inv == nil {
		return nil
	}
	clone := *inv
	if inv.Message != nil {
		clone.Message = inv.Message.Clone()
	}
	if inv.History != nil {
		clone.History = make([]*Message, len(inv.History))
		for i, m := range inv.History {
			clone.History[i] = m.Clone()
		}
	}
	if inv.Context != nil {
		clone.Context = make(map[string]any, len(inv.Context))
		for k, v := range inv.Context {
			clone.Context[k] = v
		}
	}
	return &clone
}

// RunConfig controls agent execution behaviour.
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
