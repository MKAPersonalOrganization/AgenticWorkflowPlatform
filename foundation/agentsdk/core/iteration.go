package core

// IterationContext encapsulates the parameters for an agent iteration.
type IterationContext struct {
	ThreadID  string
	MessageID string
	Thread    AgentThread
	Options   RunOptions
}
