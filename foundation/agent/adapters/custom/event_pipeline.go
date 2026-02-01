package custom

import (
	"context"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
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
	DomainEvent domain.Event
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
		ep.log.Info(ctx, "Event Pipeline: Skipping unsupported core event", "coreType", coreEventType)
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
func (ep *EventPipeline) convertCoreEvent(e core.Event) domain.Event {
	switch evt := e.(type) {
	case *core.UserMessageAdded:
		return &domain.UserMessageAdded{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Content:   evt.Content,
			UserID:    evt.UserID,
			Timestamp: evt.Timestamp,
		}
	case *core.AssistantMessageAdded:
		return &domain.AssistantMessageAdded{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Content:   evt.Content,
			ToolCalls: MapCoreToolCallsToDomain(evt.ToolCalls),
			AgentName: evt.AgentName,
			Timestamp: evt.Timestamp,
		}
	case *core.ToolResultAdded:
		return &domain.ToolResultAdded{
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
		return (*domain.TokenUsageReported)(evt)
	case *core.ExecutionStateReported:
		return (*domain.ExecutionStateReported)(evt)
	case *core.AssistantMessageStarted:
		return (*domain.AssistantMessageStarted)(evt)
	case *core.AssistantContentStreamed:
		return (*domain.AssistantContentStreamed)(evt)
	case *core.AssistantMessageCompleted:
		return &domain.AssistantMessageCompleted{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Timestamp: evt.Timestamp,
		}
	case *core.RunStarted:
		return (*domain.RunStarted)(evt)
	case *core.RunFinished:
		return (*domain.RunFinished)(evt)
	case *core.TurnFinished:
		return (*domain.TurnFinished)(evt)
	
	// Tool lifecycle events
	case *core.ToolStart:
		return (*domain.ToolStart)(evt)
	case *core.ToolSuccess:
		return (*domain.ToolSuccess)(evt)
	case *core.ToolError:
		return (*domain.ToolError)(evt)
	case *core.InputProvided:
		return (*domain.InputProvided)(evt)
	case *core.AuthGranted:
		return (*domain.AuthGranted)(evt)
	case *core.AuthDenied:
		return (*domain.AuthDenied)(evt)
	case *core.ToolCancelled:
		return (*domain.ToolCancelled)(evt)
	
	// Events that need manual conversion
	case *core.ActionsDetected:
		return &domain.ActionsDetected{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			ToolCalls: MapCoreToolCallsToDomain(evt.ToolCalls),
			Timestamp: evt.Timestamp,
		}
	case *core.ToolNeedsInput:
		return &domain.ToolNeedsInput{
			ThreadID:    evt.ThreadID,
			MessageID:   evt.MessageID,
			ToolCallID:  evt.ToolCallID,
			ToolName:    evt.ToolName,
			Elicitation: (*domain.ElicitationSpec)(evt.Elicitation),
			AgentName:   evt.AgentName,
			Timestamp:   evt.Timestamp,
		}
	
	// Handle value types (not pointers)
	case core.EngineStateChanged:
		return &domain.EngineStateChanged{
			ThreadID:  evt.ThreadID,
			NewState:  evt.State,
			Timestamp: evt.Timestamp,
		}
	
	
	default:
		// Handle internal engine events by event type (not exported structs)
		if e.EventType() == "RunCompleted" {
			// This is an internal engine event, not meant to be persisted
			return nil
		}
		
		// Check if it's already a domain event
		if de, ok := e.(domain.Event); ok {
			return de
		}
		return nil
	}
}

// validateDomainEvent validates that domain events have required fields
func (ep *EventPipeline) validateDomainEvent(event domain.Event) error {
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
func (ep *EventPipeline) ProcessEventStream(ctx context.Context, coreEvents func(yield func(core.Event, error) bool)) func(yield func(domain.Event, error) bool) {
	return func(yield func(domain.Event, error) bool) {
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