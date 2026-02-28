package custom

import (
	"context"
	"fmt"
	"sort"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
)

// UserAwareAgentProvider implements agent.AgentProvider by loading all agents
// (including the default assistant) from the database. Every user gets a
// default agent created lazily on first access, stored in user_agents with
// is_default=true. User-created agents are stored identically — the system
// treats them all the same way.
type UserAwareAgentProvider struct {
	repo domain.AgentDefinitionRepository

	// Middleware hooks injected at construction so every agent built from a
	// definition gets the same observability wiring.
	agentMiddlewares    []agent.Middleware
	chatMiddlewares     []agent.Middleware
	functionMiddlewares []agent.Middleware
}

// NewUserAwareAgentProvider creates a DB-only provider.
// Middlewares are optional hooks applied to every agent built from the DB.
func NewUserAwareAgentProvider(
	repo domain.AgentDefinitionRepository,
	agentMw []agent.Middleware,
	chatMw []agent.Middleware,
	funcMw []agent.Middleware,
) *UserAwareAgentProvider {
	return &UserAwareAgentProvider{
		repo:                repo,
		agentMiddlewares:    agentMw,
		chatMiddlewares:     chatMw,
		functionMiddlewares: funcMw,
	}
}

// Get retrieves an agent by name. Without user context, this can only return
// an error directing callers to use GetForUser.
func (p *UserAwareAgentProvider) Get(name string) (agent.Agent, error) {
	return nil, fmt.Errorf("agent %q requires user context; use GetForUser", name)
}

// GetForUser retrieves an agent by ID for a specific user. If name is empty,
// returns the user's default agent (creating it lazily if needed).
func (p *UserAwareAgentProvider) GetForUser(ctx context.Context, name, userID string) (agent.Agent, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required to resolve agent %q", name)
	}

	// Empty name → default agent
	if name == "" {
		def, err := p.repo.EnsureDefaultAgent(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("resolving default agent: %w", err)
		}
		return p.agentFromDefinition(def), nil
	}

	// Try to load agent by ID
	agentDef, err := p.repo.GetByID(ctx, name, userID)
	if err != nil {
		return nil, fmt.Errorf("agent %q not found for user %s: %w", name, userID, err)
	}

	return p.agentFromDefinition(agentDef), nil
}

// List returns an empty list — without user context we cannot list agents.
func (p *UserAwareAgentProvider) List() []agent.Agent {
	return []agent.Agent{}
}

// ListForUser returns all agents (default + user-created) for the given user.
// The default agent is created lazily if it doesn't exist yet.
func (p *UserAwareAgentProvider) ListForUser(ctx context.Context, userID string) ([]agent.Agent, error) {
	if userID == "" {
		return nil, nil
	}

	// Ensure the default agent exists before listing
	if _, err := p.repo.EnsureDefaultAgent(ctx, userID); err != nil {
		return nil, fmt.Errorf("ensuring default agent: %w", err)
	}

	definitions, err := p.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}

	agents := make([]agent.Agent, 0, len(definitions))
	for i := range definitions {
		agents = append(agents, p.agentFromDefinition(&definitions[i]))
	}

	// Default agent first, then sorted by name
	sort.Slice(agents, func(i, j int) bool {
		ai := agents[i].(*dbAgent)
		aj := agents[j].(*dbAgent)
		if ai.isDefault != aj.isDefault {
			return ai.isDefault // default comes first
		}
		return ai.Name() < aj.Name()
	})

	return agents, nil
}

// agentFromDefinition builds an agent.Agent from a domain.AgentDefinition.
// All agents — default or user-created — go through this single path.
func (p *UserAwareAgentProvider) agentFromDefinition(def *domain.AgentDefinition) agent.Agent {
	cfg := agent.Config{
		Name:          def.ID, // Use UUID as routing key
		Description:   def.Description,
		SystemPrompt:  def.SystemPrompt,
		ModelID:       def.ModelID,
		MaxIterations: def.MaxIterations,
		Tools:         []agent.Tool{},
		Requirements: []agent.Requirement{
			agent.NewRequireUserInput(),
		},
		AgentMiddlewares:    p.agentMiddlewares,
		ChatMiddlewares:     p.chatMiddlewares,
		FunctionMiddlewares: p.functionMiddlewares,
	}

	a, _ := NewAgent(cfg)
	return &dbAgent{
		Agent:       a,
		displayName: def.Name,
		id:          def.ID,
		isDefault:   def.IsDefault,
	}
}

// dbAgent wraps a CustomAgent to carry display metadata from the DB row.
// Every agent (default or user-created) is a dbAgent — no special types.
type dbAgent struct {
	agent.Agent
	displayName string
	id          string
	isDefault   bool
}

func (a *dbAgent) ID() agent.ID        { return a.id }
func (a *dbAgent) Name() string        { return a.displayName }
func (a *dbAgent) Description() string { return a.Agent.Description() }
func (a *dbAgent) IsDefault() bool     { return a.isDefault }
