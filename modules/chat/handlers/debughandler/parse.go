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

	// Parse agent_tools from entry
	var agentToolRefs []agentdefinition.AgentToolRef
	if entry.AgentToolsJSON != "" {
		if err := json.Unmarshal([]byte(entry.AgentToolsJSON), &agentToolRefs); err != nil {
			return "", nil, fmt.Errorf("parsing agent_tools: %w", err)
		}
	}

	if !entry.IsWorkflow() {
		// Handle single-agent with llm_agents (for agent-as-tool pattern)
		// Parse llm_agents to create sub-agents
		var subs []agentdefinition.CatalogSubAgent
		if entry.LLMAgentsJSON != "" {
			if err := json.Unmarshal([]byte(entry.LLMAgentsJSON), &subs); err != nil {
				return "", nil, fmt.Errorf("parsing llm_agents: %w", err)
			}
		}

		// Parse agent_tools to expose sub-agents as tools
		var agentToolRefs []agentdefinition.AgentToolRef
		if entry.AgentToolsJSON != "" {
			if err := json.Unmarshal([]byte(entry.AgentToolsJSON), &agentToolRefs); err != nil {
				return "", nil, fmt.Errorf("parsing agent_tools: %w", err)
			}
		}

		// Create sub-agents from llm_agents (first entry can be root if it matches entry name)
		idMap := make(map[string]string)
		var defs []*agentdefinition.AgentDefinition

		// Create first entry as root. GlobalInstruction only takes effect on
		// the root (see .READ/adk-go/agent/llmagent/llmagent.go:232), so it
		// lives here regardless of whether the entry declares sub-agents.
		rootID := fagent.GenerateID()
		root := &agentdefinition.AgentDefinition{
			ID:                rootID,
			Name:              entry.Name,
			Description:       entry.Description,
			SystemPrompt:      entry.SystemPrompt,
			GlobalInstruction: entry.GlobalInstruction,
			Icon:              entry.Icon,
		}

		// If there are sub-agents in llm_agents, create them
		if len(subs) > 0 {
			for _, sa := range subs {
				subID := fagent.GenerateID()
				sub := &agentdefinition.AgentDefinition{
					ID:           subID,
					Name:         sa.Name,
					Description:  sa.Description,
					SystemPrompt: sa.SystemPrompt,
					Icon:         sa.Icon,
					OutputKey:    sa.OutputKey,
				}
				idMap[sa.ID] = subID
				defs = append(defs, sub)
			}

			// Remap agent_tools references and apply input_schema
			for i := range agentToolRefs {
				if newID, ok := idMap[agentToolRefs[i].AgentID]; ok {
					agentToolRefs[i].AgentID = newID
					// Set input_schema on the sub-agent from agent_tools ref
					if agentToolRefs[i].InputSchema != "" {
						if subAgent, ok := idMap[agentToolRefs[i].AgentID]; ok {
							// Find the sub-agent and set its input_schema
							for _, d := range defs {
								if d.ID == subAgent {
									d.InputSchemaJSON = agentToolRefs[i].InputSchema
									break
								}
							}
						}
					}
				}
			}

			// Set agent_tools on root
			tools := make([]*agentdefinition.AgentToolRef, len(agentToolRefs))
			for i := range agentToolRefs {
				tools[i] = &agentToolRefs[i]
			}
			root.AgentTools = tools
		}

		defs = append([]*agentdefinition.AgentDefinition{root}, defs...)
		return root.ID, defs, nil
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

	// Remap agent_tools references to the generated sub-agent IDs.
	for i := range agentToolRefs {
		if newID, ok := idMap[agentToolRefs[i].AgentID]; ok {
			agentToolRefs[i].AgentID = newID
		}
	}

	// Convert to pointer slice
	tools := make([]*agentdefinition.AgentToolRef, len(agentToolRefs))
	for i := range agentToolRefs {
		tools[i] = &agentToolRefs[i]
	}

	// Workflow root. GlobalInstruction propagates from every agent's LLM call
	// via the orchestration engine's instruction processor, which walks to the
	// root before appending its global instruction to each request (see
	// .READ/adk-go/internal/llminternal/instruction_processor.go:48-56).
	root := &agentdefinition.AgentDefinition{
		ID:                fagent.GenerateID(),
		Name:              entry.Name,
		Description:       entry.Description,
		SystemPrompt:      entry.SystemPrompt,
		GlobalInstruction: entry.GlobalInstruction,
		Icon:              entry.Icon,
		Graph:             &graph,
		AgentTools:        tools,
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
