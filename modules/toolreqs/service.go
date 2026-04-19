package toolreqs

import (
	"context"

	"aicompass/foundation/agent"
	"aicompass/foundation/logger"
	"aicompass/foundation/toolregistry"
)

// =============================================================================
// Service — the evaluation / confirmation / middleware authority
// =============================================================================
//
// Service is the runtime decision-making surface for tool requirements:
// "given this user/agent/tool, should we allow it? does it need a
// confirmation? should the block middleware be installed?". It does NOT
// own persistence — that's [PermissionStore]'s job.
//
// The split follows the cohesion principle: the CRUD surface (15+
// methods that all read and write the same database rows) lives on
// PermissionStore; the evaluator pipeline + confirmation policies +
// middleware factory live here. Callers take whichever surface they
// actually use:
//
//   - adkgo adapter (evaluator + confirmation + middleware) → *Service
//   - chat HTTP handlers (config CRUD)                      → *PermissionStore
//
// Service holds a *PermissionStore reference internally because it
// needs storage to answer runtime questions (e.g. "what tool sets has
// this user opted into?"). It never exposes that reference; the store
// is supplied directly to callers that want to read/write config.
type Service struct {
	log          *logger.Logger
	evaluator    *Evaluator
	permReq      *PermissionRequirement
	confirmReq   *ConfirmationRequirement
	store        *PermissionStore
	toolRegistry *toolregistry.Registry
}

// ServiceConfig contains dependencies for creating a [Service].
type ServiceConfig struct {
	Log          *logger.Logger
	Store        *PermissionStore
	ToolRegistry *toolregistry.Registry // for IsToolInToolSet lookups
}

// NewService creates a new tool requirements service.
//
// The service is initialized with the built-in Permission and
// Confirmation requirements; additional requirements can be registered
// via [RegisterRequirement]. The store is required — the service cannot
// answer "is this tool set allowed?" without it.
func NewService(cfg ServiceConfig) *Service {
	permReq := newPermissionRequirement()
	confirmReq := NewConfirmationRequirement()
	evaluator := NewEvaluator(permReq, confirmReq)

	s := &Service{
		log:          cfg.Log,
		evaluator:    evaluator,
		permReq:      permReq,
		confirmReq:   confirmReq,
		store:        cfg.Store,
		toolRegistry: cfg.ToolRegistry,
	}

	// Wire the permission requirement against the store (the authority on
	// which tool sets / tools are allowed). We adapt the store's public
	// API to the lowercase permissionChecker contract — see the adapter
	// in permission_store.go.
	permReq.setChecker(permissionCheckerAdapter{store: cfg.Store})

	return s
}

// =============================================================================
// Evaluation — the entry points the adk adapter uses at runtime
// =============================================================================

// GetEnabledToolSets returns the tool set IDs that are allowed for the
// given user/agent. This is the primary method used during tool resolution.
func (s *Service) GetEnabledToolSets(ctx context.Context, userID, agentID string) ([]string, error) {
	return s.store.AllowedToolSets(ctx, userID, agentID)
}

// HasToolSetPermission checks if a tool set passes all requirements.
func (s *Service) HasToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) bool {
	result, err := s.evaluator.Evaluate(ctx, &ToolEvalRequest{
		UserID:    userID,
		AgentID:   agentID,
		ToolSetID: toolSetID,
	})
	if err != nil {
		return false
	}
	return result.Allowed
}

// HasPermission checks if a specific tool passes all requirements.
func (s *Service) HasPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) bool {
	result, err := s.evaluator.Evaluate(ctx, &ToolEvalRequest{
		UserID:    userID,
		AgentID:   agentID,
		ToolSetID: toolSetID,
		ToolID:    toolID,
		ToolName:  toolID,
	})
	if err != nil {
		return false
	}
	return result.Allowed
}

// EvaluateTool performs a full evaluation of all requirements for a tool
// call. Returns the complete [ToolDecision] (confirmation state, reasons,
// etc.). The adapter layer uses this for per-tool-call decisions.
func (s *Service) EvaluateTool(ctx context.Context, req *ToolEvalRequest) (*ToolDecision, error) {
	return s.evaluator.Evaluate(ctx, req)
}

// =============================================================================
// Confirmation management — ADK-Go style user approval
// =============================================================================

// Confirmation returns the [ConfirmationRequirement] for direct policy management.
//
// Usage:
//
//	svc.Confirmation().SetGlobalPolicy("file_delete", &toolreqs.ConfirmationPolicy{
//	    AlwaysConfirm: true,
//	    Hint:          "This tool will permanently delete files. Continue?",
//	})
func (s *Service) Confirmation() *ConfirmationRequirement {
	return s.confirmReq
}

// CheckConfirmation determines whether a tool call needs user confirmation
// for the given user/agent/tool combination. Checks both per-user DB
// config and admin-level global policies.
//
// Returns (needsConfirmation, hint). Used by the adapter layer to build
// ADK-native BeforeToolCallbacks that call ctx.RequestConfirmation().
func (s *Service) CheckConfirmation(ctx context.Context, userID, agentID, toolSetID, toolName string, args map[string]any) (bool, string) {
	return CheckConfirmation(ctx, s.store, s, s.confirmReq,
		userID, agentID, toolSetID, toolName, args)
}

// RequiresConfirmation checks if a specific tool call requires user
// confirmation. Convenience method used by the adapter's before-tool
// callback for in-memory/global policies only — DB-driven per-user
// confirmation goes through [CheckConfirmation].
func (s *Service) RequiresConfirmation(ctx context.Context, agentID, toolName string, args map[string]any) bool {
	return s.confirmReq.RequiresConfirmation(agentID, toolName, args)
}

// =============================================================================
// Block middleware + membership — runtime safety net
// =============================================================================

// BlockMiddleware returns a [agent.Middleware] scoped to the given agent
// that prevents execution of tools the user has blocked. Defense-in-depth:
// blocked tools are normally filtered at resolution time, but this catches
// race conditions or resolution bugs.
func (s *Service) BlockMiddleware(agentID string) agent.Middleware {
	return BlockMiddleware(s, agentID)
}

// IsToolInToolSet checks if a tool belongs to a specific toolset, looking
// the membership up through the registry.
//
// Lives on Service (not the store) because it consults the tool registry,
// which is an evaluation-time dependency distinct from the permission
// database.
func (s *Service) IsToolInToolSet(ctx context.Context, toolSetID, toolName string) (bool, error) {
	if s.toolRegistry == nil {
		return false, nil
	}
	tools, err := s.toolRegistry.GetToolsInSet(ctx, toolSetID)
	if err != nil {
		return false, err
	}
	for _, t := range tools {
		if t.ID == toolName || t.Name == toolName {
			return true, nil
		}
	}
	return false, nil
}

// =============================================================================
// Extensibility — register new requirement types
// =============================================================================

// RegisterRequirement adds a new requirement to the evaluation pipeline.
//
// This is the extensibility point for future requirement types:
//   - Rate limiting
//   - Cost budgets
//   - Time-of-day restrictions
//   - Audit logging requirements
func (s *Service) RegisterRequirement(req ToolRequirement) {
	s.evaluator.AddRequirement(req)
}
