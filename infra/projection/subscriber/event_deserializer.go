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

	case "UserMessageAdded":
		var evt domain.UserMessageAdded
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal UserMessageAdded: %w", err)
		}
		return &evt, nil

	case "AssistantMessageAdded":
		var evt domain.AssistantMessageAdded
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantMessageAdded: %w", err)
		}
		return &evt, nil

	case "ToolResultAdded":
		var evt domain.ToolResultAdded
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolResultAdded: %w", err)
		}
		return &evt, nil

	case "ThreadTitleChanged", "ThreadTitleUpdated":
		var evt domain.ThreadTitleUpdated
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadTitleUpdated: %w", err)
		}
		return &evt, nil

	case "ThreadDeleted":
		var evt domain.ThreadDeleted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadDeleted: %w", err)
		}
		return &evt, nil

	case "RunStarted":
		var evt domain.RunStarted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal RunStarted: %w", err)
		}
		return &evt, nil

	case "RunFinished":
		var evt domain.RunFinished
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal RunFinished: %w", err)
		}
		return &evt, nil

	case "RunError":
		var evt domain.RunError
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal RunError: %w", err)
		}
		return &evt, nil

	case "AssistantMessageStarted":
		var evt domain.AssistantMessageStarted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantMessageStarted: %w", err)
		}
		return &evt, nil

	case "AssistantContentStreamed":
		var evt domain.AssistantContentStreamed
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantContentStreamed: %w", err)
		}
		return &evt, nil

	case "AssistantMessageCompleted":
		var evt domain.AssistantMessageCompleted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantMessageCompleted: %w", err)
		}
		return &evt, nil

	case "AgentHandoff":
		var evt domain.AgentHandoff
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

	case "ToolStart":
		var evt domain.ToolStart
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolStart: %w", err)
		}
		return &evt, nil

	case "ToolSuccess":
		var evt domain.ToolSuccess
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolSuccess: %w", err)
		}
		return &evt, nil

	case "ToolError":
		var evt domain.ToolError
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolError: %w", err)
		}
		return &evt, nil

	case "ToolNeedsInput":
		var evt domain.ToolNeedsInput
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolNeedsInput: %w", err)
		}
		return &evt, nil

	case "ToolNeedsAuth":
		var evt domain.ToolNeedsAuth
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolNeedsAuth: %w", err)
		}
		return &evt, nil

	case "InputProvided":
		var evt domain.InputProvided
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal InputProvided: %w", err)
		}
		return &evt, nil

	case "AuthGranted":
		var evt domain.AuthGranted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AuthGranted: %w", err)
		}
		return &evt, nil

	case "AuthDenied":
		var evt domain.AuthDenied
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AuthDenied: %w", err)
		}
		return &evt, nil

	case "ToolCancelled":
		var evt domain.ToolCancelled
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolCancelled: %w", err)
		}
		return &evt, nil

	case "EngineStateChanged":
		var evt domain.EngineStateChanged
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EngineStateChanged: %w", err)
		}
		return &evt, nil

	case "ActionsDetected":
		var evt domain.ActionsDetected
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ActionsDetected: %w", err)
		}
		return &evt, nil

	case "TurnFinished":
		var evt domain.TurnFinished
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal TurnFinished: %w", err)
		}
		return &evt, nil

	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
}
