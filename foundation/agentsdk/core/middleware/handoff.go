package middleware

import (
	"context"
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
		},
		"required": []string{"agent_name"},
	}
}

func (t *TransferToAgentTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		AgentName string `json:"agent_name"`
	}

	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse transfer arguments: %w", err)
	}

	// Validate agent_name is provided
	if params.AgentName == "" {
		return nil, ErrAgentNameRequired
	}

	// Prevent self-transfer
	if params.AgentName == t.reqCtx.Config.Name {
		return nil, fmt.Errorf("%w (%s)", ErrSelfTransfer, params.AgentName)
	}

	// Store handoff in request context state
	t.reqCtx.State["handoff"] = params.AgentName

	return core.NewToolResultWithCommand(
		map[string]any{"status": fmt.Sprintf("Transferred to %s", params.AgentName)},
		&core.Command{
			Goto:          params.AgentName,
			StopIteration: true,
		},
	), nil
}

// HandoffMiddleware provides agent handoff/transfer capabilities.
type HandoffMiddleware struct{}

// NewHandoffMiddleware creates a new HandoffMiddleware.
func NewHandoffMiddleware() *HandoffMiddleware {
	return &HandoffMiddleware{}
}

// Process implements the Middleware interface.
func (m *HandoffMiddleware) Process(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
	// Skip if no handoffs configured
	if len(reqCtx.Config.Handoffs) == 0 {
		return next(ctx, reqCtx)
	}

	// Add transfer_to_agent tool
	reqCtx.AddTool(NewTransferToAgentTool(reqCtx.Config.Handoffs, reqCtx))

	// Build and add system prompt with available agents
	var sb strings.Builder
	sb.WriteString(HandoffSystemPrompt)
	sb.WriteString("\n\nAvailable agents:\n")
	for _, handoff := range reqCtx.Config.Handoffs {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", handoff.AgentName, handoff.Description))
	}
	reqCtx.AddSystemPrompt(sb.String())

	// Continue with next middleware
	return next(ctx, reqCtx)
}
