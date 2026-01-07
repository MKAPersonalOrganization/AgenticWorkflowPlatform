package eventbus

import (
	"sync"
)

// GlobalContracts is the singleton contract registry
var GlobalContracts = NewContractRegistry()

// ContractRegistry manages all agent contracts
type ContractRegistry struct {
	mu        sync.RWMutex
	contracts map[string]*AgentContract // agent name -> contract
}

// NewContractRegistry creates a new contract registry
func NewContractRegistry() *ContractRegistry {
	return &ContractRegistry{
		contracts: make(map[string]*AgentContract),
	}
}

// Register adds an agent contract to the registry
func (r *ContractRegistry) Register(contract *AgentContract) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.contracts[contract.Name] = contract
}

// GetContract returns the contract for an agent by name
func (r *ContractRegistry) GetContract(name string) (*AgentContract, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	contract, ok := r.contracts[name]
	return contract, ok
}
