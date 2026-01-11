package main

import (
	"context"
	"expvar"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"school-with-ai/components"
	"school-with-ai/foundation/debug"
	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/middleware"
	"school-with-ai/handlers"
	"school-with-ai/infra/auth"
	"school-with-ai/infra/eventstore/kurrentdb"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/infra/projection/querytime"
	"school-with-ai/infra/projection/whiteboard"
	"school-with-ai/infra/projections"
	"school-with-ai/sdk/agentsdk/llm"
	"school-with-ai/services/chatwrite"
	"school-with-ai/services/chatwrite/agents/advancedmath/linearalgebra"
	"school-with-ai/services/chatwrite/agents/chemistry"
	"school-with-ai/services/chatwrite/agents/diagramming"
	"school-with-ai/services/chatwrite/agents/math"
	"school-with-ai/services/chatwrite/agents/physics"
	"school-with-ai/services/chatwrite/agents/shared"
	"school-with-ai/services/chatwrite/agents/statistics"
	"school-with-ai/services/chatwrite/agents/studentfacing"
	"school-with-ai/services/orchestration"
	"syscall"
	"time"

	authService "school-with-ai/services/auth"

	"github.com/go-chi/chi/v5"
	"github.com/openai/openai-go/v3"
)

var build = "develop"

func main() {
	var log *logger.Logger

	events := logger.Events{
		Error: func(ctx context.Context, r logger.Record) {
			log.Info(ctx, "******* SEND ALERT *******")
		},
	}

	log = logger.NewWithEvents(os.Stdout, logger.LevelInfo, "SCHOOL", nil, events)

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

	// Get server address
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		return fmt.Errorf("SERVER_ADDR environment variable is not set")
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid SERVER_ADDR: %v", err)
	}

	log.Info(ctx, "startup", "status", "starting server", "host", host, "port", port)
	defer log.Info(ctx, "shutdown complete")

	// Set build info in expvar for metrics
	expvar.NewString("build").Set(build)

	// Get debug server address from environment
	debugAddr := os.Getenv("DEBUG_ADDR")
	if debugAddr == "" {
		debugAddr = "0.0.0.0:4010"
		log.Info(ctx, "startup", "status", "using default debug address", "addr", debugAddr)
	}

	// -------------------------------------------------------------------------
	// Start Debug Service

	go func() {
		log.Info(ctx, "startup", "status", "debug server started", "host", debugAddr)

		debugServer := &http.Server{
			Addr:              debugAddr,
			Handler:           debug.Mux(),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       120 * time.Second,
		}

		if err := debugServer.ListenAndServe(); err != nil {
			log.Error(ctx, "shutdown", "status", "debug server closed", "host", debugAddr, "err", err)
		}
	}()

	// -------------------------------------------------------------------------

	// Initialize KurrentDB EventStore (production-ready, PostgreSQL-backed)
	kurrentDBConnectionString := os.Getenv("KURRENTDB_CONNECTION_STRING")
	if kurrentDBConnectionString == "" {
		kurrentDBConnectionString = "esdb://localhost:2113?tls=false"
		log.Info(ctx, "startup", "status", "using default KurrentDB connection", "connectionString", kurrentDBConnectionString)
	}

	baseEventStore, err := kurrentdb.NewKurrentDBEventStore(kurrentDBConnectionString)
	if err != nil {
		return fmt.Errorf("failed to create KurrentDB event store: %v", err)
	}
	defer func() {
		closeErr := baseEventStore.Close()
		if closeErr != nil {
			log.Error(ctx, "shutdown", "status", "failed to close event store", "ERROR", closeErr)
		}
	}()

	// Initialize query-time AGUI projection (reads directly from event store on each query)
	aguiProjection := agui.NewQueryTimeAguiProjection(baseEventStore)

	// Initialize query-time Whiteboard projection (reads directly from event store on each query)
	whiteboardProjection := whiteboard.NewQueryTimeWhiteboardProjection(baseEventStore, baseEventStore)

	// All projections are now query-time, so we use the base event store directly
	// No need for ProjectionEventStore wrapper anymore
	eventLog := baseEventStore

	// Initialize query-time LLM context projection (reads directly from event store)
	// Uses AGUI projection to get list of all threads for the All() method
	llmContextProjection := querytime.NewQueryTimeLLMContextProjection(baseEventStore, aguiProjection)

	// Initialize OpenAI chat client with model
	model := string(openai.ChatModelGPT4oMini) // Use GPT-4o-mini for cost efficiency
	chatClient := llm.NewOpenAIClient(model)

	// Register all agent factories for routing
	shared.GlobalAgents.Register("student-facing-agent", studentfacing.CreateStudentFacingAgent)
	shared.GlobalAgents.Register("math-instructor", math.CreateMathInstructor)
	shared.GlobalAgents.Register("physics-instructor", physics.CreatePhysicsInstructor)
	shared.GlobalAgents.Register("chemistry-instructor", chemistry.CreateChemistryInstructor)
	shared.GlobalAgents.Register("statistics-instructor", statistics.CreateStatisticsInstructor)
	shared.GlobalAgents.Register("linear-algebra-instructor", linearalgebra.CreateLinearAlgebraProfessor)
	shared.GlobalAgents.Register("diagramming-instructor", diagramming.CreateDiagrammingInstructor)

	// Initialize chat write service
	chatWriteService := chatwrite.New(log, chatClient, eventLog, llmContextProjection)

	// Initialize service layer
	authSvc := authService.NewAuthService()
	projectionService := projections.NewProjectionService(aguiProjection, whiteboardProjection, llmContextProjection)

	// Get Redis address from environment
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
		log.Info(ctx, "startup", "status", "using default Redis address", "addr", redisAddr)
	}

	// Initialize Asynq server
	asynqServer := orchestration.NewAsynqServer(
		log,
		orchestration.AsynqConfig{
			RedisAddr:           redisAddr,
			Concurrency:         5,
			ShutdownTimeout:     10 * time.Second, // Wait 10s for workers to finish
			HealthCheckInterval: 15 * time.Second, // Check Redis every 15s
		},
		eventLog,
		llmContextProjection,
		whiteboardProjection,
		chatClient,
		chatWriteService,
	)
	defer asynqServer.Shutdown()
	log.Info(ctx, "startup", "status", "asynq server started",
		"redis", redisAddr,
		"concurrency", 5,
		"shutdownTimeout", "10s",
		"healthCheckInterval", "15s")

	// Create HTTP handler
	handler := handlers.NewAsynqHandler(log, chatClient, asynqServer, chatWriteService, authSvc, projectionService)

	// Create health handler
	healthHandler := handlers.NewHealthHandler(log)

	// Create chi router
	r := chi.NewRouter()

	// Add panic recovery middleware first (most important - must be first)
	r.Use(middleware.Panics())

	// Add error handling middleware (should be early in the chain)
	r.Use(middleware.Errors(log))

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
	aguiHandler := agui.NewAguiProjectionHandler(aguiProjection, whiteboardProjection, projectionService)

	// Initialize Kratos client and auth handlers
	kratosClient := auth.NewKratosClient()
	authHandlers := auth.NewAuthHandlers(log, kratosClient)
	authMiddleware := auth.NewMiddleware(kratosClient)

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

	r.Post("/v1/message:send", authMiddleware.RequireAuthFunc(handler.SendMessage))
	r.Get("/v1/tasks/{id}:subscribe", authMiddleware.RequireAuthFunc(handler.StreamJobEvents))
	r.Post("/v1/tasks/{id}:cancel", authMiddleware.RequireAuthFunc(handler.CancelJob))

	// AGUI projection query endpoints (protected)
	r.Get("/api/agui/threads", authMiddleware.RequireAuthFunc(aguiHandler.ListThreads))
	r.Get("/api/agui/threads/{threadId}", authMiddleware.RequireAuthFunc(aguiHandler.GetThreadState))
	r.Get("/api/agui/threads/{threadId}/messages", authMiddleware.RequireAuthFunc(aguiHandler.GetMessages))
	r.Get("/api/agui/threads/{threadId}/active-run", authMiddleware.RequireAuthFunc(aguiHandler.GetActiveRun))
	r.Delete("/api/agui/threads/{threadId}", authMiddleware.RequireAuthFunc(handler.DeleteThread))

	// Home route
	r.Get("/", authMiddleware.RequireAuthFunc(HandleViewChatInputScreen(log)))

	// Create server with proper timeouts to prevent slowloris and similar attacks
	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
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

		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			if closeErr := server.Close(); closeErr != nil {
				log.Error(ctx, "failed to close server", "error", closeErr)
			}
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil
}

func HandleViewChatInputScreen(log *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user from context
		user := auth.GetUserFromContext(r.Context())

		component := components.ChatInputScreen(user)
		if err := component.Render(r.Context(), w); err != nil {
			log.Error(r.Context(), "failed to render chat input screen", "error", err)
		}
	}
}
