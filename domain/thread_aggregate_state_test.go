package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
)

func TestThreadAggregate_ActiveToolStates(t *testing.T) {
	thread := domain.NewThreadAggregate("thread-1")

	// 1. Tool enters AwaitingInput
	awaitInput := domain.ToolNeedsInput{
		ThreadID:   "thread-1",
		MessageID:  "msg-1",
		ToolCallID: "call-1",
		ToolName:   "my-tool",
		Elicitation: &core.ElicitationSpec{
			Question: "Input?",
		},
		AgentName: "agent",
		Timestamp: time.Now(),
	}
	thread.Apply(awaitInput)

	assert.Contains(t, thread.ActiveToolStates, "call-1")
	assert.Equal(t, "AwaitingInput", thread.ActiveToolStates["call-1"].Status)
	assert.Equal(t, "Input?", thread.ActiveToolStates["call-1"].Elicitation["question"])

	// 2. Input Provided
	inputProvided := domain.InputProvided{
		ThreadID:   "thread-1",
		MessageID:  "msg-2",
		ToolCallID: "call-1",
		Input:      "yes",
		AgentName:  "agent",
		Timestamp:  time.Now(),
	}
	thread.Apply(inputProvided)

	assert.Contains(t, thread.ActiveToolStates, "call-1")
	assert.Equal(t, "InputProvided", thread.ActiveToolStates["call-1"].Status)

	// 3. Tool Result (ToolResultAdded)
	// This usually comes from execution after input
	toolResult := domain.ToolResultAdded{
		ThreadID:   "thread-1",
		MessageID:  "msg-3",
		ToolCallID: "call-1",
		Result:     "Result",
		AgentName:  "agent",
		Timestamp:  time.Now(),
	}
	thread.Apply(toolResult)

	assert.NotContains(t, thread.ActiveToolStates, "call-1", "Tool state should be removed after result")
}

func TestThreadAggregate_ActiveToolStates_Auth(t *testing.T) {
	thread := domain.NewThreadAggregate("thread-1")

	// 1. Awaiting Auth
	awaitAuth := domain.ToolNeedsAuth{
		ThreadID:   "thread-1",
		MessageID:  "msg-1",
		ToolCallID: "call-2",
		ToolName:   "auth-tool",
		AuthSpec: &core.AuthorizationSpec{
			Resource: "res",
			Action:   "read",
		},
		AgentName: "agent",
		Timestamp: time.Now(),
	}
	thread.Apply(awaitAuth)

	assert.Contains(t, thread.ActiveToolStates, "call-2")
	assert.Equal(t, "AwaitingAuth", thread.ActiveToolStates["call-2"].Status)
	assert.Equal(t, "read", thread.ActiveToolStates["call-2"].AuthSpec["action"])

	// 2. Auth Granted
	granted := domain.AuthGranted{
		ThreadID:   "thread-1",
		MessageID:  "msg-2",
		ToolCallID: "call-2",
		AgentName:  "agent",
		Timestamp:  time.Now(),
	}
	thread.Apply(granted)

	assert.Equal(t, "AuthGranted", thread.ActiveToolStates["call-2"].Status)

	// 3. Tool Result
	toolResult := domain.ToolResultAdded{
		ThreadID:   "thread-1",
		MessageID:  "msg-3",
		ToolCallID: "call-2",
		Result:     "Done",
		AgentName:  "agent",
		Timestamp:  time.Now(),
	}
	thread.Apply(toolResult)

	assert.NotContains(t, thread.ActiveToolStates, "call-2")
}
