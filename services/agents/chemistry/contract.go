package chemistry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/chemistry/tools"
	sharedtools "school-with-ai/services/agents/shared/tools"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/sdk"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

func domainTools() []sdk.Tool {
	return []sdk.Tool{
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
	whiteboardState, _ := handlerCtx.Event.Data["whiteboard_state"].(string)
	emittedBy, _ := handlerCtx.Event.Data["emitted_by"].(string)

	slog.Info("Processing chemistry question",
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
		Content: fmt.Sprintf(`You have received a chemistry question. The whiteboard has the context you need.

WHITEBOARD CONTENT:
%s

Follow your workflow:
1. Read and understand the content on the whiteboard
2. Use your chemistry tools to compute the solution
3. Write your contribution to the whiteboard using write_whiteboard with action="append"
4. Signal completion: emit_question_answered(subject="chemistry", is_complete=true)`, whiteboardState),
	}
	messages = append(messages, triggerMsg)

	messageID := aguievents.GenerateMessageID()

	err := handlerCtx.Agent.Run(ctx, handlerCtx.ThreadID, messageID, messages, func(event sdk.Event) error {
		return nil
	})
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
