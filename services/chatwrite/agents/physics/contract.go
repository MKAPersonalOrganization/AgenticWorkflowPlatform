package physics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/chatwrite/agents"
	"school-with-ai/services/chatwrite/agents/physics/tools"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
	"school-with-ai/sdk/agent-sdk/core"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

func domainTools() []sdk.Tool {
	return []sdk.Tool{
		sharedtools.NewCalculatorTool(),
		sharedtools.NewUnitConversionTool(),
		tools.NewWeatherTool(),
	}
}

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,  // "question.answered"
		agents.EventTypeMathQuestion,      // "question.math"
		agents.EventTypeChemistryQuestion, // "question.chemistry"
		agents.EventTypeWhiteboardUpdated, // "whiteboard.updated"
	}
}

func handlers() agents.EventHandlerRegistry {
	registry := make(agents.EventHandlerRegistry)

	registry.Register(agents.EventTypePhysicsQuestion, handlePhysicsQuestion)

	return registry
}

func handlePhysicsQuestion(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	slog.Info("Physics question received",
		"threadID", handlerCtx.ThreadID,
		"question", handlerCtx.Event.Data["question"],
	)

	question, _ := handlerCtx.Event.Data["question"].(string)
	context, _ := handlerCtx.Event.Data["context"].(string)
	emittedBy, _ := handlerCtx.Event.Data["emitted_by"].(string)

	slog.Info("Processing physics question",
		"question", question,
		"emittedBy", emittedBy,
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

	// Create a clean handoff message with just the question and optional context
	handoffContent := question
	if context != "" {
		handoffContent = fmt.Sprintf("%s\n\nContext: %s", question, context)
	}

	triggerMsg := domain.Message{
		ID:        aguievents.GenerateMessageID(),
		Role:      emittedBy,
		Timestamp: time.Now(),
		Content:   handoffContent,
		Metadata: map[string]any{
			"handoff_from": emittedBy,
			"event_type":   "question.physics",
		},
	}
	agentThread.AddMessages([]domain.Message{triggerMsg})

	messageID := aguievents.GenerateMessageID()

	err := handlerCtx.Agent.RunWithThread(ctx, handlerCtx.ThreadID, messageID, agentThread, func(event sdk.Event) error {
		return nil
	}, sdk.DefaultRunOptions())
	if err != nil {
		slog.Error("Physics handler failed", "error", err)
		return err
	}

	slog.Info("Physics question handled successfully")
	return nil
}

// Contract defines the event-driven contract for the physics instructor.
var Contract = &eventbus.AgentContract{
	Name: "physics-instructor",

	ListensTo: []agents.EventType{
		agents.EventTypePhysicsQuestion,
	},

	Emits:    emits(),
	Handlers: handlers(),
	Tools:    domainTools(),

	Factory: CreatePhysicsInstructor,
}
