package debughandler

import (
	"context"
	"errors"

	fagent "aicompass/foundation/agent"
	"aicompass/modules/agentdefinition"
)

// staticModelConfigResolver returns the request-supplied model config for any
// userID. It lets /debug/agent:run run an agent without any user_ai_settings row.
type staticModelConfigResolver struct {
	cfg *fagent.ModelConfig
}

func (r staticModelConfigResolver) GetModelConfigForUser(ctx context.Context, userID string) (*fagent.ModelConfig, error) {
	return r.cfg, nil
}

// staticAPIKeyResolver returns the same API key for every provider. The debug
// endpoint does not support per-provider keys — callers that rely on ModelOverride
// are expected to supply a key that works for both base and override providers,
// or to omit ModelOverride.
type staticAPIKeyResolver struct {
	apiKey string
}

func (r staticAPIKeyResolver) GetAPIKeyForProvider(ctx context.Context, userID, provider string) (string, error) {
	return r.apiKey, nil
}

// memRepo is an in-memory AgentDefinitionRepository scoped to a single request.
// Only the read methods used by UserAwareProvider + the adapter are meaningful;
// the rest return errNotSupported so misuse is loud.
type memRepo struct {
	byID map[string]*agentdefinition.AgentDefinition
}

func newMemRepo(defs []*agentdefinition.AgentDefinition) *memRepo {
	byID := make(map[string]*agentdefinition.AgentDefinition, len(defs))
	for _, d := range defs {
		byID[d.ID] = d
	}
	return &memRepo{byID: byID}
}

var errNotSupported = errors.New("debug in-memory repo: operation not supported")

func (r *memRepo) Create(context.Context, *agentdefinition.AgentDefinition) error {
	return errNotSupported
}
func (r *memRepo) Update(context.Context, *agentdefinition.AgentDefinition) error {
	return errNotSupported
}
func (r *memRepo) Delete(context.Context, string, string) error { return errNotSupported }

func (r *memRepo) GetByID(_ context.Context, id, _ string) (*agentdefinition.AgentDefinition, error) {
	if def, ok := r.byID[id]; ok {
		return def, nil
	}
	return nil, errors.New("agent not found in debug in-memory repo: " + id)
}

func (r *memRepo) ListByUser(_ context.Context, _ string) ([]agentdefinition.AgentDefinition, error) {
	out := make([]agentdefinition.AgentDefinition, 0, len(r.byID))
	for _, d := range r.byID {
		out = append(out, *d)
	}
	return out, nil
}

func (r *memRepo) Search(context.Context, string, string) ([]agentdefinition.AgentDefinition, error) {
	return nil, nil
}

func (r *memRepo) ListAddedCatalogIDs(context.Context, string) ([]string, error) {
	return nil, nil
}

func (r *memRepo) ExistsByCatalogID(context.Context, string, string) (bool, error) {
	return false, nil
}

func (r *memRepo) GetByShareToken(context.Context, string) (*agentdefinition.AgentDefinition, error) {
	return nil, errNotSupported
}

func (r *memRepo) UpdateShareToken(context.Context, string, string, string, string) error {
	return errNotSupported
}

func (r *memRepo) SetParent(context.Context, string, string, string) error {
	return errNotSupported
}

func (r *memRepo) DeleteByParent(context.Context, string, string) error {
	return errNotSupported
}

var _ agentdefinition.AgentDefinitionRepository = (*memRepo)(nil)
