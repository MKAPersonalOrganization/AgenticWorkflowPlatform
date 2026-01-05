package statistics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra"
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/agents/statistics/tools"
	"school-with-ai/services/sdk"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

func domainTools() []sdk.Tool {
	return []sdk.Tool{
		tools.NewDescriptiveStatsTool(),
		tools.NewCorrelationTool(),
		tools.NewDistanceMetricsTool(),
		tools.NewSamplingTool(),
		tools.NewDataTransformTool(),
		tools.NewNormalDistributionTool(),
		tools.NewGeometricDistributionTool(),
		tools.NewRegressionTool(),
	}
}

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,  // "question.answered"
		agents.EventTypeWhiteboardUpdated, // "whiteboard.updated"
	}
}

func handlers() agents.EventHandlerRegistry {
	registry := make(agents.EventHandlerRegistry)

	registry.Register(agents.EventTypeStatisticsQuestion, handleStatisticsQuestion)

	return registry
}

func handleStatisticsQuestion(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	slog.Info("Statistics question received",
		"threadID", handlerCtx.ThreadID,
		"question", handlerCtx.Event.Data["question"],
	)

	question, _ := handlerCtx.Event.Data["question"].(string)
	context, _ := handlerCtx.Event.Data["context"].(string)
	emittedBy, _ := handlerCtx.Event.Data["emitted_by"].(string)

	slog.Info("Processing statistics question",
		"question", question,
		"emittedBy", emittedBy,
	)

	// Get the AgentThread from state
	state, ok := handlerCtx.State.(*infra.InMemoryThreadState)
	if !ok {
		return fmt.Errorf("state is not InMemoryThreadState")
	}

	agentThread, exists := state.GetAgentThread(handlerCtx.ThreadID)
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
		Role:      "user",
		Timestamp: time.Now(),
		Content:   handoffContent,
		Metadata: map[string]interface{}{
			"handoff_from": emittedBy,
			"event_type":   "question.statistics",
		},
	}
	agentThread.AddMessages([]domain.Message{triggerMsg})

	messageID := aguievents.GenerateMessageID()

	err := handlerCtx.Agent.RunWithThread(ctx, handlerCtx.ThreadID, messageID, agentThread, func(event sdk.Event) error {
		return nil
	}, sdk.DefaultRunOptions())
	if err != nil {
		slog.Error("Statistics handler failed", "error", err)
		return err
	}

	slog.Info("Statistics question handled successfully")
	return nil
}

// Contract defines the event-driven contract for the statistics instructor.
var Contract = &eventbus.AgentContract{
	Name: "statistics-instructor",

	ListensTo: []agents.EventType{
		agents.EventTypeStatisticsQuestion,
	},

	Emits:    emits(),
	Handlers: handlers(),
	Tools:    domainTools(),

	Factory: CreateStatisticsInstructor,
}
