package core

import (
	"school-with-ai/foundation/agentsdk/internal/llm"
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

// ProcessUpdate processes an llm.Response and extracts tool call information.
// Returns a slice of update info and whether any tool call content was found.
func (a *ToolCallAccumulator) ProcessUpdate(resp *llm.Response) ([]ToolCallUpdate, bool) {
	// Get function calls from the response
	functionCalls := resp.FunctionCalls()
	if len(functionCalls) == 0 {
		return nil, false
	}

	var updates []ToolCallUpdate

	for i, fc := range functionCalls {
		// Try to get index from Extensions
		currentIndex := i
		if val, ok := resp.Extensions["index"]; ok {
			if idx, ok := val.(int64); ok {
				currentIndex = int(idx)
			} else if idx, ok := val.(int); ok {
				currentIndex = idx
			}
		}

		// Check if this is a new tool call (has ID and Name)
		if fc.ID != "" && fc.Name != "" {
			state := &toolCallState{
				ID:        fc.ID,
				Index:     currentIndex,
				Name:      fc.Name,
				Arguments: string(fc.Arguments),
			}
			a.byID[fc.ID] = state
			a.byIndex[currentIndex] = state

			updates = append(updates, ToolCallUpdate{
				ID:        fc.ID,
				Name:      fc.Name,
				IsNew:     true,
				ArgsDelta: string(fc.Arguments),
			})
			continue
		}

		// This is a continuation - find by index or ID
		state, exists := a.byIndex[currentIndex]
		if !exists && fc.ID != "" {
			state, exists = a.byID[fc.ID]
		}

		if exists {
			// Accumulate arguments
			delta := string(fc.Arguments)
			if delta != "" {
				state.Arguments += delta
			}

			updates = append(updates, ToolCallUpdate{
				ID:        state.ID,
				Name:      state.Name,
				IsNew:     false,
				ArgsDelta: delta,
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
