package physics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents"
	"school-with-ai/services/chatwrite/agents/physics/tools"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
)

func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
		sharedtools.NewUnitConversionTool(),
		tools.NewWeatherTool(),
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

// Contract defines the event-driven contract for the physics instructor.
var Contract = &eventbus.AgentContract{
	Name: "physics-instructor",

	ListensTo: []agents.EventType{
		agents.EventTypeAgentHandoff,
	},

	Handlers: handlers(),
	Tools:    domainTools(),

	Factory: CreatePhysicsInstructor,
}
