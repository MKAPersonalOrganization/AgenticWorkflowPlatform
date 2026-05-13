package agentdefinition

import (
	"aicompass/foundation/agent"
	"context"
	"errors"
	"testing"
)

type stubRepo struct {
	byID   map[string]*AgentDefinition
	byUser []AgentDefinition
}

func (s *stubRepo) Create(context.Context, *AgentDefinition) error { return nil }
func (s *stubRepo) Update(context.Context, *AgentDefinition) error { return nil }
func (s *stubRepo) Delete(context.Context, AgentID, UserID) error  { return nil }
func (s *stubRepo) Search(context.Context, UserID, string) ([]AgentDefinition, error) {
	return nil, nil
}
func (s *stubRepo) ListAddedCatalogIDs(context.Context, UserID) ([]string, error) { return nil, nil }
func (s *stubRepo) ExistsByCatalogID(context.Context, UserID, CatalogID) (bool, error) {
	return false, nil
}

func (s *stubRepo) GetByShareToken(context.Context, string) (*AgentDefinition, error) {
	return nil, nil
}

func (s *stubRepo) UpdateShareToken(context.Context, AgentID, UserID, string, string) error {
	return nil
}
func (s *stubRepo) SetParent(context.Context, AgentID, AgentID, UserID) error { return nil }
func (s *stubRepo) DeleteByParent(context.Context, AgentID, UserID) error     { return nil }
func (s *stubRepo) GetByID(_ context.Context, id AgentID, _ UserID) (*AgentDefinition, error) {
	if def, ok := s.byID[string(id)]; ok {
		return def, nil
	}
	return nil, errors.New("not found")
}

func (s *stubRepo) ListByUser(context.Context, UserID) ([]AgentDefinition, error) {
	return s.byUser, nil
}

func TestUserAwareProviderGetForUserBuildsWorkflowAgent(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		byID: map[string]*AgentDefinition{
			"root": {
				ID:           "root",
				UserID:       "u1",
				Name:         "Root",
				Description:  "desc",
				SystemPrompt: "prompt",
				Graph: &CompositionNode{
					ID:   "g1",
					Type: NodeSequential,
					Children: []CompositionNode{
						{ID: "child", Type: NodeAgent, AgentID: "leaf"},
					},
				},
			},
			"leaf": {
				ID:           "leaf",
				UserID:       "u1",
				Name:         "Leaf",
				Description:  "leaf-desc",
				SystemPrompt: "leaf-prompt",
			},
		},
	}

	provider := NewUserAwareProvider(repo)
	got, err := provider.GetForUser(context.Background(), "root", "u1")
	if err != nil {
		t.Fatalf("GetForUser() error = %v", err)
	}

	if _, ok := got.(agent.WorkflowAgent); !ok {
		t.Fatalf("expected workflow agent, got %T", got)
	}
	if got.Name() != "Root" {
		t.Fatalf("Name() = %q, want %q", got.Name(), "Root")
	}
}

func TestUserAwareProviderBuildsCoordinatorSubAgents(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		byID: map[string]*AgentDefinition{
			"root": {
				ID:           "root",
				UserID:       "u1",
				Name:         "Root",
				Description:  "desc",
				SystemPrompt: "prompt",
				SubAgents:    []*SubAgentRef{{AgentID: "leaf"}},
			},
			"leaf": {
				ID:           "leaf",
				UserID:       "u1",
				Name:         "Leaf",
				Description:  "leaf-desc",
				SystemPrompt: "leaf-prompt",
			},
		},
	}

	provider := NewUserAwareProvider(repo)
	got, err := provider.GetForUser(context.Background(), "root", "u1")
	if err != nil {
		t.Fatalf("GetForUser() error = %v", err)
	}
	if len(got.SubAgents()) != 1 {
		t.Fatalf("SubAgents() len = %d, want 1", len(got.SubAgents()))
	}
	if got.SubAgents()[0].Name() != "Leaf" {
		t.Fatalf("sub-agent name = %q, want %q", got.SubAgents()[0].Name(), "Leaf")
	}
}
