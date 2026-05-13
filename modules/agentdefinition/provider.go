package agentdefinition

import (
	"aicompass/foundation/agent"
	"context"
	"encoding/json"
	"fmt"
)

// UserAwareProvider resolves user-scoped agent definitions into runtime agents.
type UserAwareProvider struct {
	repo AgentDefinitionRepository
}

// NewUserAwareProvider creates a provider backed by the agent-definition repository.
func NewUserAwareProvider(repo AgentDefinitionRepository) *UserAwareProvider {
	return &UserAwareProvider{repo: repo}
}

// Get requires user context; callers should use GetForUser instead.
func (p *UserAwareProvider) Get(name string) (agent.Agent, error) {
	return nil, fmt.Errorf("agent %q requires user context; use GetForUser", name)
}

// GetForUser resolves an agent by ID for a specific user.
func (p *UserAwareProvider) GetForUser(ctx context.Context, name, userID string) (agent.Agent, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required to resolve agent %q", name)
	}
	if name == "" {
		return nil, fmt.Errorf("an agent ID must be provided")
	}

	agentDef, err := p.repo.GetByID(ctx, AgentID(name), UserID(userID))
	if err != nil {
		return nil, fmt.Errorf("agent %q not found for user %s: %w", name, userID, err)
	}
	return p.agentFromDefinition(agentDef), nil
}

// List returns an empty list because agent visibility is user-scoped.
func (p *UserAwareProvider) List() []agent.Agent {
	return []agent.Agent{}
}

// ListForUser lists all runtime agents available to a user.
func (p *UserAwareProvider) ListForUser(ctx context.Context, userID string) ([]agent.Agent, error) {
	if userID == "" {
		return nil, nil
	}

	definitions, err := p.repo.ListByUser(ctx, UserID(userID))
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}
	agents := make([]agent.Agent, 0, len(definitions))
	for i := range definitions {
		agents = append(agents, p.agentFromDefinition(&definitions[i]))
	}
	return agents, nil
}

func (p *UserAwareProvider) agentFromDefinition(def *AgentDefinition) agent.Agent {
	cfg := agent.Config{
		Name:         def.ID,
		Description:  def.Description,
		SystemPrompt: def.SystemPrompt,
		TransferPolicy: agent.TransferPolicy{
			DisallowTransferToParent: def.DisallowTransferToParent,
			DisallowTransferToPeers:  def.DisallowTransferToPeers,
		},
		OutputKey:           def.OutputKey,
		IncludeContents:     agent.IncludeContents(def.IncludeContents),
		GlobalInstruction:   def.GlobalInstruction,
		InputSchema:         parseAgentSchema(def.InputSchemaJSON),
		OutputSchema:        parseAgentSchema(def.OutputSchemaJSON),
		ModelConfig:         toAgentModelConfig(def.ModelConfig),
		ModelOverride:       toAgentModelOverride(def.ModelOverride),
		MemoryPolicy:        toFoundationMemoryPolicy(def.MemoryPolicy),
		ArtifactPolicy:      toFoundationArtifactPolicy(def.ArtifactPolicy),
		CodeExecutionPolicy: toFoundationCodeExecutionPolicy(def.CodeExecutionPolicy),
		StreamingPolicy:     toFoundationStreamingPolicy(def.StreamingPolicy),
		AgentTools:          toFoundationAgentToolRefs(def.AgentTools),
	}

	if len(def.SubAgents) > 0 {
		subAgents := make([]agent.Agent, 0, len(def.SubAgents))
		for _, ref := range def.SubAgents {
			if ref.AgentID == "" {
				continue
			}
			subDef, err := p.repo.GetByID(context.Background(), AgentID(ref.AgentID), UserID(def.UserID))
			if err != nil {
				continue
			}
			subAgents = append(subAgents, p.agentFromDefinition(subDef))
		}
		cfg.SubAgents = subAgents
	}

	a := newConfigAgent(cfg)
	if def.IsComposed() {
		return &composedAgentAdapter{
			dbAgent: dbAgent{
				Agent:       a,
				displayName: def.Name,
				id:          def.ID,
				icon:        def.Icon,
			},
			graph: convertCompositionNode(def.Graph),
		}
	}

	return &dbAgent{
		Agent:       a,
		displayName: def.Name,
		id:          def.ID,
		icon:        def.Icon,
	}
}

type dbAgent struct {
	agent.Agent
	displayName string
	id          string
	icon        string
}

func (a *dbAgent) ID() agent.ID             { return a.id }
func (a *dbAgent) Name() string             { return a.displayName }
func (a *dbAgent) Description() string      { return a.Agent.Description() }
func (a *dbAgent) Icon() string             { return a.icon }
func (a *dbAgent) SubAgents() []agent.Agent { return a.Agent.SubAgents() }
func (a *dbAgent) Config() agent.Config     { return a.Agent.(agent.Configurable).Config() }

type composedAgentAdapter struct {
	dbAgent
	graph agent.WorkflowNodeInfo
}

func (a *composedAgentAdapter) WorkflowGraph() agent.WorkflowNodeInfo { return a.graph }

var _ agent.WorkflowAgent = (*composedAgentAdapter)(nil)

func parseAgentSchema(jsonStr string) *agent.Schema {
	if jsonStr == "" {
		return nil
	}
	var s agent.Schema
	if err := json.Unmarshal([]byte(jsonStr), &s); err != nil {
		return nil
	}
	return &s
}

func toAgentModelConfig(src *AgentModelConfig) *agent.AgentModelConfig {
	if src == nil {
		return nil
	}
	return &agent.AgentModelConfig{
		Temperature:     src.Temperature,
		TopP:            src.TopP,
		TopK:            src.TopK,
		MaxOutputTokens: src.MaxOutputTokens,
		StopSequences:   src.StopSequences,
	}
}

func toAgentModelOverride(src *AgentModelOverride) *agent.AgentModelOverride {
	if src == nil {
		return nil
	}
	return &agent.AgentModelOverride{
		Provider:  src.Provider,
		ModelName: src.ModelName,
	}
}

// toFoundationMemoryPolicy converts the persisted memory policy to the
// foundation type consumed by the adapter. Nil means memory is off.
func toFoundationMemoryPolicy(src *AgentMemoryPolicy) *agent.AgentMemoryPolicy {
	if src == nil {
		return nil
	}
	return &agent.AgentMemoryPolicy{
		Enabled: src.Enabled,
		Preload: src.Preload,
	}
}

// toFoundationArtifactPolicy converts the persisted artifact policy to the
// foundation type consumed by the adapter. Nil means artifacts are off.
func toFoundationArtifactPolicy(src *AgentArtifactPolicy) *agent.AgentArtifactPolicy {
	if src == nil {
		return nil
	}
	return &agent.AgentArtifactPolicy{
		Enabled:           src.Enabled,
		AutoSaveUserBlobs: src.AutoSaveUserBlobs,
	}
}

// toFoundationCodeExecutionPolicy converts the persisted policy to the
// foundation type consumed by the adapter. Nil means code execution is
// off — adapter never tries to add the provider's code-interpreter tool.
func toFoundationCodeExecutionPolicy(src *AgentCodeExecutionPolicy) *agent.AgentCodeExecutionPolicy {
	if src == nil {
		return nil
	}
	return &agent.AgentCodeExecutionPolicy{
		Enabled: src.Enabled,
	}
}

// toFoundationStreamingPolicy converts the persisted policy to the
// foundation type consumed by the adapter. Nil means "stream" — matches
// the adapter's historical default before this field existed.
func toFoundationStreamingPolicy(src *AgentStreamingPolicy) *agent.AgentStreamingPolicy {
	if src == nil {
		return nil
	}
	return &agent.AgentStreamingPolicy{
		Enabled: src.Enabled,
	}
}

// toFoundationAgentToolRefs converts persisted AgentToolRefs to the
// foundation type consumed by the adapter.
func toFoundationAgentToolRefs(src []*AgentToolRef) []agent.AgentToolRef {
	if len(src) == 0 {
		return nil
	}
	out := make([]agent.AgentToolRef, 0, len(src))
	for _, ref := range src {
		if ref == nil || ref.AgentID == "" {
			continue
		}
		out = append(out, agent.AgentToolRef{
			AgentID:           ref.AgentID,
			SkipSummarization: ref.SkipSummarization,
		})
	}
	return out
}

func convertCompositionNode(n *CompositionNode) agent.WorkflowNodeInfo {
	if n == nil {
		return agent.WorkflowNodeInfo{}
	}
	info := agent.WorkflowNodeInfo{
		ID:             n.ID,
		Type:           string(n.Type),
		Label:          n.Label,
		AgentID:        n.AgentID,
		A2AConnectorID: n.A2AConnectorID,
		MaxIterations:  n.MaxIterations,
		AllowEarlyExit: n.AllowEarlyExit,
		Condition:      n.Condition,
	}
	if len(n.Children) > 0 {
		info.Children = make([]agent.WorkflowNodeInfo, len(n.Children))
		for i := range n.Children {
			info.Children[i] = convertCompositionNode(&n.Children[i])
		}
	}
	return info
}

type configAgent struct {
	config agent.Config
}

func newConfigAgent(cfg agent.Config) agent.Agent {
	return &configAgent{config: cfg}
}

func (a *configAgent) ID() agent.ID             { return a.config.Name }
func (a *configAgent) Name() string             { return a.config.Name }
func (a *configAgent) Description() string      { return a.config.Description }
func (a *configAgent) SubAgents() []agent.Agent { return a.config.SubAgents }
func (a *configAgent) Config() agent.Config     { return a.config }

var _ agent.Configurable = (*configAgent)(nil)
