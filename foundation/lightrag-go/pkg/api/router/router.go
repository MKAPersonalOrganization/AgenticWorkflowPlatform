package router

import (
	"github.com/gin-gonic/gin"
	"github.com/lightrag/lightrag-go/pkg/api/handlers"
	"github.com/lightrag/lightrag-go/pkg/auth"
	"github.com/lightrag/lightrag-go/pkg/config"
	"github.com/lightrag/lightrag-go/pkg/middleware"
)

// Router wraps gin.Engine and provides route configuration
type Router struct {
	engine *gin.Engine
	cfg    *config.Config
}

// NewRouter creates a new router with all routes configured
func NewRouter(cfg *config.Config) *Router {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(gin.Logger())

	r := &Router{
		engine: engine,
		cfg:    cfg,
	}

	r.setupRoutes()
	return r
}

// Engine returns the underlying gin.Engine
func (r *Router) Engine() *gin.Engine {
	return r.engine
}

// setupRoutes configures all API routes
func (r *Router) setupRoutes() {
	// Create handlers
	authHandler := auth.NewAuthHandler(r.cfg)
	healthHandler := handlers.NewHealthHandler(r.cfg)
	queryHandler := handlers.NewQueryHandler(r.cfg)
	documentHandler := handlers.NewDocumentHandler(r.cfg)
	graphHandler := handlers.NewGraphHandler(r.cfg)
	ollamaHandler := handlers.NewOllamaHandler(r.cfg)
	loginHandler := handlers.NewAuthHandler(r.cfg, authHandler)

	// Setup middlewares
	corsMiddleware := middleware.NewCORSMiddleware(r.cfg)
	authMiddleware := middleware.NewAuthMiddleware(authHandler, r.cfg)

	// Apply global middlewares
	r.engine.Use(corsMiddleware.Handler())

	// Health check endpoint (no auth required)
	r.engine.GET("/health", healthHandler.Health)

	// Auth routes
	authGroup := r.engine.Group("/auth")
	{
		authGroup.POST("/login", loginHandler.Login)
		authGroup.POST("/guest-login", loginHandler.GuestLogin)
		authGroup.GET("/me", authMiddleware.Handler(), loginHandler.Me)
		authGroup.POST("/refresh", authMiddleware.Handler(), loginHandler.RefreshToken)
	}

	// API routes with authentication
	api := r.engine.Group("")
	api.Use(authMiddleware.Handler())
	{
		// Query routes
		api.POST("/query", queryHandler.Query)
		api.POST("/query/stream", queryHandler.QueryStream)
		api.POST("/retrieval", queryHandler.Retrieval)
		api.POST("/query_data", queryHandler.QueryData)

		// Document routes
		api.POST("/documents/text", documentHandler.InsertText)
		api.POST("/documents/texts", documentHandler.InsertTexts)
		api.POST("/documents/file", documentHandler.InsertFile)
		api.POST("/documents/batch", documentHandler.InsertBatch)
		api.POST("/documents/scan", documentHandler.Scan)
		api.GET("/documents", documentHandler.GetPaginatedDocuments)
		api.GET("/documents/:id", documentHandler.GetDocument)
		api.DELETE("/documents/:id", documentHandler.DeleteDocument)
		api.DELETE("/documents", documentHandler.DeleteDocuments)
		api.DELETE("/documents/all", documentHandler.ClearDocuments)
		api.GET("/documents/pipeline-status", documentHandler.GetPipelineStatus)
		api.POST("/documents/index-all", documentHandler.IndexAllDocuments)

		// Graph routes
		api.GET("/graphs/labels", graphHandler.GetLabels)
		api.GET("/graphs/:label", graphHandler.GetKnowledgeGraph)
		api.GET("/graphs/:label/text-chunks", graphHandler.GetTextChunks)
		api.PUT("/graphs/entities/:entity_id", graphHandler.UpdateEntity)
		api.POST("/graphs/entities/merge", graphHandler.MergeEntities)
		api.DELETE("/graphs/entities/:entity_id", graphHandler.DeleteEntity)
		api.DELETE("/graphs/relations/:relation_id", graphHandler.DeleteRelation)
		api.POST("/graphs/relations", graphHandler.CreateRelation)
		api.DELETE("/graphs/nodes/orphan", graphHandler.DeleteOrphanNodes)
	}

	// Ollama-compatible API routes
	ollamaAPI := r.engine.Group("/api")
	{
		ollamaAPI.GET("/version", ollamaHandler.GetVersion)
		ollamaAPI.GET("/tags", ollamaHandler.GetTags)
		ollamaAPI.GET("/ps", ollamaHandler.GetPs)
		ollamaAPI.POST("/generate", ollamaHandler.Generate)
		ollamaAPI.POST("/chat", ollamaHandler.Chat)
		ollamaAPI.POST("/embeddings", ollamaHandler.Embeddings)
	}

	// WebUI static file serving (if enabled)
	if r.cfg.EnableWebUI {
		r.setupWebUI()
	}
}

// setupWebUI configures static file serving for the web UI
func (r *Router) setupWebUI() {
	// Serve static files from webui directory
	r.engine.Static("/assets", "./webui/assets")
	r.engine.StaticFile("/", "./webui/index.html")
	r.engine.StaticFile("/favicon.ico", "./webui/favicon.ico")

	// Catch-all for SPA routing
	r.engine.NoRoute(func(c *gin.Context) {
		c.File("./webui/index.html")
	})
}
