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

	// 3. Run Agent
	// Note: We shouldn't hardcode "student-facing-agent" here if we want Engine to be generic.
	// However, ProcessUserMessage is currently a specific flow.
	// Ideally, the agent name should be passed in or determined by the thread state?
	// For now, I will accept agentName as parameter?
	// The interface in AgentOrchestrator didn't take agentName.
	// But AgentOrchestrator hardcoded it to "student-facing-agent".
	// Since Engine describes a generic engine, maybe ProcessUserMessage should take agentName?
	// Let's check the AgentOrchestrator implementation. It hardcoded "student-facing-agent".
	// The user request said "the processuser message function should go to the engine!".
	// If I modify signature, I break the interface in AgentOrchestrator service layer if I want to expose it directly?
	// Or AgentOrchestrator delegates to Engine.ProcessUserMessage(..., agentName).
	// Yes. I will add agentName parameter to Engine.ProcessUserMessage.

	runIter := e.Run(ctx, agentName, core.RunContext{
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
				runErr = err
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

	title := e.titleGenerator.FromFirstMessage(content)

	// Update thread title via thread command service (internal threadMgmt)
	event, err := e.threadMgmt.UpdateThreadTitle(ctx, thread.GetID(), title)
	if err != nil {
		return nil, fmt.Errorf("failed to update thread title: %w", err)
	}

	return event, nil
}
