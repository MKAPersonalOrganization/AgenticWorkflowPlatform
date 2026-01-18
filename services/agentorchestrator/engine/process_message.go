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

	// 2. Handle Title
	thread := e.threadMgmt.ToAgentThread(agg)
	titleEvent, err := e.handleThreadTitle(ctx, thread, content)
	if err != nil {
		e.log.Error(ctx, "failed to handle thread title", "error", err)
	}

	// 3. Determine Target Agent (Sticky Session)
	// We check the conversation history to see which agent was last active.
	// This ensures that if the user is talking to a specialist, they stay with them.
	targetAgent := e.determineTargetAgent(ctx, thread, agentName)

	// Inject User into context for tools that need it (e.g. MemoryTool)
	// We construct a partial user object with the ID we have.
	if userID != "" {
		ctx = context.WithValue(ctx, domain.UserContextKey, &domain.User{ID: userID})
	}

	runIter := e.Run(ctx, targetAgent, core.RunContext{
		ThreadID:  threadID,
		MessageID: runID,
		Thread:    thread,
		Options:   core.DefaultRunOptions(),
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
		var runErr error
		for evt, err := range runIter {
			if err != nil {
				// Emit RunError
				yield(events.RunError(threadID, err.Error(), "RUN_EXECUTION_ERROR"), nil)
				return // Stop iteration on error
			}
			if !yield(evt, nil) {
				return
			}
		}

		if runErr == nil {
			// Yield RunFinished
			yield(events.RunFinished(threadID, runID), nil)
		}
	}, nil
}

// handleThreadTitle generates and updates a thread title if it's the first message.
func (e *Engine) handleThreadTitle(ctx context.Context, thread core.AgentThread, content string) (core.Event, error) {
	msgs, err := thread.RetrieveFullConversationHistory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve conversation history: %w", err)
	}

	// Only generate title on the first message
	if len(msgs) != 1 {
		return nil, nil
	}

	title := GenerateThreadTitle(content)

	// Update thread title via thread command service (internal threadMgmt)
	event, err := e.threadMgmt.UpdateThreadTitle(ctx, thread.GetID(), title)
	if err != nil {
		return nil, fmt.Errorf("failed to update thread title: %w", err)
	}

	return event, nil
}

// determineTargetAgent inspects the thread history to find the last active agent.
// If found, it returns that agent's name. Otherwise, it returns the defaultAgent.
func (e *Engine) determineTargetAgent(ctx context.Context, thread core.AgentThread, defaultAgent string) string {
	history, err := thread.RetrieveFullConversationHistory(ctx)
	if err != nil {
		e.log.Warn(ctx, "failed to retrieve history for sticky session check", "error", err)
		return defaultAgent
	}

	// Iterate backwards to find the last assistant message
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Role == "assistant" && msg.AgentName != "" {
			return msg.AgentName
		}
	}

	return defaultAgent
}
