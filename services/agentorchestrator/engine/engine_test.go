package engine

import (
	"context"
	"io"
	"iter"
	"sync"
	"testing"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/threadmgmt"
)

// MockChatClient implements types.ChatClient for testing
type MockChatClient struct {
	responses []llm.ChatResponseUpdate
	err       error
}

func (m *MockChatClient) GetStreamingResponse(
	ctx context.Context,
	messages []llm.ChatMessage,
	options *llm.ChatOptions,
) iter.Seq2[llm.ChatResponseUpdate, error] {
	return func(yield func(llm.ChatResponseUpdate, error) bool) {
		if m.err != nil {
			yield(llm.ChatResponseUpdate{}, m.err)
			return
		}
		for _, resp := range m.responses {
			if !yield(resp, nil) {
				return
			}
		}
	}
}

// MockEventStore implements domain.EventStore
type MockEventStore struct {
	mu      sync.Mutex
	events  map[string][]domain.Event
	streams map[string]int64
}

func NewMockEventStore() *MockEventStore {
	return &MockEventStore{
		events:  make(map[string][]domain.Event),
		streams: make(map[string]int64),
	}
}

func (m *MockEventStore) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...domain.Event) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentVersion, exists := m.streams[streamID]
	
	// Check concurrency
	if expectedVersion == -2 { // NoStream
		if exists {
			return 0, core.ErrConcurrencyConflict
		}
		currentVersion = 0
	} else if expectedVersion != -1 { // Any
		if !exists && expectedVersion != 0 {
             if expectedVersion != 0 {
                  return 0, core.ErrConcurrencyConflict
             }
             currentVersion = 0
		} else if exists && currentVersion != expectedVersion {
			return 0, core.ErrConcurrencyConflict
		}
	}

	if !exists {
		m.streams[streamID] = 0
		m.events[streamID] = []domain.Event{}
	}

	for range events {
		currentVersion++
	}
	
	m.streams[streamID] = currentVersion
	m.events[streamID] = append(m.events[streamID], events...)

	return currentVersion, nil
}

func (m *MockEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]domain.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	events, exists := m.events[streamID]
	if !exists {
		return []domain.Event{}, nil
	}

	return events, nil
}

func (m *MockEventStore) DeleteStream(ctx context.Context, streamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.events, streamID)
	delete(m.streams, streamID)
	return nil
}

func TestEngine_Run(t *testing.T) {
	// Setup
	log := logger.New(io.Discard, logger.LevelInfo, "test", nil)
	eventStore := NewMockEventStore()
	repo := domain.NewThreadRepository(eventStore)
	tm := threadmgmt.New(log, repo, "TestAgent")
	
	// Create a thread
	ctx := context.Background()
	threadID := "thread-1"
	_, _, err := tm.CreateThread(ctx, threadID, "Test Thread")
	if err != nil {
		t.Fatalf("Failed to create thread: %v", err)
	}

    // Add user message via manager so event store is populated
    _, _, err = tm.AddUserMessage(ctx, threadID, "msg-1", "Hello")
    if err != nil {
        t.Fatalf("Failed to add user message: %v", err)
    }

	mockChatClient := &MockChatClient{
		responses: []llm.ChatResponseUpdate{
			{
				Contents: []llm.AIContent{
					llm.TextContent{Text: "Hello "},
				},
			},
			{
				Contents: []llm.AIContent{
					llm.TextContent{Text: "World"},
				},
				FinishReason: "stop",
			},
		},
	}

	config := core.AgentConfig{
		Name: "TestAgent",
	}

	eng := New(
		log,
		mockChatClient,
		tm,
		config,
		nil,
		nil, // toolTracer
	)

    // Load the thread aggregate to create agent thread
    threadAgg, err := repo.Load(ctx, threadID)
    if err != nil {
        t.Fatalf("Failed to load thread: %v", err)
    }
    agentThread := tm.ToAgentThread(threadAgg)

	runCtx := core.RunContext{
		ThreadID:  threadID,
		MessageID: "msg-2", // Expected response ID
		Thread:    agentThread,
		Options:   core.RunOptions{},
	}

	// Execute
	var events []core.Event
	for event, err := range eng.Run(ctx, runCtx) {
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		events = append(events, event)
	}

	// Verify
	// Expected events: AssistantStarted, AssistantContentStreamed (x2), AssistantCompleted
	// Note: Engine emits AssistantStarted, AssistantCompleted. 
	// StreamProcessor emits AssistantContentStreamed.
	
	if len(events) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(events))
	}

    var foundStarted, foundStreamed, foundCompleted bool
    
    for _, evt := range events {
        switch e := evt.(type) {
        case *domain.AssistantMessageStartedEvent:
            foundStarted = true
        case *domain.AssistantContentStreamedEvent:
            foundStreamed = true
        case *domain.AssistantMessageCompletedEvent:
            foundCompleted = true
            if e.Content != "Hello World" {
                t.Errorf("Expected content 'Hello World', got '%s'", e.Content)
            }
        }
    }

    if !foundStarted {
        t.Error("Did not find AssistantStartedEvent")
    }
    if !foundStreamed {
        t.Error("Did not find AssistantContentStreamedEvent")
    }
    if !foundCompleted {
        t.Error("Did not find AssistantCompletedEvent")
    }
}
