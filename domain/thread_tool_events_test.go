package domain_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/threadmgmt"
)

// MockEventStore implements core.EventStore for testing
type MockEventStore struct {
	events map[string][]core.Event
}

func NewMockEventStore() *MockEventStore {
	return &MockEventStore{
		events: make(map[string][]core.Event),
	}
}

func (m *MockEventStore) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...core.Event) (int64, error) {
	m.events[streamID] = append(m.events[streamID], events...)
	return int64(len(m.events[streamID])), nil
}

func (m *MockEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]core.Event, error) {
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
	event := &domain.ToolAwaitingInputEvent{
		ThreadID:   threadID,
		MessageID:  "msg-789",
		ToolCallID: "call-abc",
		ToolName:   "test-tool",
		Elicitation: &core.ElicitationSpec{
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
	_, _, err := manager.CreateThread(ctx, threadID, "Test Thread")
	require.NoError(t, err)

	// Create event
	event := &domain.ToolAwaitingInputEvent{
		ThreadID:   threadID,
		MessageID:  "msg-111",
		ToolCallID: "call-222",
		ToolName:   "my-tool",
		Elicitation: &core.ElicitationSpec{
			Question: "Need input",
		},
		AgentName: "agent-x",
		Timestamp: time.Now(),
	}

	// Act
	updatedThread, err := manager.RecordToolEvent(ctx, threadID, event)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, threadID, updatedThread.ID)

	// Verify event is in store
	// events, err := eventStore.ReadStream(ctx, "Thread-" + threadID, 0)
	// We check the mock internal state directly instead of relying on ReadStream return for now
	// as we want to inspect all streams easily.

	found := false
	for _, streamEvents := range eventStore.events {
		for _, e := range streamEvents {
			if te, ok := e.(*domain.ToolAwaitingInputEvent); ok {
				if te.ToolCallID == "call-222" {
					found = true
					break
				}
			}
		}
	}
	assert.True(t, found, "Expected ToolAwaitingInputEvent to be persisted")
}
