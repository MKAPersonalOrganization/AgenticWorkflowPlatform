package thread_test

import (
	"testing"
	"time"

	"school-with-ai/modules/chat/thread"

	"github.com/stretchr/testify/assert"
)

func TestThreadAggregate_ActiveToolStates(t *testing.T) {
	th := thread.NewThreadAggregate("thread-1")

	// 1. Tool enters AwaitingInput
	awaitInput := thread.ToolNeedsInput{
		ThreadID:   "thread-1",
		MessageID:  "msg-1",
		ToolCallID: "call-1",
		ToolName:   "my-tool",
		Elicitation: &thread.ElicitationSpec{
			Question: "Input?",
		},
		AgentName: "agent",
		Timestamp: time.Now(),
	}
	th.Apply(&awaitInput)

	assert.Contains(t, th.ActiveToolStates, "call-1")
	assert.Equal(t, "AwaitingInput", th.ActiveToolStates["call-1"].Status)
	assert.Equal(t, "Input?", th.ActiveToolStates["call-1"].Elicitation["question"])

	// 2. Input Provided — adds user's answer as the real tool result message
	// and removes the tool from ActiveToolStates.
	inputProvided := thread.InputProvided{
		ThreadID:   "thread-1",
		MessageID:  "msg-2",
		ToolCallID: "call-1",
		Input:      "yes",
		AgentName:  "agent",
		Timestamp:  time.Now(),
	}
	th.Apply(&inputProvided)

	// Tool should be removed from active states (answer recorded)
	assert.NotContains(t, th.ActiveToolStates, "call-1",
		"Tool state should be removed after user provides input")

	// The user's answer should appear as a tool-role message in the thread
	lastMsg := th.Messages[len(th.Messages)-1]
	assert.Equal(t, "tool", lastMsg.Role)
	assert.Equal(t, "yes", lastMsg.Content)
	assert.Equal(t, "call-1", lastMsg.ToolCallID)
	assert.Equal(t, "my-tool", lastMsg.ToolName)
	assert.Equal(t, "agent", lastMsg.AgentName)
}

func TestThreadAggregate_ActiveToolStates_Auth(t *testing.T) {
	th := thread.NewThreadAggregate("thread-1")

	// 1. Awaiting Auth
	awaitAuth := thread.ToolNeedsAuth{
		ThreadID:   "thread-1",
		MessageID:  "msg-1",
		ToolCallID: "call-2",
		ToolName:   "auth-tool",
		AuthSpec: &thread.AuthorizationSpec{
			Resource: "res",
			Action:   "read",
		},
		AgentName: "agent",
		Timestamp: time.Now(),
	}
	th.Apply(&awaitAuth)

	assert.Contains(t, th.ActiveToolStates, "call-2")
	assert.Equal(t, "AwaitingAuth", th.ActiveToolStates["call-2"].Status)
	assert.Equal(t, "read", th.ActiveToolStates["call-2"].AuthSpec["action"])

	// 2. Auth Granted
	granted := thread.AuthGranted{
		ThreadID:   "thread-1",
		MessageID:  "msg-2",
		ToolCallID: "call-2",
		AgentName:  "agent",
		Timestamp:  time.Now(),
	}
	th.Apply(&granted)

	assert.Equal(t, "AuthGranted", th.ActiveToolStates["call-2"].Status)

	// 3. Tool Result
	toolResult := thread.ToolResultAdded{
		ThreadID:   "thread-1",
		MessageID:  "msg-3",
		ToolCallID: "call-2",
		Result:     "Done",
		AgentName:  "agent",
		Timestamp:  time.Now(),
	}
	th.Apply(&toolResult)

	assert.NotContains(t, th.ActiveToolStates, "call-2")
}
