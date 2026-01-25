package kurrentdb

import (
	"context"
	"testing"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: Unit tests use MockEventStore and run in all environments.
// Integration tests require a running KurrentDB instance.
// Run 'task up' to start KurrentDB, then run tests without -short flag.

func TestEventStore_AppendAndRead(t *testing.T) {
	t.Parallel()
	// Use mock for unit tests
	store := NewMockEventStore()

	ctx := context.Background()
	streamID := "test-stream-" + time.Now().Format("20060102150405")

	// Create test events
	events := []core.Event{
		domain.ThreadCreated{
			ThreadID:  streamID,
			UserID:    "user-1",
			Title:     "Test Thread",
			Timestamp: time.Now(),
		},
		domain.UserMessageAdded{
			ThreadID:  streamID,
			MessageID: "msg-1",
			Content:   "Hello, world!",
			Timestamp: time.Now(),
		},
	}

	// Append events
	version, err := store.AppendWithVersion(ctx, streamID, -1, events...)
	require.NoError(t, err)
	assert.Positive(t, version)

	// Read events back
	readEvents, err := store.ReadStream(ctx, streamID, 0)
	require.NoError(t, err)
	assert.Len(t, readEvents, 2)

	// Verify first event
	threadCreated, ok := readEvents[0].(domain.ThreadCreated)
	assert.True(t, ok)
	assert.Equal(t, "Test Thread", threadCreated.Title)

	// Verify second event
	userMessage, ok := readEvents[1].(domain.UserMessageAdded)
	assert.True(t, ok)
	assert.Equal(t, "msg-1", userMessage.MessageID)
	assert.Equal(t, "Hello, world!", userMessage.Content)
}

func TestEventStore_ReadFromVersion(t *testing.T) {
	t.Parallel()
	// Use mock for unit tests
	store := NewMockEventStore()

	ctx := context.Background()
	streamID := "test-stream-versions-" + time.Now().Format("20060102150405")

	// Append 3 events
	events := []core.Event{
		domain.UserMessageAdded{
			ThreadID:  streamID,
			MessageID: "msg-1",
			Content:   "First message",
			Timestamp: time.Now(),
		},
		domain.UserMessageAdded{
			ThreadID:  streamID,
			MessageID: "msg-2",
			Content:   "Second message",
			Timestamp: time.Now(),
		},
		domain.UserMessageAdded{
			ThreadID:  streamID,
			MessageID: "msg-3",
			Content:   "Third message",
			Timestamp: time.Now(),
		},
	}

	_, err := store.AppendWithVersion(ctx, streamID, -1, events...)
	require.NoError(t, err)

	// Read from version 1 (should get 2nd and 3rd events)
	readEvents, err := store.ReadStream(ctx, streamID, 1)
	require.NoError(t, err)
	assert.Len(t, readEvents, 2)

	userMessage, ok := readEvents[0].(domain.UserMessageAdded)
	assert.True(t, ok)
	assert.Equal(t, "msg-2", userMessage.MessageID)
}

func TestEventStore_ReadNonExistentStream(t *testing.T) {
	t.Parallel()
	// Use mock for unit tests
	store := NewMockEventStore()

	ctx := context.Background()
	streamID := "non-existent-stream"

	// Read from non-existent stream should return empty slice
	events, err := store.ReadStream(ctx, streamID, 0)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestEventStore_DeleteStream(t *testing.T) {
	t.Parallel()
	// Use mock for unit tests
	store := NewMockEventStore()

	ctx := context.Background()
	streamID := "test-stream-delete-" + time.Now().Format("20060102150405")

	// Append some events
	events := []core.Event{
		domain.UserMessageAdded{
			ThreadID:  streamID,
			MessageID: "msg-1",
			Content:   "Test message",
			Timestamp: time.Now(),
		},
	}

	_, err := store.AppendWithVersion(ctx, streamID, -1, events...)
	require.NoError(t, err)

	// Verify events exist
	readEvents, err := store.ReadStream(ctx, streamID, 0)
	require.NoError(t, err)
	assert.Len(t, readEvents, 1)

	// Delete the stream
	err = store.DeleteStream(ctx, streamID)
	require.NoError(t, err)

	// Verify stream is deleted (returns empty)
	readEvents, err = store.ReadStream(ctx, streamID, 0)
	require.NoError(t, err)
	assert.Empty(t, readEvents)
}
