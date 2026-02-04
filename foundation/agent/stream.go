package agent

import (
	"iter"
)

// Stream is a sequence of events or errors from agent execution.
// This is an alias for Go 1.23 iterators, compatible with both ADK-Go and Blades.
type Stream = iter.Seq2[Event, error]

// MessageStream is a sequence of messages or errors.
// Used when the caller wants messages directly instead of wrapped events.
type MessageStream = iter.Seq2[*Message, error]

// MessageStatus indicates the current state of a message.
// This aligns with Blades' Status type for streaming support.
type MessageStatus string

const (
	// StatusInProgress indicates the message is being generated.
	StatusInProgress MessageStatus = "in_progress"

	// StatusIncomplete indicates the message is partially generated
	// but streaming was interrupted.
	StatusIncomplete MessageStatus = "incomplete"

	// StatusCompleted indicates the message is fully generated.
	StatusCompleted MessageStatus = "completed"
)

// StreamingMode defines how events are streamed during agent execution.
type StreamingMode string

const (
	// StreamingModeNone disables streaming; return only final results.
	StreamingModeNone StreamingMode = "none"

	// StreamingModeSSE enables server-sent events streaming.
	// LLM response parts are streamed as they're generated.
	StreamingModeSSE StreamingMode = "sse"

	// StreamingModeBidi enables bidirectional streaming.
	// Reserved for future real-time interaction support.
	StreamingModeBidi StreamingMode = "bidi"
)

// Just creates a Stream that emits the provided events in order.
// Useful for testing and simple event generation.
func Just(events ...Event) Stream {
	return func(yield func(Event, error) bool) {
		for _, e := range events {
			if !yield(e, nil) {
				return
			}
		}
	}
}

// Error creates a Stream that emits a single error.
func Error(err error) Stream {
	return func(yield func(Event, error) bool) {
		yield(nil, err)
	}
}

// Collect consumes a Stream and returns all events and the first error.
func Collect(stream Stream) ([]Event, error) {
	var events []Event
	var firstErr error

	for event, err := range stream {
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if event != nil {
			events = append(events, event)
		}
	}

	return events, firstErr
}

// Filter returns a Stream that only emits events matching the predicate.
func Filter(stream Stream, predicate func(Event) bool) Stream {
	return func(yield func(Event, error) bool) {
		for event, err := range stream {
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}
			if predicate(event) {
				if !yield(event, nil) {
					return
				}
			}
		}
	}
}

// Map transforms events in a Stream using the provided mapper function.
func Map[T any](stream Stream, mapper func(Event) (T, error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for event, err := range stream {
			if err != nil {
				var zero T
				if !yield(zero, err) {
					return
				}
				continue
			}
			mapped, mapErr := mapper(event)
			if !yield(mapped, mapErr) {
				return
			}
		}
	}
}
