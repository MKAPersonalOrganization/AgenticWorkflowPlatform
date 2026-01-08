//go:build integration

package kurrentdb

import (
	"context"
	"testing"
	"time"

	"school-with-ai/sdk/agentsdk/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests that require a running KurrentDB instance.
// Run with: go test -tags=integration
// Make sure KurrentDB is running: task up

func TestKurrentDBIntegration_AppendAndRead(t *testing.T) {
	t.Parallel()
	connectionString := "esdb://localhost:2113?tls=false"
	store, err := NewKurrentDBEventStore(connectionString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	streamID := "integration-test-" + time.Now().Format("20060102150405.000000")

	events := []core.Event{
		core.ThreadCreatedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "ThreadCreatedEvent",
				Timestamp: time.Now(),
				ThreadID:  streamID,
			},
			Title: "Integration Test Thread",
		},
		core.UserMessageAddedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "UserMessageAddedEvent",
				Timestamp: time.Now(),
				ThreadID:  streamID,
			},
			MessageID: "msg-1",
			Content:   "Integration test message",
		},
	}

	version, err := store.Append(ctx, streamID, events...)
	require.NoError(t, err)
	assert.Greater(t, version, int64(0))

	readEvents, err := store.ReadStream(ctx, streamID, 0)
	require.NoError(t, err)
	assert.Len(t, readEvents, 2)

	threadCreated, ok := readEvents[0].(core.ThreadCreatedEvent)
	assert.True(t, ok)
	assert.Equal(t, "Integration Test Thread", threadCreated.Title)
}

func TestKurrentDBIntegration_DeleteStream(t *testing.T) {
	t.Parallel()
	connectionString := "esdb://localhost:2113?tls=false"
	store, err := NewKurrentDBEventStore(connectionString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	streamID := "integration-delete-test-" + time.Now().Format("20060102150405.000000")

	// Append event
	events := []core.Event{
		core.UserMessageAddedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "UserMessageAddedEvent",
				Timestamp: time.Now(),
				ThreadID:  streamID,
			},
			MessageID: "msg-1",
			Content:   "Test",
		},
	}

	_, err = store.Append(ctx, streamID, events...)
	require.NoError(t, err)

	// Delete stream
	err = store.DeleteStream(ctx, streamID)
	require.NoError(t, err)

	// Verify deleted
	readEvents, err := store.ReadStream(ctx, streamID, 0)
	require.NoError(t, err)
	assert.Empty(t, readEvents)
}

func TestKurrentDBIntegration_ReadAllThreadIDs(t *testing.T) {
	t.Parallel()
	connectionString := "esdb://localhost:2113?tls=false"
	store, err := NewKurrentDBEventStore(connectionString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Read all thread IDs - should not error
	threadIDs, err := store.ReadAllThreadIDs(ctx)
	require.NoError(t, err)

	// Should return a list (may be empty or contain threads)
	assert.NotNil(t, threadIDs)

	// All IDs should start with "thread_" prefix
	for _, id := range threadIDs {
		assert.Contains(t, id, "thread_", "All stream IDs should contain thread_ prefix")
	}
}
