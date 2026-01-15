package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"school-with-ai/foundation/agentsdk/core"
	"sort"
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
	if _, exists := t.availableAgents[params.AgentName]; !exists {
		agentNames := make([]string, 0, len(t.availableAgents))
		for name := range t.availableAgents {
			agentNames = append(agentNames, name)
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

	messages := reqCtx.Messages

	// Determine the current agent name
	// First try from Config, then fall back to checking recent assistant messages
	currentAgentName := ""
	if reqCtx.Config != nil && reqCtx.Config.Name != "" {
		currentAgentName = reqCtx.Config.Name
	} else {
		// Fall back: look for the most recent assistant message's author name
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "assistant" && messages[i].AuthorName != "" {
				currentAgentName = messages[i].AuthorName
				break
			}
		}
	}

	// Check if this agent is receiving control back from another agent
	// Look at recent messages (not just the last one, since tool results are added after transfers)
	var returnHandoffPrompt string

	// Check the last few messages for a return handoff
	// We need to look back a bit because after transfer_to_agent is called,
	// a tool result message is added, so the transfer isn't the last message
	startIdx := len(messages) - 5
	if startIdx < 0 {
		startIdx = 0
	}

	for i := len(messages) - 1; i >= startIdx; i-- {
		msg := messages[i]

		// Check if this is an assistant message with function calls
		if msg.Role == "assistant" {
			functionCalls := msg.FunctionCalls()
			for _, fc := range functionCalls {
				if fc.Name == "transfer_to_agent" {
					// Parse the arguments to see if this is a return handoff
					var args struct {
						AgentName string `json:"agent_name"`
						Context   string `json:"context"`
					}
					if err := json.Unmarshal([]byte(fc.ArgumentsJSON()), &args); err == nil {
						if args.AgentName == currentAgentName && currentAgentName != "" {
							// This agent is receiving control back!
							// The Context field contains the result from the previous agent
							returnHandoffPrompt = fmt.Sprintf(`
## IMPORTANT: You Are Receiving Control Back

The previous agent has completed their work and returned control to you.

**Their result:** %s

**Your task:** Relay this result to the user in a natural, conversational way. 

**CRITICAL:** DO NOT call transfer_to_agent again. The work is done. Simply respond to the user with the result.
`, args.Context)
							// Found it, stop looking
							goto found
						}
					}
				}
			}
		}
	}
found:

	// Build system prompt
	promptWithAgents := m.buildSystemPromptWithAgents()

	// If this is a return handoff, prepend the special instructions
	if returnHandoffPrompt != "" {
		promptWithAgents = returnHandoffPrompt + "\n\n" + promptWithAgents
	}

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
