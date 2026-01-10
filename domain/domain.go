package domain

import (
	"errors"

	"school-with-ai/sdk/agentsdk/core"
)

// Common errors.
var (
	ErrThreadNotFound = errors.New("thread not found")
)

// ToolCall is an alias for core.ToolCall.
type ToolCall = core.ToolCall

// ToolFunction is an alias for core.ToolFunction.
type ToolFunction = core.ToolFunction

// Message is an alias for core.Message.
type Message = core.Message

// Thread is an alias for core.Thread.
type Thread = core.Thread

// GenerateMessageID is an alias for core.GenerateMessageID.
var GenerateMessageID = core.GenerateMessageID
