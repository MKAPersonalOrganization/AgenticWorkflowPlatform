package studentfacing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/sdk"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

func domainTools() []sdk.Tool {
	// No domain-specific tools - this agent only routes
	return []sdk.Tool{}
}

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeMathQuestion,       // "question.math"
		agents.EventTypePhysicsQuestion,    // "question.physics"
		agents.EventTypeChemistryQuestion,  // "question.chemistry"
		agents.EventTypeStatisticsQuestion, // "question.statistics"
		agents.EventTypeWhiteboardUpdated,  // "whiteboard.updated"
	}
}

func handlers() agents.EventHandlerRegistry {
	registry := make(agents.EventHandlerRegistry)

	registry.Register(agents.EventTypeUserMessage, handleUserMessage)
	registry.Register(agents.EventTypeQuestionAnswered, handleQuestionAnswered)

	return registry
}

func handleUserMessage(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	slog.Info("User message received",
		"threadID", handlerCtx.ThreadID,
		"message", handlerCtx.Event.Data["message"],
	)

	// Get the AgentThread from state using ThreadStateWithStore interface
	stateWithStore, ok := handlerCtx.State.(sdk.ThreadStateWithStore)
	if !ok {
		return fmt.Errorf("state does not implement ThreadStateWithStore")
	}

	agentThread, exists := stateWithStore.GetAgentThread(handlerCtx.ThreadID)
	if !exists {
		return fmt.Errorf("thread not found: %s", handlerCtx.ThreadID)
	}

	// Note: The user message has already been added to the thread via UserMessageAddedEvent
	// No need to add it again - the agent will see it in the thread's message history

	messageID := aguievents.GenerateMessageID()

	err := handlerCtx.Agent.RunWithThread(ctx, handlerCtx.ThreadID, messageID, agentThread, func(event sdk.Event) error {
		return nil
	}, sdk.DefaultRunOptions())
	if err != nil {
		slog.Error("User message handler failed", "error", err)
		return err
	}

	slog.Info("User message handled successfully")
	return nil
}

func handleQuestionAnswered(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	subject, _ := handlerCtx.Event.Data["subject"].(string)
	isComplete, _ := handlerCtx.Event.Data["is_complete"].(bool)
	answer, _ := handlerCtx.Event.Data["answer"].(string)

	slog.Info("Question answered",
		"threadID", handlerCtx.ThreadID,
		"subject", subject,
		"isComplete", isComplete,
	)

	// If complete, directly emit the answer - no LLM call needed
	if isComplete {
		messageID := aguievents.GenerateMessageID()
		assistantEvent := &sdk.AssistantMessageCompletedEvent{
			BaseEvent: sdk.BaseEvent{
				Type:      "AssistantMessageCompleted",
				Timestamp: time.Now(),
				ThreadID:  handlerCtx.ThreadID,
			},
			MessageID: messageID,
			Content:   answer,
			AgentName: "student-facing-agent",
		}

		if err := handlerCtx.State.Update(assistantEvent); err != nil {
			return err
		}
		if err := handlerCtx.Notifier.HandleDomainEvent(assistantEvent); err != nil {
			return err
		}

		slog.Info("Final answer presented to user")
		return nil
	}

	// If incomplete, invoke agent to determine what's still needed
	stateWithStore, ok := handlerCtx.State.(sdk.ThreadStateWithStore)
	if !ok {
		return fmt.Errorf("state does not implement ThreadStateWithStore")
	}

	agentThread, exists := stateWithStore.GetAgentThread(handlerCtx.ThreadID)
	if !exists {
		return fmt.Errorf("thread not found: %s", handlerCtx.ThreadID)
	}

	triggerMsg := domain.Message{
		ID:        aguievents.GenerateMessageID(),
		Role:      "user",
		Timestamp: time.Now(),
		Content: fmt.Sprintf(`A %s specialist has completed partial work (marked as incomplete).

Current answer: %s

Determine what's still missing from the original question and route to the appropriate specialist.`, subject, answer),
	}
	agentThread.AddMessages([]domain.Message{triggerMsg})

	messageID := aguievents.GenerateMessageID()
	err := handlerCtx.Agent.RunWithThread(ctx, handlerCtx.ThreadID, messageID, agentThread, func(event sdk.Event) error {
		return nil
	}, sdk.DefaultRunOptions())
	if err != nil {
		slog.Error("Student-facing handler failed", "error", err)
		return err
	}

	slog.Info("Question answered event handled successfully")
	return nil
}

// Contract defines the event-driven contract for the student-facing agent.
// This agent is special - it receives user messages and routes them to specialists.
var Contract = &eventbus.AgentContract{
	Name: "student-facing-agent",

	// Listens to user messages and specialist callbacks
	ListensTo: []agents.EventType{
		agents.EventTypeUserMessage,
		agents.EventTypeQuestionAnswered,
	},

	Emits:    emits(),
	Handlers: handlers(),
	Tools:    domainTools(),

	Factory: CreateStudentFacingAgent,
}
