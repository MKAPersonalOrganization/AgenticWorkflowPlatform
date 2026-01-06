package eventbus

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents"
)

// GlobalContracts is the singleton contract registry
var GlobalContracts = NewContractRegistry()

// ContractRegistry manages all agent contracts and provides routing capabilities
type ContractRegistry struct {
	mu        sync.RWMutex
	contracts map[string]*AgentContract // agent name -> contract
	eventBus  *EventBus
	byTool    map[string]*AgentContract // emission tool name -> contract
}

// NewContractRegistry creates a new contract registry
func NewContractRegistry() *ContractRegistry {
	return &ContractRegistry{
		contracts: make(map[string]*AgentContract),
		eventBus:  NewEventBus(),
		byTool:    make(map[string]*AgentContract),
	}
}

// Register adds an agent contract to the registry.
// It automatically:
// - Subscribes the agent to its ListensTo events
// - Indexes emission tools for routing
func (r *ContractRegistry) Register(contract *AgentContract) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.contracts[contract.Name] = contract

	// Subscribe agent to its ListensTo events
	r.eventBus.Subscribe(contract.Name, contract.ListensTo)

	// Index by emission tool names (derived from what other agents emit)
	// When another agent emits an event that this agent listens to,
	// we need to be able to find this agent
	for _, et := range contract.ListensTo {
		toolName := EmissionToolName(et)
		r.byTool[toolName] = contract
	}
}

// GetContract returns the contract for an agent by name
func (r *ContractRegistry) GetContract(name string) (*AgentContract, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	contract, ok := r.contracts[name]
	return contract, ok
}

// RouteEvent returns the agent name that should handle a given event type.
// This is derived from subscriptions in the EventBus.
func (r *ContractRegistry) RouteEvent(eventType agents.EventType) (string, bool) {
	return r.eventBus.GetFirstSubscriber(eventType), r.eventBus.HasSubscribers(eventType)
}

// IsEmissionTool checks if a tool name is an emission tool
func (r *ContractRegistry) IsEmissionTool(toolName string) bool {
	if !strings.HasPrefix(toolName, "emit_") || !strings.HasSuffix(toolName, "_question") {
		return false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.byTool[toolName]
	return exists
}

// GetTargetAgentForTool returns the agent that should be activated when
// an emission tool is called
func (r *ContractRegistry) GetTargetAgentForTool(toolName string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	contract, exists := r.byTool[toolName]
	if !exists {
		return "", false
	}
	return contract.Name, true
}

// CreateEventFromToolCall creates a domain event from an emission tool call.
// This is used when the SDK detects a collaboration tool has been called.
func (r *ContractRegistry) CreateEventFromToolCall(
	threadID string,
	agentName string,
	toolCall domain.ToolCall,
	getWhiteboardState func(string) string,
) (*agents.DomainEvent, error) {
	toolName := toolCall.Function.Name

	// Parse the arguments
	var params struct {
		Question string `json:"question"`
		Request  string `json:"request"` // Alternative field name
		Context  string `json:"context"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
		return nil, fmt.Errorf("failed to parse %s arguments: %w", toolName, err)
	}

	// Use "question" or "request" field
	questionValue := params.Question
	if questionValue == "" {
		questionValue = params.Request
	}

	// Derive event type from tool name
	eventType := EventTypeFromToolName(toolName)

	return &agents.DomainEvent{
		ID:        agents.GenerateEventID(),
		Type:      eventType,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"question":         questionValue,
			"emitted_by":       agentName,
			"context":          params.Context,
			"whiteboard_state": getWhiteboardState(threadID),
		},
	}, nil
}

// CreateHandoffEvent creates an agent handoff event for UI display
func (r *ContractRegistry) CreateHandoffEvent(
	threadID string,
	fromAgent string,
	toolName string,
) *agents.DomainEvent {
	targetAgent, ok := r.GetTargetAgentForTool(toolName)
	if !ok {
		return nil
	}

	slug := EventTypeSlug(EventTypeFromToolName(toolName))
	// Capitalize first letter of slug for reason
	capitalizedSlug := slug
	if len(slug) > 0 {
		capitalizedSlug = string(slug[0]-32) + slug[1:]
	}
	reason := fmt.Sprintf("%s question requires specialist", capitalizedSlug)

	return agents.NewAgentHandoffEvent(threadID, fromAgent, targetAgent, reason)
}

// All returns all registered contracts
func (r *ContractRegistry) All() []*AgentContract {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*AgentContract, 0, len(r.contracts))
	for _, contract := range r.contracts {
		result = append(result, contract)
	}
	return result
}

// EventBus returns the event bus for direct access
func (r *ContractRegistry) EventBus() *EventBus {
	return r.eventBus
}

// GetEmissionToolsFor returns all emission tools that an agent is allowed to use.
// An agent can emit events that have subscribers (other agents listening).
func (r *ContractRegistry) GetEmissionToolsFor(agentName string) []core.Tool {
	r.mu.RLock()
	contract, ok := r.contracts[agentName]
	r.mu.RUnlock()

	if !ok {
		return nil
	}

	return GenerateEmissionTools(contract.Emits)
}
