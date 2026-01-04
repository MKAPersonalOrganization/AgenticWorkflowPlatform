package math

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/math/tools"
	sharedtools "school-with-ai/services/agents/shared/tools"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/sdk"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// domainTools returns the domain-specific tools for the math instructor.
// Defined as a function to avoid initialization cycles.
func domainTools() []sdk.Tool {
	return []sdk.Tool{
		sharedtools.NewCalculatorTool(),
		tools.NewGraphingTool(),
		tools.NewEquationSolverTool(),
		tools.NewPlotTool(),
		tools.NewAnimateTool(),
	}
}

// emits returns the events this agent can emit.
// Defined as a function to avoid initialization cycles.
func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,      // "question.answered"
		agents.EventTypePhysicsQuestion,       // "question.physics"
		agents.EventTypeChemistryQuestion,     // "question.chemistry"
		agents.EventTypeLinearAlgebraQuestion, // "question.linearalgebra"
		agents.EventTypeDiagrammingQuestion,   // "question.diagramming"
		agents.EventTypeWhiteboardUpdated,     // "whiteboard.updated"
	}
}

func handlers() agents.EventHandlerRegistry {
	registry := make(agents.EventHandlerRegistry)

	registry.Register(agents.EventTypeMathQuestion, handleMathQuestion)

	return registry
}

func handleMathQuestion(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	slog.Info("Math question received",
		"threadID", handlerCtx.ThreadID,
		"question", handlerCtx.Event.Data["question"],
	)

	question, _ := handlerCtx.Event.Data["question"].(string)
	whiteboardState, _ := handlerCtx.Event.Data["whiteboard_state"].(string)
	emittedBy, _ := handlerCtx.Event.Data["emitted_by"].(string)

	slog.Info("Processing math question",
		"question", question,
		"emittedBy", emittedBy,
	)

	thread, exists := handlerCtx.State.Get(handlerCtx.ThreadID)
	if !exists {
		return fmt.Errorf("thread not found: %s", handlerCtx.ThreadID)
	}

	messages := thread.Messages

	triggerMsg := domain.Message{
		ID:        aguievents.GenerateMessageID(),
		Role:      "user",
		Timestamp: time.Now(),
		Content: fmt.Sprintf(`You have received a math question. The whiteboard has the context you need.

WHITEBOARD CONTENT:
%s

Follow your workflow:
1. Read and understand the content on the whiteboard
2. Use your math tools to compute the solution (calculator, graphing, equation solver, etc.)
3. Write your contribution to the whiteboard using write_whiteboard with action="append"
4. Signal completion: emit_question_answered(subject="math", is_complete=true)`, whiteboardState),
	}
	messages = append(messages, triggerMsg)

	messageID := aguievents.GenerateMessageID()

	err := handlerCtx.Agent.Run(ctx, handlerCtx.ThreadID, messageID, messages, func(event sdk.Event) error {
		return nil
	})
	if err != nil {
		slog.Error("Math handler failed", "error", err)
		return err
	}

	slog.Info("Math question handled successfully")
	return nil
}

// Contract defines the event-driven contract for the math instructor.
// The system uses this to:
// - Route question.math events to this agent
// - Auto-generate emission tools for the events this agent can emit
var Contract = &eventbus.AgentContract{
	Name: "math-instructor",

	// Events this agent reacts to
	ListensTo: []agents.EventType{
		agents.EventTypeMathQuestion, // "question.math"
	},

	// Events this agent can emit (emission tools are auto-generated)
	Emits:    emits(),
	Handlers: handlers(),

	// Domain-specific tools (emission tools are auto-generated from Emits)
	Tools: domainTools(),

	Factory: CreateMathInstructor,
}
