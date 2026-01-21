package main

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"school-with-ai/components"
	"school-with-ai/components/checkout"
	"school-with-ai/components/pricing"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/llm"
	"school-with-ai/foundation/debug"
	"school-with-ai/foundation/event"
	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/middleware"
	"school-with-ai/handlers/auth"
	"school-with-ai/handlers/profile"
	"school-with-ai/infra/eventstore/kurrentdb"
	"school-with-ai/infra/kratos"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/infra/projection/llmcontext"
	"school-with-ai/infra/projection/subscriber"
	"school-with-ai/infra/projections"
	"school-with-ai/infra/sqldb"
	"school-with-ai/infra/sqldb/migrate"
	"school-with-ai/services/agentorchestrator"
	"school-with-ai/services/agentorchestrator/agents/chemistry"
	"school-with-ai/services/agentorchestrator/agents/diagramming"
	"school-with-ai/services/agentorchestrator/agents/math"
	"school-with-ai/services/agentorchestrator/agents/physics"
	"school-with-ai/services/agentorchestrator/agents/shared"
	"school-with-ai/services/agentorchestrator/agents/statistics"
	"school-with-ai/services/artifact"
	"school-with-ai/services/studentbooks"
	"school-with-ai/services/taskmanagement"
	"school-with-ai/services/threadmgmt"
	"syscall"
	"time"

	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"

	aguihandlers "school-with-ai/handlers/agui"
	bannerhandlers "school-with-ai/handlers/banner"
	healthhandlers "school-with-ai/handlers/health"

	taskmgmthandlers "school-with-ai/handlers/taskmgmt"

	authService "school-with-ai/services/auth"

	"github.com/ardanlabs/conf/v3"
	"github.com/go-chi/chi/v5"
)

var build = "develop"

func main() {
	var log *logger.Logger

	events := logger.Events{
		Error: func(ctx context.Context, r logger.Record) {
			log.Info(ctx, "******* SEND ALERT *******")
		},
	}

	log = logger.NewWithEvents(os.Stdout, logger.LevelError, "SCHOOL", nil, events)

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
			Concurrency         int           `conf:"default:5"`
			ShutdownTimeout     time.Duration `conf:"default:10s"`
			HealthCheckInterval time.Duration `conf:"default:15s"`
		}
		AI struct {
			Model string `conf:"default:gpt-4o-mini"`
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
	// Start Debug Service

	go func() {
		log.Info(ctx, "startup", "status", "debug server started", "host", cfg.Web.DebugHost)

		debugServer := &http.Server{
			Addr:              cfg.Web.DebugHost,
			Handler:           debug.Mux(),
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       120 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
		}

		if err := debugServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(ctx, "shutdown", "status", "debug server closed", "host", cfg.Web.DebugHost, "msg", err)
		}
	}()

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

	// Run migrations
	log.Info(ctx, "startup", "status", "running database migrations")
	if err := migrate.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrating database: %w", err)
	}

	// -------------------------------------------------------------------------
	// Initialize Projections

	// Initialize Event Router for decoupled dispatching
	eventRouter := event.NewRouter()

	// Initialize Redis broadcaster for cross-pod event delivery
	// This enables horizontal scaling by using Redis pub/sub instead of in-memory channels
	redisBroadcaster := subscriber.NewRedisBroadcaster(log, cfg.Redis.Addr)
	defer func() {
		if closeErr := redisBroadcaster.Close(); closeErr != nil {
			log.Error(ctx, "shutdown", "status", "failed to close Redis broadcaster", "ERROR", closeErr)
		}
	}()
	eventRouter.RegisterAny(redisBroadcaster.Broadcast)

	// Initialize PostgreSQL AGUI projection (materialized views for fast reads and writes)
	aguiProjection := agui.NewPostgresAguiProjection(log, db)
	aguiProjection.RegisterHandlers(eventRouter)

	// Initialize PostgreSQL LLM context projection (materialized views for fast reads)
	llmContextProjection := llmcontext.NewPostgresLLMContextProjection(log, db)
	llmContextProjection.RegisterHandlers(eventRouter)

	// All projections are now query-time, so we use the base event store directly
	// No need for ProjectionEventStore wrapper anymore
	eventLog := baseEventStore

	// -------------------------------------------------------------------------
	// Start Projection Updater (subscribes to events and updates PostgreSQL)

	log.Info(ctx, "startup", "status", "starting projection updater")

	// Create projection updater with the router
	projectionUpdater := subscriber.NewProjectionUpdater(
		log,
		baseEventStore.Client(), // baseEventStore is already *KurrentDBEventStore
		db,
		eventRouter,
	)

	// Start projection updater in background
	go func() {
		if err := projectionUpdater.Start(ctx); err != nil {
			log.Error(ctx, "projection updater stopped", "ERROR", err)
		}
	}()

	// Initialize OpenAI chat client with model
	log.Info(ctx, "startup", "status", "initializing AI client", "model", cfg.AI.Model)
	chatClient := llm.NewOpenAIClient(cfg.AI.Model)

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

	// Register all agent factories for routing

	shared.GlobalAgents.Register("math-instructor", math.CreateMathInstructor)
	shared.GlobalAgents.Register("physics-instructor", physics.CreatePhysicsInstructor)
	shared.GlobalAgents.Register("chemistry-instructor", chemistry.CreateChemistryInstructor)
	shared.GlobalAgents.Register("statistics-instructor", statistics.CreateStatisticsInstructor)
	shared.GlobalAgents.Register("diagramming-instructor", diagramming.CreateDiagrammingInstructor)

	// Initialize Student Books Service
	booksSvc := studentbooks.NewFileSystemService("books")
	sharedtools.InitStudentBooks(booksSvc)

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

	// Initialize orchestrator and thread services
	agentOrchestrator := agentorchestrator.New(log, chatClient, eventLog,
		agentorchestrator.WithArtifactService(artifactService),
		agentorchestrator.WithMemoryEnabled(cfg.AgentCapabilities.LongTermMemory),
		agentorchestrator.WithStudentProfileRepository(studentProfileRepo),
	)

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

	// Initialize agent factory
	agentFactory := agentorchestrator.NewFactory(log, chatClient, eventLog)

	taskManager := taskmanagement.NewAsynqTaskManager(
		log,
		taskmanagement.AsynqConfig{
			RedisAddr:           cfg.Redis.Addr,
			Concurrency:         cfg.Orchestration.Concurrency,
			ShutdownTimeout:     cfg.Orchestration.ShutdownTimeout,
			HealthCheckInterval: cfg.Orchestration.HealthCheckInterval,
		},
		eventLog,
		llmContextProjection,
		chatClient,
		agentOrchestrator,
		agentFactory,
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
	handler := taskmgmthandlers.NewTaskManagementHandler(log, chatClient, taskManager, agentOrchestrator, threadMgmt, authSvc, threadQueryService, aguiProjection, redisBroadcaster)

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

	// Pricing page
	r.Get("/pricing", authMiddleware.RequireAuthFunc(HandlePricingPage(log)))

	// Checkout page
	r.Get("/checkout", authMiddleware.RequireAuthFunc(HandleCheckoutPage(log)))

	r.Post("/v1/message:send", authMiddleware.RequireAuthFunc(handler.SendMessage))
	r.Get("/v1/tasks/{id}:subscribe", authMiddleware.RequireAuthFunc(handler.StreamJobEvents))
	r.Post("/v1/tasks/{id}:cancel", authMiddleware.RequireAuthFunc(handler.CancelJob))
	r.Get("/v1/agents", authMiddleware.RequireAuthFunc(handler.ListAgents))

	// AGUI projection query endpoints (protected)
	r.Get("/api/agui/threads", authMiddleware.RequireAuthFunc(aguiHandler.ListThreads))
	r.Get("/api/agui/threads/{threadId}", authMiddleware.RequireAuthFunc(aguiHandler.GetThreadState))
	r.Get("/api/agui/threads/{threadId}/messages", authMiddleware.RequireAuthFunc(aguiHandler.GetMessages))
	r.Delete("/api/agui/threads/{threadId}", authMiddleware.RequireAuthFunc(handler.DeleteThread))

	// Home route
	r.Get("/", authMiddleware.RequireAuthFunc(HandleViewChatInputScreen(log)))

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

func HandleViewChatInputScreen(log *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user from context
		user := domain.GetUserFromContext(r.Context())

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
		plans := pricing.GetDefaultPlans()

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
		plans := pricing.GetDefaultPlans()
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
