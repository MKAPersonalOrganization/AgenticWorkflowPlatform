package engine

import (
	"context"
	"fmt"
	"iter"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
)

// ProcessUserMessage handles the full flow of processing a user message:
// 1. Adds the message to the thread.
// 2. Generates/Updates the thread title (if new).
// 3. Runs the agent execution loop.
func (e *Engine) ProcessUserMessage(ctx context.Context, agentName, threadID, content, userID string) (iter.Seq2[core.Event, error], error) {
	// 1. Add user message
	messageID := domain.GenerateMessageID()
	runID := domain.GenerateMessageID() // Unique ID for this execution run

	agg, event, err := e.threadMgmt.AddUserMessage(ctx, threadID, messageID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to add user message: %w", err)
	}

	// 2. State Reconciliation & Elicitation Pairing
	thread := e.threadMgmt.ToAgentThread(agg)
	messages, err := thread.RetrieveFullConversationHistory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve conversation history: %w", err)
	}

	// 3. Determine Target Agent (Sticky Session)
	// We check the conversation history to see which agent was last active.
	// This ensures that if the user is talking to a specialist, they stay with them.
	targetAgent := e.determineTargetAgent(ctx, messages, agentName)

	titleEvent, err := e.handleThreadTitle(ctx, threadID, content, messages)
	if err != nil {
		e.log.Error(ctx, "failed to handle thread title", "error", err)
	}

	// Inject User into context for tools that need it (e.g. MemoryTool)
	// We construct a partial user object with the ID we have.
	if userID != "" {
		ctx = context.WithValue(ctx, domain.UserContextKey, &domain.User{ID: userID})
	}

	runIter := e.Run(ctx, targetAgent, core.RunContext{
		ThreadID:       threadID,
		MessageID:      runID,
		Thread:         thread,
		InitialHistory: messages,
		Options:        core.DefaultRunOptions(),
	})

	// Create event factory for run events
	events := NewEventFactory(agentName)

	// Return combined iterator
	return func(yield func(core.Event, error) bool) {
		// Yield UserMessageAdded
		if !yield(event, nil) {
			return
		}

		// Yield TitleUpdated if any
		if titleEvent != nil {
			if !yield(titleEvent, nil) {
				return
			}
		}

		// Yield RunStarted
		if !yield(events.RunStarted(threadID, runID), nil) {
			return
		}

		// Yield Run events
		// The last event will be an internal runCompletedEvent with termination reason
		var terminationReason TerminationReason
		for evt, err := range runIter {
			if err != nil {
				// Emit RunError
				yield(events.RunError(threadID, err.Error(), "RUN_EXECUTION_ERROR"), nil)
				return // Stop iteration on error
			}

			// Catch the internal completion event (don't forward to frontend)
			if completed, ok := evt.(runCompletedEvent); ok {
				terminationReason = completed.Reason
				continue // Don't yield this internal event
			}

			if !yield(evt, nil) {
				return
			}
		}

		// Only emit RunFinished if the run truly completed (not paused for elicitation)
		if terminationReason != TerminationAwaitingInput {
			yield(events.RunFinished(threadID, runID), nil)
		}
	}, nil
}

// handleThreadTitle generates and updates a thread title if it's the first message.
func (e *Engine) handleThreadTitle(ctx context.Context, threadID, content string, messages []core.Message) (core.Event, error) {
	// Only generate title on the first message
	if len(messages) != 1 {
		return nil, nil
	}

	title := GenerateThreadTitle(content)

	// Update thread title via thread command service (internal threadMgmt)
	event, err := e.threadMgmt.UpdateThreadTitle(ctx, threadID, title)
	if err != nil {
		return nil, fmt.Errorf("failed to update thread title: %w", err)
	}

	return event, nil
}

// determineTargetAgent inspects the history to find the last active agent.
// If found, it returns that agent's name. Otherwise, it returns the defaultAgent.
func (e *Engine) determineTargetAgent(ctx context.Context, history []core.Message, defaultAgent string) string {
	// Iterate backwards to find the last assistant message
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Role == "assistant" && msg.AgentName != "" {
			return msg.AgentName
		}
	}

	return defaultAgent
}
