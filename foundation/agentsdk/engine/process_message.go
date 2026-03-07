package engine

import (
	"context"
	"school-with-ai/shared"
	"school-with-ai/foundation/agentsdk/core"
)

// ProcessUserMessage handles a new user message submission.
func (e *Engine) ProcessUserMessage(ctx context.Context, agentName string, runCtx core.RunContext) error {
	// Determine target agent (sticky session)
	targetAgent, err := e.determineTargetAgent(ctx, runCtx.InitialHistory, agentName)
	if err != nil {
		return err
	}

	// Inject User into context for tools that need it
	if runCtx.UserID != "" {
		ctx = context.WithValue(ctx, shared.UserContextKey, &shared.User{ID: runCtx.UserID})
	}

	return e.executeRun(ctx, targetAgent, runCtx)
}

// executeRun runs the engine and consumes the event iterator.
func (e *Engine) executeRun(ctx context.Context, agentName string, runCtx core.RunContext) error {
	runIter := e.Run(ctx, agentName, runCtx)

	// Consume the run iterator
	for evt, err := range runIter {
		if err != nil {
			// Engine has already emitted RunError via StateError transition
			return err
		}

		if _, ok := evt.(runCompletedEvent); ok {
			continue
		}
		// Event emission is already handled by e.Run using wrapYield
	}

	return nil
}

// determineTargetAgent inspects the history to find the last active agent.
// If an explicit agent is requested, it takes precedence.
// Otherwise, it checks sticky session (last active agent).
// If neither, it defaults to "math-instructor".
func (e *Engine) determineTargetAgent(ctx context.Context, history []core.Message, requestedAgent string) (string, error) {
	// 1. Explicit request takes precedence
	if requestedAgent != "" {
		return requestedAgent, nil
	}

	// 2. Sticky Session: Iterate backwards to find the last assistant message
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Role == "assistant" && msg.AgentName != "" {
			return msg.AgentName, nil
		}
	}

	// 3. No agent determined
	return "", ErrNoAgentSelected
}
