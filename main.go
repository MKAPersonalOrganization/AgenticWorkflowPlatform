package main

import (
	"context"
	"crypto/tls"
	"errors"
	"expvar"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"school-with-ai/components"
	"school-with-ai/components/checkout"
	"school-with-ai/components/pricing"
	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/adapters/custom"
	"school-with-ai/foundation/debug"
	"school-with-ai/foundation/event"
	"school-with-ai/foundation/i18n"
	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/middleware"
	"school-with-ai/handlers/auth"
	"school-with-ai/handlers/profile"
	statisticsHandler "school-with-ai/handlers/statistics"
	pgstore "school-with-ai/infra/eventstore/postgres"
	"school-with-ai/infra/kratos"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/infra/projection/llmcontext"
	projectionStats "school-with-ai/infra/projection/statistics"
	"school-with-ai/infra/projection/subscriber"
	"school-with-ai/infra/projections"
	"school-with-ai/infra/sqldb"
	"school-with-ai/infra/sqldb/migrate"
	"school-with-ai/services/agentorchestrator"
	"school-with-ai/services/aisettings"
	"school-with-ai/services/artifact"
	"school-with-ai/services/edgequake"
	"school-with-ai/services/memory"
	"school-with-ai/services/permissions"
	"school-with-ai/services/rag/users"
	"school-with-ai/services/resources"
	"school-with-ai/services/taskmanagement"
	"school-with-ai/services/threadmgmt"
	"school-with-ai/services/toolregistry"
	"school-with-ai/services/workspacepermissions"

	"school-with-ai/services/agentorchestrator/agents/math"
	"school-with-ai/services/agentorchestrator/agents/statistics"

	aguihandlers "school-with-ai/handlers/agui"
	aiSettingsHandlers "school-with-ai/handlers/aisettings"
	bannerhandlers "school-with-ai/handlers/banner"
	healthhandlers "school-with-ai/handlers/health"
	knowledgehandlers "school-with-ai/handlers/knowledge"
	resourceshandlers "school-with-ai/handlers/resources"
	taskmgmthandlers "school-with-ai/handlers/taskmgmt"
	wkspermhandlers "school-with-ai/handlers/workspacepermissions"

	authService "school-with-ai/services/auth"

	"github.com/ardanlabs/conf/v3"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

var build = "develop"

func main() {
	var log *logger.Logger

	events := logger.Events{
		Error: func(ctx context.Context, r logger.Record) {
			log.Info(ctx, "******* SEND ALERT *******")
		},
	}

	log = logger.NewTextWithEvents(os.Stdout, logger.LevelInfo, "SCHOOL", nil, events)

	// -------------------------------------------------------------------------

	ctx := context.Background()

	if err := run(ctx, log); err != nil {
		log.Error(ctx, "startup", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *logger.Logger) error {
	// -------------------------------------------------------------------------
	// GOMAXPROCS

	log.Info(ctx, "startup", "GOMAXPROCS", runtime.GOMAXPROCS(0))

	// -------------------------------------------------------------------------
	// Configuration

	cfg := struct {
		conf.Version
		Web struct {
			ReadTimeout       time.Duration `conf:"default:60s"`
			WriteTimeout      time.Duration `conf:"default:60s"`
			IdleTimeout       time.Duration `conf:"default:60s"`
			ReadHeaderTimeout time.Duration `conf:"default:10s"`
			ShutdownTimeout   time.Duration `conf:"default:20s"`
			APIHost           string        `conf:"default:localhost:8090"`
			DebugHost         string        `conf:"default:0.0.0.0:4010"`
			TLSCert           string        `conf:"default:./tls/cert.pem"`
			TLSKey            string        `conf:"default:./tls/key.pem"`
		}
		Auth struct {
			AppURL        string `conf:"default:https://localhost:8090"`
			KratosPublic  string `conf:"default:http://localhost:4433"`
			KratosAdmin   string `conf:"default:http://localhost:4434"`
			KratosBrowser string `conf:"default:http://localhost:4433"`
			AdminAPIToken string
		}
		DashboardAdmin struct {
			User string `conf:"default:admin"`
			Pass string `conf:"default:admin,mask"`
		}
		DB struct {
			User         string `conf:"default:schoolai"`
			Password     string `conf:"default:dev-password,mask"`
			Host         string `conf:"default:localhost:5435"`
			Name         string `conf:"default:projections"`
			MaxIdleConns int    `conf:"default:100"`
			MaxOpenConns int    `conf:"default:100"`
			DisableTLS   bool   `conf:"default:true"`
		}
		Redis struct {
			Addr     string `conf:"default:localhost:6379"`
			Password string `conf:"mask"`
		}
		Orchestration struct {
			Framework           string        `conf:"default:custom"`
			Concurrency         int           `conf:"default:5"`
			ShutdownTimeout     time.Duration `conf:"default:10s"`
			HealthCheckInterval time.Duration `conf:"default:15s"`
		}

		Artifacts struct {
			S3Endpoint  string `conf:"default:localhost:9100"`
			S3AccessKey string `conf:"default:minio"`
			S3SecretKey string `conf:"default:miniosecret"`
			S3Bucket    string `conf:"default:artifacts"`
			S3UseSSL    bool   `conf:"default:false"`
			S3Region    string `conf:"default:us-east-1"`
		}
		AgentCapabilities struct {
			LongTermMemory bool `conf:"default:true"`
		}
		Edgequake struct {
			BaseURL  string `conf:"default:http://localhost:8081"`
			TenantID string
		}
	}{
		Version: conf.Version{
			Build: build,
			Desc:  "School with AI",
		},
	}

	const prefix = "SCHOOL"
	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}

	log.Info(ctx, "startup", "config_tenant_id", cfg.Edgequake.TenantID)

	// -------------------------------------------------------------------------
	// Production Configuration Guard
	//
	// Reject known development defaults when the binary is built for
	// production (build != "develop").

	if err := validateProductionConfig(cfg.Build, cfg.DashboardAdmin.User, cfg.DashboardAdmin.Pass, cfg.DB.Password, cfg.Artifacts.S3AccessKey, cfg.Artifacts.S3SecretKey); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Initialize I18n

	if err := i18n.LoadCatalogs("assets/locales"); err != nil {
		log.Error(ctx, "failed to load i18n catalogs", "error", err)
		// Don't fail matching, just continue with defaults
	}

	// -------------------------------------------------------------------------
	// App Starting

	log.Info(ctx, "starting service", "version", cfg.Build)
	defer log.Info(ctx, "shutdown complete")

	out, err := conf.String(&cfg)
	if err != nil {
		return fmt.Errorf("generating config for output: %w", err)
	}
	log.Info(ctx, "startup", "config", out)

	expvar.NewString("build").Set(cfg.Build)

	// -------------------------------------------------------------------------
	// Initialize Debug Mux

	debugMux := debug.Mux()

	// -------------------------------------------------------------------------
	// Database Support (Postgres — single database for events + projections)

	log.Info(ctx, "startup", "status", "initializing database support", "host", cfg.DB.Host)

	db, err := sqldb.Open(sqldb.Config{
		User:         cfg.DB.User,
		Password:     cfg.DB.Password,
		Host:         cfg.DB.Host,
		Name:         cfg.DB.Name,
		MaxIdleConns: cfg.DB.MaxIdleConns,
		MaxOpenConns: cfg.DB.MaxOpenConns,
		DisableTLS:   cfg.DB.DisableTLS,
	})
	if err != nil {
		return fmt.Errorf("connecting to db: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error(ctx, "shutdown", "status", "failed to close database", "ERROR", closeErr)
		}
	}()

	// Initialize PostgreSQL Event Store
	log.Info(ctx, "startup", "status", "initializing PostgreSQL event store")
	baseEventStore := pgstore.NewPostgresEventStore(db)

	// Initialize Student Profile Repository
	studentProfileRepo := sqldb.NewPostgresStudentProfileRepository(db)
	resourcesRepo := sqldb.NewPostgresResourcesRepository(db)

	// Run migrations
	log.Info(ctx, "startup", "status", "running database migrations")
	if err := migrate.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrating database: %w", err)
	}

	// -------------------------------------------------------------------------
	// Initialize Projections
	// persistenceRouter is used by the ProjectionUpdater to update projections AND broadcast to Redis
	persistenceRouter := event.NewRouter()

	// transientRouter is used by the orchestrator for non-persistent events (streaming deltas, etc.)
	transientRouter := event.NewRouter()

	// Initialize Redis broadcaster for cross-pod event delivery
	redisBroadcaster := subscriber.NewRedisBroadcaster(log, cfg.Redis.Addr, cfg.Redis.Password)
	defer func() {
		if closeErr := redisBroadcaster.Close(); closeErr != nil {
			log.Error(ctx, "shutdown", "status", "failed to close Redis broadcaster", "ERROR", closeErr)
		}
	}()

	// Initialize Redis client for distributed rate limiting (shared across pods)
	rateLimitRedis := goredis.NewClient(&goredis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password})
	defer func() {
		if closeErr := rateLimitRedis.Close(); closeErr != nil {
			log.Error(ctx, "shutdown", "status", "failed to close rate-limit Redis client", "ERROR", closeErr)
		}
	}()
	log.Info(ctx, "startup", "status", "rate-limit Redis client initialized", "addr", cfg.Redis.Addr)

	// Register Redis broadcaster to BOTH routers
	// persistenceRouter will broadcast durable events once they are persisted
	// transientRouter will broadcast transient events immediately
	persistenceRouter.RegisterAny(redisBroadcaster.Broadcast)
	transientRouter.RegisterAny(redisBroadcaster.Broadcast)

	// Initialize PostgreSQL AGUI projection
	aguiProjection := agui.NewPostgresAguiProjection(log, db)
	aguiProjection.RegisterHandlers(persistenceRouter)

	// Initialize PostgreSQL LLM context projection
	llmContextProjection := llmcontext.NewPostgresLLMContextProjection(log, db)
	llmContextProjection.RegisterHandlers(persistenceRouter)

	// Use the Postgres event store directly as the write model
	eventLog := baseEventStore

	// -------------------------------------------------------------------------
	// Start Projection Updater (polls Postgres event store for new events)

	log.Info(ctx, "startup", "status", "starting projection updater")

	// Create projection updater with the persistence router
	projectionUpdater := subscriber.NewProjectionUpdater(
		log,
		baseEventStore,
		db,
		persistenceRouter,
	)

	// Initialize Statistics Projection (Postgres)
	statsProjection := projectionStats.NewPostgresProjection(db)
	statsProjection.RegisterHandlers(persistenceRouter)

	// Start projection updater in background with a cancellable context
	projCtx, projCancel := context.WithCancel(ctx)
	defer projCancel()

	go func() {
		if err := projectionUpdater.Start(projCtx); err != nil && projCtx.Err() == nil {
			log.Error(ctx, "projection updater stopped", "ERROR", err)
		}
	}()

	// Initialize per-user AI settings
	aiSettingsRepo := sqldb.NewPostgresAISettingsRepository(db)
	aiSettingsService := aisettings.New(aiSettingsRepo)
	log.Info(ctx, "startup", "status", "ai settings service initialized (per-user)")

	// Initialize artifact service
	log.Info(ctx, "startup", "status", "initializing s3 artifact service", "endpoint", cfg.Artifacts.S3Endpoint, "bucket", cfg.Artifacts.S3Bucket)
	artifactService, err := artifact.NewS3Service(ctx, artifact.S3Config{
		Endpoint:        cfg.Artifacts.S3Endpoint,
		AccessKeyID:     cfg.Artifacts.S3AccessKey,
		SecretAccessKey: cfg.Artifacts.S3SecretKey,
		UseSSL:          cfg.Artifacts.S3UseSSL,
		BucketName:      cfg.Artifacts.S3Bucket,
		Region:          cfg.Artifacts.S3Region,
	})
	if err != nil {
		return fmt.Errorf("initializing s3 artifact service: %w", err)
	}

	// Initialize memory service
	memoryService := memory.NewArtifactMemoryService(artifactService)

	// Initialize resources service
	resourcesService := resources.NewService(resourcesRepo, artifactService)

	// Create and initialize tool registry
	toolRegistry := toolregistry.NewRegistry(log, sqldb.NewPostgresToolRegistryRepository(db))
	if err := toolRegistry.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize tool registry: %w", err)
	}
	log.Info(ctx, "startup", "status", "tool registry initialized")

	// Create and initialize permissions service
	permissionsService := permissions.NewService(log, sqldb.NewPostgresPermissionsRepository(db))
	log.Info(ctx, "startup", "status", "permissions service initialized")

	// Create workspace permissions service
	workspacePermissionsService := workspacepermissions.NewService(log, sqldb.NewPostgresWorkspacePermissionsRepository(db))
	log.Info(ctx, "startup", "status", "workspace permissions service initialized")

	// Initialize Edgequake service
	edgequakeService := edgequake.NewService(cfg.Edgequake.BaseURL)
	log.Info(ctx, "startup", "status", "edgequake service initialized", "baseURL", cfg.Edgequake.BaseURL)

	// Initialize Agents directly (Step 2 of Registry Removal)
	// Initialize Agents directly (Step 2 of Registry Removal) - Updated for Service Decoupling
	slogLogger := slog.New(log)
	agentLoggingMw := agent.AgentLogging(slogLogger)
	chatLoggingMw := agent.ChatLogging(slogLogger)
	toolLoggingMw := agent.ToolLogging(slogLogger)

	mathConfig, err := math.NewMathInstructorConfig(ctx, artifactService)
	if err != nil {
		return fmt.Errorf("creating math instructor config: %w", err)
	}
	mathConfig.AgentMiddlewares = []agent.Middleware{agentLoggingMw}
	mathConfig.ChatMiddlewares = []agent.Middleware{chatLoggingMw}
	mathConfig.FunctionMiddlewares = []agent.Middleware{toolLoggingMw}
	mathInstructor, err := custom.NewAgent(mathConfig)
	if err != nil {
		return fmt.Errorf("creating math instructor agent: %w", err)
	}

	statsConfig, err := statistics.NewStatisticsInstructorConfig(ctx)
	if err != nil {
		return fmt.Errorf("creating statistics instructor config: %w", err)
	}
	statsConfig.AgentMiddlewares = []agent.Middleware{agentLoggingMw}
	statsConfig.ChatMiddlewares = []agent.Middleware{chatLoggingMw}
	statsConfig.FunctionMiddlewares = []agent.Middleware{toolLoggingMw}
	statsInstructor, err := custom.NewAgent(statsConfig)
	if err != nil {
		return fmt.Errorf("creating statistics instructor agent: %w", err)
	}

	// Create a simple map of available agents
	availableAgents := map[string]agent.Agent{
		"math-instructor":       mathInstructor,
		"statistics-instructor": statsInstructor,
	}
	agentProvider := custom.NewMapAgentProvider(availableAgents)

	var runner agentorchestrator.Runner
	var agentOrchestrator *agentorchestrator.Service
	switch cfg.Orchestration.Framework {
	case "custom":
		agentOrchestrator = agentorchestrator.New(log, eventLog,
			agentorchestrator.WithArtifactService(artifactService),
			agentorchestrator.WithStudentProfileRepository(studentProfileRepo),
			agentorchestrator.WithEmitter(transientRouter),
		)

		// Custom adapter creates its own LLM client internally
		var err error
		runner, err = custom.NewRunnerAdapter(
			ctx,
			log,
			agentOrchestrator, // ThreadManager interface
			agentProvider,     // AgentProvider
			aiSettingsService, // Per-user model config resolver
			artifactService,
			studentProfileRepo,
			cfg.AgentCapabilities.LongTermMemory,
			toolRegistry,                // Tool registry service
			permissionsService,          // Permissions service
			workspacePermissionsService, // Workspace permissions
			edgequakeService,            // Edgequake service for KB tools
			cfg.Edgequake.BaseURL,       // Edgequake base URL for raw KB queries
			cfg.Edgequake.TenantID,      // Tenant ID for edgequake
		)
		if err != nil {
			return fmt.Errorf("creating custom runner: %w", err)
		}

		// Wire the runner back to the service to complete the dependency cycle
		agentOrchestrator.SetRunner(runner)
	default:
		return fmt.Errorf("unsupported agent framework: %s", cfg.Orchestration.Framework)
	}

	// -------------------------------------------------------------------------
	// Start Debug Service

	debugMux.HandleFunc("/api/debug/state-machines/engine", func(w http.ResponseWriter, r *http.Request) {
		// Engine and tool graphs are now handled by the custom adapter
		// but we need to expose them if needed.
		// (Note: Currently removed from Service to keep it agnostic)
		http.Error(w, "Debug graphs temporarily unavailable during refactor", http.StatusNotFound)
	})

	debugServer := &http.Server{
		Addr:              cfg.Web.DebugHost,
		Handler:           debugMux,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info(ctx, "startup", "status", "debug server started", "host", cfg.Web.DebugHost)

		if err := debugServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(ctx, "shutdown", "status", "debug server closed", "host", cfg.Web.DebugHost, "msg", err)
		}
	}()

	threadRepo := domain.NewThreadRepository(threadmgmt.NewEventStoreAdapter(eventLog))
	threadMgmt := threadmgmt.New(log, threadRepo)

	// Initialize service layer
	authSvc := authService.NewAuthService()
	threadQueryService := projections.NewUIProjectionService(aguiProjection)

	// -------------------------------------------------------------------------
	// Initialize Asynq server

	log.Info(ctx, "startup", "status", "initializing asynq server", "redis", cfg.Redis.Addr)

	// (agentFactory removed)

	taskManager := taskmanagement.NewAsynqTaskManager(
		log,
		taskmanagement.AsynqConfig{
			RedisAddr:           cfg.Redis.Addr,
			RedisPassword:       cfg.Redis.Password,
			Concurrency:         cfg.Orchestration.Concurrency,
			ShutdownTimeout:     cfg.Orchestration.ShutdownTimeout,
			HealthCheckInterval: cfg.Orchestration.HealthCheckInterval,
		},
		agentOrchestrator, // Pass orchestrator instead of runner

	)
	defer taskManager.Shutdown()
	log.Info(ctx, "startup", "status", "task manager started",
		"redis", cfg.Redis.Addr,
		"concurrency", cfg.Orchestration.Concurrency,
		"shutdownTimeout", cfg.Orchestration.ShutdownTimeout,
		"healthCheckInterval", cfg.Orchestration.HealthCheckInterval)

	// Create HTTP handler
	handler := taskmgmthandlers.NewTaskManagementHandler(log, taskManager, agentProvider, authSvc, threadQueryService, aguiProjection, redisBroadcaster, threadMgmt, artifactService, memoryService, toolRegistry, permissionsService, edgequakeService, cfg.Edgequake.TenantID, cfg.Auth.AppURL)

	// Create health handler with dependency checkers
	healthHandler := healthhandlers.NewHealthHandler(log,
		healthhandlers.WithChecker("db", healthhandlers.CheckerFunc(func(ctx context.Context) error {
			return db.PingContext(ctx)
		})),
	)

	// Create chi router (Root Router)
	root := chi.NewRouter()

	// Add panic recovery middleware first (most important - must be first)
	root.Use(middleware.Panics())

	// Add security headers middleware (early — sets headers before response body)
	root.Use(middleware.SecurityHeaders())

	// Add request body size limit middleware (1 MB default for JSON APIs).
	// Multipart/form-data requests (file uploads) are excluded — handlers
	// control their own limits via ParseMultipartForm.
	root.Use(middleware.BodyLimit(0))

	// Add per-IP rate limiting (Moved to specific route groups below for split limits)
	// r.Use(middleware.RateLimiter(100, 300))

	// Add logging middleware
	root.Use(middleware.Logger(log))

	// Add metrics middleware
	root.Use(middleware.Metrics())

	// Add I18n middleware (detects lang from query/cookie/header)
	root.Use(middleware.I18n)

	// Add chi middleware (we can keep or remove these depending on preference)
	// Note: We now have our own Logger and Recoverer, so chi's can be removed
	// r.Use(chimiddleware.Logger)
	// r.Use(chimiddleware.Recoverer)

	// Serve static assets — Mount uses a catch-all so nested paths
	// like /assets/js/chat/index.js are served correctly.
	// Use higher rate limit for assets (100 req/s, 300 burst) to support parallel loading
	root.Group(func(r chi.Router) {
		r.Use(middleware.RateLimiter(rateLimitRedis, 100, 300))
		r.Mount("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	})

	// Create App Router for all other routes
	// This router will enforce strict rate limits (10 req/s, 20 burst)
	r := chi.NewRouter()
	r.Use(middleware.RateLimiter(rateLimitRedis, 10, 20))

	// Mount App Router on Root
	root.Mount("/", r)

	// Create AGUI projection handler
	aguiHandler := aguihandlers.NewAguiProjectionHandler(aguiProjection, threadQueryService)

	// Initialize Kratos client and auth handlers
	kratosConfig := kratos.Config{
		AppURL:        cfg.Auth.AppURL,
		KratosPublic:  cfg.Auth.KratosPublic,
		KratosAdmin:   cfg.Auth.KratosAdmin,
		KratosBrowser: cfg.Auth.KratosBrowser,
		AdminAPIToken: cfg.Auth.AdminAPIToken,
	}
	kratosClient := kratos.NewKratosClient(kratosConfig)
	authHandlers := auth.NewAuthHandlers(log, kratosClient, cfg.Auth.AppURL)
	authMiddleware := auth.NewMiddleware(kratosClient, cfg.Auth.AppURL)

	// Profile Handler
	profileHandler := profile.NewHandler(log, studentProfileRepo)

	// Resources Handler
	resourcesHandler := resourceshandlers.NewHandler(resourcesService)

	// Workspace Permissions Handler
	workspacePermHandler := wkspermhandlers.NewHandler(log, workspacePermissionsService, edgequakeService, agentProvider, cfg.Edgequake.TenantID)

	// Banner Handler
	bannerHandler := bannerhandlers.NewHandler(log)
	r.Get("/api/banner", authMiddleware.RequireAuthFunc(bannerHandler.GetBanner))

	// Health check endpoints (public)
	r.Get("/liveness", healthHandler.Liveness)
	r.Get("/readiness", healthHandler.Readiness)

	// Auth routes (public)
	r.Get("/auth/login", authHandlers.LoginPage)
	r.Get("/auth/registration", authHandlers.RegistrationPage)
	r.Get("/auth/recovery", authHandlers.RecoveryPage)
	r.Get("/auth/verification", authHandlers.VerificationPage)
	r.Get("/auth/logout", authHandlers.LogoutHandler)
	r.Get("/auth/error", authHandlers.ErrorPage)

	// Protected auth routes
	r.Get("/auth/settings", authMiddleware.RequireAuthFunc(authHandlers.SettingsPage))

	// Profile routes (protected)
	r.Get("/profile", authMiddleware.RequireAuthFunc(profileHandler.HandleProfilePage))
	r.Get("/api/profile", authMiddleware.RequireAuthFunc(profileHandler.GetProfile))
	r.Put("/api/profile", authMiddleware.RequireAuthFunc(profileHandler.UpdateProfile))

	// AI Settings routes (protected)
	aiSettingsHandler := aiSettingsHandlers.NewHandler(log, aiSettingsService)
	r.Get("/api/ai-settings/providers", authMiddleware.RequireAuthFunc(aiSettingsHandler.ListProviders))
	r.Get("/api/ai-settings", authMiddleware.RequireAuthFunc(aiSettingsHandler.GetSettings))
	r.Put("/api/ai-settings", authMiddleware.RequireAuthFunc(aiSettingsHandler.SaveSettings))
	r.Delete("/api/ai-settings/{id}", authMiddleware.RequireAuthFunc(aiSettingsHandler.DeleteSettings))
	r.Put("/api/ai-settings/{id}/default", authMiddleware.RequireAuthFunc(aiSettingsHandler.SetDefault))

	// Analytics Dashboard (dedicated Basic Auth)
	statsHandler := statisticsHandler.NewHandler(log, statsProjection, cfg.DashboardAdmin.User, cfg.DashboardAdmin.Pass)
	r.Get("/analytics", statsHandler.BasicAuth(statsHandler.ServeDashboard))
	r.Get("/analytics/api/timeseries", statsHandler.BasicAuth(statsHandler.ServeToolTimeSeries))
	r.Get("/analytics/api/tools/{tool}", statsHandler.BasicAuth(statsHandler.ServeToolDetail))

	// Pricing page
	r.Get("/pricing", authMiddleware.RequireAuthFunc(HandlePricingPage(log)))

	// Checkout page
	r.Get("/checkout", authMiddleware.RequireAuthFunc(HandleCheckoutPage(log)))

	r.Post("/v1/message:send", authMiddleware.RequireAuthFunc(handler.SendMessage))
	r.Get("/v1/tasks/{id}:subscribe", authMiddleware.RequireAuthFunc(handler.StreamJobEvents))
	r.Post("/v1/tasks/{id}:cancel", authMiddleware.RequireAuthFunc(handler.CancelJob))
	r.Get("/v1/agents", authMiddleware.RequireAuthFunc(handler.ListAgents))
	r.Get("/v1/threads/{threadId}/artifacts", authMiddleware.RequireAuthFunc(handler.ListArtifacts))
	r.Get("/v1/threads/{threadId}/artifacts/{fileName}", authMiddleware.RequireAuthFunc(handler.DownloadArtifact))
	r.Get("/v1/memory", authMiddleware.RequireAuthFunc(handler.GetMemory))
	r.Get("/v1/agents/{agentId}/toolsets", authMiddleware.RequireAuthFunc(handler.ListAgentToolSets))
	r.Put("/v1/agents/{agentId}/toolsets/{toolSetId}", authMiddleware.RequireAuthFunc(handler.ToggleAgentToolSet))
	r.Put("/v1/agents/{agentId}/toolsets/{toolSetId}/tools/{toolId}", authMiddleware.RequireAuthFunc(handler.ToggleIndividualTool))
	r.Get("/v1/documents", authMiddleware.RequireAuthFunc(handler.ListDocuments))
	r.Post("/v1/documents", authMiddleware.RequireAuthFunc(handler.UploadDocument))

	// Workspace permission routes
	r.Get("/v1/agents/{agentId}/workspaces", authMiddleware.RequireAuthFunc(workspacePermHandler.ListAgentWorkspaces))
	r.Put("/v1/agents/{agentId}/workspaces/{workspaceId}", authMiddleware.RequireAuthFunc(workspacePermHandler.ToggleAgentWorkspace))

	// AGUI projection query endpoints (protected)
	r.Get("/api/agui/threads", authMiddleware.RequireAuthFunc(aguiHandler.ListThreads))
	r.Get("/api/agui/threads/{threadId}", authMiddleware.RequireAuthFunc(aguiHandler.GetThreadState))
	r.Get("/api/agui/threads/{threadId}/messages", authMiddleware.RequireAuthFunc(aguiHandler.GetMessages))
	r.Delete("/api/agui/threads/{threadId}", authMiddleware.RequireAuthFunc(handler.DeleteThread))
	r.Patch("/api/agui/threads/{threadId}", authMiddleware.RequireAuthFunc(handler.UpdateThreadTitle))

	// Resources routes
	r.Get("/api/resources", authMiddleware.RequireAuthFunc(resourcesHandler.List))
	r.Post("/api/resources/upload", authMiddleware.RequireAuthFunc(resourcesHandler.Upload))
	r.Get("/api/resources/{id}/content", authMiddleware.RequireAuthFunc(resourcesHandler.GetContent))
	r.Delete("/api/resources/{id}", authMiddleware.RequireAuthFunc(resourcesHandler.Delete))

	// Knowledge Base / RAG routes
	knowledgeHandler := knowledgehandlers.NewHandler(log, edgequakeService, cfg.Edgequake.TenantID)

	// Pages
	r.Get("/knowledge", authMiddleware.RequireAuthFunc(knowledgeHandler.WorkspacesPage))
	r.Get("/knowledge/dashboard", authMiddleware.RequireAuthFunc(knowledgeHandler.DashboardPage))
	r.Get("/knowledge/workspaces/{workspaceId}", authMiddleware.RequireAuthFunc(knowledgeHandler.WorkspaceDetailsPage))

	// Documents API (maps to Edgequake /api/v1/documents)
	r.Get("/api/knowledge/documents", authMiddleware.RequireAuthFunc(knowledgeHandler.ListDocuments))
	r.Get("/api/knowledge/documents/{documentId}", authMiddleware.RequireAuthFunc(knowledgeHandler.GetDocument))
	r.Post("/api/knowledge/documents/upload", authMiddleware.RequireAuthFunc(knowledgeHandler.UploadDocument))
	r.Delete("/api/knowledge/documents/{documentId}", authMiddleware.RequireAuthFunc(knowledgeHandler.DeleteDocument))
	r.Delete("/api/knowledge/documents", authMiddleware.RequireAuthFunc(knowledgeHandler.DeleteAllDocuments))
	r.Post("/api/knowledge/documents/reprocess", authMiddleware.RequireAuthFunc(knowledgeHandler.ReprocessDocument))
	r.Post("/api/knowledge/documents/reprocess-failed", authMiddleware.RequireAuthFunc(knowledgeHandler.ReprocessFailed))
	r.Post("/api/knowledge/documents/scan", authMiddleware.RequireAuthFunc(knowledgeHandler.ScanDocuments))

	// PDFs API (maps to Edgequake /api/v1/documents/pdf)
	r.Get("/api/knowledge/pdfs", authMiddleware.RequireAuthFunc(knowledgeHandler.ListPDFs))
	r.Post("/api/knowledge/pdfs/upload", authMiddleware.RequireAuthFunc(knowledgeHandler.UploadPDF))
	r.Delete("/api/knowledge/pdfs/{pdfId}", authMiddleware.RequireAuthFunc(knowledgeHandler.DeletePDF))
	r.Get("/api/knowledge/pdfs/{pdfId}/content", authMiddleware.RequireAuthFunc(knowledgeHandler.GetPDFContent))
	r.Get("/api/knowledge/pdfs/{pdfId}/download", authMiddleware.RequireAuthFunc(knowledgeHandler.DownloadPDF))
	r.Get("/api/knowledge/pdfs/{trackId}/progress", authMiddleware.RequireAuthFunc(knowledgeHandler.GetPDFProgress))
	r.Post("/api/knowledge/pdfs/{pdfId}/retry", authMiddleware.RequireAuthFunc(knowledgeHandler.RetryPdfProcessing))

	// Workspaces API (maps to Edgequake /api/v1/workspaces)
	r.Get("/api/knowledge/workspaces", authMiddleware.RequireAuthFunc(knowledgeHandler.ListWorkspaces))
	r.Post("/api/knowledge/workspaces", authMiddleware.RequireAuthFunc(knowledgeHandler.CreateWorkspace))
	r.Get("/api/knowledge/workspaces/{workspaceId}/stats", authMiddleware.RequireAuthFunc(knowledgeHandler.GetWorkspaceStats))
	r.Delete("/api/knowledge/workspaces/{workspaceId}", authMiddleware.RequireAuthFunc(knowledgeHandler.DeleteWorkspace))
	r.Post("/api/knowledge/workspaces/{workspaceId}/rebuild", authMiddleware.RequireAuthFunc(knowledgeHandler.RebuildWorkspace))
	r.Post("/api/knowledge/workspaces/reprocess", authMiddleware.RequireAuthFunc(knowledgeHandler.ReprocessAll))

	// Models API (maps to Edgequake /api/v1/models)
	r.Get("/api/knowledge/models/llm", authMiddleware.RequireAuthFunc(knowledgeHandler.ListLLMModels))
	r.Get("/api/knowledge/models/embedding", authMiddleware.RequireAuthFunc(knowledgeHandler.ListEmbeddingModels))

	// Pipeline API (maps to Edgequake /api/v1/pipeline + /api/v1/tasks)
	r.Get("/api/knowledge/pipeline/enhanced-status", authMiddleware.RequireAuthFunc(knowledgeHandler.GetEnhancedPipelineStatus))
	r.Get("/api/knowledge/pipeline/queue-metrics", authMiddleware.RequireAuthFunc(knowledgeHandler.GetQueueMetrics))
	r.Post("/api/knowledge/pipeline/cancel", authMiddleware.RequireAuthFunc(knowledgeHandler.CancelPipelineTask))
	r.Post("/api/knowledge/pipeline/cancel-all", authMiddleware.RequireAuthFunc(knowledgeHandler.CancelPipeline))
	r.Post("/api/knowledge/pipeline/retry", authMiddleware.RequireAuthFunc(knowledgeHandler.RetryTask))

	// Graph API (maps to Edgequake /api/v1/graph)
	r.Get("/api/knowledge/graph", authMiddleware.RequireAuthFunc(knowledgeHandler.GetGraph))
	r.Get("/api/knowledge/graph/nodes/search", authMiddleware.RequireAuthFunc(knowledgeHandler.SearchGraphNodes))
	r.Get("/api/knowledge/graph/nodes/{nodeId}", authMiddleware.RequireAuthFunc(knowledgeHandler.GetGraphNode))
	r.Get("/api/knowledge/graph/labels/popular", authMiddleware.RequireAuthFunc(knowledgeHandler.GetPopularLabels))
	r.Get("/api/knowledge/graph/labels/search", authMiddleware.RequireAuthFunc(knowledgeHandler.SearchGraphLabels))

	// Home route
	r.Get("/", authMiddleware.RequireAuthFunc(HandleViewChatInputScreen(log, edgequakeService)))

	// Configure TLS: restrict to elliptic curves with assembly implementations
	// (X25519 and P256) to ensure the server remains performant under heavy load.
	tlsConfig := &tls.Config{
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
	}

	// Create server with proper timeouts to prevent slowloris and similar attacks
	server := &http.Server{
		Addr:              cfg.Web.APIHost,
		Handler:           root,
		ReadTimeout:       cfg.Web.ReadTimeout,
		ReadHeaderTimeout: cfg.Web.ReadHeaderTimeout,
		WriteTimeout:      cfg.Web.WriteTimeout,
		IdleTimeout:       cfg.Web.IdleTimeout,
		TLSConfig:         tlsConfig,
	}

	// -------------------------------------------------------------------------
	// Start API Service

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	serverErrors := make(chan error, 1)

	go func() {
		log.Info(ctx, "startup", "status", "api router started (HTTPS)", "host", server.Addr)

		serverErrors <- server.ListenAndServeTLS(cfg.Web.TLSCert, cfg.Web.TLSKey)
	}()

	log.Info(ctx, "startup", "msg", "Press CTRL+C or send SIGTERM to shutdown")

	// -------------------------------------------------------------------------
	// Shutdown

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Info(ctx, "shutdown", "status", "shutdown started", "signal", sig)
		defer log.Info(ctx, "shutdown", "status", "shutdown complete", "signal", sig)

		// Cancel the projection updater goroutine
		projCancel()

		ctx, cancel := context.WithTimeout(ctx, cfg.Web.ShutdownTimeout)
		defer cancel()

		// Shut down the debug server gracefully
		if err := debugServer.Shutdown(ctx); err != nil {
			log.Error(ctx, "shutdown", "status", "debug server shutdown error", "msg", err)
		}

		if err := server.Shutdown(ctx); err != nil {
			if closeErr := server.Close(); closeErr != nil {
				return fmt.Errorf("could not stop server gracefully (shutdown: %w, close: %v)", err, closeErr)
			}
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil
}

func HandleViewChatInputScreen(log *logger.Logger, edgequakeService *edgequake.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user from context
		user := domain.GetUserFromContext(r.Context())

		// Sync user to RAG (Idempotent)
		if user != nil {
			go func(u *domain.User) {
				// Generate a random password since RAG requires one, but we don't use it for auth
				randomPass := uuid.New().String()

				// Create user request
				req := &users.CreateUserRequest{
					Username: u.GetDisplayName(),
					Email:    u.Email,
					Password: randomPass,
					Role:     &u.Role,
				}

				// Use a bounded context so this goroutine cannot block forever
				syncCtx, syncCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer syncCancel()

				_, err := edgequakeService.Users.Create(syncCtx, req)
				if err != nil {
					// Log error but don't fail the request (best effort sync)
					// In production, we might want to check if error is "already exists" and ignore
					log.Error(syncCtx, "failed to sync user to RAG", "error", err, "email", u.Email)
				} else {
					log.Info(syncCtx, "synced user to RAG", "email", u.Email, "role", u.Role)
				}
			}(user)
		}

		component := components.ChatInputScreen(user)
		if err := component.Render(r.Context(), w); err != nil {
			log.Error(r.Context(), "failed to render chat input screen", "error", err)
		}
	}
}

func HandlePricingPage(log *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user from context
		user := domain.GetUserFromContext(r.Context())

		// Get default pricing plans
		plans := pricing.GetDefaultPlans(r.Context())

		component := pricing.PricingPage(pricing.PricingPageProps{
			Plans: plans,
			User:  user,
		})
		if err := component.Render(r.Context(), w); err != nil {
			log.Error(r.Context(), "failed to render pricing page", "error", err)
		}
	}
}

func HandleCheckoutPage(log *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user from context
		user := domain.GetUserFromContext(r.Context())

		// Get plan from query parameter
		planID := r.URL.Query().Get("plan")
		if planID == "" {
			http.Redirect(w, r, "/pricing", http.StatusSeeOther)
			return
		}

		// Find the plan details
		plans := pricing.GetDefaultPlans(r.Context())
		var selectedPlan *pricing.Plan
		for _, p := range plans {
			if p.ID == planID {
				selectedPlan = &p
				break
			}
		}

		if selectedPlan == nil {
			http.Redirect(w, r, "/pricing", http.StatusSeeOther)
			return
		}

		// Convert to checkout plan details
		planDetails := checkout.PlanDetails{
			ID:       selectedPlan.ID,
			Name:     selectedPlan.Name,
			Price:    selectedPlan.Price,
			Period:   selectedPlan.Period,
			Features: selectedPlan.Features,
		}

		component := checkout.CheckoutPage(checkout.CheckoutPageProps{
			Plan: planDetails,
			User: user,
		})
		if err := component.Render(r.Context(), w); err != nil {
			log.Error(r.Context(), "failed to render checkout page", "error", err)
		}
	}
}

func renderGraph(w http.ResponseWriter, log *logger.Logger, r *http.Request, graph string) {
	format := r.URL.Query().Get("format")
	if format == "dot" || format == "text" {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(graph))
		return
	}

	cmd := exec.Command("dot", "-Tsvg")
	cmd.Stdin = strings.NewReader(graph)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error(r.Context(), "failed to render graph", "error", err, "output", string(output))
		http.Error(w, "Failed to render graph", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Write(output)
}

// validateProductionConfig rejects known insecure defaults when the
// binary is built for production (build != "develop"). This prevents
// accidental deployment with development credentials.
func validateProductionConfig(buildTag, adminUser, adminPass, dbPass, s3Key, s3Secret string) error {
	if buildTag == "develop" {
		return nil
	}

	insecure := map[string]string{
		"dashboard admin user":     adminUser,
		"dashboard admin password": adminPass,
		"database password":        dbPass,
		"S3 access key":            s3Key,
		"S3 secret key":            s3Secret,
	}

	defaults := map[string]bool{
		"admin":        true,
		"dev-password": true,
		"minio":        true,
		"miniosecret":  true,
	}

	for label, val := range insecure {
		if defaults[val] {
			return fmt.Errorf("production config guard: %s is set to the development default %q — set a secure value", label, val)
		}
	}

	return nil
}
