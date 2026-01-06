package studentfacing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
)

func domainTools() []core.Tool {
	// No domain-specific tools - this agent only routes
	return []core.Tool{}
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
	registry.Register(agents.EventTypeAgentHandoff, handleAgentHandoff)

	return registry
}

func handleUserMessage(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	slog.Info("User message received",
		"threadID", handlerCtx.ThreadID,
		"message", handlerCtx.Event.Data["message"],
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

	// Note: The user message has already been added to the thread via UserMessageAddedEvent
	// No need to add it again - the agent will see it in the thread's message history

	messageID := domain.GenerateMessageID()

	err := handlerCtx.Agent.RunWithThread(ctx, handlerCtx.ThreadID, messageID, agentThread, func(event core.Event) error {
		return nil
	}, core.DefaultRunOptions())
	if err != nil {
		slog.Error("User message handler failed", "error", err)
		return err
	}

	slog.Info("User message handled successfully")
	return nil
}

func handleQuestionAnswered(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	subject, _ := handlerCtx.Event.Data["subject"].(string)

	slog.Info("Question answered",
		"threadID", handlerCtx.ThreadID,
		"subject", subject,
	)

	// Get the AgentThread to let the agent process the answer and update todos
	stateWithStore, ok := handlerCtx.State.(core.ThreadStateWithStore)
	if !ok {
		return fmt.Errorf("state does not implement ThreadStateWithStore")
	}

	agentThread, exists := stateWithStore.GetAgentThread(handlerCtx.ThreadID)
	if !exists {
		return fmt.Errorf("thread not found: %s", handlerCtx.ThreadID)
	}

	// Create a message instructing the agent to update todos and check completion
	triggerMsg := domain.Message{
		ID:        domain.GenerateMessageID(),
		Role:      "system",
		Timestamp: time.Now(),
		Content: fmt.Sprintf(`A %s specialist has answered. The answer is on the whiteboard.

Instructions:
1. Update your todos with write_todos() - mark the completed routing as "completed"
2. Sync the updated plan with write_whiteboard()
3. Check if ALL todos are completed:
   - If YES: Present the final answer to the user based on the whiteboard content
   - If NO: Continue routing to the next specialist for pending todos

Remember: You are a routing coordinator. Check the routing plan and continue until everything is done.`, subject),
	}
	_ = agentThread.AddMessages([]domain.Message{triggerMsg})

	messageID := domain.GenerateMessageID()
	err := handlerCtx.Agent.RunWithThread(ctx, handlerCtx.ThreadID, messageID, agentThread, func(event core.Event) error {
		return nil
	}, core.DefaultRunOptions())
	if err != nil {
		slog.Error("Student-facing handler failed", "error", err)
		return err
	}

	slog.Info("Question answered event handled - agent processing completion")
	return nil
}

func handleAgentHandoff(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	slog.Info("Agent handoff received (coordinator)",
		"threadID", handlerCtx.ThreadID,
		"to_agent", handlerCtx.Event.Data["to_agent"],
	)

	question, _ := handlerCtx.Event.Data["question"].(string)
	context, _ := handlerCtx.Event.Data["context"].(string)
	fromAgent, _ := handlerCtx.Event.Data["from_agent"].(string)

	slog.Info("Coordinator receiving handoff back",
		"from", fromAgent,
		"context", context,
	)

	// Get the AgentThread from state
	stateWithStore, ok := handlerCtx.State.(core.ThreadStateWithStore)
	if !ok {
		return fmt.Errorf("state does not implement ThreadStateWithStore")
	}

	agentThread, exists := stateWithStore.GetAgentThread(handlerCtx.ThreadID)
	if !exists {
		return fmt.Errorf("thread not found: %s", handlerCtx.ThreadID)
	}

	// Create handoff message - specialist is returning with their answer
	handoffContent := fmt.Sprintf("[Specialist %s completed their work]\n\n%s", fromAgent, context)
	if question != "" {
		handoffContent = question + "\n\n" + context
	}

	triggerMsg := domain.Message{
		ID:        domain.GenerateMessageID(),
		Role:      fromAgent,
		Timestamp: time.Now(),
		Content:   handoffContent,
		Metadata: map[string]any{
			"handoff_from": fromAgent,
			"event_type":   "agent.handoff",
			"return":       true,
		},
	}
	_ = agentThread.AddMessages([]domain.Message{triggerMsg})

	messageID := domain.GenerateMessageID()

	err := handlerCtx.Agent.RunWithThread(ctx, handlerCtx.ThreadID, messageID, agentThread, func(event core.Event) error {
		return nil
	}, core.DefaultRunOptions())
	if err != nil {
		slog.Error("Coordinator handoff handler failed", "error", err)
		return err
	}

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
		agents.EventTypeAgentHandoff,
	},

	Emits:    emits(),
	Handlers: handlers(),
	Tools:    domainTools(),

	Factory: CreateStudentFacingAgent,
}
