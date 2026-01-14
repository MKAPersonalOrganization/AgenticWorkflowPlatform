package core

// RunContext encapsulates the state and parameters for an agent run.
// It is inspired by the InvocationContext in Google ADK and AgentRunOptions in Microsoft Agent Framework.
type RunContext struct {
	ThreadID  string
	MessageID string
	Thread    AgentThread
	Options   RunOptions
}
