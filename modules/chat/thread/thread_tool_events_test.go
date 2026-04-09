package thread_test

import (
	"aicompass/foundation/logger"
	"aicompass/modules/chat/thread"
	"aicompass/modules/chat/threadmgmt"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockEventStore implements thread.EventStore for testing
type MockEventStore struct {
	events map[string][]thread.Event
}

func NewMockEventStore() *MockEventStore {
	return &MockEventStore{
		events: make(map[string][]thread.Event),
	}
}

func (m *MockEventStore) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...thread.Event) (int64, error) {
	m.events[streamID] = append(m.events[streamID], events...)
	return int64(len(m.events[streamID])), nil
}

func (m *MockEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]thread.Event, error) {
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
	th, err := thread.OpenThread(threadID, userID, "Test Thread")
	require.NoError(t, err)

	// Create a dummy tool event
	event := &thread.ToolNeedsInput{
		ThreadID:   threadID,
		MessageID:  "msg-789",
		ToolCallID: "call-abc",
		ToolName:   "test-tool",
		Elicitation: &thread.ElicitationSpec{
			Question: "Please provide input",
		},
		AgentName: "test-agent",
		Timestamp: time.Now(),
	}

	// Act
	err = th.RecordToolEvent(event)

	// Assert
	assert.NoError(t, err)
}

func TestManager_RecordToolEvent(t *testing.T) {
	// Setup in-memory event store
	eventStore := NewMockEventStore()
	logger := logger.New(io.Discard, logger.LevelDebug, "test", nil)
	repo := thread.NewThreadRepository(threadmgmt.NewEventStoreAdapter(eventStore))
	manager := threadmgmt.New(logger, repo)
	ctx := context.Background()

	// Create thread
	threadID := "thread-manager-test"
	_, _, err := manager.CreateThread(ctx, threadID, "user-setup", "Test Thread")
	require.NoError(t, err)

	// Create event
	event := &thread.ToolNeedsInput{
		ThreadID:   threadID,
		MessageID:  "msg-111",
		ToolCallID: "call-222",
		ToolName:   "my-tool",
		Elicitation: &thread.ElicitationSpec{
			Question: "Need input",
		},
		AgentName: "agent-x",
		Timestamp: time.Now(),
	}

	// Load thread and record event
	th, err := repo.Load(ctx, threadID)
	require.NoError(t, err)

	err = th.RecordToolEvent(event)
	require.NoError(t, err)

	// Save thread
	err = repo.Save(ctx, th)
	require.NoError(t, err)

	// Verify event is in store
	found := false
	for _, streamEvents := range eventStore.events {
		for _, e := range streamEvents {
			if te, ok := e.(*thread.ToolNeedsInput); ok {
				if te.ToolCallID == "call-222" {
					found = true
					break
				}
			}
		}
	}
	assert.True(t, found, "Expected ToolNeedsInput to be persisted")
}
