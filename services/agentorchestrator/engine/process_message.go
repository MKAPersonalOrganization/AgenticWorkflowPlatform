package engine

import (
	"context"
	"errors"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
)

// ProcessUserMessage handles a new user message submission.
// The engine state machine handles message persistence and event emission.
func (e *Engine) ProcessUserMessage(ctx context.Context, agentName, threadID, content, userID string) error {
	runID := domain.GenerateMessageID()

	// Load thread for history (may not exist yet for new threads)
	agg, err := e.threadMgmt.Load(ctx, threadID)
	if err != nil && !errors.Is(err, domain.ErrThreadNotFound) {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	// Get conversation history (empty for new threads)
	var thread core.AgentThread
	var messages []core.Message

	if agg != nil {
		thread = e.threadMgmt.ToAgentThread(agg)
		messages, err = thread.RetrieveFullConversationHistory(ctx)
		if err != nil {
			return fmt.Errorf("failed to retrieve conversation history: %w", err)
		}
	} else {
		// New thread - empty history
		thread = core.NewInMemoryThreadWithID(threadID)
		messages = []core.Message{}
	}

	// Determine target agent (sticky session)
	targetAgent := e.determineTargetAgent(ctx, messages, agentName)

	// Inject User into context for tools that need it
	if userID != "" {
		ctx = context.WithValue(ctx, domain.UserContextKey, &domain.User{ID: userID})
	}

	// Prepare RunContext with user message
	// The engine state machine will persist the message
	runCtx := core.RunContext{
		ThreadID:       threadID,
		MessageID:      runID,
		Thread:         thread,
		InitialHistory: messages,
		Options:        core.DefaultRunOptions(),
		UserContent:    content,
		UserID:         userID,
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
func (e *Engine) determineTargetAgent(ctx context.Context, history []core.Message, requestedAgent string) string {
	// 1. Explicit request takes precedence
	if requestedAgent != "" {
		return requestedAgent
	}

	// 2. Sticky Session: Iterate backwards to find the last assistant message
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Role == "assistant" && msg.AgentName != "" {
			return msg.AgentName
		}
	}

	// 3. Default fallback
	return "math-instructor"
}
