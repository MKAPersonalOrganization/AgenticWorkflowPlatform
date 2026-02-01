package core

// RunContext encapsulates the state and parameters for an agent run.
// It is inspired by the InvocationContext in Google ADK and AgentRunOptions in Microsoft Agent Framework.
type RunContext struct {
	ThreadID       string
	MessageID      string
	Thread         AgentThread
	Options        RunOptions
	InitialHistory []Message // Pre-fetched history to avoid redundant I/O
	// User message to process (if this is a new user message submission)
	UserContent string
	UserID      string

	// Execution state for resumption
	Iteration int
	Total     int
	LoopState string // Hex serialized LoopDetector state (optional)
}
