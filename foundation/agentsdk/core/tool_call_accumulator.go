package core

import (
	"school-with-ai/foundation/agentsdk/llm"
)

// ToolCallAccumulator tracks streaming tool call deltas and builds complete tool calls.
type ToolCallAccumulator struct {
	byID    map[string]*toolCallState
	byIndex map[int]*toolCallState
}

// toolCallState tracks the state of a single tool call during streaming.
type toolCallState struct {
	ID        string
	Index     int
	Name      string
	Arguments string
}

// NewToolCallAccumulator creates a new accumulator.
func NewToolCallAccumulator() *ToolCallAccumulator {
	return &ToolCallAccumulator{
		byID:    make(map[string]*toolCallState),
		byIndex: make(map[int]*toolCallState),
	}
}

// ToolCallUpdate represents the result of processing an update.
type ToolCallUpdate struct {
	ID        string
	Name      string
	IsNew     bool   // true if this is a new tool call
	ArgsDelta string // the new arguments added in this update
}

// ProcessUpdate processes a ChatResponseUpdate and extracts tool call information.
// Returns a slice of update info and whether any tool call content was found.
func (a *ToolCallAccumulator) ProcessUpdate(update llm.ChatResponseUpdate) ([]ToolCallUpdate, bool) {
	// Get function calls from the update
	functionCalls := update.FunctionCalls()
	if len(functionCalls) == 0 {
		return nil, false
	}

	var updates []ToolCallUpdate
	index := update.ToolCallIndex

	for i, fc := range functionCalls {
		// If multiple function calls are present, the index usually increments
		// but providers vary. We use the updated index if provided for the first one,
		// and increment for subsequent ones if they are part of the same chunk.
		currentIndex := index + i

		// Check if this is a new tool call (has ToolCallID and Name)
		if fc.ToolCallID != "" && fc.Name != "" {
			state := &toolCallState{
				ID:        fc.ToolCallID,
				Index:     currentIndex,
				Name:      fc.Name,
				Arguments: fc.RawArguments,
			}
			a.byID[fc.ToolCallID] = state
			a.byIndex[currentIndex] = state

			updates = append(updates, ToolCallUpdate{
				ID:        fc.ToolCallID,
				Name:      fc.Name,
				IsNew:     true,
				ArgsDelta: fc.RawArguments,
			})
			continue
		}

		// This is a continuation - find by index or ID
		state, exists := a.byIndex[currentIndex]
		if !exists && update.ToolCallID != "" {
			state, exists = a.byID[update.ToolCallID]
		}

		if exists {
			// Accumulate arguments
			if fc.RawArguments != "" {
				state.Arguments += fc.RawArguments
			}

			updates = append(updates, ToolCallUpdate{
				ID:        state.ID,
				Name:      state.Name,
				IsNew:     false,
				ArgsDelta: fc.RawArguments,
			})
		}
	}

	return updates, len(updates) > 0
}

// GetAll returns all accumulated tool calls as ToolCall slices.
func (a *ToolCallAccumulator) GetAll() []ToolCall {
	result := make([]ToolCall, 0, len(a.byID))
	for _, state := range a.byID {
		result = append(result, ToolCall{
			ID:        state.ID,
			Type:      "function",
			Name:      state.Name,
			Arguments: state.Arguments,
		})
	}

	return result
}

// Count returns the number of tool calls being tracked.
func (a *ToolCallAccumulator) Count() int {
	return len(a.byID)
}

// HasToolCalls returns true if any tool calls have been accumulated.
func (a *ToolCallAccumulator) HasToolCalls() bool {
	return len(a.byID) > 0
}
