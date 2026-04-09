package filesystem

import (
	"aicompass/foundation/auth"
	"aicompass/infra/githubauth"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the filesystem bounded context entry point.
type Module struct {
	handler   *handler
	ghHandler *githubHandler
	svc       *service
}

// New creates a new filesystem module.
// ghClient and ghStore may be nil when GitHub OAuth is not configured.
func New(db *sqlx.DB, s3Cfg S3Config, ghClient *githubauth.Client, ghStore *githubauth.Store, appURL string) (*Module, error) {
	repo := newPostgresRepo(db)
	svc, err := newService(repo, s3Cfg, ghStore)
	if err != nil {
		return nil, err
	}
	h := newHandler(svc)

	var ghh *githubHandler
	if ghClient != nil && ghStore != nil {
		ghh = newGitHubHandler(ghClient, ghStore, appURL)
	}

	return &Module{handler: h, ghHandler: ghh, svc: svc}, nil
}

// RegisterRoutes mounts all filesystem HTTP endpoints.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	// Filesystem CRUD
	r.Post("/api/filesystems", authMW.RequireAuthFunc(m.handler.createFilesystem))
	r.Get("/api/filesystems", authMW.RequireAuthFunc(m.handler.listFilesystems))
	r.Get("/api/filesystems/{id}", authMW.RequireAuthFunc(m.handler.getFilesystem))
	r.Put("/api/filesystems/{id}", authMW.RequireAuthFunc(m.handler.updateFilesystem))
	r.Delete("/api/filesystems/{id}", authMW.RequireAuthFunc(m.handler.deleteFilesystem))

	// File and directory operations
	r.Get("/api/filesystems/{id}/ls", authMW.RequireAuthFunc(m.handler.listDirectory))
	r.Post("/api/filesystems/{id}/mkdir", authMW.RequireAuthFunc(m.handler.createDirectory))
	r.Post("/api/filesystems/{id}/upload", authMW.RequireAuthFunc(m.handler.uploadFile))
	r.Get("/api/filesystems/{id}/read", authMW.RequireAuthFunc(m.handler.readFile))
	r.Delete("/api/filesystems/{id}/rm", authMW.RequireAuthFunc(m.handler.removeEntry))
	r.Post("/api/filesystems/{id}/rename", authMW.RequireAuthFunc(m.handler.renameEntry))

	// Permissions (agent access)
	r.Get("/api/filesystems/{id}/permissions", authMW.RequireAuthFunc(m.handler.listPermissions))
	r.Post("/api/filesystems/{id}/permissions", authMW.RequireAuthFunc(m.handler.grantPermission))
	r.Delete("/api/filesystems/{id}/permissions/{agentId}", authMW.RequireAuthFunc(m.handler.revokePermission))

	// Agent-centric permissions
	r.Get("/v1/agents/{agentId}/filesystems", authMW.RequireAuthFunc(m.handler.listAgentFilesystems))
	r.Put("/v1/agents/{agentId}/filesystems/{id}", authMW.RequireAuthFunc(m.handler.toggleAgentPermission))

	// GitHub OAuth connection — routes are always registered so the frontend
	// gets a clear error instead of a 404 when GitHub OAuth is not configured.
	if m.ghHandler != nil {
		r.Get("/api/github/connect", authMW.RequireAuthFunc(m.ghHandler.connect))
		r.Get("/api/github/callback", authMW.RequireAuthFunc(m.ghHandler.callback))
		r.Get("/api/github/status", authMW.RequireAuthFunc(m.ghHandler.status))
		r.Get("/api/github/repos", authMW.RequireAuthFunc(m.ghHandler.repos))
		r.Delete("/api/github/disconnect", authMW.RequireAuthFunc(m.ghHandler.disconnect))
	} else {
		ghNotConfigured := func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"connected": false,
				"error":     "GitHub OAuth is not configured. Set APP_AUTH_GITHUB_CLIENT_ID and APP_AUTH_GITHUB_CLIENT_SECRET.",
			})
		}
		r.Get("/api/github/connect", authMW.RequireAuthFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusServiceUnavailable, "GitHub OAuth is not configured. Set APP_AUTH_GITHUB_CLIENT_ID and APP_AUTH_GITHUB_CLIENT_SECRET.")
		}))
		r.Get("/api/github/callback", authMW.RequireAuthFunc(ghNotConfigured))
		r.Get("/api/github/status", authMW.RequireAuthFunc(ghNotConfigured))
		r.Get("/api/github/repos", authMW.RequireAuthFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusServiceUnavailable, "GitHub OAuth is not configured")
		}))
		r.Delete("/api/github/disconnect", authMW.RequireAuthFunc(ghNotConfigured))
	}
}

// Service returns the underlying service for cross-module use.
func (m *Module) Service() *service {
	return m.svc
}

// AgentFS returns an [AgentFS] adapter for use by agent-facing tools (plugins).
func (m *Module) AgentFS() AgentFS {
	return &agentFSAdapter{svc: m.svc}
}
