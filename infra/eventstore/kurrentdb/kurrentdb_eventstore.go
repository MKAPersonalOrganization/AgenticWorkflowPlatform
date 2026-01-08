package kurrentdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"school-with-ai/sdk/agentsdk/core"

	"github.com/kurrent-io/KurrentDB-Client-Go/kurrentdb"
)

// KurrentDBEventStore is an implementation of EventStore using KurrentDB (PostgreSQL-backed)
type KurrentDBEventStore struct {
	client *kurrentdb.Client
}

// NewKurrentDBEventStore creates a new KurrentDB event store
func NewKurrentDBEventStore(connectionString string) (*KurrentDBEventStore, error) {
	settings, err := kurrentdb.ParseConnectionString(connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	client, err := kurrentdb.NewClient(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create KurrentDB client: %w", err)
	}

	return &KurrentDBEventStore{
		client: client,
	}, nil
}

// Append adds events to a stream in KurrentDB
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

	return int64(result.NextExpectedVersion), nil
}

// ReadStream reads all events for a stream from a specific version
func (s *KurrentDBEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]core.Event, error) {
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

// Close closes the KurrentDB client connection
func (s *KurrentDBEventStore) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// convertFromRecordedEvent converts a KurrentDB RecordedEvent to a domain event
func (s *KurrentDBEventStore) convertFromRecordedEvent(recordedEvent *kurrentdb.RecordedEvent) (core.Event, error) {
	var event core.Event

	switch recordedEvent.EventType {
	case "ThreadCreatedEvent":
		var e core.ThreadCreatedEvent
		if err := json.Unmarshal(recordedEvent.Data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadCreatedEvent: %w", err)
		}
		event = e

	case "UserMessageAddedEvent":
		var e core.UserMessageAddedEvent
		if err := json.Unmarshal(recordedEvent.Data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal UserMessageAddedEvent: %w", err)
		}
		event = e

	case "AssistantMessageStartedEvent":
		var e core.AssistantMessageStartedEvent
		if err := json.Unmarshal(recordedEvent.Data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantMessageStartedEvent: %w", err)
		}
		event = e

	case "AssistantContentStreamedEvent":
		var e core.AssistantContentStreamedEvent
		if err := json.Unmarshal(recordedEvent.Data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantContentStreamedEvent: %w", err)
		}
		event = e

	case "ToolCallStartedEvent":
		var e core.ToolCallStartedEvent
		if err := json.Unmarshal(recordedEvent.Data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolCallStartedEvent: %w", err)
		}
		event = e

	case "ToolCallArgumentsStreamedEvent":
		var e core.ToolCallArgumentsStreamedEvent
		if err := json.Unmarshal(recordedEvent.Data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolCallArgumentsStreamedEvent: %w", err)
		}
		event = e

	case "ToolCallCompletedEvent":
		var e core.ToolCallCompletedEvent
		if err := json.Unmarshal(recordedEvent.Data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolCallCompletedEvent: %w", err)
		}
		event = e

	case "AssistantMessageCompletedEvent":
		var e core.AssistantMessageCompletedEvent
		if err := json.Unmarshal(recordedEvent.Data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AssistantMessageCompletedEvent: %w", err)
		}
		event = e

	case "ToolCallExecutedEvent":
		var e core.ToolCallExecutedEvent
		if err := json.Unmarshal(recordedEvent.Data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ToolCallExecutedEvent: %w", err)
		}
		event = e

	case "ThreadTitleUpdatedEvent":
		var e core.ThreadTitleUpdatedEvent
		if err := json.Unmarshal(recordedEvent.Data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadTitleUpdatedEvent: %w", err)
		}
		event = e

	case "ThreadDeletedEvent":
		var e core.ThreadDeletedEvent
		if err := json.Unmarshal(recordedEvent.Data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadDeletedEvent: %w", err)
		}
		event = e

	default:
		return nil, fmt.Errorf("unknown event type: %s", recordedEvent.EventType)
	}

	return event, nil
}

// Verify KurrentDBEventStore implements core.EventStore
var _ core.EventStore = (*KurrentDBEventStore)(nil)
