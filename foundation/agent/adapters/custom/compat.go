package custom

import (
	"context"
	"encoding/json"
	"fmt"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
)

// ToolExecutionError represents an error from tool execution.
type ToolExecutionError struct {
	Message string
}

func (e *ToolExecutionError) Error() string {
	return fmt.Sprintf("tool execution error: %s", e.Message)
}

// WrapAgentTool converts an agent.Tool to core.Tool.
func WrapAgentTool(t agent.Tool) core.Tool {
	return &agentToolWrapper{tool: t}
}

type agentToolWrapper struct {
	tool agent.Tool
}

func (w *agentToolWrapper) Name() string               { return w.tool.Name() }
func (w *agentToolWrapper) Description() string        { return w.tool.Description() }
func (w *agentToolWrapper) Parameters() map[string]any { return w.tool.InputSchema() }

func (w *agentToolWrapper) Invoke(ctx core.ToolContext, args any) (any, error) {
	// Convert core.ToolContext to agent.ToolContext
	tctx := &agent.ToolContext{
		Context:    ctx.Context,
		SessionID:  ctx.ThreadID,
		UserID:     "", // Not available in core.ToolContext
		ToolCallID: ctx.ToolCallID,
		Arguments:  argsToJSON(args),
	}

	result, err := w.tool.Execute(ctx.Context, tctx)
	if err != nil {
		return nil, err
	}

	if result.IsError {
		// Return error as string, core doesn't have ToolExecutionError
		return nil, &ToolExecutionError{Message: result.Output}
	}

	return result.Output, nil
}

// WrapRequestMiddleware converts agent.RequestMiddleware to core.Middleware.
func WrapRequestMiddleware(mw agent.RequestMiddleware) core.Middleware {
	return &requestMiddlewareWrapper{middleware: mw}
}

type requestMiddlewareWrapper struct {
	middleware agent.RequestMiddleware
}

func (w *requestMiddlewareWrapper) Process(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
	// Convert core.RequestContext to agent.RequestContext
	agentReqCtx := &agent.RequestContext{
		ThreadID:              reqCtx.ThreadID,
		Messages:              convertCoreMsgsToAgent(reqCtx.Messages),
		SystemPrompt:          reqCtx.SystemPrompt,
		Tools:                 []agent.Tool{}, // We don't need to convert tools here
		Config:                nil,            // Config not needed for middleware
		State:                 reqCtx.State,
		SystemPromptAdditions: reqCtx.SystemPromptAdditions,
	}

	// Call the agent middleware
	err := w.middleware.Process(ctx, agentReqCtx, func(ctx context.Context, actx *agent.RequestContext) error {
		// Update core context with changes from agent context
		reqCtx.SystemPromptAdditions = append(reqCtx.SystemPromptAdditions, actx.SystemPromptAdditions...)
		return next(ctx, reqCtx)
	})

	return err
}

// argsToJSON converts args to JSON string.
func argsToJSON(args any) string {
	if str, ok := args.(string); ok {
		return str
	}
	data, _ := json.Marshal(args)
	return string(data)
}

// convertCoreMsgsToAgent converts llm.Message to agent.Message.
func convertCoreMsgsToAgent(msgs []llm.Message) []*agent.Message {
	result := make([]*agent.Message, len(msgs))
	for i, msg := range msgs {
		result[i] = &agent.Message{
			Role: agent.MessageRole(msg.Role),
			Content: []agent.Content{
				{
					Type: agent.ContentTypeText,
					Text: msg.Text(),
				},
			},
		}
	}
	return result
}
