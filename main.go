package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"school-with-ai/components"
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
	"time"

	authService "school-with-ai/services/auth"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/openai/openai-go/v3"
)

func main() {
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		panic("SERVER_ADDR environment variable is not set")
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		panic(fmt.Sprintf("invalid SERVER_ADDR: %v", err))
	}

	slog.Info("Starting server with Asynq", "host", host, "port", port)

	// Initialize KurrentDB EventStore (production-ready, PostgreSQL-backed)
	kurrentDBConnectionString := os.Getenv("KURRENTDB_CONNECTION_STRING")
	if kurrentDBConnectionString == "" {
		kurrentDBConnectionString = "esdb://localhost:2113?tls=false"
		slog.Info("Using default KurrentDB connection string", "connectionString", kurrentDBConnectionString)
	}

	baseEventStore, err := kurrentdb.NewKurrentDBEventStore(kurrentDBConnectionString)
	if err != nil {
		panic(fmt.Sprintf("failed to create KurrentDB event store: %v", err))
	}
	defer func() {
		closeErr := baseEventStore.Close()
		if closeErr != nil {
			slog.Error("Failed to close event store", "error", closeErr)
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
	chatWriteService := chatwrite.New(chatClient, eventLog, llmContextProjection)

	// Initialize service layer
	authSvc := authService.NewAuthService()
	projectionService := projections.NewProjectionService(aguiProjection, whiteboardProjection, llmContextProjection)

	// Get Redis address from environment
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
		slog.Info("Using default Redis address", "addr", redisAddr)
	}

	// Initialize Asynq server
	asynqServer := orchestration.NewAsynqServer(
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
	slog.Info("Asynq server started",
		"redis", redisAddr,
		"concurrency", 5,
		"shutdownTimeout", "10s",
		"healthCheckInterval", "15s")

	// Create HTTP handler
	handler := handlers.NewAsynqHandler(chatClient, asynqServer, chatWriteService, authSvc, projectionService)

	// Create chi router
	r := chi.NewRouter()

	// Add chi middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Serve static assets
	r.Handle("/assets/css/*", http.StripPrefix("/assets/css/", http.FileServer(http.Dir("./assets/css"))))
	r.Handle("/assets/js/*", http.StripPrefix("/assets/js/", http.FileServer(http.Dir("./assets/js"))))
	r.Handle("/assets/images/*", http.StripPrefix("/assets/images/", http.FileServer(http.Dir("./assets/images"))))
	r.Handle("/assets/charts/*", http.StripPrefix("/assets/charts/", http.FileServer(http.Dir("./assets/charts"))))

	// Create AGUI projection handler
	aguiHandler := agui.NewAguiProjectionHandler(aguiProjection, whiteboardProjection)

	// Initialize Kratos client and auth handlers
	kratosClient := auth.NewKratosClient()
	authHandlers := auth.NewAuthHandlers(kratosClient)
	authMiddleware := auth.NewMiddleware(kratosClient)

	// Health check endpoint (public)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get Asynq metrics
		metrics, err := asynqServer.GetMetrics()
		if err != nil {
			slog.Error("Failed to get Asynq metrics", "error", err)
			metrics = nil
		}

		response := map[string]any{
			"status":   "healthy",
			"redis":    redisAddr,
			"mode":     "asynq",
			"asynqmon": "http://localhost:8080",
		}

		if metrics != nil {
			response["metrics"] = metrics
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("Failed to encode health response", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	})

	// Auth routes (public)
	r.Get("/auth/login", authHandlers.LoginPage)
	r.Get("/auth/registration", authHandlers.RegistrationPage)
	r.Get("/auth/recovery", authHandlers.RecoveryPage)
	r.Get("/auth/verification", authHandlers.VerificationPage)
	r.Get("/auth/logout", authHandlers.LogoutHandler)
	r.Get("/auth/error", authHandlers.ErrorPage)

	// Protected auth routes
	r.Get("/auth/settings", authMiddleware.RequireAuthFunc(authHandlers.SettingsPage))

	// A2A REST API routes (spec-compliant)
	r.Post("/v1/message:send", authMiddleware.RequireAuthFunc(handler.SendMessage))
	r.Get("/v1/tasks/{id}", authMiddleware.RequireAuthFunc(handler.GetJobStatus))
	r.Get("/v1/tasks/{id}:subscribe", authMiddleware.RequireAuthFunc(handler.StreamJobEvents))
	r.Post("/v1/tasks/{id}:cancel", authMiddleware.RequireAuthFunc(handler.CancelJob))

	// AGUI projection query endpoints (protected)
	r.Get("/api/agui/threads", authMiddleware.RequireAuthFunc(aguiHandler.ListThreads))
	r.Get("/api/agui/threads/{threadId}", authMiddleware.RequireAuthFunc(aguiHandler.GetThreadState))
	r.Get("/api/agui/threads/{threadId}/messages", authMiddleware.RequireAuthFunc(aguiHandler.GetMessages))
	r.Get("/api/agui/threads/{threadId}/tool-calls", authMiddleware.RequireAuthFunc(aguiHandler.GetToolCalls))
	r.Get("/api/agui/threads/{threadId}/agent-activity", authMiddleware.RequireAuthFunc(aguiHandler.GetAgentActivity))
	r.Get("/api/agui/threads/{threadId}/active-run", authMiddleware.RequireAuthFunc(aguiHandler.GetActiveRun))
	r.Delete("/api/agui/threads/{threadId}", authMiddleware.RequireAuthFunc(handler.DeleteThread))

	// Home route
	r.Get("/", authMiddleware.RequireAuthFunc(HandleViewChatInputScreen))

	err = http.ListenAndServe(addr, r)
	if err != nil {
		panic(err)
	}
}

func HandleViewChatInputScreen(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := auth.GetUserFromContext(r.Context())

	component := components.ChatInputScreen(user)
	err := component.Render(r.Context(), w)
	if err != nil {
		slog.Error("Failed to render chat screen", "error", err)
		// Don't call http.Error after Render has already written headers
		// Just log the error - the partial response will be sent
	}
}
