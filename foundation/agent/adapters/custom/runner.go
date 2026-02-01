package custom

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/registry"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator"
)

// RunnerAdapter adapts agentorchestrator.Service to the agent.Runner interface.
type RunnerAdapter struct {
	log     *logger.Logger
	service *agentorchestrator.Service
}

// NewRunnerAdapter creates a runner adapter from an orchestrator service.
func NewRunnerAdapter(log *logger.Logger, service *agentorchestrator.Service) agent.Runner {
	return &RunnerAdapter{
		log:     log,
		service: service,
	}
}

// GetService returns the underlying orchestrator service.
func (r *RunnerAdapter) GetService() *agentorchestrator.Service {
	return r.service
}

// Run executes an agent with the given request.
func (r *RunnerAdapter) Run(ctx context.Context, req *agent.RunRequest) iter.Seq2[agent.Event, error] {
	return func(yield func(agent.Event, error) bool) {
		// Load thread and history
		threadMgmt := r.service.GetThreadManager()
		agg, err := threadMgmt.Load(ctx, req.SessionID)
		if err != nil && !errors.Is(err, domain.ErrThreadNotFound) {
			yield(nil, fmt.Errorf("failed to load thread: %w", err))
			return
		}

		var thread core.AgentThread
		var messages []core.Message

		if agg != nil {
			thread = threadMgmt.ToAgentThread(agg)
			var hErr error
			messages, hErr = thread.RetrieveFullConversationHistory(ctx)
			if hErr != nil {
				yield(nil, fmt.Errorf("failed to retrieve history: %w", hErr))
				return
			}
		} else {
			thread = core.NewInMemoryThreadWithID(req.SessionID)
			messages = []core.Message{}
		}

		// Create RunContext
		runCtx := core.RunContext{
			ThreadID:       req.SessionID,
			UserID:         req.UserID,
			UserContent:    extractUserPrompt(req.Message),
			Thread:         thread,
			InitialHistory: messages,
		}

		if req.Message != nil && req.Message.ID != "" {
			runCtx.MessageID = req.Message.ID
		} else {
			runCtx.MessageID = core.GenerateID()
		}

		// Apply configuration if provided
		if req.Config != nil {
			runCtx.Options = core.RunOptions{
				Temperature:   &req.Config.Temperature,
				MaxTokens:     &req.Config.MaxTokens,
				ModelID:       req.Config.ModelID,
				StopSequences: req.Config.StopSequences,
			}
		}

		// Run the agent and convert events
		for event, err := range r.service.Run(ctx, req.AgentName, runCtx) {
			if err != nil {
				yield(nil, err)
				continue
			}
			// Convert core.Event to agent.Event
			agentEvent := ConvertCoreEventToAgent(event)
			if agentEvent != nil {
				if !yield(agentEvent, nil) {
					return
				}
			}
		}
	}
}

// GetAgent returns an agent by name.
func (r *RunnerAdapter) GetAgent(name string) (agent.Agent, error) {
	factory, ok := registry.GlobalAgents.Get(name)
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", name)
	}

	// Create the agent config using the factory
	config := factory(context.Background(), nil)

	return NewAgentAdapter(config), nil
}

// ListAgents returns all available agent names.
func (r *RunnerAdapter) ListAgents() []string {
	return registry.GlobalAgents.List()
}

// extractUserPrompt extracts the text from a message.
func extractUserPrompt(msg *agent.Message) string {
	if msg == nil {
		return ""
	}

	for _, c := range msg.Content {
		if c.Type == agent.ContentTypeText {
			return c.Text
		}
	}

	return ""
}
