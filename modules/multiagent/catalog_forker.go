package multiagent

import (
	"aicompass/foundation/logger"
	"aicompass/modules/agentdefinition"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// catalogSubAgent mirrors agentdefinition.CatalogSubAgent for JSON unmarshalling.
type catalogSubAgent struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
	Icon         string `json:"icon"`
	OutputKey    string `json:"output_key,omitempty"`
}

// CatalogWorkflowForker handles forking workflow catalog entries.
// It creates sub-agents and a workflow definition when a user forks a
// workflow catalog entry.
type CatalogWorkflowForker struct {
	log       *logger.Logger
	agentRepo agentdefinition.AgentDefinitionRepository
	wfRepo    WorkflowDefinitionRepository
}

// NewCatalogWorkflowForker creates a forker that can create sub-agents and
// workflow definitions from catalog workflow templates.
func NewCatalogWorkflowForker(
	log *logger.Logger,
	agentRepo agentdefinition.AgentDefinitionRepository,
	wfRepo WorkflowDefinitionRepository,
) *CatalogWorkflowForker {
	return &CatalogWorkflowForker{
		log:       log,
		agentRepo: agentRepo,
		wfRepo:    wfRepo,
	}
}

// ForkCatalogWorkflow creates llm agents and a workflow definition for the user.
//
// Process:
//  1. Parse the llm_agents JSON into individual agent definitions.
//  2. Create a placeholder workflow ID so llm agents can reference it.
//  3. Create each llm agent as a user_agent (with workflow_id set), building a placeholder→real ID map.
//  4. Parse the workflow_graph JSON into a WorkflowNode tree.
//  5. Remap all agent_id fields from placeholder IDs to real IDs.
//  6. Create the workflow_definition with the remapped graph.
func (f *CatalogWorkflowForker) ForkCatalogWorkflow(
	ctx context.Context,
	userID, catalogID, name, description, icon, graphJSON, llmAgentsJSON string,
) (string, error) {
	// 1. Parse llm agents.
	var subs []catalogSubAgent
	if err := json.Unmarshal([]byte(llmAgentsJSON), &subs); err != nil {
		return "", fmt.Errorf("parsing llm_agents: %w", err)
	}

	// 2. Pre-generate the workflow ID so llm agents can reference it.
	wfID := uuid.New().String()

	// Collect output_key from workflow nodes (legacy templates declare it on nodes).
	nodeOutputKeys := collectOutputKeysFromJSON([]byte(graphJSON))

	// 3. Create each llm agent and build ID map.
	idMap := make(map[string]string, len(subs)) // placeholder → real ID
	for _, sub := range subs {
		outputKey := sub.OutputKey
		if outputKey == "" {
			outputKey = nodeOutputKeys[sub.ID]
		}
		agent := &agentdefinition.AgentDefinition{
			UserID:       userID,
			Name:         sub.Name,
			Description:  sub.Description,
			SystemPrompt: sub.SystemPrompt,
			Icon:         sub.Icon,
			OutputKey:    outputKey,
			WorkflowID:   wfID,
		}
		if err := f.agentRepo.Create(ctx, agent); err != nil {
			return "", fmt.Errorf("creating llm agent %q: %w", sub.Name, err)
		}
		idMap[sub.ID] = agent.ID
		f.log.Info(ctx, "created llm agent for workflow fork",
			"placeholder", sub.ID, "real_id", agent.ID, "name", sub.Name)
	}

	// 4. Parse the graph.
	var graph WorkflowNode
	if err := json.Unmarshal([]byte(graphJSON), &graph); err != nil {
		return "", fmt.Errorf("parsing workflow_graph: %w", err)
	}

	// 5. Remap agent IDs.
	remapAgentIDs(&graph, idMap)

	// 6. Create the workflow definition with the pre-generated ID.
	wf := &WorkflowDefinition{
		ID:          wfID,
		UserID:      userID,
		Name:        name,
		Description: description,
		Icon:        icon,
		Graph:       graph,
	}
	if err := f.wfRepo.Create(ctx, wf); err != nil {
		return "", fmt.Errorf("creating workflow definition: %w", err)
	}

	// Establish ownership: set parent_id on all llm agents so cascade delete works.
	for _, realID := range idMap {
		if err := f.agentRepo.SetParent(ctx, realID, wfID, userID); err != nil {
			f.log.Warn(ctx, "failed to set parent_id on llm agent",
				"llmAgentID", realID, "parentID", wfID, "error", err)
		}
	}

	f.log.Info(ctx, "forked catalog workflow",
		"catalog_id", catalogID, "workflow_id", wf.ID, "llm_agents", len(subs))

	return wf.ID, nil
}

// remapAgentIDs recursively replaces placeholder agent_ids with real IDs.
func remapAgentIDs(node *WorkflowNode, idMap map[string]string) {
	if node.Type == NodeAgent && node.AgentID != "" {
		if realID, ok := idMap[node.AgentID]; ok {
			node.AgentID = realID
		}
	}
	for i := range node.Children {
		remapAgentIDs(&node.Children[i], idMap)
	}
}

// collectOutputKeysFromJSON extracts output_key values from workflow node JSON.
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
