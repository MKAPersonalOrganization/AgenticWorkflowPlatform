package thread

import (
	es "aicompass/foundation/eventsourcing"
)

// RegisterThreadEvents registers all thread/chat domain events with the given
// deserializer registry. This includes legacy aliases for backward compatibility
// with events stored under old type names.
func RegisterThreadEvents(r *es.DeserializerRegistry) {
	r.Register("ThreadCreated", func() es.Event { return &ThreadCreated{} })
	r.Register("UserMessageAdded", func() es.Event { return &UserMessageAdded{} })
	r.Register("AssistantMessageAdded", func() es.Event { return &AssistantMessageAdded{} })
	r.Register("ThreadTitleUpdated", func() es.Event { return &ThreadTitleUpdated{} })
	r.Register("ThreadDeleted", func() es.Event { return &ThreadDeleted{} })
	r.Register("RunStarted", func() es.Event { return &RunStarted{} })
	r.Register("RunFinished", func() es.Event { return &RunFinished{} })
	r.Register("RunError", func() es.Event { return &RunError{} })
	r.Register("AssistantMessageStarted", func() es.Event { return &AssistantMessageStarted{} })
	r.Register("AssistantContentStreamed", func() es.Event { return &AssistantContentStreamed{} })
	r.Register("AssistantMessageCompleted", func() es.Event { return &AssistantMessageCompleted{} })
	r.Register("TokenUsageReported", func() es.Event { return &TokenUsageReported{} })
	r.Register("ToolStart", func() es.Event { return &ToolStart{} })
	r.Register("ToolContentStreamed", func() es.Event { return &ToolContentStreamed{} })
	r.Register("AgentTransferStarted", func() es.Event { return &AgentTransferStarted{} })
	r.Register("ToolResultAdded", func() es.Event { return &ToolResultAdded{} })
	r.Register("ToolNeedsInput", func() es.Event { return &ToolNeedsInput{} })
	r.Register("ToolNeedsAuth", func() es.Event { return &ToolNeedsAuth{} })
	r.Register("InputProvided", func() es.Event { return &InputProvided{} })
	r.Register("AuthGranted", func() es.Event { return &AuthGranted{} })
	r.Register("AuthDenied", func() es.Event { return &AuthDenied{} })
	r.Register("ToolCancelled", func() es.Event { return &ToolCancelled{} })
	r.Register("StateChangeReported", func() es.Event { return &StateChangeReported{} })
}
