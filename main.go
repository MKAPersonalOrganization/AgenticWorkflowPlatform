package main

import (
	"context"
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
	"school-with-ai/infra/eventstore/kurrentdb"
	"school-with-ai/infra/kratos"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/infra/projection/llmcontext"
	projectionStats "school-with-ai/infra/projection/statistics"
	"school-with-ai/infra/projection/subscriber"
	"school-with-ai/infra/projections"
	"school-with-ai/infra/sqldb"
	"school-with-ai/infra/sqldb/migrate"
	"school-with-ai/services/agentorchestrator"
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

	"school-with-ai/services/agentorchestrator/agents/chemistry"
	"school-with-ai/services/agentorchestrator/agents/diagramming"
	"school-with-ai/services/agentorchestrator/agents/math"
	"school-with-ai/services/agentorchestrator/agents/physics"
	"school-with-ai/services/agentorchestrator/agents/statistics"

	aguihandlers "school-with-ai/handlers/agui"
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
			ReadTimeout       time.Duration `conf:"default:15s"`
			WriteTimeout      time.Duration `conf:"default:15s"`
			IdleTimeout       time.Duration `conf:"default:60s"`
			ReadHeaderTimeout time.Duration `conf:"default:10s"`
			ShutdownTimeout   time.Duration `conf:"default:20s"`
			APIHost           string        `conf:"default:localhost:8090"`
			DebugHost         string        `conf:"default:0.0.0.0:4010"`
		}
		Auth struct {
			AppURL        string `conf:"default:http://localhost:8090"`
			KratosPublic  string `conf:"default:http://localhost:4433"`
			KratosAdmin   string `conf:"default:http://localhost:4434"`
			KratosBrowser string `conf:"default:http://localhost:4433"`
			AdminAPIToken string
		}
		DashboardAdmin struct {
			User string `conf:"default:admin"`
			Pass string `conf:"default:admin,mask"`
		}
		KurrentDB struct {
			ConnectionString string `conf:"default:esdb://localhost:2113?tls=false"`
		}
		DB struct {
			User         string `conf:"default:schoolai"`
			Password     string `conf:"default:dev-password,mask"`
			Host         string `conf:"default:localhost:5435"`
			Name         string `conf:"default:projections"`
			MaxIdleConns int    `conf:"default:2"`
			MaxOpenConns int    `conf:"default:0"`
			DisableTLS   bool   `conf:"default:true"`
		}
		Redis struct {
			Addr string `conf:"default:localhost:6379"`
		}
		Orchestration struct {
			Framework           string        `conf:"default:custom"`
			Concurrency         int           `conf:"default:5"`
			ShutdownTimeout     time.Duration `conf:"default:10s"`
			HealthCheckInterval time.Duration `conf:"default:15s"`
		}
		AI struct {
			Provider string `conf:"default:openai"`
			Model    string `conf:"default:gpt-4o"`
		}
		Langfuse struct {
			Enabled   bool   `conf:"default:false"`
			BaseURL   string `conf:"default:http://localhost:3000"`
			PublicKey string
			SecretKey string
		}
		Observability struct {
			Release    string   // Release version override (defaults to build version)
			Env        string   `conf:"default:development"`
			Deployment string   // Optional deployment context (e.g., us-east-1)
			Tags       []string // Additional custom tags for traces
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
	// Initialize KurrentDB EventStore

	log.Info(ctx, "startup", "status", "initializing KurrentDB event store", "host", cfg.KurrentDB.ConnectionString)

	baseEventStore, err := kurrentdb.NewKurrentDBEventStore(cfg.KurrentDB.ConnectionString)
	if err != nil {
		return fmt.Errorf("failed to create KurrentDB event store: %w", err)
	}
	defer func() {
		closeErr := baseEventStore.Close()
		if closeErr != nil {
			log.Error(ctx, "shutdown", "status", "failed to close event store", "ERROR", closeErr)
		}
	}()

	// -------------------------------------------------------------------------
	// Database Support for Projections

	log.Info(ctx, "startup", "status", "initializing database support for projections", "host", cfg.DB.Host)

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
	// persistenceRouter is used by the ProjectionUpdater to update Postgres AND broadcast to Redis
	persistenceRouter := event.NewRouter()

	// transientRouter is used by the orchestrator for non-persistent events (streaming deltas, etc.)
	transientRouter := event.NewRouter()

	// Initialize Redis broadcaster for cross-pod event delivery
	redisBroadcaster := subscriber.NewRedisBroadcaster(log, cfg.Redis.Addr)
	defer func() {
		if closeErr := redisBroadcaster.Close(); closeErr != nil {
			log.Error(ctx, "shutdown", "status", "failed to close Redis broadcaster", "ERROR", closeErr)
		}
	}()

	// Register Redis broadcaster to BOTH routers
	// persistenceRouter will broadcast durable events once they hit ESDB
	// transientRouter will broadcast transient events immediately
	persistenceRouter.RegisterAny(redisBroadcaster.Broadcast)
	transientRouter.RegisterAny(redisBroadcaster.Broadcast)

	// Initialize PostgreSQL AGUI projection
	aguiProjection := agui.NewPostgresAguiProjection(log, db)
	aguiProjection.RegisterHandlers(persistenceRouter)

	// Initialize PostgreSQL LLM context projection
	llmContextProjection := llmcontext.NewPostgresLLMContextProjection(log, db)
	llmContextProjection.RegisterHandlers(persistenceRouter)

	// All projections are now query-time, so we use the base event store directly
	// No need for ProjectionEventStore wrapper anymore
	eventLog := baseEventStore

	// -------------------------------------------------------------------------
	// Start Projection Updater (subscribes to events and updates PostgreSQL)

	log.Info(ctx, "startup", "status", "starting projection updater")

	// Create projection updater with the persistence router
	projectionUpdater := subscriber.NewProjectionUpdater(
		log,
		baseEventStore.Client(),
		db,
		persistenceRouter,
	)

	// Initialize Statistics Projection (Postgres)
	statsProjection := projectionStats.NewPostgresProjection(db)
	statsProjection.RegisterHandlers(persistenceRouter)

	// Start projection updater in background
	go func() {
		if err := projectionUpdater.Start(ctx); err != nil {
			log.Error(ctx, "projection updater stopped", "ERROR", err)
		}
	}()

	// Get AI provider configuration
	// LLM client creation is now handled internally by each framework adapter
	log.Info(ctx, "startup", "status", "preparing AI configuration", "provider", cfg.AI.Provider, "model", cfg.AI.Model)

	var apiKey string
	switch cfg.AI.Provider {
	case "openai":
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("OPENAI_API_KEY environment variable is not set")
		}

	case "gemini":
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("GEMINI_API_KEY environment variable is not set")
		}

	default:
		return fmt.Errorf("unsupported AI provider: %s", cfg.AI.Provider)
	}

	// Create model configuration - framework adapters will create their own clients
	modelConfig := &agent.ModelConfig{
		Provider: cfg.AI.Provider,
		Model:    cfg.AI.Model,
		APIKey:   apiKey,
	}

	// Initialize Langfuse tracer (optional observability)
	var langfuseTracer *taskmanagement.LangfuseTracer
	if cfg.Langfuse.Enabled {
		log.Info(ctx, "startup", "status", "initializing Langfuse tracer", "baseURL", cfg.Langfuse.BaseURL)
		langfuseTracer = taskmanagement.NewLangfuseTracer(taskmanagement.LangfuseConfig{
			BaseURL:   cfg.Langfuse.BaseURL,
			PublicKey: cfg.Langfuse.PublicKey,
			SecretKey: cfg.Langfuse.SecretKey,
		})
	} else {
		log.Info(ctx, "startup", "status", "Langfuse tracing disabled")
	}

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
	toolRegistry := toolregistry.NewRegistry(log, toolregistry.NewInMemoryRepository())
	if err := toolRegistry.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize tool registry: %w", err)
	}
	log.Info(ctx, "startup", "status", "tool registry initialized")

	// Create and initialize permissions service
	permissionsService := permissions.NewService(log, permissions.NewInMemoryRepository())
	log.Info(ctx, "startup", "status", "permissions service initialized")

	// Create workspace permissions service
	workspacePermissionsService := workspacepermissions.NewService(log, workspacepermissions.NewInMemoryRepository())
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

	physicsConfig, err := physics.NewPhysicsInstructorConfig(ctx)
	if err != nil {
		return fmt.Errorf("creating physics instructor config: %w", err)
	}
	physicsConfig.AgentMiddlewares = []agent.Middleware{agentLoggingMw}
	physicsConfig.ChatMiddlewares = []agent.Middleware{chatLoggingMw}
	physicsConfig.FunctionMiddlewares = []agent.Middleware{toolLoggingMw}
	physicsInstructor, err := custom.NewAgent(physicsConfig)
	if err != nil {
		return fmt.Errorf("creating physics instructor agent: %w", err)
	}

	chemistryConfig, err := chemistry.NewChemistryInstructorConfig(ctx)
	if err != nil {
		return fmt.Errorf("creating chemistry instructor config: %w", err)
	}
	chemistryConfig.AgentMiddlewares = []agent.Middleware{agentLoggingMw}
	chemistryConfig.ChatMiddlewares = []agent.Middleware{chatLoggingMw}
	chemistryConfig.FunctionMiddlewares = []agent.Middleware{toolLoggingMw}
	chemistryInstructor, err := custom.NewAgent(chemistryConfig)
	if err != nil {
		return fmt.Errorf("creating chemistry instructor agent: %w", err)
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

	diagrammingConfig, err := diagramming.NewDiagrammingInstructorConfig(ctx)
	if err != nil {
		return fmt.Errorf("creating diagramming instructor config: %w", err)
	}
	diagrammingConfig.AgentMiddlewares = []agent.Middleware{agentLoggingMw}
	diagrammingConfig.ChatMiddlewares = []agent.Middleware{chatLoggingMw}
	diagrammingConfig.FunctionMiddlewares = []agent.Middleware{toolLoggingMw}
	diagrammingInstructor, err := custom.NewAgent(diagrammingConfig)
	if err != nil {
		return fmt.Errorf("creating diagramming instructor agent: %w", err)
	}

	// Create a simple map of available agents
	availableAgents := map[string]agent.Agent{
		"math-instructor":        mathInstructor,
		"physics-instructor":     physicsInstructor,
		"chemistry-instructor":   chemistryInstructor,
		"statistics-instructor":  statsInstructor,
		"diagramming-instructor": diagrammingInstructor,
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
			modelConfig,       // Model configuration (provider, model, API key)
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

	go func() {
		log.Info(ctx, "startup", "status", "debug server started", "host", cfg.Web.DebugHost)

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

	// Prepare observability configuration
	observabilityConfig := taskmanagement.ObservabilityConfig{
		Release:    cfg.Observability.Release,
		Env:        cfg.Observability.Env,
		Deployment: cfg.Observability.Deployment,
		Tags:       cfg.Observability.Tags,
	}
	if observabilityConfig.Release == "" {
		observabilityConfig.Release = build
	}

	// (agentFactory removed)

	taskManager := taskmanagement.NewAsynqTaskManager(
		log,
		taskmanagement.AsynqConfig{
			RedisAddr:           cfg.Redis.Addr,
			Concurrency:         cfg.Orchestration.Concurrency,
			ShutdownTimeout:     cfg.Orchestration.ShutdownTimeout,
			HealthCheckInterval: cfg.Orchestration.HealthCheckInterval,
		},
		agentOrchestrator, // Pass orchestrator instead of runner
		langfuseTracer,
		observabilityConfig,
	)
	defer taskManager.Shutdown()
	log.Info(ctx, "startup", "status", "task manager started",
		"redis", cfg.Redis.Addr,
		"concurrency", cfg.Orchestration.Concurrency,
		"shutdownTimeout", cfg.Orchestration.ShutdownTimeout,
		"healthCheckInterval", cfg.Orchestration.HealthCheckInterval)

	// Create HTTP handler
	handler := taskmgmthandlers.NewTaskManagementHandler(log, taskManager, agentProvider, authSvc, threadQueryService, aguiProjection, redisBroadcaster, threadMgmt, artifactService, memoryService, toolRegistry, permissionsService, edgequakeService, cfg.Edgequake.TenantID)

	// Create health handler
	healthHandler := healthhandlers.NewHealthHandler(log)

	// Create chi router
	r := chi.NewRouter()

	// Add panic recovery middleware first (most important - must be first)
	r.Use(middleware.Panics())

	// Add logging middleware
	r.Use(middleware.Logger(log))

	// Add metrics middleware
	r.Use(middleware.Metrics())

	// Add I18n middleware (detects lang from query/cookie/header)
	r.Use(middleware.I18n)

	// Add chi middleware (we can keep or remove these depending on preference)
	// Note: We now have our own Logger and Recoverer, so chi's can be removed
	// r.Use(chimiddleware.Logger)
	// r.Use(chimiddleware.Recoverer)

	// Serve static assets
	r.Handle("/assets/css/*", http.StripPrefix("/assets/css/", http.FileServer(http.Dir("./assets/css"))))
	r.Handle("/assets/js/*", http.StripPrefix("/assets/js/", http.FileServer(http.Dir("./assets/js"))))
	r.Handle("/assets/images/*", http.StripPrefix("/assets/images/", http.FileServer(http.Dir("./assets/images"))))
	r.Handle("/assets/charts/*", http.StripPrefix("/assets/charts/", http.FileServer(http.Dir("./assets/charts"))))

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

	// Analytics Dashboard (dedicated Basic Auth)
	statsHandler := statisticsHandler.NewHandler(log, statsProjection, cfg.DashboardAdmin.User, cfg.DashboardAdmin.Pass)
	r.Get("/analytics", statsHandler.BasicAuth(statsHandler.ServeDashboard))

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

	// Create server with proper timeouts to prevent slowloris and similar attacks
	server := &http.Server{
		Addr:              cfg.Web.APIHost,
		Handler:           r,
		ReadTimeout:       cfg.Web.ReadTimeout,
		ReadHeaderTimeout: cfg.Web.ReadHeaderTimeout,
		WriteTimeout:      cfg.Web.WriteTimeout,
		IdleTimeout:       cfg.Web.IdleTimeout,
	}

	// -------------------------------------------------------------------------
	// Start API Service

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	serverErrors := make(chan error, 1)

	go func() {
		log.Info(ctx, "startup", "status", "api router started", "host", server.Addr)

		serverErrors <- server.ListenAndServe()
	}()

	// -------------------------------------------------------------------------
	// Shutdown

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Info(ctx, "shutdown", "status", "shutdown started", "signal", sig)
		defer log.Info(ctx, "shutdown", "status", "shutdown complete", "signal", sig)

		ctx, cancel := context.WithTimeout(ctx, cfg.Web.ShutdownTimeout)
		defer cancel()

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

				// We use a background context as the request context might cancel
				_, err := edgequakeService.Users.Create(context.Background(), req)
				if err != nil {
					// Log error but don't fail the request (best effort sync)
					// In production, we might want to check if error is "already exists" and ignore
					log.Error(context.Background(), "failed to sync user to RAG", "error", err, "email", u.Email)
				} else {
					log.Info(context.Background(), "synced user to RAG", "email", u.Email, "role", u.Role)
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
