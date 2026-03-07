package permissions

import (
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/handlers/auth"
	wkperms "school-with-ai/handlers/workspacepermissions"
	"school-with-ai/modules/knowledge/edgequake"
	"school-with-ai/services/workspacepermissions"

	"github.com/go-chi/chi/v5"
)

// Module is the permissions bounded context, combining workspace permissions
// (and, in future, resource permissions) into a single module entry point.
type Module struct {
	workspacePermHandler *wkperms.Handler
}

// New creates a new permissions module by wiring the existing handler
// implementations with the provided service dependencies.
func New(
	log *logger.Logger,
	workspacePermService *workspacepermissions.Service,
	edgequakeService *edgequake.Service,
	agentProvider agent.AgentProvider,
	tenantID string,
) *Module {
	wh := wkperms.NewHandler(log, workspacePermService, edgequakeService, agentProvider, tenantID)
	return &Module{
		workspacePermHandler: wh,
	}
}

// RegisterRoutes mounts the workspace-permission HTTP routes onto the
// provided chi router.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	// Workspace permission routes
	r.Get("/v1/agents/{agentId}/workspaces", authMW.RequireAuthFunc(m.workspacePermHandler.ListAgentWorkspaces))
	r.Put("/v1/agents/{agentId}/workspaces/{workspaceId}", authMW.RequireAuthFunc(m.workspacePermHandler.ToggleAgentWorkspace))
}
