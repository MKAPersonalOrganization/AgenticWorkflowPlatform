package multiagent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"school-with-ai/foundation/logger"
	"school-with-ai/modules/agentstore"
)

// catalogSubAgent mirrors agentstore.CatalogSubAgent for JSON unmarshalling.
type catalogSubAgent struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
	Icon         string `json:"icon"`
}

// CatalogWorkflowForker implements agentstore.WorkflowForker.
// It creates sub-agents and a workflow definition when a user forks a
// workflow catalog entry.
type CatalogWorkflowForker struct {
	log       *logger.Logger
	agentRepo agentstore.AgentDefinitionRepository
	wfRepo    WorkflowDefinitionRepository
}

// NewCatalogWorkflowForker creates a forker that can create sub-agents and
// workflow definitions from catalog workflow templates.
func NewCatalogWorkflowForker(
	log *logger.Logger,
	agentRepo agentstore.AgentDefinitionRepository,
	wfRepo WorkflowDefinitionRepository,
) *CatalogWorkflowForker {
	return &CatalogWorkflowForker{
		log:       log,
		agentRepo: agentRepo,
		wfRepo:    wfRepo,
	}
}

// ForkCatalogWorkflow creates sub-agents and a workflow definition for the user.
//
// Process:
//  1. Parse the sub_agents JSON into individual agent definitions.
//  2. Create a placeholder workflow ID so sub-agents can reference it.
//  3. Create each sub-agent as a user_agent (with workflow_id set), building a placeholder→real ID map.
//  4. Parse the workflow_graph JSON into a WorkflowNode tree.
//  5. Remap all agent_id fields from placeholder IDs to real IDs.
//  6. Create the workflow_definition with the remapped graph.
func (f *CatalogWorkflowForker) ForkCatalogWorkflow(
	ctx context.Context,
	userID, catalogID, name, description, icon, graphJSON, subAgentsJSON string,
) (string, error) {
	// 1. Parse sub-agents.
	var subs []catalogSubAgent
	if err := json.Unmarshal([]byte(subAgentsJSON), &subs); err != nil {
		return "", fmt.Errorf("parsing sub_agents: %w", err)
	}

	// 2. Pre-generate the workflow ID so sub-agents can reference it.
	wfID := uuid.New().String()

	// 3. Create each sub-agent and build ID map.
	idMap := make(map[string]string, len(subs)) // placeholder → real ID
	for _, sub := range subs {
		agent := &agentstore.AgentDefinition{
			UserID:       userID,
			Name:         sub.Name,
			Description:  sub.Description,
			SystemPrompt: sub.SystemPrompt,
			Icon:         sub.Icon,
			WorkflowID:   wfID,
		}
		if err := f.agentRepo.Create(ctx, agent); err != nil {
			return "", fmt.Errorf("creating sub-agent %q: %w", sub.Name, err)
		}
		idMap[sub.ID] = agent.ID
		f.log.Info(ctx, "created sub-agent for workflow fork",
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

	f.log.Info(ctx, "forked catalog workflow",
		"catalog_id", catalogID, "workflow_id", wf.ID, "sub_agents", len(subs))

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
