package agentdefinition

import (
	"aicompass/foundation/logger"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Composition tree limits prevent unbounded nesting.
const (
	maxTreeDepth = 5
	maxTreeNodes = 50
)

// Service manages user-created agent definitions (both simple and composed).
type Service struct {
	log  *logger.Logger
	repo AgentDefinitionRepository
}

func newService(log *logger.Logger, repo AgentDefinitionRepository) *Service {
	return &Service{log: log, repo: repo}
}

// validate runs the shared invariants for create and update: required
// identifiers, model config/override shape, graph well-formedness,
// coordinator sub-agent refs, and agent-tool refs. AgentTools are
// orthogonal to SubAgents and Graph — any agent may expose other agents
// as callable tools.
func (s *Service) validate(ctx context.Context, agent *AgentDefinition) error {
	if agent.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if agent.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if err := agent.ModelConfig.Validate(); err != nil {
		return fmt.Errorf("invalid model config: %w", err)
	}
	if err := agent.ModelOverride.Validate(); err != nil {
		return fmt.Errorf("invalid model override: %w", err)
	}
	if agent.Graph != nil {
		if err := s.validateGraph(ctx, agent.Graph, agent.UserID); err != nil {
			return fmt.Errorf("invalid composition: %w", err)
		}
	}
	if len(agent.SubAgents) > 0 {
		if agent.Graph != nil {
			return fmt.Errorf("an agent cannot have both a composition graph and inline sub_agents")
		}
		for i, sa := range agent.SubAgents {
			if sa.AgentID == "" {
				return fmt.Errorf("sub_agents[%d]: agent_id is required", i)
			}
		}
	}
	return s.validateAgentTools(ctx, agent)
}

// create validates and persists a new agent definition.
func (s *Service) create(ctx context.Context, agent *AgentDefinition) error {
	if err := s.validate(ctx, agent); err != nil {
		return err
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

// ImportFromCatalogYAML creates agent definition(s) from catalog-format YAML (see ParseCatalogEntryYAML).
// Workflow entries create sub-agents plus a composed parent; simple entries create one LLM agent.
func (s *Service) ImportFromCatalogYAML(ctx context.Context, userID string, yamlBytes []byte) (agentID string, isComposed bool, err error) {
	entry, err := ParseCatalogEntryYAML(yamlBytes)
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(entry.Name) == "" {
		return "", false, fmt.Errorf("YAML must include name")
	}
	if entry.IsWorkflow() {
		id, wErr := s.ImportComposedFromCatalogEntry(ctx, userID, &entry, "", "")
		if wErr != nil {
			return "", false, wErr
		}
		return id, true, nil
	}
	if strings.TrimSpace(entry.SystemPrompt) == "" {
		return "", false, fmt.Errorf("YAML must include system_prompt for a single agent")
	}
	agent := &AgentDefinition{
		UserID:       userID,
		Name:         entry.Name,
		Description:  entry.Description,
		SystemPrompt: entry.SystemPrompt,
		Icon:         entry.Icon,
	}
	if err := s.create(ctx, agent); err != nil {
		return "", false, err
	}
	return agent.ID, false, nil
}

// ImportComposedFromCatalogEntry creates a composed agent from a catalog entry.
func (s *Service) ImportComposedFromCatalogEntry(ctx context.Context, userID string, entry *AgentCatalogEntry, sourceCatalogID string, customName string) (string, error) {
	var subAgents []CatalogSubAgent
	if entry.LLMAgentsJSON != "" {
		if err := json.Unmarshal([]byte(entry.LLMAgentsJSON), &subAgents); err != nil {
			return "", fmt.Errorf("parsing llm_agents: %w", err)
		}
	}

	// Collect output_key from workflow nodes in the raw JSON before unmarshalling.
	// Legacy templates declare output_key on workflow nodes; we transfer them to
	// the forked AgentDefinition.OutputKey where ADK-Go reads them at runtime.
	nodeOutputKeys := collectOutputKeysFromJSON([]byte(entry.WorkflowGraphJSON))

	var graph CompositionNode
	if err := json.Unmarshal([]byte(entry.WorkflowGraphJSON), &graph); err != nil {
		return "", fmt.Errorf("parsing workflow graph: %w", err)
	}

	idMap := make(map[string]string, len(subAgents))
	for _, sa := range subAgents {
		agent := &AgentDefinition{
			UserID:       userID,
			Name:         sa.Name,
			Description:  sa.Description,
			SystemPrompt: sa.SystemPrompt,
			Icon:         sa.Icon,
			OutputKey:    sa.OutputKey,
		}
		// Fall back to legacy workflow node output_key
		if agent.OutputKey == "" {
			agent.OutputKey = nodeOutputKeys[sa.ID]
		}
		if err := s.repo.Create(ctx, agent); err != nil {
			return "", fmt.Errorf("creating sub-agent %q: %w", sa.Name, err)
		}
		idMap[sa.ID] = agent.ID
	}

	remapGraphAgentIDs(&graph, idMap)

	if err := s.validateGraph(ctx, &graph, userID); err != nil {
		return "", fmt.Errorf("invalid workflow graph: %w", err)
	}

	agentName := entry.Name
	if customName != "" {
		agentName = customName
	}

	composed := &AgentDefinition{
		UserID:          userID,
		Name:            agentName,
		Description:     entry.Description,
		SystemPrompt:    entry.SystemPrompt,
		Icon:            entry.Icon,
		SourceCatalogID: sourceCatalogID,
		Graph:           &graph,
	}
	if err := s.repo.Create(ctx, composed); err != nil {
		return "", fmt.Errorf("creating workflow agent: %w", err)
	}

	// Establish ownership: set parent_id on all sub-agents so cascade delete works.
	for _, subID := range idMap {
		if err := s.repo.SetParent(ctx, AgentID(subID), AgentID(composed.ID), UserID(userID)); err != nil {
			s.log.Warn(ctx, "failed to set parent_id on sub-agent",
				"subAgentID", subID, "parentID", composed.ID, "error", err)
		}
	}

	return composed.ID, nil
}

// update modifies an existing agent definition.
func (s *Service) update(ctx context.Context, agent *AgentDefinition) error {
	if agent.ID == "" {
		return fmt.Errorf("agent ID is required")
	}
	if err := s.validate(ctx, agent); err != nil {
		return err
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

// delete removes an agent definition and cascade deletes all owned sub-agents.
func (s *Service) delete(ctx context.Context, id, userID string) error {
	s.log.Info(ctx, "deleting agent definition", "agentID", id, "userID", userID)

	// Fetch the agent to check if it owns sub-agents via graph references.
	agent, err := s.repo.GetByID(ctx, AgentID(id), UserID(userID))
	if err != nil {
		return fmt.Errorf("fetching agent: %w", err)
	}

	// If this is a composed agent (has a graph), cascade delete all referenced sub-agents.
	if agent.Graph != nil {
		subAgentIDs := agent.Graph.CollectAgentIDs()
		for _, subID := range subAgentIDs {
			if err := s.repo.Delete(ctx, AgentID(subID), UserID(userID)); err != nil {
				s.log.Warn(ctx, "failed to delete sub-agent during cascade",
					"subAgentID", subID, "error", err)
			}
		}
	}

	// Also delete any agents with parent_id = this agent's ID (ownership-based delete).
	if err := s.repo.DeleteByParent(ctx, AgentID(id), UserID(userID)); err != nil {
		s.log.Warn(ctx, "failed to delete sub-agents by parent_id",
			"parentID", id, "error", err)
	}

	return s.repo.Delete(ctx, AgentID(id), UserID(userID))
}

// getByID retrieves a single agent by ID.
func (s *Service) getByID(ctx context.Context, id, userID string) (*AgentDefinition, error) {
	return s.repo.GetByID(ctx, AgentID(id), UserID(userID))
}

// listByUser returns all agents created by a specific user.
// Agents that are referenced as sub-agents inside a workflow graph are
// excluded so they don't appear as standalone entries in the list.
func (s *Service) listByUser(ctx context.Context, userID string) ([]AgentDefinition, error) {
	agents, err := s.repo.ListByUser(ctx, UserID(userID))
	if err != nil {
		return nil, err
	}
	return excludeSubAgents(agents), nil
}

// StateKeySuggestion describes a template variable that can appear in an
// instruction prompt. It feeds the designer's "{"-triggered picker.
//
// Shape is driven by ADK's template rules in
// .READ/adk-go/internal/llminternal/instruction_processor.go — at runtime
// InjectSessionState replaces {key}, {key?}, {app:key}, {user:key},
// {temp:key}, {artifact.name}, {artifact.name?} with values from session
// state or the artifact store; anything that doesn't match the valid-state
// regex (^[a-zA-Z_][a-zA-Z0-9_]*$, optionally with an app:/user:/temp:
// prefix) is left as literal text.
//
// We can only enumerate OutputKey-driven session keys at design time.
// The other write patterns visible in ADK samples — callback-written
// state (.READ/adk-samples/.../callbacks/after_agent_callback/callback.py)
// and tool-written state (.READ/adk-samples/.../tools/…) — live in code
// we don't parse, so they can't be auto-suggested. Scoped prefixes
// (app:/user:/temp:) and {artifact.*} are surfaced by the UI as static
// hints since their names are only known at runtime.
type StateKeySuggestion struct {
	// Key is the bare identifier to insert between the braces (no '?' and
	// no leading '{'/'}'). For session-scoped OutputKeys it's a single
	// identifier; the UI composes the final `{key}` form.
	Key string `json:"key"`
	// Scope is "session" for OutputKey-backed keys. Reserved for future
	// values ("artifact", "user", "app", "temp") if we ever start tracking
	// those at design time.
	Scope string `json:"scope"`
	// Source is a human-readable hint about where the key comes from
	// (e.g. "self", "sub-agent output: Researcher"). Shown as secondary
	// text next to each suggestion in the picker.
	Source string `json:"source,omitempty"`
	// Priority ranks suggestions in the picker. Higher = more relevant
	// to the agent being edited. The ordering mirrors ADK's sample usage:
	// an agent's own OutputKey and its direct sub-agents' outputs are the
	// most common references, then workflow peers (sub-agent OutputKey →
	// sibling instruction is the dominant multi-agent read pattern), then
	// everything else in the user's library as a fallback.
	Priority int `json:"priority"`
}

// priority tiers for StateKeySuggestion. Values are coarse; only ordering
// matters. See GetAvailableStateKeys for where each tier is applied.
const (
	prioritySelf        = 100 // the agent's own OutputKey
	priorityDirectChild = 80  // sub-agent / agent-tool of the current agent
	priorityGraphChild  = 70  // leaf inside the current agent's own Graph
	priorityWorkflow    = 60  // peer in a workflow that contains this agent
	priorityLibrary     = 20  // any other OutputKey declared by the user
)

// GetAvailableStateKeys collects the template variables that can appear
// in this agent's instruction, ranked by how likely they are to be the
// right choice. The ranking matches ADK's sample corpus — a leaf agent
// inside a workflow overwhelmingly references OutputKeys set by earlier
// peers in the same workflow (pattern A in
// .READ/adk-samples/python/agents/academic-research/academic_research/agent.py
// and pattern B in
// .READ/adk-samples/python/agents/small-business-loan-agent/...callbacks/after_agent_callback/callback.py),
// so those are elevated above the rest of the library.
//
// Tiers (highest first):
//
//   - self           — the agent's own OutputKey
//   - sub-agent      — OutputKeys of agents referenced as SubAgents/AgentTools
//   - graph child    — OutputKeys of agents referenced inside this agent's Graph
//   - workflow peer  — OutputKeys of peers in any workflow that embeds this
//     agent (the reverse lookup — matches Sequential/Parallel/Loop layouts)
//   - library        — any other OutputKey declared across the user's agents,
//     so references still resolve if the workflow isn't wired yet
func (s *Service) GetAvailableStateKeys(ctx context.Context, userID, agentID string) ([]StateKeySuggestion, error) {
	agent, err := s.repo.GetByID(ctx, AgentID(agentID), UserID(userID))
	if err != nil {
		return nil, err
	}
	all, err := s.repo.ListByUser(ctx, UserID(userID))
	if err != nil {
		return nil, err
	}

	byID := make(map[string]*AgentDefinition, len(all))
	for i := range all {
		byID[all[i].ID] = &all[i]
	}

	acc := newStateKeyAccumulator()
	acc.addSelfKey(agent)
	acc.addDirectChildKeys(agent, byID)
	acc.addGraphChildKeys(agent, byID)
	acc.addWorkflowPeerKeys(agent, all, byID)
	acc.addLibraryKeys(agent, all)
	return acc.sorted(), nil
}

// stateKeyAccumulator is the shared mutable state for
// GetAvailableStateKeys' tier passes. Callers repeatedly invoke add with
// (key, source, priority) — the highest priority wins when a key shows
// up in multiple tiers, and source/priority both update on upgrade so
// the label matches the tier actually displayed.
type stateKeyAccumulator struct {
	best map[string]StateKeySuggestion
}

func newStateKeyAccumulator() *stateKeyAccumulator {
	return &stateKeyAccumulator{best: map[string]StateKeySuggestion{}}
}

func (a *stateKeyAccumulator) add(key, source string, priority int) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	if existing, ok := a.best[key]; ok && existing.Priority >= priority {
		return
	}
	a.best[key] = StateKeySuggestion{
		Key:      key,
		Scope:    "session",
		Source:   source,
		Priority: priority,
	}
}

func (a *stateKeyAccumulator) addSelfKey(agent *AgentDefinition) {
	if agent.OutputKey != "" {
		a.add(agent.OutputKey, "self", prioritySelf)
	}
}

// addDirectChildKeys surfaces OutputKeys from agents referenced as
// SubAgents or AgentTools. SubAgentRef may override the child's
// OutputKey at the call site — both the override and the child's own
// key are surfaced so either name works in the prompt.
func (a *stateKeyAccumulator) addDirectChildKeys(agent *AgentDefinition, byID map[string]*AgentDefinition) {
	for _, ref := range agent.SubAgents {
		if ref == nil {
			continue
		}
		child := byID[ref.AgentID]
		if ref.OutputKey != "" {
			source := "sub-agent output"
			if child != nil {
				source = "sub-agent output: " + child.Name
			}
			a.add(ref.OutputKey, source, priorityDirectChild)
		}
		if child != nil && child.OutputKey != "" {
			a.add(child.OutputKey, "sub-agent output: "+child.Name, priorityDirectChild)
		}
	}
	for _, ref := range agent.AgentTools {
		if ref == nil {
			continue
		}
		if child, ok := byID[ref.AgentID]; ok && child.OutputKey != "" {
			a.add(child.OutputKey, "agent-tool output: "+child.Name, priorityDirectChild)
		}
	}
}

// addGraphChildKeys surfaces OutputKeys of agents embedded inside this
// agent's own composition graph (sequential/parallel/loop children).
func (a *stateKeyAccumulator) addGraphChildKeys(agent *AgentDefinition, byID map[string]*AgentDefinition) {
	if agent.Graph == nil {
		return
	}
	for _, id := range agent.Graph.CollectAgentIDs() {
		if id == agent.ID {
			continue
		}
		if child, ok := byID[id]; ok && child.OutputKey != "" {
			a.add(child.OutputKey, "workflow step: "+child.Name, priorityGraphChild)
		}
	}
}

// addWorkflowPeerKeys does the reverse lookup: for every workflow agent
// that embeds the current agent, surface its peers' OutputKeys. This
// covers the dominant ADK pattern — editing a leaf that's a sub-agent
// inside a Sequential/Parallel/Loop, referencing an earlier peer's output.
func (a *stateKeyAccumulator) addWorkflowPeerKeys(agent *AgentDefinition, all []AgentDefinition, byID map[string]*AgentDefinition) {
	for i := range all {
		workflow := &all[i]
		if workflow.ID == agent.ID || workflow.Graph == nil {
			continue
		}
		members := workflow.Graph.CollectAgentIDs()
		if !slices.Contains(members, agent.ID) {
			continue
		}
		for _, id := range members {
			if id == agent.ID {
				continue
			}
			peer, ok := byID[id]
			if !ok || peer.OutputKey == "" {
				continue
			}
			a.add(peer.OutputKey,
				"workflow peer: "+workflow.Name+" → "+peer.Name,
				priorityWorkflow)
		}
	}
}

// addLibraryKeys is the fallback tier: any other OutputKey the user has
// declared, so a reference still resolves if the workflow isn't wired yet.
func (a *stateKeyAccumulator) addLibraryKeys(agent *AgentDefinition, all []AgentDefinition) {
	for i := range all {
		other := &all[i]
		if other.ID == agent.ID || other.OutputKey == "" {
			continue
		}
		a.add(other.OutputKey, "agent: "+other.Name, priorityLibrary)
	}
}

// sorted returns the accumulated suggestions by descending priority,
// tiebreaking alphabetically for stable output.
func (a *stateKeyAccumulator) sorted() []StateKeySuggestion {
	out := make([]StateKeySuggestion, 0, len(a.best))
	for _, sug := range a.best {
		out = append(out, sug)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// AnalyzeWorkflowLineage runs the static state-flow analyzer against the
// provided composition graph. Leaf agents referenced by the graph are
// loaded from the user's library so the analyzer can read their
// SystemPrompt / OutputKey / GlobalInstruction.
//
// `leafOverrides` lets the caller substitute in-flight (unsaved) leaf
// edits — when the designer is in mid-edit, the prompt the user is
// actually typing should drive the analysis, not the stale repo copy.
// Pass nil when no overrides are needed.
func (s *Service) AnalyzeWorkflowLineage(ctx context.Context, userID string, root *CompositionNode, leafOverrides map[string]*AgentDefinition) (StateLineage, error) {
	if root == nil {
		return StateLineage{
			ProducibleBefore: map[string][]string{},
			Produces:         map[string][]string{},
			References:       map[string][]Reference{},
		}, nil
	}
	all, err := s.repo.ListByUser(ctx, UserID(userID))
	if err != nil {
		return StateLineage{}, err
	}
	byID := make(map[string]*AgentDefinition, len(all))
	for i := range all {
		byID[all[i].ID] = &all[i]
	}
	// Apply caller-supplied overrides last — the in-flight edit wins
	// over the repo snapshot.
	for id, override := range leafOverrides {
		if override == nil {
			continue
		}
		byID[id] = override
	}
	return AnalyzeLineage(root, byID), nil
}

// GetConsumers returns all agents that reference the given agent ID in their graph.
// This identifies workflows that use this agent as a sub-agent.
func (s *Service) GetConsumers(ctx context.Context, userID, agentID string) ([]AgentDefinition, error) {
	agents, err := s.repo.ListByUser(ctx, UserID(userID))
	if err != nil {
		return nil, err
	}

	var consumers []AgentDefinition
	for i := range agents {
		agent := &agents[i]
		if agent.Graph == nil {
			continue
		}
		if agent.ID == agentID {
			continue
		}
		refs := agent.Graph.CollectAgentIDs()
		for _, refID := range refs {
			if refID == agentID {
				consumers = append(consumers, *agent)
				break
			}
		}
	}
	return consumers, nil
}

// search finds agents matching a query string within a user's agents.
// Sub-agents consumed by workflows are excluded from results.
func (s *Service) search(ctx context.Context, userID, query string) ([]AgentDefinition, error) {
	agents, err := s.repo.Search(ctx, UserID(userID), query)
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
func (s *Service) share(ctx context.Context, id, userID string) (string, error) {
	// Check the agent exists and is owned by the user.
	agent, err := s.repo.GetByID(ctx, AgentID(id), UserID(userID))
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

	if err := s.repo.UpdateShareToken(ctx, AgentID(id), UserID(userID), token, "shared"); err != nil {
		return "", fmt.Errorf("sharing agent: %w", err)
	}

	s.log.Info(ctx, "agent shared", "agentID", id, "userID", userID)
	return token, nil
}

// unshare revokes the share token for an agent.
func (s *Service) unshare(ctx context.Context, id, userID string) error {
	if err := s.repo.UpdateShareToken(ctx, AgentID(id), UserID(userID), "", "private"); err != nil {
		return fmt.Errorf("unsharing agent: %w", err)
	}
	s.log.Info(ctx, "agent unshared", "agentID", id, "userID", userID)
	return nil
}

// getShared retrieves a shared agent by its token.
func (s *Service) getShared(ctx context.Context, token string) (*AgentDefinition, error) {
	return s.repo.GetByShareToken(ctx, token)
}

// forkShared creates a copy of a shared agent for the target user.
func (s *Service) forkShared(ctx context.Context, token, targetUserID string) (*AgentDefinition, error) {
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
			subAgent, subErr := s.repo.GetByID(ctx, AgentID(origID), UserID(source.UserID))
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

// ── Agent-Tool Validation ──

// validateAgentTools ensures each AgentToolRef names a real, user-owned agent
// and rejects self-reference. Cycles between two agents (A → B, B → A) are
// allowed at persist time — they can't produce runtime loops because the
// adapter builds the child as a plain leaf LLM agent without propagating
// its own AgentTools (see adapters/adkgo resolveAgentTools).
func (s *Service) validateAgentTools(ctx context.Context, agent *AgentDefinition) error {
	seen := make(map[string]bool, len(agent.AgentTools))
	for i, ref := range agent.AgentTools {
		if ref == nil || ref.AgentID == "" {
			return fmt.Errorf("agent_tools[%d]: agent_id is required", i)
		}
		if agent.ID != "" && ref.AgentID == agent.ID {
			return fmt.Errorf("agent_tools[%d]: an agent cannot reference itself", i)
		}
		if seen[ref.AgentID] {
			return fmt.Errorf("agent_tools[%d]: duplicate agent_id %q", i, ref.AgentID)
		}
		seen[ref.AgentID] = true
		if _, err := s.repo.GetByID(ctx, AgentID(ref.AgentID), UserID(agent.UserID)); err != nil {
			return fmt.Errorf("agent_tools[%d]: agent %q not found: %w", i, ref.AgentID, err)
		}
	}
	return nil
}

// ── Composition Graph Validation ──

// validateGraph recursively validates the entire composition tree.
// A workflow agent's root must be a composite node (sequential/parallel/loop)
// since its purpose is to orchestrate sub-agents.
func (s *Service) validateGraph(ctx context.Context, node *CompositionNode, userID string) error {
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
func (s *Service) validateNode(ctx context.Context, node *CompositionNode, userID string, depth int) error {
	if !ValidNodeTypes[node.Type] {
		return fmt.Errorf("node %q: invalid type %q", node.ID, node.Type)
	}

	switch node.Type {
	case NodeAgent:
		return s.validateAgentNode(ctx, node, userID)
	case NodeRemote:
		return validateRemoteNode(node)
	case NodeSequential, NodeParallel, NodeLoop, NodeConditional:
		return s.validateCompositeNode(ctx, node, userID, depth)
	}
	return nil
}

func (s *Service) validateAgentNode(ctx context.Context, node *CompositionNode, userID string) error {
	if node.AgentID == "" {
		return fmt.Errorf("node %q: agent node requires agent_id", node.ID)
	}
	if len(node.Children) > 0 {
		return fmt.Errorf("node %q: agent node cannot have children", node.ID)
	}
	if _, err := s.repo.GetByID(ctx, AgentID(node.AgentID), UserID(userID)); err != nil {
		return fmt.Errorf("node %q: agent %q not found: %w", node.ID, node.AgentID, err)
	}
	return nil
}

func validateRemoteNode(node *CompositionNode) error {
	if node.A2AConnectorID == "" {
		return fmt.Errorf("node %q: remote node requires a2a_connector_id", node.ID)
	}
	if len(node.Children) > 0 {
		return fmt.Errorf("node %q: remote node cannot have children", node.ID)
	}
	return nil
}

func (s *Service) validateCompositeNode(ctx context.Context, node *CompositionNode, userID string, depth int) error {
	if len(node.Children) == 0 {
		return fmt.Errorf("node %q: %s node requires at least one child", node.ID, node.Type)
	}
	if node.AgentID != "" {
		return fmt.Errorf("node %q: composite node should not have agent_id", node.ID)
	}
	if node.Type == NodeLoop {
		if err := normalizeLoopIterations(node); err != nil {
			return err
		}
	}
	for i := range node.Children {
		if err := s.validateNode(ctx, &node.Children[i], userID, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// normalizeLoopIterations clamps MaxIterations into [0, 100], defaulting 0 → 3.
func normalizeLoopIterations(node *CompositionNode) error {
	if node.MaxIterations < 0 {
		node.MaxIterations = 0
	}
	if node.MaxIterations > 100 {
		return fmt.Errorf("node %q: max_iterations cannot exceed 100", node.ID)
	}
	if node.MaxIterations == 0 {
		node.MaxIterations = 3
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

// collectOutputKeysFromJSON extracts output_key values from workflow node JSON.
// Used during fork to transfer legacy workflow node output_keys to AgentDefinition.
func collectOutputKeysFromJSON(data []byte) map[string]string {
	result := make(map[string]string)
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return result
	}
	walkJSONForOutputKeys(m, result)
	return result
}

func walkJSONForOutputKeys(v any, result map[string]string) {
	obj, ok := v.(map[string]any)
	if !ok {
		return
	}
	nodeType, _ := obj["type"].(string)
	agentID, _ := obj["agent_id"].(string)
	outputKey, _ := obj["output_key"].(string)
	if nodeType == "agent" && agentID != "" && outputKey != "" {
		result[agentID] = outputKey
	}
	if children, ok := obj["children"].([]any); ok {
		for _, c := range children {
			walkJSONForOutputKeys(c, result)
		}
	}
}
