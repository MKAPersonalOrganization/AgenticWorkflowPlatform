package sdk

import (
	"school-with-ai/domain"
	"school-with-ai/services/llm"
)

// ToolCallAccumulator tracks streaming tool call deltas and builds complete tool calls
type ToolCallAccumulator struct {
	byID    map[string]*toolCallState
	byIndex map[int]*toolCallState
}

// toolCallState tracks the state of a single tool call during streaming
type toolCallState struct {
	ID        string
	Index     int
	Name      string
	Arguments string
}

// NewToolCallAccumulator creates a new accumulator
func NewToolCallAccumulator() *ToolCallAccumulator {
	return &ToolCallAccumulator{
		byID:    make(map[string]*toolCallState),
		byIndex: make(map[int]*toolCallState),
	}
}

// ToolCallUpdate represents the result of processing an update
type ToolCallUpdate struct {
	ID        string
	Name      string
	IsNew     bool   // true if this is a new tool call
	ArgsDelta string // the new arguments added in this update
}

// ProcessUpdate processes a ChatResponseUpdate and extracts tool call information.
// Returns the update info and whether any tool call content was found.
func (a *ToolCallAccumulator) ProcessUpdate(update llm.ChatResponseUpdate) (ToolCallUpdate, bool) {
	// Get function calls from the update
	functionCalls := update.FunctionCalls()
	if len(functionCalls) == 0 {
		return ToolCallUpdate{}, false
	}

	// Process the first function call in the update
	fc := functionCalls[0]
	index := update.ToolCallIndex

	// Check if this is a new tool call (has CallID and Name)
	if fc.CallID != "" && fc.Name != "" {
		state := &toolCallState{
			ID:        fc.CallID,
			Index:     index,
			Name:      fc.Name,
			Arguments: fc.RawArguments,
		}
		a.byID[fc.CallID] = state
		a.byIndex[index] = state

		return ToolCallUpdate{
			ID:        fc.CallID,
			Name:      fc.Name,
			IsNew:     true,
			ArgsDelta: fc.RawArguments,
		}, true
	}

	// This is a continuation - find by index or ID
	state, exists := a.byIndex[index]
	if !exists && update.ToolCallID != "" {
		state, exists = a.byID[update.ToolCallID]
	}
	if !exists {
		return ToolCallUpdate{}, false
	}

	// Accumulate arguments
	if fc.RawArguments != "" {
		state.Arguments += fc.RawArguments
	}

	return ToolCallUpdate{
		ID:        state.ID,
		Name:      state.Name,
		IsNew:     false,
		ArgsDelta: fc.RawArguments,
	}, true
}

// GetAll returns all accumulated tool calls as domain.ToolCall slices
func (a *ToolCallAccumulator) GetAll() []domain.ToolCall {
	result := make([]domain.ToolCall, 0, len(a.byID))
	for _, state := range a.byID {
		result = append(result, domain.ToolCall{
			ID:   state.ID,
			Type: "function",
			Function: domain.ToolFunction{
				Name:      state.Name,
				Arguments: state.Arguments,
			},
		})
	}
	return result
}

// Count returns the number of tool calls being tracked
func (a *ToolCallAccumulator) Count() int {
	return len(a.byID)
}

// HasToolCalls returns true if any tool calls have been accumulated
func (a *ToolCallAccumulator) HasToolCalls() bool {
	return len(a.byID) > 0
}
