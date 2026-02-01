package engine

import (
	"context"
	"encoding/json"
	"testing"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/foundation/agentsdk/llm"
)

func TestEngine_ElicitationFlow(t *testing.T) {
	t.Parallel()
	// Setup
	ctx := context.Background()
	threadID := "thread-elicitation"

	// Mock Chat Client
	mockClient := &MockChatClient{
		streams: [][]llm.ChatResponseUpdate{
			// Turn 1: Assistant calls quiz tool
			{
				{
					Contents: []llm.AIContent{
						llm.TextContent{Text: "I need input. "},
						llm.FunctionCallContent{
							ToolCallID:   "call_1",
							Name:         "quiz_yes_no",
							RawArguments: `{"question": "Is the sky blue?"}`,
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
		Middleware: []core.Middleware{
			middleware.NewElicitationMiddleware(),
		},
	}

	provider := &MockAgentProvider{
		configs: map[string]core.AgentConfig{
			"test-elicitor": agentConfig,
		},
	}

	eng := New(mockClient, provider, nil, nil)
	userID := "user1"

	// 1. Initial User Message
	runCtx1 := core.RunContext{
		ThreadID:       threadID,
		MessageID:      core.GenerateID(),
		Thread:         core.NewInMemoryThreadWithID(threadID),
		InitialHistory: []core.Message{},
		Options:        core.DefaultRunOptions(),
		UserContent:    "Hello",
		UserID:         userID,
	}

	var events1 []core.Event
	for evt, err := range eng.Run(ctx, "test-elicitor", runCtx1) {
		if err != nil {
			t.Fatalf("Run 1 failed: %v", err)
		}
		events1 = append(events1, evt)
	}

	// Build history from events for next step
	var history []core.Message
	for _, evt := range events1 {
		switch e := evt.(type) {
		case *core.UserMessageAdded:
			history = append(history, core.Message{ID: e.MessageID, Role: "user", Content: e.Content})
		case *core.AssistantMessageAdded:
			history = append(history, core.Message{ID: e.MessageID, Role: "assistant", Content: e.Content, ToolCalls: e.ToolCalls})
		case *core.ToolResultAdded:
			history = append(history, core.Message{ID: e.MessageID, Role: "tool", Content: e.Result, ToolCallID: e.ToolCallID})
		}
	}

	// Verify turn 1
	if len(history) < 3 {
		t.Fatalf("Expected at least 3 messages (User, Assistant, Tool Result), got %d", len(history))
	}
	asstMsg := history[1]
	if len(asstMsg.ToolCalls) == 0 || asstMsg.ToolCalls[0].Name != middleware.ToolNameTrueFalse {
		t.Errorf("Expected %s tool call, got %+v", middleware.ToolNameTrueFalse, asstMsg)
	}

	// 2. User Response (Answering the elicitation)
	runCtx2 := core.RunContext{
		ThreadID:       threadID,
		MessageID:      core.GenerateID(),
		Thread:         core.NewInMemoryThreadWithID(threadID),
		InitialHistory: history,
		Options:        core.DefaultRunOptions(),
		UserContent:    "Blue",
		UserID:         userID,
	}

	var events2 []core.Event
	for evt, err := range eng.Run(ctx, "test-elicitor", runCtx2) {
		if err != nil {
			t.Fatalf("Run 2 failed: %v", err)
		}
		events2 = append(events2, evt)
	}

	// Verify turn 2
	foundFinalAssistant := false
	for _, evt := range events2 {
		if e, ok := evt.(*core.AssistantMessageAdded); ok {
			if e.Content == "Thanks! I see you like blue." {
				foundFinalAssistant = true
			}
		}
	}

	if !foundFinalAssistant {
		t.Error("Did not find the final assistant response answering the elicitation")
	}
}

func TestEngine_ElicitationRobustness(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	threadID := "thread-robustness"

	mockClient := &MockChatClient{
		streams: [][]llm.ChatResponseUpdate{
			{
				{
					Contents: []llm.AIContent{
						llm.FunctionCallContent{
							ToolCallID:   "call_1",
							Name:         "quiz_yes_no",
							RawArguments: `{"statement": "Is 2+2=4?"}`, // Using "statement" alias
						},
					},
					FinishReason: "tool_calls",
				},
			},
		},
	}

	eng := New(mockClient, &MockAgentProvider{
		configs: map[string]core.AgentConfig{
			"test-robust": {
				Name:          "test-robust",
				MaxIterations: 5,
				Middleware:    []core.Middleware{middleware.NewElicitationMiddleware()},
			},
		},
	}, nil, nil)

	runCtx := core.RunContext{
		ThreadID:       threadID,
		MessageID:      core.GenerateID(),
		Thread:         core.NewInMemoryThreadWithID(threadID),
		InitialHistory: []core.Message{},
		Options:        core.DefaultRunOptions(),
		UserContent:    "Start",
		UserID:         "user1",
	}

	var events []core.Event
	for evt, err := range eng.Run(ctx, "test-robust", runCtx) {
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		events = append(events, evt)
	}

	foundSuccess := false
	for _, evt := range events {
		if e, ok := evt.(*core.ToolResultAdded); ok {
			var result struct {
				Status string `json:"status"`
			}
			json.Unmarshal([]byte(e.Result), &result)
			if result.Status == "Requesting input: Is 2+2=4?" {
				foundSuccess = true
				break
			}
		}
	}

	if !foundSuccess {
		t.Error("Expected successful tool execution with 'statement' alias")
	}
}

func TestEngine_ElicitationOptionsRobustness(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	threadID := "thread-options-robustness"

	mockClient := &MockChatClient{
		streams: [][]llm.ChatResponseUpdate{
			{
				{
					Contents: []llm.AIContent{
						llm.FunctionCallContent{
							ToolCallID:   "call_1",
							Name:         "quiz_multiple_choice",
							RawArguments: `{"question": "What is 2+3?", "options": "4, 5, 6"}`, // String options
						},
					},
					FinishReason: "tool_calls",
				},
			},
		},
	}

	eng := New(mockClient, &MockAgentProvider{
		configs: map[string]core.AgentConfig{
			"test-robust-opts": {
				Name:          "test-robust-opts",
				MaxIterations: 5,
				Middleware:    []core.Middleware{middleware.NewElicitationMiddleware()},
			},
		},
	}, nil, nil)

	runCtx := core.RunContext{
		ThreadID:       threadID,
		MessageID:      core.GenerateID(),
		Thread:         core.NewInMemoryThreadWithID(threadID),
		InitialHistory: []core.Message{},
		Options:        core.DefaultRunOptions(),
		UserContent:    "Start",
		UserID:         "user1",
	}

	var events []core.Event
	for evt, err := range eng.Run(ctx, "test-robust-opts", runCtx) {
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		events = append(events, evt)
	}

	foundSuccess := false
	for _, evt := range events {
		if e, ok := evt.(*core.ToolResultAdded); ok {
			var result struct {
				Status string `json:"status"`
			}
			json.Unmarshal([]byte(e.Result), &result)
			if result.Status == "Requesting input: What is 2+3?" {
				foundSuccess = true
				break
			}
		}
	}

	if !foundSuccess {
		t.Error("Expected successful tool execution with string 'options'")
	}
}
