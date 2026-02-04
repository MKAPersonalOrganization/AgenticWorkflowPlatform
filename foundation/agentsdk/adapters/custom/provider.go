package custom

import (
	"fmt"
	"sort"

	"school-with-ai/foundation/agent"
)

// MapAgentProvider implements agent.AgentProvider using a simple map.
type MapAgentProvider struct {
	agents map[string]agent.Agent
}

// NewMapAgentProvider creates a new MapAgentProvider.
func NewMapAgentProvider(agents map[string]agent.Agent) *MapAgentProvider {
	return &MapAgentProvider{
		agents: agents,
	}
}

// Get retrieves an agent by name.
func (p *MapAgentProvider) Get(name string) (agent.Agent, error) {
	a, ok := p.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q not found", name)
	}
	return a, nil
}

// List returns all available agent names.
func (p *MapAgentProvider) List() []string {
	names := make([]string, 0, len(p.agents))
	for name := range p.agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
