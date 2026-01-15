package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"school-with-ai/foundation/agentsdk/core"
	"strings"
)

var (
	// ErrAgentNameRequired is returned when agent_name is missing from transfer arguments.
	ErrAgentNameRequired = errors.New("agent_name is required")

	// ErrSelfTransfer is returned when an agent attempts to transfer to itself.
	ErrSelfTransfer = errors.New("cannot transfer to yourself")

	// ErrUnknownAgent is returned when attempting to transfer to a non-existent agent.
	ErrUnknownAgent = errors.New("unknown agent")
)

const TransferToAgentToolDescription = `Transfer control to another specialist agent when they are better suited to handle the question.

Use this tool to delegate tasks to agents with specific expertise. Review the available agents below and choose the most appropriate one based on their description and the nature of the task.

When transferring:
- Provide a clear question or task for the target agent
- Include relevant context from the conversation
- Only transfer if the target agent is better suited than you
- Do not generate additional text - just call the tool

After calling this tool, wait for the target agent to complete their work and return control.`

const HandoffSystemPrompt = `## Agent Transfer

You can transfer questions to specialist agents using the transfer_to_agent tool.

Transfer guidelines:
1. Review available agents and their expertise areas
2. Use transfer_to_agent only when another agent is better suited
3. Provide clear question and context when transferring
4. Wait for specialist to complete and return control
5. If you are the best agent for the task, answer directly
6. When transferring, call the tool without additional commentary`

// Handoff represents a transfer request to another agent.
type Handoff struct {
	TargetAgent string `json:"target_agent"` // Name of the agent to transfer to
	Question    string `json:"question"`     // Question or task for the target agent
	Context     string `json:"context"`      // Relevant context from the conversation
}

// TransferToAgentTool is the tool for transferring to another agent.
type TransferToAgentTool struct {
	handoffs []core.Handoff
	reqCtx   *core.RequestContext
}

// NewTransferToAgentTool creates a new TransferToAgentTool.
func NewTransferToAgentTool(handoffs []core.Handoff, reqCtx *core.RequestContext) *TransferToAgentTool {
	return &TransferToAgentTool{
		handoffs: handoffs,
		reqCtx:   reqCtx,
	}
}

func (t *TransferToAgentTool) Name() string {
	return "transfer_to_agent"
}

func (t *TransferToAgentTool) Description() string {
	var sb strings.Builder
	sb.WriteString(TransferToAgentToolDescription)
	sb.WriteString("\n\nAvailable agents:\n")

	for _, handoff := range t.handoffs {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", handoff.AgentName, handoff.Description))
	}

	return sb.String()
}

func (t *TransferToAgentTool) Parameters() map[string]any {
	// Build enum of available agent names
	agentNames := make([]string, 0, len(t.handoffs))
	for _, handoff := range t.handoffs {
		agentNames = append(agentNames, handoff.AgentName)
	}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_name": map[string]any{
				"type":        "string",
				"description": "The name of the agent to transfer to",
				"enum":        agentNames,
			},
			"question": map[string]any{
				"type":        "string",
				"description": "The question or task for the specialist agent. Can be empty when returning to coordinator with results.",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Relevant context from the conversation. When returning to coordinator, use this to provide your results/answer.",
			},
		},
		"required": []string{"agent_name"},
	}
}

func (t *TransferToAgentTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		AgentName string `json:"agent_name"`
		Question  string `json:"question"`
		Context   string `json:"context"`
	}

	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse transfer arguments: %w", err)
	}

	// Validate agent_name is provided
	if params.AgentName == "" {
		return nil, ErrAgentNameRequired
	}

	// Prevent self-transfer
	currentAgentName := t.reqCtx.Config.Name
	if params.AgentName == currentAgentName {
		return nil, fmt.Errorf("%w (%s). If you can handle this task, answer directly instead of transferring", ErrSelfTransfer, currentAgentName)
	}

	// Validate agent exists
	found := false
	for _, handoff := range t.handoffs {
		if handoff.AgentName == params.AgentName {
			found = true
			break
		}
	}
	if !found {
		agentNames := make([]string, 0, len(t.handoffs))
		for _, handoff := range t.handoffs {
			agentNames = append(agentNames, handoff.AgentName)
		}
		return nil, fmt.Errorf("%w: %s. Available agents: %v", ErrUnknownAgent, params.AgentName, agentNames)
	}

	// Store handoff in request context state for task management to detect
	handoff := Handoff{
		TargetAgent: params.AgentName,
		Question:    params.Question,
		Context:     params.Context,
	}

	SetHandoff(t.reqCtx, handoff)

	// In the new pattern, we return a ToolResult that wraps the display map
	// AND includes a Command for the engine to handle the routing/stop iteration.
	resultMap := map[string]any{
		"status":      fmt.Sprintf("Successfully transferred to %s", params.AgentName),
		"next_agent":  params.AgentName,
		"is_transfer": true,
	}

	return core.NewToolResultWithCommand(resultMap, &core.Command{
		Goto:          params.AgentName,
		StopIteration: true,
	}), nil
}

// HandoffMiddleware provides agent handoff/transfer capabilities.
// It reads handoff configuration from the agent's config.Handoffs field,
// following LangGraph's pattern where agents declaratively specify their handoffs.
type HandoffMiddleware struct {
	systemPrompt string
}

// NewHandoffMiddleware creates a new HandoffMiddleware.
// Handoff destinations are read from the agent's config.Handoffs field.
func NewHandoffMiddleware() *HandoffMiddleware {
	return &HandoffMiddleware{
		systemPrompt: HandoffSystemPrompt,
	}
}

// NewHandoffMiddlewareWithPrompt creates a new HandoffMiddleware with custom system prompt.
// Handoff destinations are read from the agent's config.Handoffs field.
func NewHandoffMiddlewareWithPrompt(systemPrompt string) *HandoffMiddleware {
	return &HandoffMiddleware{
		systemPrompt: systemPrompt,
	}
}

// Process implements the Middleware interface.
func (m *HandoffMiddleware) Process(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
	// Skip if no handoffs configured
	if len(reqCtx.Config.Handoffs) == 0 {
		return next(ctx, reqCtx)
	}

	// Add transfer_to_agent tool
	reqCtx.AddTool(NewTransferToAgentTool(reqCtx.Config.Handoffs, reqCtx))

	// Check if this agent is receiving control back from another agent
	returnHandoffPrompt := m.detectReturnHandoff(reqCtx)

	// Build and add system prompt
	promptWithAgents := m.buildSystemPromptWithAgents(reqCtx.Config.Handoffs)
	if returnHandoffPrompt != "" {
		promptWithAgents = returnHandoffPrompt + "\n\n" + promptWithAgents
	}
	reqCtx.AddSystemPrompt(promptWithAgents)

	// Continue with next middleware
	return next(ctx, reqCtx)
}

// detectReturnHandoff checks if this agent is receiving control back from another agent.
func (m *HandoffMiddleware) detectReturnHandoff(reqCtx *core.RequestContext) string {
	currentAgentName := reqCtx.Config.Name
	if currentAgentName == "" {
		return ""
	}

	messages := reqCtx.Messages
	// Check the last few messages for a return handoff
	startIdx := len(messages) - 5
	if startIdx < 0 {
		startIdx = 0
	}

	for i := len(messages) - 1; i >= startIdx; i-- {
		msg := messages[i]
		if msg.Role != "assistant" {
			continue
		}

		for _, fc := range msg.FunctionCalls() {
			if fc.Name != "transfer_to_agent" {
				continue
			}

			var args struct {
				AgentName string `json:"agent_name"`
				Context   string `json:"context"`
			}
			if err := json.Unmarshal([]byte(fc.ArgumentsJSON()), &args); err != nil {
				continue
			}

			if args.AgentName == currentAgentName {
				// This agent is receiving control back!
				return fmt.Sprintf(`
## IMPORTANT: You Are Receiving Control Back

The previous agent has completed their work and returned control to you.

**Their result:** %s

**Your task:** Relay this result to the user in a natural, conversational way. 

**CRITICAL:** DO NOT call transfer_to_agent again. The work is done. Simply respond to the user with the result.
`, args.Context)
			}
		}
	}

	return ""
}

// buildSystemPromptWithAgents creates the system prompt including the list of available agents.
func (m *HandoffMiddleware) buildSystemPromptWithAgents(handoffs []core.Handoff) string {
	var sb strings.Builder
	sb.WriteString(m.systemPrompt)
	sb.WriteString("\n\nAvailable agents:\n")

	for _, handoff := range handoffs {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", handoff.AgentName, handoff.Description))
	}

	return sb.String()
}

// State helper functions

// GetHandoff retrieves the current handoff from request context state.
func GetHandoff(reqCtx *core.RequestContext) (*Handoff, bool) {
	val, ok := reqCtx.State["handoff"]
	if !ok {
		return nil, false
	}
	h, ok := val.(Handoff)
	if !ok {
		return nil, false
	}

	return &h, true
}

// SetHandoff stores a handoff in the request context state.
func SetHandoff(reqCtx *core.RequestContext, h Handoff) {
	reqCtx.State["handoff"] = h
}

// ClearHandoff removes the handoff from request context state.
func ClearHandoff(reqCtx *core.RequestContext) {
	delete(reqCtx.State, "handoff")
}

// HasHandoff checks if a handoff is present in the request context.
func HasHandoff(reqCtx *core.RequestContext) bool {
	_, exists := reqCtx.State["handoff"]

	return exists
}
