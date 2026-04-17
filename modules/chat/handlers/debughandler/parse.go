package debughandler

import (
	"encoding/json"
	"errors"
	"fmt"

	fagent "aicompass/foundation/agent"
	"aicompass/modules/agentdefinition"
)

// parseAgents turns the request's agent_yaml or agent JSON into a list of
// AgentDefinitions (root first) and returns the root ID.
func parseAgents(req *DebugRequest) (string, []*agentdefinition.AgentDefinition, error) {
	switch {
	case req.AgentYAML != "" && len(req.Agent) > 0:
		return "", nil, errors.New("provide exactly one of `agent` or `agent_yaml`")
	case len(req.Agent) > 0:
		return parseJSONAgent(req.Agent)
	case req.AgentYAML != "":
		return parseYAMLAgent([]byte(req.AgentYAML))
	default:
		return "", nil, errors.New("one of `agent` or `agent_yaml` is required")
	}
}

func parseJSONAgent(data []byte) (string, []*agentdefinition.AgentDefinition, error) {
	var def agentdefinition.AgentDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return "", nil, fmt.Errorf("parsing agent JSON: %w", err)
	}
	if def.ID == "" {
		def.ID = fagent.GenerateID()
	}
	if def.Name == "" {
		def.Name = "debug-agent"
	}
	return def.ID, []*agentdefinition.AgentDefinition{&def}, nil
}

func parseYAMLAgent(data []byte) (string, []*agentdefinition.AgentDefinition, error) {
	entry, err := agentdefinition.ParseCatalogEntryYAML(data)
	if err != nil {
		return "", nil, fmt.Errorf("parsing agent YAML: %w", err)
	}

	if !entry.IsWorkflow() {
		root := &agentdefinition.AgentDefinition{
			ID:           fagent.GenerateID(),
			Name:         entry.Name,
			Description:  entry.Description,
			SystemPrompt: entry.SystemPrompt,
			Icon:         entry.Icon,
		}
		return root.ID, []*agentdefinition.AgentDefinition{root}, nil
	}

	// Workflow catalog entry → expand llm_agents + composition graph into a
	// set of AgentDefinitions, mirroring Service.ImportComposedFromCatalogEntry
	// but without touching the DB.
	var subs []agentdefinition.CatalogSubAgent
	if entry.LLMAgentsJSON != "" {
		if err := json.Unmarshal([]byte(entry.LLMAgentsJSON), &subs); err != nil {
			return "", nil, fmt.Errorf("parsing llm_agents: %w", err)
		}
	}

	var graph agentdefinition.CompositionNode
	if err := json.Unmarshal([]byte(entry.WorkflowGraphJSON), &graph); err != nil {
		return "", nil, fmt.Errorf("parsing workflow graph: %w", err)
	}

	defs := make([]*agentdefinition.AgentDefinition, 0, len(subs)+1)
	idMap := make(map[string]string, len(subs))
	for _, sa := range subs {
		sub := &agentdefinition.AgentDefinition{
			ID:           fagent.GenerateID(),
			Name:         sa.Name,
			Description:  sa.Description,
			SystemPrompt: sa.SystemPrompt,
			Icon:         sa.Icon,
			OutputKey:    sa.OutputKey,
		}
		idMap[sa.ID] = sub.ID
		defs = append(defs, sub)
	}

	remapGraphAgentIDs(&graph, idMap)

	root := &agentdefinition.AgentDefinition{
		ID:           fagent.GenerateID(),
		Name:         entry.Name,
		Description:  entry.Description,
		SystemPrompt: entry.SystemPrompt,
		Icon:         entry.Icon,
		Graph:        &graph,
	}
	defs = append([]*agentdefinition.AgentDefinition{root}, defs...)
	return root.ID, defs, nil
}

// remapGraphAgentIDs rewrites placeholder agent IDs in a CompositionNode tree
// to the generated IDs of the expanded sub-agents. Mirror of the unexported
// helper in modules/agentdefinition/service.go.
func remapGraphAgentIDs(n *agentdefinition.CompositionNode, idMap map[string]string) {
	if n.Type == agentdefinition.NodeAgent && n.AgentID != "" {
		if newID, ok := idMap[n.AgentID]; ok {
			n.AgentID = newID
		}
	}
	for i := range n.Children {
		remapGraphAgentIDs(&n.Children[i], idMap)
	}
}
