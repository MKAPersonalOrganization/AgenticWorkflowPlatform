package chemistry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/sdk/agent-sdk/core"
	"school-with-ai/services/chatwrite/agents"
	"school-with-ai/services/chatwrite/agents/chemistry/tools"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
)

func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
		sharedtools.NewUnitConversionTool(),
		tools.NewPeriodicTableTool(),
	}
}

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,  // "question.answered"
		agents.EventTypeMathQuestion,      // "question.math"
		agents.EventTypePhysicsQuestion,   // "question.physics"
		agents.EventTypeWhiteboardUpdated, // "whiteboard.updated"
	}
}

func handlers() agents.EventHandlerRegistry {
	registry := make(agents.EventHandlerRegistry)

	registry.Register(agents.EventTypeChemistryQuestion, handleChemistryQuestion)

	return registry
}

func handleChemistryQuestion(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	slog.Info("Chemistry question received",
		"threadID", handlerCtx.ThreadID,
		"question", handlerCtx.Event.Data["question"],
	)

	question, _ := handlerCtx.Event.Data["question"].(string)
	context, _ := handlerCtx.Event.Data["context"].(string)
	emittedBy, _ := handlerCtx.Event.Data["emitted_by"].(string)

	slog.Info("Processing chemistry question",
		"question", question,
		"emittedBy", emittedBy,
	)

	// Get the AgentThread from state using ThreadStateWithStore interface
	stateWithStore, ok := handlerCtx.State.(core.ThreadStateWithStore)
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
		ID:        domain.GenerateMessageID(),
		Role:      emittedBy,
		Timestamp: time.Now(),
		Content:   handoffContent,
		Metadata: map[string]any{
			"handoff_from": emittedBy,
			"event_type":   "question.chemistry",
		},
	}
	_ = agentThread.AddMessages([]domain.Message{triggerMsg})

	messageID := domain.GenerateMessageID()

	err := handlerCtx.Agent.RunWithThread(ctx, handlerCtx.ThreadID, messageID, agentThread, func(event core.Event) error {
		return nil
	}, core.DefaultRunOptions())
	if err != nil {
		slog.Error("Chemistry handler failed", "error", err)
		return err
	}

	slog.Info("Chemistry question handled successfully")
	return nil
}

// Contract defines the event-driven contract for the chemistry instructor.
var Contract = &eventbus.AgentContract{
	Name: "chemistry-instructor",

	ListensTo: []agents.EventType{
		agents.EventTypeChemistryQuestion,
	},

	Emits:    emits(),
	Handlers: handlers(),
	Tools:    domainTools(),

	Factory: CreateChemistryInstructor,
}
