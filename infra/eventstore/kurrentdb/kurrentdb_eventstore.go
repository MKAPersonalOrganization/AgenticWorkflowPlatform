package kurrentdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"school-with-ai/sdk/agentsdk/core"

	"github.com/kurrent-io/KurrentDB-Client-Go/kurrentdb"
)

// ErrUnknownEventType is returned when an unknown event type is encountered during deserialization.
var ErrUnknownEventType = errors.New("unknown event type")

var (
	// ErrVersionOverflow is returned when a version exceeds max int64.
	ErrVersionOverflow = errors.New("version overflow")
	// ErrInvalidFromVersion is returned when fromVersion is negative.
	ErrInvalidFromVersion = errors.New("invalid fromVersion: must be non-negative")
)

// KurrentDBEventStore is an implementation of EventStore using KurrentDB (PostgreSQL-backed).
type KurrentDBEventStore struct {
	client *kurrentdb.Client
}

// NewKurrentDBEventStore creates a new KurrentDB event store.
func NewKurrentDBEventStore(connectionString string) (*KurrentDBEventStore, error) {
	settings, err := kurrentdb.ParseConnectionString(connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	client, err := kurrentdb.NewClient(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create KurrentDB client: %w", err)
	}

	store := &KurrentDBEventStore{
		client: client,
	}

	// Enable system projections on startup (best effort - ignore errors)
	go store.enableSystemProjections()

	return store, nil
}

// enableSystemProjections enables required system projections
// Note: The KurrentDB Go SDK does not expose projection management APIs.
// The $streams system projection must be enabled via HTTP API or Admin UI:
//
//	curl -X POST http://localhost:2113/projection/$streams/command/enable \
//	  -H "Content-Type: application/json" -u admin:changeit
func (s *KurrentDBEventStore) enableSystemProjections() {
	// No-op: System projections must be enabled externally
}

// Append adds events to a stream in KurrentDB.
func (s *KurrentDBEventStore) Append(ctx context.Context, streamID string, events ...core.Event) (int64, error) {
	if len(events) == 0 {
		return 0, nil
	}

	// Convert domain events to KurrentDB events
	eventDataList := make([]kurrentdb.EventData, 0, len(events))
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal event: %w", err)
		}

		eventDataList = append(eventDataList, kurrentdb.EventData{
			ContentType: kurrentdb.ContentTypeJson,
			EventType:   event.EventType(),
			Data:        data,
		})
	}

	// Append to stream
	result, err := s.client.AppendToStream(ctx, streamID, kurrentdb.AppendToStreamOptions{}, eventDataList...)
	if err != nil {
		return 0, fmt.Errorf("failed to append to stream: %w", err)
	}

	// Check for integer overflow before converting uint64 to int64
	if result.NextExpectedVersion > math.MaxInt64 {
		return 0, fmt.Errorf("%w: %d exceeds max int64", ErrVersionOverflow, result.NextExpectedVersion)
	}

	return int64(result.NextExpectedVersion), nil
}

// ReadStream reads all events for a stream from a specific version.
func (s *KurrentDBEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]core.Event, error) {
	// Check for negative version before converting int64 to uint64
	if fromVersion < 0 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidFromVersion, fromVersion)
	}

	readOpts := kurrentdb.ReadStreamOptions{
		From:      kurrentdb.Revision(uint64(fromVersion)),
		Direction: kurrentdb.Forwards,
	}

	stream, err := s.client.ReadStream(ctx, streamID, readOpts, 4096)
	if err != nil {
		// Return empty slice on stream not found (matches in-memory behavior)
		return []core.Event{}, nil
	}
	defer stream.Close()

	var events []core.Event
	for {
		resolvedEvent, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// Check if it's a "stream not found" error by checking the error message
			errMsg := err.Error()
			if errMsg != "" && (errMsg == "stream is not found" ||
				(len(errMsg) > 30 && errMsg[:30] == "[ErrorCodeResourceNotFound] a ")) {
				return []core.Event{}, nil
			}

			return nil, fmt.Errorf("failed to receive event: %w", err)
		}

		event, err := s.convertFromRecordedEvent(resolvedEvent.Event)
		if err != nil {
			return nil, fmt.Errorf("failed to convert event: %w", err)
		}

		events = append(events, event)
	}

	return events, nil
}

// Close closes the KurrentDB client connection.
func (s *KurrentDBEventStore) Close() error {
	if s.client != nil {
		err := s.client.Close()
		if err != nil {
			return fmt.Errorf("failed to close KurrentDB client: %w", err)
		}
	}

	return nil
}

// convertFromRecordedEvent converts a KurrentDB RecordedEvent to a domain event.
func (s *KurrentDBEventStore) convertFromRecordedEvent(recordedEvent *kurrentdb.RecordedEvent) (core.Event, error) {
	var event core.Event

	switch recordedEvent.EventType {
	case "ThreadCreatedEvent":
		var e core.ThreadCreatedEvent
		err := json.Unmarshal(recordedEvent.Data, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadCreatedEvent: %w", err)
		}
		event = e

	case "UserMessageAddedEvent":
		var e core.UserMessageAddedEvent
		err := json.Unmarshal(recordedEvent.Data, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal UserMessageAddedEvent: %w", err)
		}
		event = e

	case "AssistantMessageStartedEvent":
		var e core.AssistantMessageStartedEvent
		err := json.Unmarshal(recordedEvent.Data, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantMessageStartedEvent: %w", err)
		}
		event = e

	case "AssistantContentStreamedEvent":
		var e core.AssistantContentStreamedEvent
		err := json.Unmarshal(recordedEvent.Data, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantContentStreamedEvent: %w", err)
		}
		event = e

	case "ToolCallStartedEvent":
		var e core.ToolCallStartedEvent
		err := json.Unmarshal(recordedEvent.Data, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolCallStartedEvent: %w", err)
		}
		event = e

	case "ToolCallArgumentsStreamedEvent":
		var e core.ToolCallArgumentsStreamedEvent
		err := json.Unmarshal(recordedEvent.Data, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolCallArgumentsStreamedEvent: %w", err)
		}
		event = e

	case "ToolCallCompletedEvent":
		var e core.ToolCallCompletedEvent
		err := json.Unmarshal(recordedEvent.Data, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolCallCompletedEvent: %w", err)
		}
		event = e

	case "AssistantMessageCompletedEvent":
		var e core.AssistantMessageCompletedEvent
		err := json.Unmarshal(recordedEvent.Data, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantMessageCompletedEvent: %w", err)
		}
		event = e

	case "ToolCallExecutedEvent":
		var e core.ToolCallExecutedEvent
		err := json.Unmarshal(recordedEvent.Data, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolCallExecutedEvent: %w", err)
		}
		event = e

	case "ThreadTitleUpdatedEvent":
		var e core.ThreadTitleUpdatedEvent
		err := json.Unmarshal(recordedEvent.Data, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadTitleUpdatedEvent: %w", err)
		}
		event = e

	case "ThreadDeletedEvent":
		var e core.ThreadDeletedEvent
		err := json.Unmarshal(recordedEvent.Data, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadDeletedEvent: %w", err)
		}
		event = e

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownEventType, recordedEvent.EventType)
	}

	return event, nil
}

// ReadAllThreadIDs reads all thread IDs by scanning the $streams system projection.
// The $streams projection must be enabled before calling this method.
// Returns an empty slice if no threads are found or if the projection is not available.
func (s *KurrentDBEventStore) ReadAllThreadIDs(ctx context.Context) ([]string, error) {
	threadIDs, err := s.readFromStreamsProjection(ctx)
	if err != nil {
		// Check if error is specifically "stream not found" which indicates projection not available
		// Only ignore that specific error, return other errors (network, permissions, etc.)
		if strings.Contains(err.Error(), "stream not found") || strings.Contains(err.Error(), "STREAM_NOT_FOUND") {
			// Projection not enabled - return empty list gracefully
			return []string{}, nil
		}
		// Return actual errors (network, auth, etc.)
		return nil, fmt.Errorf("failed to read thread IDs: %w", err)
	}

	return threadIDs, nil
}

// DeleteStream permanently deletes a stream from the event store.
// This uses KurrentDB's DeleteStream which sets the "Truncate Before" to the stream's current last event.
// After deletion, reads will return "stream not found" but the stream can be written to again.
func (s *KurrentDBEventStore) DeleteStream(ctx context.Context, streamID string) error {
	result, err := s.client.DeleteStream(ctx, streamID, kurrentdb.DeleteStreamOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete stream %s: %w", streamID, err)
	}
	_ = result // DeleteResult contains position info if needed

	return nil
}

// readFromStreamsProjection reads all thread IDs from the $streams system projection.
// The $streams projection stores metadata for all streams in the format: "{version}@{stream-name}"
// We parse this format to extract stream names that start with "thread".
func (s *KurrentDBEventStore) readFromStreamsProjection(ctx context.Context) ([]string, error) {
	readOpts := kurrentdb.ReadStreamOptions{
		From:      kurrentdb.Revision(0),
		Direction: kurrentdb.Forwards,
	}

	stream, err := s.client.ReadStream(ctx, "$streams", readOpts, 10000)
	if err != nil {
		return nil, fmt.Errorf("failed to read $streams: %w", err)
	}
	defer stream.Close()

	threadIDs := make(map[string]bool)
	for {
		resolvedEvent, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to receive event from $streams: %w", err)
		}

		// Parse $streams event data format: "{version}@{stream-name}"
		// Example: "0@thread_1234567890"
		dataStr := string(resolvedEvent.Event.Data)
		if atIndex := strings.Index(dataStr, "@"); atIndex != -1 && atIndex < len(dataStr)-1 {
			streamName := dataStr[atIndex+1:]
			// Filter for thread streams only
			if len(streamName) >= 6 && streamName[:6] == "thread" {
				threadIDs[streamName] = true
			}
		}
	}

	result := make([]string, 0, len(threadIDs))
	for threadID := range threadIDs {
		result = append(result, threadID)
	}

	return result, nil
}

// Verify KurrentDBEventStore implements core.EventStore.
var _ core.EventStore = (*KurrentDBEventStore)(nil)
