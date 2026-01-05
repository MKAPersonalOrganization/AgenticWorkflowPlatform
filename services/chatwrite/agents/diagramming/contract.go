package diagramming

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/chatwrite/agents"
	"school-with-ai/services/chatwrite/agents/diagramming/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
	"school-with-ai/sdk/agent-sdk/core"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

func domainTools() []sdk.Tool {
	return []sdk.Tool{
		tools.NewMermaidDiagramTool(),
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

	registry.Register(agents.EventTypeDiagrammingQuestion, handleDiagrammingQuestion)

	return registry
}

func handleDiagrammingQuestion(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	slog.Info("Diagramming question received",
		"threadID", handlerCtx.ThreadID,
		"request", handlerCtx.Event.Data["request"],
	)

	request, _ := handlerCtx.Event.Data["request"].(string)
	if request == "" {
		request, _ = handlerCtx.Event.Data["question"].(string)
	}
	context, _ := handlerCtx.Event.Data["context"].(string)
	emittedBy, _ := handlerCtx.Event.Data["emitted_by"].(string)

	slog.Info("Processing diagramming request",
		"request", request,
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

	// Create a clean handoff message with just the request and optional context
	handoffContent := request
	if context != "" {
		handoffContent = fmt.Sprintf("%s\n\nContext: %s", request, context)
	}

	triggerMsg := domain.Message{
		ID:        aguievents.GenerateMessageID(),
		Role:      emittedBy,
		Timestamp: time.Now(),
		Content:   handoffContent,
		Metadata: map[string]any{
			"handoff_from": emittedBy,
			"event_type":   "question.diagramming",
		},
	}
	agentThread.AddMessages([]domain.Message{triggerMsg})

	messageID := aguievents.GenerateMessageID()

	err := handlerCtx.Agent.RunWithThread(ctx, handlerCtx.ThreadID, messageID, agentThread, func(event sdk.Event) error {
		return nil
	}, sdk.DefaultRunOptions())
	if err != nil {
		slog.Error("Diagramming handler failed", "error", err)
		return err
	}

	slog.Info("Diagramming request handled successfully")
	return nil
}

// Contract defines the event-driven contract for the diagramming instructor.
var Contract = &eventbus.AgentContract{
	Name: "diagramming-instructor",

	ListensTo: []agents.EventType{
		agents.EventTypeDiagrammingQuestion,
	},

	Emits:    emits(),
	Handlers: handlers(),
	Tools:    domainTools(),

	Factory: CreateDiagrammingInstructor,
}
