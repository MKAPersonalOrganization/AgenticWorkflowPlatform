package agentstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"aicompass/foundation/logger"
)

// Composition tree limits prevent unbounded nesting.
const (
	maxTreeDepth = 5
	maxTreeNodes = 50
)

// service manages user-created agent definitions (both simple and composed).
type service struct {
	log  *logger.Logger
	repo AgentDefinitionRepository
}

func newService(log *logger.Logger, repo AgentDefinitionRepository) *service {
	return &service{log: log, repo: repo}
}

// create validates and persists a new agent definition.
func (s *service) create(ctx context.Context, agent *AgentDefinition) error {
	if agent.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if agent.UserID == "" {
		return fmt.Errorf("user ID is required")
	}

	// Validate model config if present.
	if err := agent.ModelConfig.Validate(); err != nil {
		return fmt.Errorf("invalid model config: %w", err)
	}

	// Validate model override if present.
	if err := agent.ModelOverride.Validate(); err != nil {
		return fmt.Errorf("invalid model override: %w", err)
	}

	// Validate composition graph if present.
	if agent.Graph != nil {
		if err := s.validateGraph(ctx, agent.Graph, agent.UserID); err != nil {
			return fmt.Errorf("invalid composition: %w", err)
		}
	}

	s.log.Info(ctx, "creating agent definition",
		"userID", agent.UserID,
		"name", agent.Name,
		"composed", agent.IsComposed(),
	)

	if err := s.repo.Create(ctx, agent); err != nil {
		return fmt.Errorf("creating agent: %w", err)
	}
	return nil
}

// update modifies an existing agent definition.
func (s *service) update(ctx context.Context, agent *AgentDefinition) error {
	if agent.ID == "" {
		return fmt.Errorf("agent ID is required")
	}
	if agent.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if agent.Name == "" {
		return fmt.Errorf("agent name is required")
	}

	// Validate model config if present.
	if err := agent.ModelConfig.Validate(); err != nil {
		return fmt.Errorf("invalid model config: %w", err)
	}

	// Validate model override if present.
	if err := agent.ModelOverride.Validate(); err != nil {
		return fmt.Errorf("invalid model override: %w", err)
	}

	// Validate composition graph if present.
	if agent.Graph != nil {
		if err := s.validateGraph(ctx, agent.Graph, agent.UserID); err != nil {
			return fmt.Errorf("invalid composition: %w", err)
		}
	}

	s.log.Info(ctx, "updating agent definition",
		"agentID", agent.ID,
		"userID", agent.UserID,
		"composed", agent.IsComposed(),
	)

	if err := s.repo.Update(ctx, agent); err != nil {
		return fmt.Errorf("updating agent: %w", err)
	}
	return nil
}

// delete removes an agent definition.
func (s *service) delete(ctx context.Context, id, userID string) error {
	s.log.Info(ctx, "deleting agent definition", "agentID", id, "userID", userID)

	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("deleting agent: %w", err)
	}
	return nil
}

// getByID retrieves a single agent by ID.
func (s *service) getByID(ctx context.Context, id, userID string) (*AgentDefinition, error) {
	return s.repo.GetByID(ctx, id, userID)
}

// listByUser returns all agents created by a specific user.
// Agents that are referenced as sub-agents inside a workflow graph are
// excluded so they don't appear as standalone entries in the list.
func (s *service) listByUser(ctx context.Context, userID string) ([]AgentDefinition, error) {
	agents, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return excludeSubAgents(agents), nil
}

// search finds agents matching a query string within a user's agents.
// Sub-agents consumed by workflows are excluded from results.
func (s *service) search(ctx context.Context, userID, query string) ([]AgentDefinition, error) {
	agents, err := s.repo.Search(ctx, userID, query)
	if err != nil {
		return nil, err
	}
	return excludeSubAgents(agents), nil
}

// excludeSubAgents removes agents that are referenced as leaf nodes inside
// any workflow agent's composition graph. This keeps the list clean —
// users see only top-level agents and workflows, not the internal building
// blocks that are already represented inside a workflow tree.
func excludeSubAgents(agents []AgentDefinition) []AgentDefinition {
	// Collect every agent ID that appears as a sub-agent in any graph.
	consumed := make(map[string]bool)
	for i := range agents {
		if agents[i].Graph != nil {
			for _, id := range agents[i].Graph.CollectAgentIDs() {
				consumed[id] = true
			}
		}
	}
	if len(consumed) == 0 {
		return agents
	}

	filtered := make([]AgentDefinition, 0, len(agents))
	for i := range agents {
		if !consumed[agents[i].ID] {
			filtered = append(filtered, agents[i])
		}
	}
	return filtered
}

// ── Sharing ──

// share generates a share token for an agent, making it discoverable.
func (s *service) share(ctx context.Context, id, userID string) (string, error) {
	// Check the agent exists and is owned by the user.
	agent, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return "", fmt.Errorf("agent not found: %w", err)
	}

	// If already shared, return existing token.
	if agent.ShareToken != "" && agent.Visibility == "shared" {
		return agent.ShareToken, nil
	}

	token, err := generateShareToken()
	if err != nil {
		return "", fmt.Errorf("generating share token: %w", err)
	}

	if err := s.repo.UpdateShareToken(ctx, id, userID, token, "shared"); err != nil {
		return "", fmt.Errorf("sharing agent: %w", err)
	}

	s.log.Info(ctx, "agent shared", "agentID", id, "userID", userID)
	return token, nil
}

// unshare revokes the share token for an agent.
func (s *service) unshare(ctx context.Context, id, userID string) error {
	if err := s.repo.UpdateShareToken(ctx, id, userID, "", "private"); err != nil {
		return fmt.Errorf("unsharing agent: %w", err)
	}
	s.log.Info(ctx, "agent unshared", "agentID", id, "userID", userID)
	return nil
}

// getShared retrieves a shared agent by its token.
func (s *service) getShared(ctx context.Context, token string) (*AgentDefinition, error) {
	return s.repo.GetByShareToken(ctx, token)
}

// forkShared creates a copy of a shared agent for the target user.
func (s *service) forkShared(ctx context.Context, token, targetUserID string) (*AgentDefinition, error) {
	source, err := s.repo.GetByShareToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("shared agent not found: %w", err)
	}

	fork := &AgentDefinition{
		UserID:                   targetUserID,
		Name:                     source.Name,
		Description:              source.Description,
		SystemPrompt:             source.SystemPrompt,
		Icon:                     source.Icon,
		DisallowTransferToParent: source.DisallowTransferToParent,
		DisallowTransferToPeers:  source.DisallowTransferToPeers,
		OutputKey:                source.OutputKey,
		IncludeContents:          source.IncludeContents,
		GlobalInstruction:        source.GlobalInstruction,
		InputSchemaJSON:          source.InputSchemaJSON,
		OutputSchemaJSON:         source.OutputSchemaJSON,
	}

	// For composed agents, fork all referenced sub-agents and remap IDs.
	if source.IsComposed() {
		// Create copies of all referenced sub-agents.
		agentIDs := source.Graph.CollectAgentIDs()
		idMap := make(map[string]string, len(agentIDs))

		for _, origID := range agentIDs {
			subAgent, subErr := s.repo.GetByID(ctx, origID, source.UserID)
			if subErr != nil {
				// Sub-agent may have been deleted — skip.
				continue
			}
			subFork := &AgentDefinition{
				UserID:                   targetUserID,
				Name:                     subAgent.Name,
				Description:              subAgent.Description,
				SystemPrompt:             subAgent.SystemPrompt,
				Icon:                     subAgent.Icon,
				DisallowTransferToParent: subAgent.DisallowTransferToParent,
				DisallowTransferToPeers:  subAgent.DisallowTransferToPeers,
				OutputKey:                subAgent.OutputKey,
				IncludeContents:          subAgent.IncludeContents,
			}
			if createErr := s.repo.Create(ctx, subFork); createErr != nil {
				return nil, fmt.Errorf("forking sub-agent %q: %w", subAgent.Name, createErr)
			}
			idMap[origID] = subFork.ID
		}

		// Deep-copy the graph and remap agent IDs.
		graphCopy := deepCopyGraph(source.Graph)
		remapGraphAgentIDs(graphCopy, idMap)
		fork.Graph = graphCopy
	}

	if err := s.repo.Create(ctx, fork); err != nil {
		return nil, fmt.Errorf("creating forked agent: %w", err)
	}

	s.log.Info(ctx, "agent forked from shared",
		"sourceAgent", source.ID,
		"targetUser", targetUserID,
		"forkID", fork.ID,
	)
	return fork, nil
}

// ── Composition Graph Validation ──

// validateGraph recursively validates the entire composition tree.
// A workflow agent's root must be a composite node (sequential/parallel/loop)
// since its purpose is to orchestrate sub-agents.
func (s *service) validateGraph(ctx context.Context, node *CompositionNode, userID string) error {
	// Root must be composite — a workflow agent orchestrates sub-agents.
	// A single agent doesn't need a composition graph.
	if node.Type == NodeAgent || node.Type == NodeRemote {
		return fmt.Errorf("root node must be sequential, parallel, or loop (use a simple agent definition for a single LLM agent)")
	}
	if node.CountNodes() > maxTreeNodes {
		return fmt.Errorf("tree exceeds maximum of %d nodes", maxTreeNodes)
	}
	if node.Depth() > maxTreeDepth {
		return fmt.Errorf("tree exceeds maximum depth of %d", maxTreeDepth)
	}
	return s.validateNode(ctx, node, userID, 0)
}

// validateNode validates a single node and recurses into children.
func (s *service) validateNode(ctx context.Context, node *CompositionNode, userID string, depth int) error {
	if !ValidNodeTypes[node.Type] {
		return fmt.Errorf("node %q: invalid type %q", node.ID, node.Type)
	}

	switch node.Type {
	case NodeAgent:
		if node.AgentID == "" {
			return fmt.Errorf("node %q: agent node requires agent_id", node.ID)
		}
		if len(node.Children) > 0 {
			return fmt.Errorf("node %q: agent node cannot have children", node.ID)
		}
		// Verify the referenced agent exists and belongs to the user.
		_, err := s.repo.GetByID(ctx, node.AgentID, userID)
		if err != nil {
			return fmt.Errorf("node %q: agent %q not found: %w", node.ID, node.AgentID, err)
		}

	case NodeRemote:
		if node.A2AConnectorID == "" {
			return fmt.Errorf("node %q: remote node requires a2a_connector_id", node.ID)
		}
		if len(node.Children) > 0 {
			return fmt.Errorf("node %q: remote node cannot have children", node.ID)
		}

	case NodeSequential, NodeParallel, NodeLoop:
		if len(node.Children) == 0 {
			return fmt.Errorf("node %q: %s node requires at least one child", node.ID, node.Type)
		}
		if node.AgentID != "" {
			return fmt.Errorf("node %q: composite node should not have agent_id", node.ID)
		}
		if node.Type == NodeLoop {
			if node.MaxIterations < 0 {
				node.MaxIterations = 0
			}
			if node.MaxIterations > 100 {
				return fmt.Errorf("node %q: max_iterations cannot exceed 100", node.ID)
			}
			if node.MaxIterations == 0 {
				node.MaxIterations = 3
			}
		}
		for i := range node.Children {
			if err := s.validateNode(ctx, &node.Children[i], userID, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// ── Helpers ──

func generateShareToken() (string, error) {
	b := make([]byte, 16) // 32 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func deepCopyGraph(src *CompositionNode) *CompositionNode {
	if src == nil {
		return nil
	}
	cp := *src
	if len(src.Children) > 0 {
		cp.Children = make([]CompositionNode, len(src.Children))
		for i := range src.Children {
			cp.Children[i] = *deepCopyGraph(&src.Children[i])
		}
	}
	return &cp
}

func remapGraphAgentIDs(node *CompositionNode, idMap map[string]string) {
	if node.Type == NodeAgent && node.AgentID != "" {
		if newID, ok := idMap[node.AgentID]; ok {
			node.AgentID = newID
		}
	}
	for i := range node.Children {
		remapGraphAgentIDs(&node.Children[i], idMap)
	}
}
