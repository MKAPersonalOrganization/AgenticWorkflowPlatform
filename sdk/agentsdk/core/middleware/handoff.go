package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"school-with-ai/sdk/agentsdk/core"
	"sort"
	"strings"
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
	availableAgents map[string]string // agent_name -> description
	reqCtx          *core.RequestContext
}

// NewTransferToAgentTool creates a new TransferToAgentTool.
func NewTransferToAgentTool(availableAgents map[string]string, reqCtx *core.RequestContext) *TransferToAgentTool {
	return &TransferToAgentTool{
		availableAgents: availableAgents,
		reqCtx:          reqCtx,
	}
}

func (t *TransferToAgentTool) Name() string {
	return "transfer_to_agent"
}

func (t *TransferToAgentTool) Description() string {
	// Build description with available agents
	var sb strings.Builder
	sb.WriteString(TransferToAgentToolDescription)
	sb.WriteString("\n\nAvailable agents:\n")

	// Sort agents by name for consistent ordering
	agentNames := make([]string, 0, len(t.availableAgents))
	for name := range t.availableAgents {
		agentNames = append(agentNames, name)
	}
	sort.Strings(agentNames)

	for _, name := range agentNames {
		desc := t.availableAgents[name]
		sb.WriteString(fmt.Sprintf("- %s: %s\n", name, desc))
	}

	return sb.String()
}

func (t *TransferToAgentTool) Parameters() map[string]any {
	// Build enum of available agent names
	agentNames := make([]string, 0, len(t.availableAgents))
	for name := range t.availableAgents {
		agentNames = append(agentNames, name)
	}
	sort.Strings(agentNames)

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

func (t *TransferToAgentTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args struct {
		AgentName string `json:"agent_name"`
		Question  string `json:"question"`
		Context   string `json:"context"`
	}

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		return "", fmt.Errorf("failed to parse transfer arguments: %w", err)
	}

	// Validate agent_name is provided
	if args.AgentName == "" {
		return "", errors.New("agent_name is required")
	}

	// Prevent self-transfer
	currentAgentName := t.reqCtx.Config.Name
	if args.AgentName == currentAgentName {
		return "", fmt.Errorf("cannot transfer to yourself (%s). If you can handle this task, answer directly instead of transferring", currentAgentName)
	}

	// Validate agent exists
	if _, exists := t.availableAgents[args.AgentName]; !exists {
		agentNames := make([]string, 0, len(t.availableAgents))
		for name := range t.availableAgents {
			agentNames = append(agentNames, name)
		}

		return "", fmt.Errorf("unknown agent: %s. Available agents: %v", args.AgentName, agentNames)
	}

	// Store handoff in request context state
	handoff := Handoff{
		TargetAgent: args.AgentName,
		Question:    args.Question,
		Context:     args.Context,
	}

	SetHandoff(t.reqCtx, handoff)

	return "Successfully transferred to " + args.AgentName, nil
}

// HandoffMiddleware provides agent handoff/transfer capabilities.
type HandoffMiddleware struct {
	availableAgents map[string]string // agent_name -> description
	systemPrompt    string
}

// NewHandoffMiddleware creates a new HandoffMiddleware with specified available agents.
func NewHandoffMiddleware(availableAgents map[string]string) *HandoffMiddleware {
	return &HandoffMiddleware{
		availableAgents: availableAgents,
		systemPrompt:    HandoffSystemPrompt,
	}
}

// NewHandoffMiddlewareWithPrompt creates a new HandoffMiddleware with custom system prompt.
func NewHandoffMiddlewareWithPrompt(availableAgents map[string]string, systemPrompt string) *HandoffMiddleware {
	return &HandoffMiddleware{
		availableAgents: availableAgents,
		systemPrompt:    systemPrompt,
	}
}

// Process implements the Middleware interface.
func (m *HandoffMiddleware) Process(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
	// Add transfer_to_agent tool
	reqCtx.AddTool(NewTransferToAgentTool(m.availableAgents, reqCtx))

	// Add system prompt with available agents list
	promptWithAgents := m.buildSystemPromptWithAgents()
	reqCtx.AddSystemPrompt(promptWithAgents)

	// Continue with next middleware
	return next(ctx, reqCtx)
}

// buildSystemPromptWithAgents creates the system prompt including the list of available agents.
func (m *HandoffMiddleware) buildSystemPromptWithAgents() string {
	var sb strings.Builder
	sb.WriteString(m.systemPrompt)
	sb.WriteString("\n\nAvailable agents:\n")

	// Sort agents by name for consistent ordering
	agentNames := make([]string, 0, len(m.availableAgents))
	for name := range m.availableAgents {
		agentNames = append(agentNames, name)
	}
	sort.Strings(agentNames)

	for _, name := range agentNames {
		desc := m.availableAgents[name]
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", name, desc))
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
