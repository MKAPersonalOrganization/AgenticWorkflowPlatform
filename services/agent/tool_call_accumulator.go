package agent

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

// ToolCallUpdate represents the result of processing a delta
type ToolCallUpdate struct {
	ID          string
	Name        string
	IsNew       bool   // true if this is a new tool call
	ArgsDelta   string // the new arguments added in this delta
}

// ProcessDelta processes a streaming tool call delta
// Returns the update info and whether the delta was processed
func (a *ToolCallAccumulator) ProcessDelta(delta llm.ToolCallDelta) (ToolCallUpdate, bool) {
	index := delta.Index

	// Check if this is a new tool call (has ID and Name)
	if delta.ID != "" && delta.Name != "" {
		state := &toolCallState{
			ID:        delta.ID,
			Index:     index,
			Name:      delta.Name,
			Arguments: delta.Arguments,
		}
		a.byID[delta.ID] = state
		a.byIndex[index] = state

		return ToolCallUpdate{
			ID:        delta.ID,
			Name:      delta.Name,
			IsNew:     true,
			ArgsDelta: delta.Arguments,
		}, true
	}

	// This is a continuation - find by index
	state, exists := a.byIndex[index]
	if !exists {
		return ToolCallUpdate{}, false
	}

	// Accumulate arguments
	if delta.Arguments != "" {
		state.Arguments += delta.Arguments
	}

	return ToolCallUpdate{
		ID:        state.ID,
		Name:      state.Name,
		IsNew:     false,
		ArgsDelta: delta.Arguments,
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
