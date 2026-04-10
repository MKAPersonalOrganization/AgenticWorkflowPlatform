package multiagent

import (
	"aicompass/foundation/logger"
	"aicompass/modules/agentdefinition"
	"context"
	"fmt"
)

// ──────────────────────────────────────────────────────────────────────────────
// Service
// ──────────────────────────────────────────────────────────────────────────────

const (
	maxTreeDepth = 5  // prevent absurdly deep nesting
	maxTreeNodes = 50 // total nodes across the tree
)

type service struct {
	log      *logger.Logger
	repo     WorkflowDefinitionRepository
	agentDef agentdefinition.AgentDefinitionRepository
}

func newService(
	log *logger.Logger,
	repo WorkflowDefinitionRepository,
	agentDef agentdefinition.AgentDefinitionRepository,
) *service {
	return &service{
		log:      log,
		repo:     repo,
		agentDef: agentDef,
	}
}

// create validates and persists a new workflow definition.
func (s *service) create(ctx context.Context, wf *WorkflowDefinition) error {
	if wf.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if wf.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if err := s.validateGraph(ctx, &wf.Graph, wf.UserID); err != nil {
		return fmt.Errorf("invalid graph: %w", err)
	}

	s.log.Info(ctx, "creating workflow", "userID", wf.UserID, "name", wf.Name, "rootType", wf.Graph.Type)

	if err := s.repo.Create(ctx, wf); err != nil {
		return fmt.Errorf("creating workflow: %w", err)
	}

	return nil
}

// update modifies an existing workflow definition.
func (s *service) update(ctx context.Context, wf *WorkflowDefinition) error {
	if wf.ID == "" {
		return fmt.Errorf("workflow ID is required")
	}
	if wf.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if wf.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if err := s.validateGraph(ctx, &wf.Graph, wf.UserID); err != nil {
		return fmt.Errorf("invalid graph: %w", err)
	}

	s.log.Info(ctx, "updating workflow", "workflowID", wf.ID, "userID", wf.UserID)

	if err := s.repo.Update(ctx, wf); err != nil {
		return fmt.Errorf("updating workflow: %w", err)
	}

	return nil
}

// delete removes a workflow definition.
func (s *service) delete(ctx context.Context, id, userID string) error {
	s.log.Info(ctx, "deleting workflow", "workflowID", id, "userID", userID)

	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("deleting workflow: %w", err)
	}

	return nil
}

// getByID retrieves a single workflow by ID.
func (s *service) getByID(ctx context.Context, id, userID string) (*WorkflowDefinition, error) {
	wf, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("getting workflow: %w", err)
	}
	return wf, nil
}

// listByUser returns all workflows for a user.
func (s *service) listByUser(ctx context.Context, userID string) ([]WorkflowDefinition, error) {
	workflows, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing workflows: %w", err)
	}
	return workflows, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Tree Validation
// ──────────────────────────────────────────────────────────────────────────────

// validateGraph recursively validates the entire composition tree.
func (s *service) validateGraph(ctx context.Context, node *WorkflowNode, userID string) error {
	// Check total size limits.
	if node.CountNodes() > maxTreeNodes {
		return fmt.Errorf("tree exceeds maximum of %d nodes", maxTreeNodes)
	}
	if node.Depth() > maxTreeDepth {
		return fmt.Errorf("tree exceeds maximum depth of %d", maxTreeDepth)
	}
	return s.validateNode(ctx, node, userID, 0)
}

// validateNode validates a single node and recurses into children.
func (s *service) validateNode(ctx context.Context, node *WorkflowNode, userID string, depth int) error {
	if !ValidNodeTypes[node.Type] {
		return fmt.Errorf("node %q: invalid type %q", node.ID, node.Type)
	}

	switch node.Type {
	case NodeAgent:
		// Leaf node: must reference a valid agent.
		if node.AgentID == "" {
			return fmt.Errorf("node %q: agent node requires agent_id", node.ID)
		}
		if len(node.Children) > 0 {
			return fmt.Errorf("node %q: agent node cannot have children", node.ID)
		}
		// Verify the agent exists and belongs to the user.
		_, err := s.agentDef.GetByID(ctx, node.AgentID, userID)
		if err != nil {
			return fmt.Errorf("node %q: agent %q not found: %w", node.ID, node.AgentID, err)
		}

	case NodeSequential, NodeParallel, NodeLoop:
		// Composite node: must have at least one child.
		if len(node.Children) == 0 {
			return fmt.Errorf("node %q: %s node requires at least one child", node.ID, node.Type)
		}
		if node.AgentID != "" {
			return fmt.Errorf("node %q: composite node should not have agent_id", node.ID)
		}

		// Loop-specific: validate max_iterations.
		if node.Type == NodeLoop {
			if node.MaxIterations < 0 {
				node.MaxIterations = 0
			}
			if node.MaxIterations > 100 {
				return fmt.Errorf("node %q: max_iterations cannot exceed 100", node.ID)
			}
			if node.MaxIterations == 0 {
				node.MaxIterations = 3 // sensible default
			}
		}

		// Recurse into children.
		for i := range node.Children {
			if err := s.validateNode(ctx, &node.Children[i], userID, depth+1); err != nil {
				return err
			}
		}
	}

	return nil
}
