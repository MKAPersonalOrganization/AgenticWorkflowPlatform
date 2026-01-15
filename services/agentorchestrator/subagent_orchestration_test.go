package agentorchestrator

import (
	"context"
	"io"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/foundation/agentsdk/llm"
	"school-with-ai/foundation/logger"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockChatClient struct {
	mock.Mock
}

func (m *mockChatClient) GetStreamingResponse(ctx context.Context, messages []llm.ChatMessage, options *llm.ChatOptions) iter.Seq2[llm.ChatResponseUpdate, error] {
	args := m.Called(ctx, messages, options)
	return args.Get(0).(iter.Seq2[llm.ChatResponseUpdate, error])
}

type mockEventStore struct {
	mock.Mock
}

func (m *mockEventStore) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...core.Event) (int64, error) {
	// Need to spread the args carefully because of variadic 'events'
	allArgs := []any{ctx, streamID, expectedVersion}
	for _, e := range events {
		allArgs = append(allArgs, e)
	}
	args := m.Called(ctx, streamID, expectedVersion, events)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]core.Event, error) {
	args := m.Called(ctx, streamID, fromVersion)
	return args.Get(0).([]core.Event), args.Error(1)
}

func (m *mockEventStore) DeleteStream(ctx context.Context, streamID string) error {
	args := m.Called(ctx, streamID)
	return args.Error(0)
}

func TestSubagentOrchestration(t *testing.T) {
	t.Parallel()
	chatClient := new(mockChatClient)
	eventStore := new(mockEventStore)

	// Define subagent
	subAgentConfig := core.AgentConfig{
		Name:         "weather_specialist",
		SystemPrompt: "You are a weather specialist.",
	}

	// Define main agent
	mainAgentConfig := core.AgentConfig{
		Name:         "main_assistant",
		SystemPrompt: "You are a main assistant. Delegate weather questions to the specialist.",
		SubAgents:    []core.AgentConfig{subAgentConfig},
		Middleware: []core.Middleware{
			middleware.NewSubagentMiddleware(),
		},
	}

	service := New(logger.New(io.Discard, logger.LevelInfo, "test", nil), chatClient, eventStore, WithConfig(mainAgentConfig))

	ctx := context.Background()
	threadID := "test-thread"
	messageID := "user-msg-1"

	// Subagent response
	subIter := func(yield func(llm.ChatResponseUpdate, error) bool) {
		yield(llm.ChatResponseUpdate{
			Role:     llm.ChatRoleAssistant,
			Contents: []llm.AIContent{llm.TextContent{Text: "The weather in London is sunny."}},
		}, nil)
		yield(llm.ChatResponseUpdate{
			Role:         llm.ChatRoleAssistant,
			FinishReason: llm.FinishReasonStop,
		}, nil)
	}

	// Main agent response 1: Tool call
	mainIter1 := func(yield func(llm.ChatResponseUpdate, error) bool) {
		yield(llm.ChatResponseUpdate{
			Role:          llm.ChatRoleAssistant,
			ToolCallID:    "call_1",
			ToolCallIndex: 0,
			Contents: []llm.AIContent{llm.FunctionCallContent{
				CallID:       "call_1",
				Name:         "weather_specialist",
				RawArguments: `{"question": "weather in London"}`,
			}},
		}, nil)
		yield(llm.ChatResponseUpdate{
			Role:         llm.ChatRoleAssistant,
			FinishReason: llm.FinishReasonToolCalls,
		}, nil)
	}

	// Main agent response 2: Final answer
	mainIter2 := func(yield func(llm.ChatResponseUpdate, error) bool) {
		yield(llm.ChatResponseUpdate{
			Role:     llm.ChatRoleAssistant,
			Contents: []llm.AIContent{llm.TextContent{Text: "The specialist says it is sunny in London."}},
		}, nil)
		yield(llm.ChatResponseUpdate{
			Role:         llm.ChatRoleAssistant,
			FinishReason: llm.FinishReasonStop,
		}, nil)
	}

	// Expectations
	chatClient.On("GetStreamingResponse", mock.Anything, mock.Anything, mock.MatchedBy(func(o *llm.ChatOptions) bool {
		return o.Instructions == "You are a weather specialist."
	})).Return(iter.Seq2[llm.ChatResponseUpdate, error](subIter))

	chatClient.On("GetStreamingResponse", mock.Anything, mock.Anything, mock.MatchedBy(func(o *llm.ChatOptions) bool {
		return o.Instructions == mainAgentConfig.SystemPrompt
	})).Return(iter.Seq2[llm.ChatResponseUpdate, error](mainIter1)).Once()

	chatClient.On("GetStreamingResponse", mock.Anything, mock.Anything, mock.MatchedBy(func(o *llm.ChatOptions) bool {
		return o.Instructions == mainAgentConfig.SystemPrompt
	})).Return(iter.Seq2[llm.ChatResponseUpdate, error](mainIter2)).Once()

	streamID := "Thread-" + threadID

	// Mocking event store behaviors
	// When AddUserMessage is called, it loads the thread.
	// Since Load uses ReadStream, we mock it.
	eventStore.On("ReadStream", mock.Anything, streamID, int64(0)).Return([]core.Event{
		domain.ThreadCreated{
			ThreadID:  threadID,
			UserID:    "system",
			Title:     "New Chat",
			Timestamp: time.Now(),
		},
	}, nil)

	// When AddUserMessage saves the message (Save)
	eventStore.On("AppendWithVersion", mock.Anything, streamID, int64(1), mock.Anything).Return(int64(2), nil)

	// When Engine processes and calls AddAssistantMessage (multiple times)
	eventStore.On("AppendWithVersion", mock.Anything, streamID, mock.Anything, mock.Anything).Return(int64(3), nil)

	// Initial user message
	thread, err := service.AddUserMessage(ctx, threadID, messageID, "What is the weather in London?")
	assert.NoError(t, err)

	runCtx := core.RunContext{
		ThreadID:  threadID,
		MessageID: "run-msg-id",
		Thread:    thread,
	}

	events := service.Run(ctx, runCtx)
	var finalContent string
	for event, err := range events {
		assert.NoError(t, err)
		if completed, ok := event.(*domain.AssistantMessageCompletedEvent); ok {
			finalContent = completed.Content
		}
	}

	assert.Equal(t, "The specialist says it is sunny in London.", finalContent)
	chatClient.AssertExpectations(t)
}
