package engine

import (
	"context"
	"io"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/foundation/agentsdk/llm"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/threadmgmt"
	"testing"
)

func TestEngine_ElicitationFlow(t *testing.T) {
	t.Parallel()
	// Setup
	log := logger.New(io.Discard, logger.LevelInfo, "test", nil)
	eventStore := NewMockEventStore()
	repo := domain.NewThreadRepository(eventStore)
	tm := threadmgmt.New(log, repo)

	ctx := context.Background()
	threadID := "thread-elicitation"
	_, _, _ = tm.CreateThread(ctx, threadID, "Elicitation Thread")

	// Mock Chat Client
	mockClient := &MockChatClient{
		streams: [][]llm.ChatResponseUpdate{
			// Turn 1: Assistant calls quiz tool with domain-specific parameters
			{
				{
					Contents: []llm.AIContent{
						llm.TextContent{Text: "I need input. "},
						llm.FunctionCallContent{
							CallID:       "call_1",
							Name:         "quiz_numeric_scale",
							RawArguments: `{"question_text": "How much do you like blue?", "scale_minimum": 1, "scale_maximum": 10}`,
						},
					},
					FinishReason: "tool_calls",
				},
			},
			// Turn 2: Assistant responds to the input
			{
				{
					Contents: []llm.AIContent{
						llm.TextContent{Text: "Thanks! I see you like blue."},
					},
					FinishReason: "stop",
				},
			},
		},
	}

	agentConfig := core.AgentConfig{
		Name:          "test-elicitor",
		MaxIterations: 5,
		Handoffs:      []core.Handoff{},
		Middleware: []core.Middleware{
			middleware.NewElicitationMiddleware(),
		},
	}

	provider := &MockAgentProvider{
		configs: map[string]core.AgentConfig{
			"test-elicitor": agentConfig,
		},
	}

	eng := New(
		log,
		mockClient,
		tm,
		provider,
		nil,
		nil,
	)

	userID := "user1"

	// 1. Initial User Message
	seq, err := eng.ProcessUserMessage(ctx, "test-elicitor", threadID, "Hello", userID)
	if err != nil {
		t.Fatalf("ProcessUserMessage 1 failed: %v", err)
	}
	for _, err := range seq {
		if err != nil {
			t.Fatalf("Run 1 iteration failed: %v", err)
		}
	}

	// Verify events: Should see elicitation tool call
	threadAgg, err := repo.Load(ctx, threadID)
	if err != nil {
		t.Fatalf("Failed to load thread: %v", err)
	}
	thread := tm.ToAgentThread(threadAgg)

	// Verify conversation flow
	msgs, _ := thread.RetrieveFullConversationHistory(ctx)

	// Expect assistant message with tool call (second to last, as tool result is last)
	// msgs[0]=User, msgs[1]=Assistant, msgs[2]=Tool
	if len(msgs) < 3 {
		t.Fatalf("Expected at least 3 messages, got %d", len(msgs))
	}

	asstMsg := msgs[len(msgs)-2]
	if asstMsg.Role != "assistant" {
		t.Errorf("Expected assistant message, got %s", asstMsg.Role)
	}
	if len(asstMsg.ToolCalls) == 0 || asstMsg.ToolCalls[0].Function.Name != middleware.ToolNameNumericRange {
		t.Errorf("Expected %s tool call", middleware.ToolNameNumericRange)
	}

	// 2. User Response (Answering the elicitation)
	// ProcessUserMessage should detect the pending elicitation and pair it
	seq, err = eng.ProcessUserMessage(ctx, "test-elicitor", threadID, "Blue", userID)
	if err != nil {
		t.Fatalf("ProcessUserMessage 2 failed: %v", err)
	}

	t.Log("--- Starting Run 2 ---")
	for event, err := range seq {
		if err != nil {
			t.Fatalf("Run 2 iteration failed: %v", err)
		}
		t.Logf("Event: %T", event)
	}
	t.Log("--- Finished Run 2 ---")

	// Reload thread to get latest state
	threadAgg, err = repo.Load(ctx, threadID)
	if err != nil {
		t.Fatalf("Failed to reload thread: %v", err)
	}
	thread = tm.ToAgentThread(threadAgg)

	// Verify conversation flow
	msgs, _ = thread.RetrieveFullConversationHistory(ctx)

	// We expect:
	// 1. User: Hello
	// 2. Assistant: (elicit_user_input)
	// 3. Tool: (Status update from elicit tool)
	// 4. User: Blue (This acts as the tool result/answer)
	// 5. Assistant: Thanks! I see you like blue.

	if len(msgs) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(msgs))
		for i, m := range msgs {
			t.Logf("Msg %d: %s (%s)", i, m.Role, m.Content)
		}
	}

	finalMsg := msgs[len(msgs)-1]
	if finalMsg.Role != "assistant" || finalMsg.Content != "Thanks! I see you like blue." {
		t.Errorf("Unexpected final message: %+v", finalMsg)
	}
}
