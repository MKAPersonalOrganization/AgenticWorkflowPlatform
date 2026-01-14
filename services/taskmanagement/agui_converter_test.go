package taskmanagement

import (
	"school-with-ai/domain"
	"testing"
	"time"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/stretchr/testify/assert"
)

func TestToAguiEvents(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		name     string
		event    any
		expected []any
	}{
		{
			name: "RunStartedEvent",
			event: &domain.RunStartedEvent{
				ThreadID:  "thread-1",
				RunID:     "run-1",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewRunStartedEvent("thread-1", "run-1"),
			},
		},
		{
			name: "RunFinishedEvent",
			event: &domain.RunFinishedEvent{
				ThreadID:  "thread-1",
				RunID:     "run-1",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewRunFinishedEvent("thread-1", "run-1"),
			},
		},
		{
			name: "RunErrorEvent",
			event: &domain.RunErrorEvent{
				ThreadID:  "thread-1",
				Message:   "error message",
				ErrorCode: "ERR-001",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewRunErrorEvent("error message", aguievents.WithErrorCode("ERR-001")),
			},
		},
		{
			name: "ThreadTitleUpdatedEvent",
			event: &domain.ThreadTitleUpdatedEvent{
				ThreadID:  "thread-1",
				Title:     "New Title",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewCustomEvent("THREAD_TITLE_UPDATED", aguievents.WithValue(map[string]any{
					"threadId": "thread-1",
					"title":    "New Title",
				})),
			},
		},
		{
			name: "AssistantMessageStartedEvent",
			event: &domain.AssistantMessageStartedEvent{
				ThreadID:  "thread-1",
				MessageID: "msg-1",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewTextMessageStartEvent("msg-1", aguievents.WithRole("assistant")),
			},
		},
		{
			name: "AssistantContentStreamedEvent",
			event: &domain.AssistantContentStreamedEvent{
				ThreadID:  "thread-1",
				MessageID: "msg-1",
				Delta:     "hello",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewTextMessageContentEvent("msg-1", "hello"),
			},
		},
		{
			name: "AssistantMessageCompletedEvent",
			event: &domain.AssistantMessageCompletedEvent{
				ThreadID:  "thread-1",
				MessageID: "msg-1",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewTextMessageEndEvent("msg-1"),
			},
		},
		{
			name: "ToolCallStartedEvent",
			event: &domain.ToolCallStartedEvent{
				ThreadID:        "thread-1",
				ParentMessageID: "msg-1",
				ToolCallID:      "tool-1",
				ToolCallName:    "get_weather",
				AgentName:       "weather-agent",
				Timestamp:       now,
			},
			expected: []any{
				aguievents.NewCustomEvent("TOOL_CALL_AGENT", aguievents.WithValue(map[string]any{
					"toolCallId": "tool-1",
					"agentName":  "weather-agent",
				})),
				aguievents.NewToolCallStartEvent("tool-1", "get_weather", aguievents.WithParentMessageID("msg-1")),
			},
		},
		{
			name: "ToolCallArgumentsStreamedEvent",
			event: &domain.ToolCallArgumentsStreamedEvent{
				ThreadID:   "thread-1",
				ToolCallID: "tool-1",
				Delta:      "{\"city\":",
				Timestamp:  now,
			},
			expected: []any{
				aguievents.NewToolCallArgsEvent("tool-1", "{\"city\":"),
			},
		},
		{
			name: "ToolCallCompletedEvent",
			event: &domain.ToolCallCompletedEvent{
				ThreadID:   "thread-1",
				ToolCallID: "tool-1",
				Timestamp:  now,
			},
			expected: []any{
				aguievents.NewToolCallEndEvent("tool-1"),
			},
		},
		{
			name: "ToolCallExecutedEvent",
			event: &domain.ToolCallExecutedEvent{
				ThreadID:   "thread-1",
				MessageID:  "msg-2",
				ToolCallID: "tool-1",
				ToolResult: "{\"temp\": 25}",
				Timestamp:  now,
			},
			expected: []any{
				aguievents.NewToolCallResultEvent("msg-2", "tool-1", "{\"temp\": 25}"),
			},
		},
		{
			name:     "NonDomainEvent",
			event:    "unknown",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ToAguiEvents(tt.event)

			// Check if both are nil
			if tt.expected == nil {
				assert.Nil(t, got)
				return
			}

			// Check event count
			assert.Equal(t, len(tt.expected), len(got), "event count mismatch")

			// Check event types (not deep equality due to timestamps)
			for i := range got {
				assert.IsType(t, tt.expected[i], got[i], "event type mismatch at index %d", i)
			}
		})
	}
}
