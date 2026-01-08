package kurrentdb

import (
	"context"
	"testing"
	"time"

	"school-with-ai/sdk/agentsdk/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: These tests require a running KurrentDB instance.
// Run 'task up' to start KurrentDB before running tests.

func TestKurrentDBEventStore_AppendAndRead(t *testing.T) {
	// Connect to KurrentDB
	connectionString := "esdb://localhost:2113?tls=false"
	store, err := NewKurrentDBEventStore(connectionString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	streamID := "test-stream-" + time.Now().Format("20060102150405")

	// Create test events
	events := []core.Event{
		core.ThreadCreatedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "ThreadCreatedEvent",
				Timestamp: time.Now(),
				ThreadID:  streamID,
			},
			Title: "Test Thread",
		},
		core.UserMessageAddedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "UserMessageAddedEvent",
				Timestamp: time.Now(),
				ThreadID:  streamID,
			},
			MessageID: "msg-1",
			Content:   "Hello, world!",
		},
	}

	// Append events
	version, err := store.Append(ctx, streamID, events...)
	require.NoError(t, err)
	assert.Greater(t, version, int64(0))

	// Read events back
	readEvents, err := store.ReadStream(ctx, streamID, 0)
	require.NoError(t, err)
	assert.Len(t, readEvents, 2)

	// Verify first event
	threadCreated, ok := readEvents[0].(core.ThreadCreatedEvent)
	assert.True(t, ok)
	assert.Equal(t, "Test Thread", threadCreated.Title)

	// Verify second event
	userMessage, ok := readEvents[1].(core.UserMessageAddedEvent)
	assert.True(t, ok)
	assert.Equal(t, "msg-1", userMessage.MessageID)
	assert.Equal(t, "Hello, world!", userMessage.Content)
}

func TestKurrentDBEventStore_ReadFromVersion(t *testing.T) {
	connectionString := "esdb://localhost:2113?tls=false"
	store, err := NewKurrentDBEventStore(connectionString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	streamID := "test-stream-versions-" + time.Now().Format("20060102150405")

	// Append 3 events
	events := []core.Event{
		core.UserMessageAddedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "UserMessageAddedEvent",
				Timestamp: time.Now(),
				ThreadID:  streamID,
			},
			MessageID: "msg-1",
			Content:   "First message",
		},
		core.UserMessageAddedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "UserMessageAddedEvent",
				Timestamp: time.Now(),
				ThreadID:  streamID,
			},
			MessageID: "msg-2",
			Content:   "Second message",
		},
		core.UserMessageAddedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "UserMessageAddedEvent",
				Timestamp: time.Now(),
				ThreadID:  streamID,
			},
			MessageID: "msg-3",
			Content:   "Third message",
		},
	}

	_, err = store.Append(ctx, streamID, events...)
	require.NoError(t, err)

	// Read from version 1 (should get 2nd and 3rd events)
	readEvents, err := store.ReadStream(ctx, streamID, 1)
	require.NoError(t, err)
	assert.Len(t, readEvents, 2)

	userMessage, ok := readEvents[0].(core.UserMessageAddedEvent)
	assert.True(t, ok)
	assert.Equal(t, "msg-2", userMessage.MessageID)
}

func TestKurrentDBEventStore_ReadNonExistentStream(t *testing.T) {
	connectionString := "esdb://localhost:2113?tls=false"
	store, err := NewKurrentDBEventStore(connectionString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	streamID := "non-existent-stream"

	// Read from non-existent stream should return empty slice
	events, err := store.ReadStream(ctx, streamID, 0)
	require.NoError(t, err)
	assert.Empty(t, events)
}
