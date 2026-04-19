package agent

// EventActions carries runtime directives emitted alongside an event.
// These instruct the framework how to proceed after processing the event.
//
// Mirrors ADK's session.EventActions — an agent or tool can bundle routing
// and state-mutation signals with the event that triggered them, keeping
// cause and effect atomic rather than split across separate events.
type EventActions struct {
	// StateDelta contains key-value pairs to apply atomically to session state
	// when this event is committed. Keys follow the state-scope prefix
	// conventions defined in state.go (e.g. "app:", "user:", "temp:").
	StateDelta map[string]any

	// TransferToAgent requests that the framework hand control to the named
	// agent after this event. The named agent continues the current invocation.
	// Mirrors ADK's EventActions.TransferToAgent.
	TransferToAgent string

	// Escalate requests that control return to the parent agent.
	// Mirrors ADK's EventActions.Escalate.
	Escalate bool

	// EndInvocation signals that the invocation should stop after this event,
	// even if the normal termination condition has not been reached.
	// Mirrors ADK's InvocationContext.EndInvocation().
	EndInvocation bool
}
