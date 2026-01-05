package linearalgebra

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/advancedmath/linearalgebra/tools"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/sdk"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

func domainTools() []sdk.Tool {
	return []sdk.Tool{
		tools.NewVectorOperationsTool(),
		tools.NewMatrixOperationsTool(),
		tools.NewLinearSolverTool(),
		tools.NewEigenvalueTool(),
	}
}

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,  // "question.answered"
		agents.EventTypeWhiteboardUpdated, // "whiteboard.updated"
	}
}

// handlers returns the event handlers for this agent
func handlers() agents.EventHandlerRegistry {
	registry := make(agents.EventHandlerRegistry)

	// Handler for linear algebra questions
	registry.Register(agents.EventTypeLinearAlgebraQuestion, handleLinearAlgebraQuestion)

	return registry
}

// handleLinearAlgebraQuestion handles incoming linear algebra questions.
// This is the complete logic for processing a linear algebra event.
func handleLinearAlgebraQuestion(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	slog.Info("Linear algebra question received",
		"threadID", handlerCtx.ThreadID,
		"question", handlerCtx.Event.Data["question"],
	)

	// Get the question from the event
	question, _ := handlerCtx.Event.Data["question"].(string)
	whiteboardState, _ := handlerCtx.Event.Data["whiteboard_state"].(string)
	emittedBy, _ := handlerCtx.Event.Data["emitted_by"].(string)

	slog.Info("Processing linear algebra question",
		"question", question,
		"emittedBy", emittedBy,
	)

	// Get current thread state to build message history
	thread, exists := handlerCtx.State.Get(handlerCtx.ThreadID)
	if !exists {
		return fmt.Errorf("thread not found: %s", handlerCtx.ThreadID)
	}

	messages := thread.Messages

	// Add a trigger message with the whiteboard content
	triggerMsg := domain.Message{
		ID:        aguievents.GenerateMessageID(),
		Role:      "user",
		Timestamp: time.Now(),
		Content: fmt.Sprintf(`You have received a linear algebra question. The whiteboard has the context you need.

WHITEBOARD CONTENT:
%s

Follow your workflow:
1. Read and understand the content on the whiteboard
2. Use your linear algebra tools to compute the solution
3. Write your contribution to the whiteboard using write_whiteboard with action="append"
4. Signal completion: emit_question_answered(subject="linearalgebra", is_complete=true)`, whiteboardState),
	}
	messages = append(messages, triggerMsg)

	// Generate message ID for agent's response
	messageID := aguievents.GenerateMessageID()

	// Run the agent to process this event
	// The agent will use its tools and system prompt to generate the response
	err := handlerCtx.Agent.Run(ctx, handlerCtx.ThreadID, messageID, messages, func(event sdk.Event) error {
		// Events emitted by the agent are handled by the worker pool's routing
		return nil
	}, sdk.DefaultRunOptions())
	if err != nil {
		slog.Error("Linear algebra handler failed", "error", err)
		return err
	}

	slog.Info("Linear algebra question handled successfully")
	return nil
}

// Contract defines the event-driven contract for the linear algebra professor.
var Contract = &eventbus.AgentContract{
	Name: "linearalgebra-professor",

	ListensTo: []agents.EventType{
		agents.EventTypeLinearAlgebraQuestion,
	},

	Emits:    emits(),
	Handlers: handlers(),
	Tools:    domainTools(),

	Factory: CreateLinearAlgebraProfessor,
}
