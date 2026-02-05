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

// List returns all available agents.
func (p *MapAgentProvider) List() []agent.Agent {
	agents := make([]agent.Agent, 0, len(p.agents))
	for _, a := range p.agents {
		agents = append(agents, a)
	}

	// Sort by name for consistency
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Name() < agents[j].Name()
	})

	return agents
}
