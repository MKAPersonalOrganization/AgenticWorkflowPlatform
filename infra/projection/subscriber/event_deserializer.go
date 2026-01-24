package subscriber

import (
	"encoding/json"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
)

// deserializeDomainEvent deserializes a domain event from JSON based on its type.
func deserializeDomainEvent(eventType string, data []byte) (core.Event, error) {
	switch eventType {
	case "ThreadCreated":
		var evt domain.ThreadCreated
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadCreated: %w", err)
		}
		return &evt, nil

	case "MessageAdded":
		var evt domain.MessageAdded
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal MessageAdded: %w", err)
		}
		return &evt, nil

	case "ThreadTitleChanged", "ThreadTitleUpdated":
		var evt domain.ThreadTitleUpdatedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadTitleUpdatedEvent: %w", err)
		}
		return &evt, nil

	case "ThreadDeleted":
		var evt domain.ThreadDeleted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadDeleted: %w", err)
		}
		return &evt, nil

	case "RunStarted":
		var evt domain.RunStartedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal RunStarted: %w", err)
		}
		return &evt, nil

	case "RunFinished":
		var evt domain.RunFinishedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal RunFinished: %w", err)
		}
		return &evt, nil

	case "RunError":
		var evt domain.RunErrorEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal RunError: %w", err)
		}
		return &evt, nil

	case "AssistantMessageStarted":
		var evt domain.AssistantMessageStartedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantMessageStarted: %w", err)
		}
		return &evt, nil

	case "AssistantContentStreamed":
		var evt domain.AssistantContentStreamedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantContentStreamed: %w", err)
		}
		return &evt, nil

	case "AssistantMessageCompleted":
		var evt domain.AssistantMessageCompletedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantMessageCompleted: %w", err)
		}
		return &evt, nil

	case "AgentHandoff":
		var evt domain.AgentHandoffEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AgentHandoff: %w", err)
		}
		return &evt, nil

	case "TokenUsageReported":
		var evt domain.TokenUsageReported
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal TokenUsageReported: %w", err)
		}
		return &evt, nil

	case "ToolStateChanged":
		var evt domain.ToolStateChangedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolStateChanged: %w", err)
		}
		return &evt, nil

	case "ToolAwaitingInput":
		var evt domain.ToolAwaitingInputEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolAwaitingInput: %w", err)
		}
		return &evt, nil

	case "ToolAwaitingAuth":
		var evt domain.ToolAwaitingAuthEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolAwaitingAuth: %w", err)
		}
		return &evt, nil

	case "ToolInputProvided":
		var evt domain.ToolInputProvidedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolInputProvided: %w", err)
		}
		return &evt, nil

	case "ToolAuthGranted":
		var evt domain.ToolAuthGrantedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolAuthGranted: %w", err)
		}
		return &evt, nil

	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
}
