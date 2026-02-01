package engine

import (
	"context"
	"errors"
	"io"
	"iter"
	"sync"
	"testing"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
	"school-with-ai/foundation/logger"
)

// MockChatClient implements types.ChatClient for testing
type MockChatClient struct {
	streams          [][]llm.ChatResponseUpdate
	mu               sync.Mutex
	err              error
	capturedMessages [][]llm.ChatMessage
}

func (m *MockChatClient) Provider() string {
	return "mock"
}

func (m *MockChatClient) GetStreamingResponse(
	ctx context.Context,
	messages []llm.ChatMessage,
	options *llm.ChatOptions,
) iter.Seq2[llm.ChatResponseUpdate, error] {
	m.mu.Lock()
	m.capturedMessages = append(m.capturedMessages, messages)
	m.mu.Unlock()

	return func(yield func(llm.ChatResponseUpdate, error) bool) {
		m.mu.Lock()
		defer m.mu.Unlock()

		if m.err != nil {
			yield(llm.ChatResponseUpdate{}, m.err)
			return
		}

		if len(m.streams) == 0 {
			return
		}

		// Pop the next stream
		responses := m.streams[0]
		m.streams = m.streams[1:]

		for _, resp := range responses {
			if !yield(resp, nil) {
				return
			}
		}
	}
}

// MockAgentProvider implements AgentProvider for testing
type MockAgentProvider struct {
	configs map[string]core.AgentConfig
}

func (p *MockAgentProvider) GetConfig(ctx context.Context, name string) (core.AgentConfig, error) {
	if config, ok := p.configs[name]; ok {
		return config, nil
	}
	return core.AgentConfig{}, errors.New("config not found") // Just a placeholder error
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
	t.Parallel()
	// Setup
	log := logger.New(io.Discard, logger.LevelInfo, "test", nil)
	eventStore := NewMockEventStore()
	repo := domain.NewThreadRepository(eventStore)
	// Create a thread
	ctx := context.Background()
	threadID := "thread-1"
	// Create thread directly in store
	event := &domain.ThreadCreated{
		ThreadID:  threadID,
		UserID:    "system",
		Title:     "Test Thread",
		Timestamp: time.Now(),
	}
	_, err := eventStore.AppendWithVersion(ctx, domain.StreamName("Thread", threadID), -2, event)
	if err != nil {
		t.Fatalf("Failed to create thread: %v", err)
	}

	// Add user message via manager so event store is populated
	userMsg := &domain.UserMessageAdded{
		ThreadID:  threadID,
		MessageID: "msg-1",
		Content:   "Calculate 2+2",
		Timestamp: time.Now(),
	}
	_, err = eventStore.AppendWithVersion(ctx, domain.StreamName("Thread", threadID), 1, userMsg)
	if err != nil {
		t.Fatalf("Failed to add user message: %v", err)
	}

	chatClient := &MockChatClient{
		streams: [][]llm.ChatResponseUpdate{
			{
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
				{
					Usage: &llm.UsageDetails{
						InputTokens:  10,
						OutputTokens: 20,
						TotalTokens:  30,
					},
				},
			},
		},
	}

	config := core.AgentConfig{
		Name:          "TestAgent",
		MaxIterations: 5,
	}

	provider := &MockAgentProvider{
		configs: map[string]core.AgentConfig{
			"TestAgent": config,
		},
	}

	eng := New(
		log,
		chatClient,
		provider,
		nil,
		nil,
	)

	// Load the thread aggregate to create agent thread
	threadAgg, err := repo.Load(ctx, threadID)
	if err != nil {
		t.Fatalf("Failed to load thread: %v", err)
	}
	agentThread := core.NewInMemoryThreadWithID(threadAgg.ID)
	var messages []core.Message
	for _, dm := range threadAgg.Messages {
		messages = append(messages, core.Message{
			ID:      dm.ID,
			Role:    dm.Role,
			Content: dm.Content,
		})
	}
	_ = agentThread.AddMessages(messages)

	runCtx := core.RunContext{
		ThreadID:  threadID,
		MessageID: "msg-2", // Expected response ID
		Thread:    agentThread,
		Options:   core.RunOptions{},
	}

	// Execute with explicit initial agent
	var events []core.Event
	for event, err := range eng.Run(ctx, "TestAgent", runCtx) {
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		events = append(events, event)
	}

	// Verify

	if len(events) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(events))
	}

	var foundStarted, foundStreamed, foundCompleted bool

	for _, evt := range events {
		switch e := evt.(type) {
		case *domain.AssistantMessageStarted:
			foundStarted = true
		case *domain.AssistantContentStreamed:
			foundStreamed = true
		case *domain.AssistantMessageCompleted:
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

	// Verify yielded events
	foundUsage := false
	for _, evt := range events {
		switch evt.(type) {
		case *domain.TokenUsageReported, domain.TokenUsageReported:
			foundUsage = true
		}
	}
	if !foundUsage {
		t.Error("TokenUsageReported event not found in yielded events")
	}
}
