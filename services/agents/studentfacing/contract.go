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

	message, _ := handlerCtx.Event.Data["message"].(string)

	thread, exists := handlerCtx.State.Get(handlerCtx.ThreadID)
	if !exists {
		return fmt.Errorf("thread not found: %s", handlerCtx.ThreadID)
	}

	messages := thread.Messages

	// The student-facing agent analyzes the user message and routes to appropriate specialist
	triggerMsg := domain.Message{
		ID:        aguievents.GenerateMessageID(),
		Role:      "user",
		Timestamp: time.Now(),
		Content:   message, // The actual user message becomes the trigger
	}
	messages = append(messages, triggerMsg)

	messageID := aguievents.GenerateMessageID()

	err := handlerCtx.Agent.Run(ctx, handlerCtx.ThreadID, messageID, messages, func(event sdk.Event) error {
		return nil
	})
	if err != nil {
		slog.Error("User message handler failed", "error", err)
		return err
	}

	slog.Info("User message handled successfully")
	return nil
}

func handleQuestionAnswered(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	slog.Info("Question answered - verifying completeness",
		"threadID", handlerCtx.ThreadID,
		"subject", handlerCtx.Event.Data["subject"],
		"isComplete", handlerCtx.Event.Data["is_complete"],
	)

	subject, _ := handlerCtx.Event.Data["subject"].(string)
	isComplete, _ := handlerCtx.Event.Data["is_complete"].(bool)
	whiteboardState, _ := handlerCtx.Event.Data["whiteboard_state"].(string)
	answeredBy, _ := handlerCtx.Event.Data["answered_by"].(string)

	slog.Info("Processing question answered callback",
		"subject", subject,
		"isComplete", isComplete,
		"answeredBy", answeredBy,
	)

	thread, exists := handlerCtx.State.Get(handlerCtx.ThreadID)
	if !exists {
		return fmt.Errorf("thread not found: %s", handlerCtx.ThreadID)
	}

	messages := thread.Messages

	// The student-facing agent needs to verify if the answer is complete
	// and potentially route to other specialists if needed
	triggerMsg := domain.Message{
		ID:        aguievents.GenerateMessageID(),
		Role:      "user",
		Timestamp: time.Now(),
		Content: fmt.Sprintf(`An instructor (%s) has completed their work on a %s question.

WHITEBOARD CONTENT:
%s

Your task (STATE 2: INSTRUCTOR CALLBACK):
1. Read the whiteboard to see what has been answered
2. Verify if the ENTIRE user request has been satisfied
3. If complete: HALT (do nothing - your work is done)
4. If incomplete: Determine what's still missing and emit the appropriate question event to activate the needed specialist`, answeredBy, subject, whiteboardState),
	}
	messages = append(messages, triggerMsg)

	messageID := aguievents.GenerateMessageID()

	err := handlerCtx.Agent.Run(ctx, handlerCtx.ThreadID, messageID, messages, func(event sdk.Event) error {
		return nil
	})
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
