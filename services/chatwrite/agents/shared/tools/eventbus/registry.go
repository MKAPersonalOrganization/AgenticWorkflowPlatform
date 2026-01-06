package eventbus

import (
	"sync"

	"school-with-ai/services/chatwrite/agents"
)

// GlobalContracts is the singleton contract registry
var GlobalContracts = NewContractRegistry()

// ContractRegistry manages all agent contracts and provides routing capabilities
type ContractRegistry struct {
	mu        sync.RWMutex
	contracts map[string]*AgentContract // agent name -> contract
	eventBus  *EventBus
}

// NewContractRegistry creates a new contract registry
func NewContractRegistry() *ContractRegistry {
	return &ContractRegistry{
		contracts: make(map[string]*AgentContract),
		eventBus:  NewEventBus(),
	}
}

// Register adds an agent contract to the registry.
// It automatically subscribes the agent to its ListensTo events.
func (r *ContractRegistry) Register(contract *AgentContract) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.contracts[contract.Name] = contract

	// Subscribe agent to its ListensTo events
	r.eventBus.Subscribe(contract.Name, contract.ListensTo)
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

// EventBus returns the underlying event bus for direct access
func (r *ContractRegistry) EventBus() *EventBus {
	return r.eventBus
}
