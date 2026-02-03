package custom

import (
	"encoding/json"
	"fmt"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
)

// ToolWrapper adapts an agent.Tool to the core.Tool interface.
type ToolWrapper struct {
	tool agent.Tool
}

// WrapTool wraps an agent.Tool.
func WrapTool(t agent.Tool) core.Tool {
	return &ToolWrapper{tool: t}
}

func (w *ToolWrapper) Name() string {
	return w.tool.Name()
}

func (w *ToolWrapper) Description() string {
	return w.tool.Description()
}

func (w *ToolWrapper) Parameters() map[string]any {
	return w.tool.InputSchema()
}

func (w *ToolWrapper) Invoke(ctx core.ToolContext, args any) (any, error) {
	// Convert core.ToolContext to agent.ToolContext
	// Note: We need to serialize args back to JSON string because agent.Tool expects JSON string
	// This is a bit inefficient but required for the abstraction boundary.
	// Or we can check if args is already string (it might be map[string]any).

	argsBytes, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args for agent tool: %w", err)
	}

	agentCtx := &agent.ToolContext{
		Context:    ctx,
		ToolCallID: ctx.ToolCallID,
		SessionID:  ctx.ThreadID,
		UserID:     "", // TODO: Get UserID from core context if available
		Arguments:  string(argsBytes),
		// Metadata:   ctx.State(),
	}

	// Try to get UserID and Metadata from RequestCtx if available
	if ctx.RequestCtx != nil {
		// ctx.RequestCtx.Config.Name?
		// Need to find where UserID is stored.
		// In runner.go, core.RunContext (not RequestContext) has UserID.
		// core.ToolContext has RequestCtx which has State.
	}

	result, err := w.tool.Execute(ctx, agentCtx)
	if err != nil {
		return nil, err
	}

	// Result is *agent.ToolResult (struct { Content []ContentPart ... })
	// core expects any (will be marshaled to JSON).
	// Likely we want to return the result directly or formatted.

	return result, nil
}
