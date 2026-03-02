package custom

import (
	"context"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
)

// UserAwareAgentProvider implements agent.AgentProvider by loading all agents
// from the database. Every user gets a starter agent created lazily on first
// access if they have none. All agents are equal — there is no "default".
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
// returns the user's first agent (creating a starter if they have none).
func (p *UserAwareAgentProvider) GetForUser(ctx context.Context, name, userID string) (agent.Agent, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required to resolve agent %q", name)
	}

	// Empty name → first agent (starter created lazily if needed)
	if name == "" {
		def, err := p.repo.EnsureInitialAgent(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("resolving initial agent: %w", err)
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

// ListForUser returns all agents for the given user.
// A starter agent is created lazily if the user has none.
func (p *UserAwareAgentProvider) ListForUser(ctx context.Context, userID string) ([]agent.Agent, error) {
	if userID == "" {
		return nil, nil
	}

	// Ensure at least one agent exists before listing
	if _, err := p.repo.EnsureInitialAgent(ctx, userID); err != nil {
		return nil, fmt.Errorf("ensuring initial agent: %w", err)
	}

	definitions, err := p.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}

	agents := make([]agent.Agent, 0, len(definitions))
	for i := range definitions {
		agents = append(agents, p.agentFromDefinition(&definitions[i]))
	}

	// Sorted by creation time (oldest first) — inherited from the DB query.
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
		icon:        def.Icon,
	}
}

// dbAgent wraps a CustomAgent to carry display metadata from the DB row.
type dbAgent struct {
	agent.Agent
	displayName string
	id          string
	icon        string
}

func (a *dbAgent) ID() agent.ID        { return a.id }
func (a *dbAgent) Name() string        { return a.displayName }
func (a *dbAgent) Description() string { return a.Agent.Description() }
func (a *dbAgent) Icon() string        { return a.icon }
