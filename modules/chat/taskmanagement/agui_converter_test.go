package taskmanagement

import (
	"aicompass/modules/chat/thread"
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
			name: "RunStarted",
			event: &thread.RunStarted{
				ThreadID:  "thread-1",
				RunID:     "run-1",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewRunStartedEvent("thread-1", "run-1"),
			},
		},
		{
			name: "RunFinished",
			event: &thread.RunFinished{
				ThreadID:  "thread-1",
				RunID:     "run-1",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewRunFinishedEvent("thread-1", "run-1"),
			},
		},
		{
			name: "RunError",
			event: &thread.RunError{
				ThreadID:  "thread-1",
				Message:   "error message",
				Code:      "ERR-001",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewRunErrorEvent("error message", aguievents.WithErrorCode("ERR-001")),
			},
		},
		{
			name: "ThreadTitleUpdated",
			event: &thread.ThreadTitleUpdated{
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
			name: "AssistantMessageStarted",
			event: &thread.AssistantMessageStarted{
				ThreadID:  "thread-1",
				MessageID: "msg-1",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewTextMessageStartEvent("msg-1", aguievents.WithRole("assistant")),
			},
		},
		{
			name: "AssistantContentStreamed",
			event: &thread.AssistantContentStreamed{
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
			name: "AssistantMessageCompleted",
			event: &thread.AssistantMessageCompleted{
				ThreadID:  "thread-1",
				MessageID: "msg-1",
				Timestamp: now,
			},
			expected: []any{
				aguievents.NewTextMessageEndEvent("msg-1"),
			},
		},
		{
			name: "ToolResultAdded",
			event: &thread.ToolResultAdded{
				ThreadID:   "thread-1",
				MessageID:  "msg-1",
				ToolCallID: "tool-1",
				ToolName:   "test-tool",
				Result:     "{\"success\": true}",
				AgentName:  "",
				Timestamp:  now,
			},
			expected: []any{
				aguievents.NewCustomEvent("TOOL_SUCCESS", aguievents.WithValue(map[string]any{
					"toolCallId": "tool-1",
					"toolName":   "test-tool",
					"result":     "{\"success\": true}",
					"isError":    false,
					"agentName":  "",
					"timestamp":  now,
				})),
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

			var input thread.Event
			if de, ok := tt.event.(thread.Event); ok {
				input = de
			}

			got := ToAguiEvents(input)

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
