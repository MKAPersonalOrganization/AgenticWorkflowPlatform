package math

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents"
	"school-with-ai/services/chatwrite/agents/math/tools"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
)

// domainTools returns the domain-specific tools for the math instructor.
// Defined as a function to avoid initialization cycles.
func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
		tools.NewGraphingTool(),
		tools.NewEquationSolverTool(),
		tools.NewPlotTool(),
		tools.NewAnimateTool(),
	}
}

func handlers() agents.EventHandlerRegistry {
	registry := make(agents.EventHandlerRegistry)

	registry.Register(agents.EventTypeAgentHandoff, handleAgentHandoff)

	return registry
}

func handleAgentHandoff(ctx context.Context, handlerCtx *agents.EventHandlerContext) error {
	slog.Info("Agent handoff received",
		"threadID", handlerCtx.ThreadID,
		"to_agent", handlerCtx.Event.Data["to_agent"],
	)

	question, _ := handlerCtx.Event.Data["question"].(string)
	context, _ := handlerCtx.Event.Data["context"].(string)
	fromAgent, _ := handlerCtx.Event.Data["from_agent"].(string)

	slog.Info("Processing agent handoff",
		"question", question,
		"from", fromAgent,
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

	// Create handoff message with question and context
	handoffContent := question
	if context != "" {
		handoffContent = fmt.Sprintf("%s\n\nContext: %s", question, context)
	}

	triggerMsg := domain.Message{
		ID:        domain.GenerateMessageID(),
		Role:      fromAgent,
		Timestamp: time.Now(),
		Content:   handoffContent,
		Metadata: map[string]any{
			"handoff_from": fromAgent,
			"event_type":   "agent.handoff",
		},
	}
	_ = agentThread.AddMessages([]domain.Message{triggerMsg})

	messageID := domain.GenerateMessageID()

	err := handlerCtx.Agent.RunWithThread(ctx, handlerCtx.ThreadID, messageID, agentThread, func(event core.Event) error {
		return nil
	}, core.DefaultRunOptions())
	if err != nil {
		slog.Error("Handoff handler failed", "error", err)
		return err
	}

	return nil
}

// Contract defines the event-driven contract for the math instructor.
// The system uses this to route agent.handoff events to this agent.
var Contract = &eventbus.AgentContract{
	Name: "math-instructor",

	// Events this agent reacts to
	ListensTo: []agents.EventType{
		agents.EventTypeAgentHandoff, // "agent.handoff"
	},

	// Event handlers for processing domain events
	Handlers: handlers(),

	// Domain-specific tools for this agent
	Tools: domainTools(),

	Factory: CreateMathInstructor,
}
