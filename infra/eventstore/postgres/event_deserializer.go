package postgres

import (
	"encoding/json"
	"fmt"
	"school-with-ai/domain"
)

// DeserializeDomainEvent deserializes a domain event from JSON based on its type.
func DeserializeDomainEvent(eventType string, data []byte) (domain.Event, error) {
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

	case "ToolContentStreamed":
		var evt domain.ToolContentStreamed
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolContentStreamed: %w", err)
		}
		return &evt, nil

	case "ToolSuccess", "ToolError", "ToolResultAdded":
		var evt domain.ToolResultAdded
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolResultAdded: %w", err)
		}
		if eventType == "ToolError" {
			evt.IsError = true
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

	// Payment account events
	case "PaymentAccountOpened":
		var evt domain.PaymentAccountOpened
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PaymentAccountOpened: %w", err)
		}
		return &evt, nil

	case "PaymentInitiated":
		var evt domain.PaymentInitiated
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PaymentInitiated: %w", err)
		}
		return &evt, nil

	case "PaymentSucceeded":
		var evt domain.PaymentSucceeded
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PaymentSucceeded: %w", err)
		}
		return &evt, nil

	case "PaymentFailed":
		var evt domain.PaymentFailed
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PaymentFailed: %w", err)
		}
		return &evt, nil

	case "PaymentRefunded":
		var evt domain.PaymentRefunded
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PaymentRefunded: %w", err)
		}
		return &evt, nil

	case "PaymentDisputed":
		var evt domain.PaymentDisputed
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PaymentDisputed: %w", err)
		}
		return &evt, nil

	case "DisputeResolved":
		var evt domain.DisputeResolved
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal DisputeResolved: %w", err)
		}
		return &evt, nil

	case "PaymentMethodUpdated":
		var evt domain.PaymentMethodUpdated
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PaymentMethodUpdated: %w", err)
		}
		return &evt, nil

	case "InvoicePaymentFailed":
		var evt domain.InvoicePaymentFailed
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal InvoicePaymentFailed: %w", err)
		}
		return &evt, nil

	case "SubscriptionCancelled":
		var evt domain.SubscriptionCancelled
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal SubscriptionCancelled: %w", err)
		}
		return &evt, nil

	case "SubscriptionExpired":
		var evt domain.SubscriptionExpired
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal SubscriptionExpired: %w", err)
		}
		return &evt, nil

	case "SubscriptionReactivated":
		var evt domain.SubscriptionReactivated
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal SubscriptionReactivated: %w", err)
		}
		return &evt, nil

	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
}
