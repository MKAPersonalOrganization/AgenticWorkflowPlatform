// Package knowledge is the Knowledge Base / RAG bounded context. It owns the
// full lifecycle of knowledge workspaces, documents, graph entities, pipeline
// tasks, and the EdgeQuake RAG API integration.
//
// Extracted as part of the modular-monolith migration
// (see arch/adr/012-modular-monolith-migration.md). It replaces three former
// packages: handlers/knowledge, services/edgequake, and services/rag (with all
// its sub-packages).
//
// Domain types referenced here (User, TenantContext) remain in the shared
// domain/ package for Phase 1.
//
// Public API for main.go:
//
// module := knowledge.New(log, baseURL, aiSettings, tenantID)
// module.RegisterRoutes(r, authMiddleware)
//
// Cross-module accessor:
//
// module.Service() — returns *edgequake.Service for shared callers
//
//	(taskmgmt, workspacepermissions, custom runner, HandleViewChatInputScreen)
package knowledge

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"
	"aicompass/modules/knowledge/edgequake"
	"context"

	"github.com/go-chi/chi/v5"
)

// ---------------------------------------------------------------------------
// Cross-module dependency interfaces (Dependency Inversion — ADR 012 §4.1)
// ---------------------------------------------------------------------------

// EmbeddingConfigResolver resolves the embedding configuration for a user.
// In production this is satisfied by the aisettings module.
type EmbeddingConfigResolver interface {
	GetEmbeddingConfigForUser(ctx context.Context, userID string) (*EmbeddingConfig, error)
}

// EmbeddingConfigResolverFunc adapts a plain function to the EmbeddingConfigResolver interface.
type EmbeddingConfigResolverFunc func(ctx context.Context, userID string) (*EmbeddingConfig, error)

// GetEmbeddingConfigForUser implements EmbeddingConfigResolver.
func (f EmbeddingConfigResolverFunc) GetEmbeddingConfigForUser(ctx context.Context, userID string) (*EmbeddingConfig, error) {
	return f(ctx, userID)
}

// EmbeddingConfig holds the resolved embedding provider/model/dimension for a user.
type EmbeddingConfig struct {
	Provider  string
	Model     string
	Dimension int
	APIKey    string
}

// ---------------------------------------------------------------------------
// Module
// ---------------------------------------------------------------------------

// Module is the Knowledge Base bounded context.
type Module struct {
	log     *logger.Logger
	handler *handler
	svc     *edgequake.Service
}

// New creates the Knowledge module.
func New(log *logger.Logger, baseURL string, aiSettings EmbeddingConfigResolver, tenantID string) *Module {
	svc := edgequake.NewService(baseURL)
	h := newHandler(log, svc, aiSettings, tenantID)

	return &Module{
		log:     log,
		handler: h,
		svc:     svc,
	}
}

// RegisterRoutes mounts all Knowledge Base HTTP routes.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	h := m.handler

	// Pages
	r.Get("/knowledge", authMW.RequireAuthFunc(h.WorkspacesPage))
	r.Get("/knowledge/dashboard", authMW.RequireAuthFunc(h.DashboardPage))
	r.Get("/knowledge/workspaces/{workspaceId}", authMW.RequireAuthFunc(h.WorkspaceDetailsPage))

	// Documents API
	r.Get("/api/knowledge/documents", authMW.RequireAuthFunc(h.ListDocuments))
	r.Get("/api/knowledge/documents/{documentId}", authMW.RequireAuthFunc(h.GetDocument))
	r.Post("/api/knowledge/documents/upload", authMW.RequireAuthFunc(h.UploadDocument))
	r.Delete("/api/knowledge/documents/{documentId}", authMW.RequireAuthFunc(h.DeleteDocument))
	r.Delete("/api/knowledge/documents", authMW.RequireAuthFunc(h.DeleteAllDocuments))
	r.Post("/api/knowledge/documents/reprocess", authMW.RequireAuthFunc(h.ReprocessDocument))
	r.Post("/api/knowledge/documents/reprocess-failed", authMW.RequireAuthFunc(h.ReprocessFailed))
	r.Post("/api/knowledge/documents/scan", authMW.RequireAuthFunc(h.ScanDocuments))

	// PDFs API
	r.Get("/api/knowledge/pdfs", authMW.RequireAuthFunc(h.ListPDFs))
	r.Post("/api/knowledge/pdfs/upload", authMW.RequireAuthFunc(h.UploadPDF))
	r.Delete("/api/knowledge/pdfs/{pdfId}", authMW.RequireAuthFunc(h.DeletePDF))
	r.Get("/api/knowledge/pdfs/{pdfId}/content", authMW.RequireAuthFunc(h.GetPDFContent))
	r.Get("/api/knowledge/pdfs/{pdfId}/download", authMW.RequireAuthFunc(h.DownloadPDF))
	r.Get("/api/knowledge/pdfs/{trackId}/progress", authMW.RequireAuthFunc(h.GetPDFProgress))
	r.Post("/api/knowledge/pdfs/{pdfId}/retry", authMW.RequireAuthFunc(h.RetryPdfProcessing))

	// Workspaces API
	r.Get("/api/knowledge/workspaces", authMW.RequireAuthFunc(h.ListWorkspaces))
	r.Post("/api/knowledge/workspaces", authMW.RequireAuthFunc(h.CreateWorkspace))
	r.Get("/api/knowledge/workspaces/{workspaceId}/stats", authMW.RequireAuthFunc(h.GetWorkspaceStats))
	r.Delete("/api/knowledge/workspaces/{workspaceId}", authMW.RequireAuthFunc(h.DeleteWorkspace))
	r.Post("/api/knowledge/workspaces/{workspaceId}/rebuild", authMW.RequireAuthFunc(h.RebuildWorkspace))
	r.Post("/api/knowledge/workspaces/reprocess", authMW.RequireAuthFunc(h.ReprocessAll))

	// Workspaces API — Extended
	r.Put("/api/knowledge/workspaces/{workspaceId}", authMW.RequireAuthFunc(h.UpdateWorkspace))

	// Models API
	r.Get("/api/knowledge/models", authMW.RequireAuthFunc(h.ListAllModels))
	r.Get("/api/knowledge/models/llm", authMW.RequireAuthFunc(h.ListLLMModels))
	r.Get("/api/knowledge/models/embedding", authMW.RequireAuthFunc(h.ListEmbeddingModels))

	// Settings API
	r.Get("/api/knowledge/settings/status", authMW.RequireAuthFunc(h.GetProviderStatus))
	r.Get("/api/knowledge/settings/providers", authMW.RequireAuthFunc(h.ListAvailableProviders))

	// Pipeline API
	r.Get("/api/knowledge/pipeline/enhanced-status", authMW.RequireAuthFunc(h.GetEnhancedPipelineStatus))
	r.Get("/api/knowledge/pipeline/queue-metrics", authMW.RequireAuthFunc(h.GetQueueMetrics))
	r.Post("/api/knowledge/pipeline/cancel", authMW.RequireAuthFunc(h.CancelPipelineTask))
	r.Post("/api/knowledge/pipeline/cancel-all", authMW.RequireAuthFunc(h.CancelPipeline))
	r.Post("/api/knowledge/pipeline/retry", authMW.RequireAuthFunc(h.RetryTask))

	// Graph API
	r.Get("/api/knowledge/graph", authMW.RequireAuthFunc(h.GetGraph))
	r.Get("/api/knowledge/graph/nodes/search", authMW.RequireAuthFunc(h.SearchGraphNodes))
	r.Get("/api/knowledge/graph/nodes/{nodeId}", authMW.RequireAuthFunc(h.GetGraphNode))
	r.Get("/api/knowledge/graph/labels/popular", authMW.RequireAuthFunc(h.GetPopularLabels))
	r.Get("/api/knowledge/graph/labels/search", authMW.RequireAuthFunc(h.SearchGraphLabels))
}

// Service returns the EdgeQuake service for cross-module usage.
// Used by task management, workspace permissions, custom runner adapter,
// and HandleViewChatInputScreen.
func (m *Module) Service() *edgequake.Service {
	return m.svc
}
