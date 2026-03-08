package custom

import (
	"context"
	"fmt"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/chat/thread"
)

// EventPipeline provides observability for event processing
type EventPipeline struct {
	log *logger.Logger
}

// NewEventPipeline creates a new event pipeline
func NewEventPipeline(log *logger.Logger) *EventPipeline {
	return &EventPipeline{log: log}
}

// ConversionResult represents the result of event conversion
type ConversionResult struct {
	DomainEvent thread.Event
	Skipped     bool
	Reason      string
}

// ConvertWithObservability converts core events to domain events with full observability
func (ep *EventPipeline) ConvertWithObservability(ctx context.Context, coreEvent core.Event) ConversionResult {
	if coreEvent == nil {
		return ConversionResult{Skipped: true, Reason: "nil core event"}
	}

	coreEventType := fmt.Sprintf("%T", coreEvent)
	ep.log.Debug(ctx, "Event Pipeline: Converting core event", "coreType", coreEventType)

	// Attempt conversion
	domainEvent := ep.convertCoreEvent(coreEvent)

	if domainEvent == nil {
		ep.log.Debug(ctx, "Event Pipeline: Skipping unsupported core event", "coreType", coreEventType)
		return ConversionResult{Skipped: true, Reason: fmt.Sprintf("unsupported core event type: %s", coreEventType)}
	}

	// Validate domain event
	if err := ep.validateDomainEvent(domainEvent); err != nil {
		ep.log.Error(ctx, "Event Pipeline: Domain event validation failed", "coreType", coreEventType, "domainType", domainEvent.EventType(), "error", err)
		return ConversionResult{Skipped: true, Reason: fmt.Sprintf("validation failed: %v", err)}
	}

	ep.log.Debug(ctx, "Event Pipeline: Successfully converted", "coreType", coreEventType, "domainType", domainEvent.EventType())
	return ConversionResult{DomainEvent: domainEvent}
}

// convertCoreEvent handles the actual conversion logic
func (ep *EventPipeline) convertCoreEvent(e core.Event) thread.Event {
	switch evt := e.(type) {
	case *core.AssistantMessageAdded:
		return &thread.AssistantMessageAdded{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Content:   evt.Content,
			ToolCalls: MapCoreToolCallsToDomain(evt.ToolCalls),
			AgentName: evt.AgentName,
			Timestamp: evt.Timestamp,
		}
	case *core.ToolResultAdded:
		return &thread.ToolResultAdded{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			ToolName:   evt.ToolName,
			Result:     evt.Result,
			IsError:    evt.IsError,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}
	case *core.TokenUsageReported:
		return (*thread.TokenUsageReported)(evt)
	case *core.AssistantMessageStarted:
		return (*thread.AssistantMessageStarted)(evt)
	case *core.AssistantContentStreamed:
		return (*thread.AssistantContentStreamed)(evt)
	case *core.AssistantMessageCompleted:
		return &thread.AssistantMessageCompleted{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Timestamp: evt.Timestamp,
		}

	// Tool lifecycle events
	case *core.ToolStart:
		return (*thread.ToolStart)(evt)
	case *core.ToolContentStreamed:
		return &thread.ToolContentStreamed{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			ToolName:   evt.ToolName,
			Delta:      evt.Delta,
			Field:      evt.Field,
			Timestamp:  evt.Timestamp,
		}
	case *core.InputProvided:
		return (*thread.InputProvided)(evt)
	case *core.AuthGranted:
		return (*thread.AuthGranted)(evt)
	case *core.AuthDenied:
		return (*thread.AuthDenied)(evt)
	case *core.ToolCancelled:
		return (*thread.ToolCancelled)(evt)

	// Events that need manual conversion
	case *core.ToolNeedsInput:
		return &thread.ToolNeedsInput{
			ThreadID:    evt.ThreadID,
			MessageID:   evt.MessageID,
			ToolCallID:  evt.ToolCallID,
			ToolName:    evt.ToolName,
			Elicitation: convertElicitationSpec(evt.Elicitation),
			AgentName:   evt.AgentName,
			Timestamp:   evt.Timestamp,
		}

	default:
		// Handle internal engine events by event type (not exported structs)
		if e.EventType() == "RunCompleted" {
			// This is an internal engine event, not meant to be persisted
			return nil
		}

		// Check if it's already a domain event
		if de, ok := e.(thread.Event); ok {
			return de
		}
		return nil
	}
}

// validateDomainEvent validates that domain events have required fields
func (ep *EventPipeline) validateDomainEvent(event thread.Event) error {
	if event == nil {
		return fmt.Errorf("domain event is nil")
	}

	if event.EventType() == "" {
		return fmt.Errorf("domain event missing EventType")
	}

	if event.AggregateID() == "" {
		return fmt.Errorf("domain event missing AggregateID")
	}

	if event.EventTimestamp().IsZero() {
		return fmt.Errorf("domain event missing Timestamp")
	}

	return nil
}

// ProcessEventStream processes a stream of core events with full observability
func (ep *EventPipeline) ProcessEventStream(ctx context.Context, coreEvents func(yield func(core.Event, error) bool)) func(yield func(thread.Event, error) bool) {
	return func(yield func(thread.Event, error) bool) {
		eventCount := 0
		convertedCount := 0
		skippedCount := 0

		for ce, err := range coreEvents {
			eventCount++

			if err != nil {
				ep.log.Error(ctx, "Event Pipeline: Core event stream error", "eventNumber", eventCount, "error", err)
				yield(nil, err)
				return
			}

			result := ep.ConvertWithObservability(ctx, ce)

			if result.Skipped {
				skippedCount++
				ep.log.Debug(ctx, "Event Pipeline: Skipped event", "eventNumber", eventCount, "reason", result.Reason)
				continue
			}

			convertedCount++
			if !yield(result.DomainEvent, nil) {
				ep.log.Info(ctx, "Event Pipeline: Stream terminated early", "totalEvents", eventCount, "converted", convertedCount, "skipped", skippedCount)
				return
			}
		}

		ep.log.Info(ctx, "Event Pipeline: Stream completed", "totalEvents", eventCount, "converted", convertedCount, "skipped", skippedCount)
	}
}
