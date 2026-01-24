package engine

import (
	"context"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
)

// ProcessUserMessage handles the full flow of processing a user message:
// 1. Adds the message to the thread.
// 2. Generates/Updates the thread title (if new).
// 3. Runs the agent execution loop.
func (e *Engine) ProcessUserMessage(ctx context.Context, agentName, threadID, content, userID string) error {
	// 1. Add user message or tool result based on Engine State
	messageID := domain.GenerateMessageID()
	runID := domain.GenerateMessageID() // Unique ID for this execution run

	var agg *domain.ThreadAggregate
	var event domain.Event
	var err error

	// Check if this is a tool resumption
	engineState, _ := e.threadMgmt.GetEngineState(ctx, threadID)
	if engineState == "AwaitingInput" {
		pendingToolID, _ := e.threadMgmt.GetPendingInputToolCall(ctx, threadID)
		if pendingToolID != "" {
			// This is an input for a tool
			agg, err = e.threadMgmt.AddToolResultMessage(ctx, threadID, messageID, pendingToolID, content, agentName)
		}
	} else if engineState == "AwaitingAuth" {
		pendingToolID, _ := e.threadMgmt.GetPendingAuthToolCall(ctx, threadID)
		if pendingToolID != "" {
			// This is an auth grant/denial for a tool
			// For now, treat content as "true" (grant) unless explicitly "deny"? 
			// Or just pass it through as a result.
			// Let's assume standard input flow for now, but specialized UI would handle this differently.
			agg, err = e.threadMgmt.AddToolResultMessage(ctx, threadID, messageID, pendingToolID, content, agentName)
		}
	}
	
	if agg == nil {
		// Default: Standard User Message
		agg, event, err = e.threadMgmt.AddUserMessage(ctx, threadID, messageID, content, userID)
	}

	if err != nil {
		return fmt.Errorf("failed to add message: %w", err)
	}

	// 2. State Reconciliation & Elicitation Pairing
	thread := e.threadMgmt.ToAgentThread(agg)
	messages, err := thread.RetrieveFullConversationHistory(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve conversation history: %w", err)
	}

	// 3. Determine Target Agent (Sticky Session)
	targetAgent := e.determineTargetAgent(ctx, messages, agentName)

	titleEvent, err := e.handleThreadTitle(ctx, threadID, content, messages)
	if err != nil {
		e.log.Error(ctx, "failed to handle thread title", "error", err)
	}

	// Inject User into context for tools that need it
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

	// Emit Initial Events
	_ = e.emit(ctx, event, nil)      // MessageAdded (already persisted)
	_ = e.emit(ctx, titleEvent, nil) // TitleUpdated (already persisted)
	_ = e.emit(ctx, events.RunStarted(threadID, runID), nil)

	// Consume the run iterator and emit all events
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
