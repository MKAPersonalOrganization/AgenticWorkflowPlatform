package domain_test

import (
	"context"
	"io"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/chat/threadmgmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockEventStore implements domain.EventStore for testing
type MockEventStore struct {
	events map[string][]domain.Event
}

func NewMockEventStore() *MockEventStore {
	return &MockEventStore{
		events: make(map[string][]domain.Event),
	}
}

func (m *MockEventStore) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...domain.Event) (int64, error) {
	m.events[streamID] = append(m.events[streamID], events...)
	return int64(len(m.events[streamID])), nil
}

func (m *MockEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]domain.Event, error) {
	return m.events[streamID], nil
}

func (m *MockEventStore) DeleteStream(ctx context.Context, streamID string) error {
	delete(m.events, streamID)
	return nil
}

func TestThreadAggregate_RecordToolEvent(t *testing.T) {
	// Setup
	threadID := "thread-123"
	userID := "user-456"
	thread, err := domain.OpenThread(threadID, userID, "Test Thread")
	require.NoError(t, err)

	// Create a dummy tool event
	event := &domain.ToolNeedsInput{
		ThreadID:   threadID,
		MessageID:  "msg-789",
		ToolCallID: "call-abc",
		ToolName:   "test-tool",
		Elicitation: &domain.ElicitationSpec{
			Question: "Please provide input",
		},
		AgentName: "test-agent",
		Timestamp: time.Now(),
	}

	// Act
	err = thread.RecordToolEvent(event)

	// Assert
	assert.NoError(t, err)
}

func TestManager_RecordToolEvent(t *testing.T) {
	// Setup in-memory event store
	eventStore := NewMockEventStore()
	logger := logger.New(io.Discard, logger.LevelDebug, "test", nil)
	repo := domain.NewThreadRepository(threadmgmt.NewEventStoreAdapter(eventStore))
	manager := threadmgmt.New(logger, repo)
	ctx := context.Background()

	// Create thread
	threadID := "thread-manager-test"
	_, _, err := manager.CreateThread(ctx, threadID, "user-setup", "Test Thread")
	require.NoError(t, err)

	// Create event
	event := &domain.ToolNeedsInput{
		ThreadID:   threadID,
		MessageID:  "msg-111",
		ToolCallID: "call-222",
		ToolName:   "my-tool",
		Elicitation: &domain.ElicitationSpec{
			Question: "Need input",
		},
		AgentName: "agent-x",
		Timestamp: time.Now(),
	}

	// Load thread and record event
	thread, err := repo.Load(ctx, threadID)
	require.NoError(t, err)

	err = thread.RecordToolEvent(event)
	require.NoError(t, err)

	// Save thread
	err = repo.Save(ctx, thread)
	require.NoError(t, err)

	// Verify event is in store
	found := false
	for _, streamEvents := range eventStore.events {
		for _, e := range streamEvents {
			if te, ok := e.(*domain.ToolNeedsInput); ok {
				if te.ToolCallID == "call-222" {
					found = true
					break
				}
			}
		}
	}
	assert.True(t, found, "Expected ToolNeedsInput to be persisted")
}
