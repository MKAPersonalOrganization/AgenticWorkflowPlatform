// Package chat is the chat bounded context module. It owns the full vertical
// slice for real-time conversational AI: HTTP handlers, agent orchestration,
// thread lifecycle (event sourcing), AGUI + LLM context projections, and
// long-term memory.
//
// This is a facade module (ADR 014, Sub-phase 5a) that encapsulates the
// construction of existing packages. A future Sub-phase 5b will relocate the
// source files into this package.
//
// # Two-phase construction
//
// The orchestrator and runner have a circular dependency that is resolved via
// two-phase construction:
//
//	chatModule := chat.New(deps)                          // Phase 1: create orchestrator (no runner)
//	runner := custom.NewRunnerAdapter(chatModule.Orchestrator(), ...)
//	chatModule.SetRunner(runner)                          // Phase 2: complete init
//	chatModule.RegisterProjection(persistenceRouter)
//	chatModule.RegisterRoutes(r, authMiddleware)
package chat

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/auth"
	"aicompass/foundation/event"
	es "aicompass/foundation/eventsourcing"
	"aicompass/foundation/logger"
	"aicompass/foundation/middleware"
	"aicompass/modules/chat/agentorchestrator"
	"aicompass/modules/chat/chatcontracts"
	"aicompass/modules/chat/projection/agui"
	"aicompass/modules/chat/projection/llmcontext"
	"aicompass/modules/chat/projection"
	"aicompass/modules/chat/thread"
	"aicompass/modules/chat/threadmgmt"
	"aicompass/foundation/artifact"
	"aicompass/foundation/toolregistry"

	aguihandlers "aicompass/modules/chat/agui"
	taskmgmthandlers "aicompass/modules/chat/taskmgmt"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Dependencies holds all external dependencies injected into the chat module.
// Cross-cutting services (artifact, permissions, toolregistry) are injected
// rather than constructed, because they are shared with the runner adapter and
// other modules.
type Dependencies struct {
	Log                      *logger.Logger
	DB                       *sqlx.DB
	EventStore               thread.EventStore
	SnapshotStore            es.SnapshotStore // optional — accelerates thread loading
	ArtifactService          artifact.Service
	UserProfileRepo          chatcontracts.UserProfileRepository
	Emitter                  agentorchestrator.EventEmitter // transient event router (optional)
	AgentProvider            agent.AgentProvider
	ToolRegistry             *toolregistry.Registry
	RequirementConfigService chatcontracts.ToolRequirementConfigService
	DocumentProvider         chatcontracts.DocumentProvider
	TenantID                 string
	AllowedOrigin            string
	SSE                      taskmgmthandlers.SSEConfig
}

// Module is the chat bounded context. It owns handlers, services, projections,
// and inline SSE streaming for real-time conversational AI.
//
// Public API for main.go:
//
//	module := chat.New(deps)
//	runner := custom.NewRunnerAdapter(module.Orchestrator(), ...)
//	module.SetRunner(runner)
//	module.RegisterProjection(persistenceRouter)
//	module.RegisterRoutes(r, authMiddleware)
type Module struct {
	log            *logger.Logger
	orchestrator   *agentorchestrator.Service
	threadMgmt     *threadmgmt.Manager
	aguiProjection *agui.PostgresAguiProjection
	llmProjection  *llmcontext.PostgresLLMContextProjection
	threadQuery    chatcontracts.ThreadQueryService
	chatHandler    *taskmgmthandlers.TaskManagementHandler
	aguiHandler    *aguihandlers.AguiProjectionHandler

	// Stored for two-phase construction (SetRunner creates handlers).
	deps Dependencies
}

// New creates the chat module. The orchestrator is constructed without a runner;
// call SetRunner() to complete initialisation before RegisterRoutes().
func New(deps Dependencies) *Module {
	log := deps.Log

	// AGUI projection (read model)
	aguiProjection := agui.NewPostgresAguiProjection(log, deps.DB)

	// LLM context projection (read model)
	llmProjection := llmcontext.NewPostgresLLMContextProjection(log, deps.DB)

	// Thread management (event sourcing)
	threadRepo := thread.NewThreadRepository(threadmgmt.NewEventStoreAdapter(deps.EventStore), deps.SnapshotStore)
	threadMgmtSvc := threadmgmt.New(log, threadRepo)

	// Thread query service (wraps AGUI projection for UI queries)
	threadQuerySvc := projection.NewUIProjectionService(aguiProjection)

	// Agent orchestrator — runner is set later via SetRunner()
	orchestrator := agentorchestrator.New(log, deps.EventStore,
		agentorchestrator.WithArtifactService(deps.ArtifactService),
		agentorchestrator.WithUserProfileRepository(deps.UserProfileRepo),
		agentorchestrator.WithEmitter(deps.Emitter),
		agentorchestrator.WithSnapshotStore(deps.SnapshotStore),
	)

	return &Module{
		log:            log,
		orchestrator:   orchestrator,
		threadMgmt:     threadMgmtSvc,
		aguiProjection: aguiProjection,
		llmProjection:  llmProjection,
		threadQuery:    threadQuerySvc,
		deps:           deps,
	}
}

// Orchestrator returns the agent orchestrator so that main.go can create the
// runner adapter (which needs the orchestrator as a ThreadManager).
func (m *Module) Orchestrator() *agentorchestrator.Service {
	return m.orchestrator
}

// SetRunner completes two-phase construction: sets the runner on the
// orchestrator and creates the HTTP handlers.
// Must be called exactly once, before RegisterRoutes.
func (m *Module) SetRunner(runner agentorchestrator.Runner) {
	m.orchestrator.SetRunner(runner)

	// Create HTTP handlers
	m.chatHandler = taskmgmthandlers.NewTaskManagementHandler(
		m.log,
		m.orchestrator,
		m.deps.AgentProvider,
		m.threadQuery,
		m.aguiProjection,
		m.threadMgmt,
		m.deps.ArtifactService,
		m.deps.ToolRegistry,
		m.deps.RequirementConfigService,
		m.deps.DocumentProvider,
		m.deps.TenantID,
		m.deps.AllowedOrigin,
		m.deps.SSE,
	)

	m.aguiHandler = aguihandlers.NewAguiProjectionHandler(m.aguiProjection, m.threadQuery)
}

// AguiProjection returns the AGUI projection for external consumers
// (e.g. statistics projection, health checks).
func (m *Module) AguiProjection() *agui.PostgresAguiProjection {
	return m.aguiProjection
}

// RegisterProjection wires the AGUI and LLM context projection handlers into
// the persistence router so that committed events update the read models.
func (m *Module) RegisterProjection(router interface {
	Register(eventPrototype any, handler event.HandlerFunc)
},
) {
	m.aguiProjection.RegisterHandlers(router)
	m.llmProjection.RegisterHandlers(router)
}

// RegisterRoutes mounts all chat HTTP routes onto the given chi.Router.
// Must be called after SetRunner.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	h := m.chatHandler
	ah := m.aguiHandler

	// message:send needs a larger body limit for base64 file attachments.
	// 5 files × 20 MB raw ≈ 133 MB base64 — round up to 150 MB.
	r.Group(func(r chi.Router) {
		r.Use(middleware.BodyLimit(150 << 20))
		r.Post("/v1/message:send", authMW.RequireAuthFunc(h.SendMessage))
	})

	// tool:confirm handles tool confirmation responses (Allow/Deny).
	// Uses a separate endpoint for clarity (vs mixing with normal messages).
	r.Post("/v1/tool:confirm", authMW.RequireAuthFunc(h.ConfirmTool))

	// tool:provide_input handles custom tool input responses (io.Read).
	r.Post("/v1/tool:provide_input", authMW.RequireAuthFunc(h.ProvideInput))

	// Attachment pre-upload (multipart form, 20 MB per file).
	r.Group(func(r chi.Router) {
		r.Use(middleware.BodyLimit(25 << 20)) // 25 MB (20 MB file + multipart overhead)
		r.Post("/v1/threads/{threadId}/attachments:upload", authMW.RequireAuthFunc(h.UploadAttachment))
	})

	// Agents
	r.Get("/v1/agents", authMW.RequireAuthFunc(h.ListAgents))
	r.Get("/v1/agents/{agentId}", authMW.RequireAuthFunc(h.GetAgent))

	// Artifacts
	r.Get("/v1/threads/{threadId}/artifacts", authMW.RequireAuthFunc(h.ListArtifacts))
	r.Get("/v1/threads/{threadId}/artifacts/{fileName}", authMW.RequireAuthFunc(h.DownloadArtifact))

	// Tool sets (per-agent)
	r.Get("/v1/agents/{agentId}/toolsets", authMW.RequireAuthFunc(h.ListAgentToolSets))

	// Tool requirement configuration (per-user, per-agent, per-tool)
	r.Get("/v1/agents/{agentId}/toolsets/{toolSetId}/requirements", authMW.RequireAuthFunc(h.GetToolSetRequirementConfig))
	r.Put("/v1/agents/{agentId}/toolsets/{toolSetId}/requirements", authMW.RequireAuthFunc(h.SetToolSetRequirementConfig))
	r.Get("/v1/agents/{agentId}/toolsets/{toolSetId}/tools/requirements", authMW.RequireAuthFunc(h.ListToolRequirementConfigs))
	r.Get("/v1/agents/{agentId}/toolsets/{toolSetId}/tools/{toolId}/requirements", authMW.RequireAuthFunc(h.GetToolRequirementConfig))
	r.Put("/v1/agents/{agentId}/toolsets/{toolSetId}/tools/{toolId}/requirements", authMW.RequireAuthFunc(h.SetToolRequirementConfig))

	// Plugins (agent-agnostic, agent_id via query param)
	r.Get("/v1/plugins", authMW.RequireAuthFunc(h.ListPlugins))

	// Documents (Edgequake passthrough)
	r.Get("/v1/documents", authMW.RequireAuthFunc(h.ListDocuments))
	r.Post("/v1/documents", authMW.RequireAuthFunc(h.UploadDocument))

	// AGUI projection query endpoints
	r.Get("/api/agui/threads", authMW.RequireAuthFunc(ah.ListThreads))
	r.Get("/api/agui/threads/{threadId}", authMW.RequireAuthFunc(ah.GetThreadState))
	r.Get("/api/agui/threads/{threadId}/messages", authMW.RequireAuthFunc(ah.GetMessages))
	r.Delete("/api/agui/threads/{threadId}", authMW.RequireAuthFunc(h.DeleteThread))
	r.Patch("/api/agui/threads/{threadId}", authMW.RequireAuthFunc(h.UpdateThreadTitle))
}
