package middleware

import (
	"context"
	"encoding/json"
	"fmt"

	"school-with-ai/sdk/agentsdk/core"
)

// WhiteboardAction represents the type of whiteboard update
type WhiteboardAction string

const (
	WhiteboardActionWrite  WhiteboardAction = "write"  // Replace entire content
	WhiteboardActionAppend WhiteboardAction = "append" // Append to existing content
	WhiteboardActionClear  WhiteboardAction = "clear"  // Clear all content
)

const ReadWhiteboardToolDescription = `Read the current content of the shared whiteboard. 

The whiteboard is a shared space where all agents can view and contribute information. It typically contains:
- Routing plans and task progress (TODOS section)
- Key formulas and equations from specialists
- Diagrams and visualizations
- Important notes and context

Always read the whiteboard at the start of your work to understand the current context and avoid duplicating effort.`

const WriteWhiteboardToolDescription = `Write information to the shared whiteboard that all agents can see. Use this to share key formulas, diagrams, or important points. ALWAYS use markdown format.

Actions:
- 'write': REPLACE entire whiteboard content (use read_whiteboard first to preserve existing content)
- 'append': Add new content to the end
- 'clear': Erase all content

Best practice: Use read_whiteboard() first, then modify the content, then write_whiteboard() with action='write' to update specific sections while preserving others.

Example workflow for updating a section:
1. content = read_whiteboard()
2. Modify the specific section you need (e.g., update TODOS)
3. Keep other sections unchanged (e.g., formulas from specialists)
4. write_whiteboard(updated_content, action='write')`

const WhiteboardSystemPrompt = `## Whiteboard Tools

You have access to read_whiteboard and write_whiteboard tools for collaborative information sharing.

The whiteboard is a shared markdown document visible to all agents. Use it to:
- Share routing plans and task progress
- Display formulas, equations, and calculations
- Show diagrams and visualizations
- Communicate important context between agents

Best practices:
- ALWAYS read_whiteboard() first to check existing content
- Use read-modify-write pattern: read → update specific sections → write entire content
- Structure content with markdown headers (# Section Name)
- Preserve content from other agents when updating
- Use action='write' (not 'append') when updating existing sections to avoid duplication`

// ReadWhiteboardTool is the tool for reading the whiteboard
type ReadWhiteboardTool struct {
	reqCtx *core.RequestContext
}

// NewReadWhiteboardTool creates a new ReadWhiteboardTool
func NewReadWhiteboardTool(reqCtx *core.RequestContext) *ReadWhiteboardTool {
	return &ReadWhiteboardTool{
		reqCtx: reqCtx,
	}
}

func (t *ReadWhiteboardTool) Name() string {
	return "read_whiteboard"
}

func (t *ReadWhiteboardTool) Description() string {
	return ReadWhiteboardToolDescription
}

func (t *ReadWhiteboardTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *ReadWhiteboardTool) Invoke(ctx context.Context, arguments string) (string, error) {
	content := GetWhiteboardContent(t.reqCtx)
	if content == "" {
		return "The whiteboard is currently empty.", nil
	}
	return content, nil
}

// WriteWhiteboardTool is the tool for writing to the whiteboard
type WriteWhiteboardTool struct {
	reqCtx *core.RequestContext
}

// NewWriteWhiteboardTool creates a new WriteWhiteboardTool
func NewWriteWhiteboardTool(reqCtx *core.RequestContext) *WriteWhiteboardTool {
	return &WriteWhiteboardTool{
		reqCtx: reqCtx,
	}
}

func (t *WriteWhiteboardTool) Name() string {
	return "write_whiteboard"
}

func (t *WriteWhiteboardTool) Description() string {
	return WriteWhiteboardToolDescription
}

func (t *WriteWhiteboardTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to write to the whiteboard (use markdown format)",
			},
			"action": map[string]interface{}{
				"type":        "string",
				"description": "How to update: 'write' (replace entire content - use read_whiteboard first to preserve sections), 'append' (add to end), 'clear' (erase all)",
				"enum":        []string{"write", "append", "clear"},
			},
		},
		"required": []string{"content", "action"},
	}
}

func (t *WriteWhiteboardTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args struct {
		Content string           `json:"content"`
		Action  WhiteboardAction `json:"action"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse whiteboard arguments: %w", err)
	}

	// Validate action
	validActions := map[WhiteboardAction]bool{
		WhiteboardActionWrite:  true,
		WhiteboardActionAppend: true,
		WhiteboardActionClear:  true,
	}

	if !validActions[args.Action] {
		return "", fmt.Errorf("invalid action: %s (must be write, append, or clear)", args.Action)
	}

	// Update whiteboard based on action
	switch args.Action {
	case WhiteboardActionWrite:
		SetWhiteboardContent(t.reqCtx, args.Content)
	case WhiteboardActionAppend:
		current := GetWhiteboardContent(t.reqCtx)
		if current == "" {
			SetWhiteboardContent(t.reqCtx, args.Content)
		} else {
			SetWhiteboardContent(t.reqCtx, current+"\n"+args.Content)
		}
	case WhiteboardActionClear:
		SetWhiteboardContent(t.reqCtx, "")
	}

	return fmt.Sprintf("Whiteboard updated (%s): %s", args.Action, args.Content), nil
}

// WhiteboardMiddleware provides whiteboard management capabilities to agents
type WhiteboardMiddleware struct {
	systemPrompt string
}

// NewWhiteboardMiddleware creates a new WhiteboardMiddleware with default prompts
func NewWhiteboardMiddleware() *WhiteboardMiddleware {
	return &WhiteboardMiddleware{
		systemPrompt: WhiteboardSystemPrompt,
	}
}

// NewWhiteboardMiddlewareWithPrompt creates a new WhiteboardMiddleware with custom prompt
func NewWhiteboardMiddlewareWithPrompt(systemPrompt string) *WhiteboardMiddleware {
	return &WhiteboardMiddleware{
		systemPrompt: systemPrompt,
	}
}

// Process implements the Middleware interface
func (m *WhiteboardMiddleware) Process(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
	// Add whiteboard tools to the request context
	reqCtx.AddTool(NewReadWhiteboardTool(reqCtx))
	reqCtx.AddTool(NewWriteWhiteboardTool(reqCtx))
	reqCtx.AddSystemPrompt(m.systemPrompt)

	// Initialize whiteboard state if not present
	if _, exists := reqCtx.State["whiteboard_content"]; !exists {
		reqCtx.State["whiteboard_content"] = ""
	}

	return next(ctx, reqCtx)
}

// GetWhiteboardContent retrieves the current whiteboard content from the request context
func GetWhiteboardContent(reqCtx *core.RequestContext) string {
	if content, ok := reqCtx.State["whiteboard_content"].(string); ok {
		return content
	}
	return ""
}

// SetWhiteboardContent updates the whiteboard content in the request context
func SetWhiteboardContent(reqCtx *core.RequestContext, content string) {
	reqCtx.State["whiteboard_content"] = content
}
