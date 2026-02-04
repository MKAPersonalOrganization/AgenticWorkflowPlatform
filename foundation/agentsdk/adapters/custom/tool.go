package custom

import (
	"context"
	"encoding/json"
	"fmt"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
)

// ToolAdapter adapts core.Tool to the agent.Tool interface.
type ToolAdapter struct {
	tool core.Tool
}

// NewToolAdapter creates an adapter from a core.Tool.
func NewToolAdapter(tool core.Tool) agent.Tool {
	return &ToolAdapter{tool: tool}
}

// Name returns the tool's function name.
func (t *ToolAdapter) Name() string {
	return t.tool.Name()
}

// Description returns the tool's description.
func (t *ToolAdapter) Description() string {
	return t.tool.Description()
}

// InputSchema returns the tool's parameter schema.
func (t *ToolAdapter) InputSchema() map[string]any {
	return t.tool.Parameters()
}

// Execute runs the tool with the given context and arguments.
func (t *ToolAdapter) Execute(ctx context.Context, toolCtx *agent.ToolContext) (*agent.ToolResult, error) {
	// Convert agent.ToolContext to core.ToolContext
	coreCtx := core.ToolContext{
		Context:    toolCtx.Context,
		ThreadID:   toolCtx.SessionID,
		MessageID:  "", // Not directly available in agent.ToolContext
		ToolCallID: toolCtx.ToolCallID,
		RequestCtx: nil, // Would need to be passed through metadata if needed
	}

	// Parse arguments from JSON string to map
	var args map[string]any
	if toolCtx.Arguments != "" {
		if err := json.Unmarshal([]byte(toolCtx.Arguments), &args); err != nil {
			return &agent.ToolResult{
				ToolCallID: toolCtx.ToolCallID,
				Error:      err.Error(),
				IsError:    true,
			}, nil
		}
	}

	// Invoke the tool
	result, err := t.tool.Invoke(coreCtx, args)
	if err != nil {
		return &agent.ToolResult{
			ToolCallID: toolCtx.ToolCallID,
			Error:      err.Error(),
			IsError:    true,
		}, nil
	}

	// Handle core.ToolResult wrapping
	actualResult := result
	if tr, ok := result.(core.ToolResult); ok {
		actualResult = tr.Result
	} else if tr, ok := result.(*core.ToolResult); ok {
		actualResult = tr.Result
	}

	// Convert result to string
	var output string
	if s, ok := actualResult.(string); ok {
		output = s
	} else if actualResult != nil {
		resultJSON, err := json.Marshal(actualResult)
		if err != nil {
			return &agent.ToolResult{
				ToolCallID: toolCtx.ToolCallID,
				Error:      fmt.Sprintf("failed to marshal tool result: %v", err),
				IsError:    true,
			}, nil
		}
		output = string(resultJSON)
	}

	return &agent.ToolResult{
		ToolCallID: toolCtx.ToolCallID,
		Output:     output,
		IsError:    false,
	}, nil
}
