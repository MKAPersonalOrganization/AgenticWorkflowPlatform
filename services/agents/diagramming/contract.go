package diagramming

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra"
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/diagramming/tools"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/sdk"

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
	whiteboardState, _ := handlerCtx.Event.Data["whiteboard_state"].(string)
	emittedBy, _ := handlerCtx.Event.Data["emitted_by"].(string)

	slog.Info("Processing diagramming request",
		"request", request,
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

	triggerMsg := domain.Message{
		ID:        aguievents.GenerateMessageID(),
		Role:      "user",
		Timestamp: time.Now(),
		Content: fmt.Sprintf(`You have received a diagramming request. The whiteboard has the context you need.

WHITEBOARD CONTENT:
%s

Follow your workflow:
1. Read and understand the content on the whiteboard
2. Create the appropriate Mermaid diagram
3. Write your contribution to the whiteboard using write_whiteboard with action="append"
4. Signal completion: emit_question_answered(subject="diagramming", is_complete=true)`, whiteboardState),
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
