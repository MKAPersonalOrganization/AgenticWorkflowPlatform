package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"school-with-ai/components"
	"school-with-ai/infra/auth"
	"school-with-ai/infra/chatapi"
	"school-with-ai/infra/eventstore"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/infra/projection/llmcontext"
	"school-with-ai/infra/projection/whiteboard"
	"school-with-ai/sdk/agent-sdk/llm"
	"school-with-ai/services/agents/advancedmath/linearalgebra"
	"school-with-ai/services/agents/chemistry"
	"school-with-ai/services/agents/diagramming"
	"school-with-ai/services/agents/math"
	"school-with-ai/services/agents/physics"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/agents/statistics"
	"school-with-ai/services/agents/studentfacing"
	"school-with-ai/services/chatwrite"

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

	slog.Info("Starting server", "host", host, "port", port)

	// Initialize EventStore and LLMContextProjection (event sourcing architecture)
	baseEventStore := eventstore.NewInMemoryEventStore()
	llmContextProjection := llmcontext.NewInMemoryLLMContextProjection()

	// Initialize AGUI projection (reads from event store)
	aguiProjection := agui.NewInMemoryAguiProjection()

	// Initialize Whiteboard projection (reads from event store)
	whiteboardProjection := whiteboard.NewInMemoryWhiteboardProjection()

	// Wrap event store to automatically update projections
	eventLog := eventstore.NewProjectionEventStore(baseEventStore, llmContextProjection, aguiProjection, whiteboardProjection)

	// Initialize OpenAI chat client with model
	model := string(openai.ChatModelGPT4oMini) // Use GPT-4o-mini for cost efficiency
	chatClient := llm.NewOpenAIClient(model)

	// Initialize job queue
	queue := chatapi.NewInMemoryJobQueue()

	// Register all agent contracts with the EventBus for routing
	eventbus.GlobalContracts.Register(studentfacing.Contract)
	eventbus.GlobalContracts.Register(math.Contract)
	eventbus.GlobalContracts.Register(physics.Contract)
	eventbus.GlobalContracts.Register(chemistry.Contract)
	eventbus.GlobalContracts.Register(statistics.Contract)
	eventbus.GlobalContracts.Register(linearalgebra.Contract)
	eventbus.GlobalContracts.Register(diagramming.Contract)

	// Initialize chat write service
	chatWriteService := chatwrite.New(chatClient, eventLog, llmContextProjection)

	// Initialize and start worker pool
	workerPool := chatapi.NewWorkerPool(queue, eventLog, llmContextProjection, aguiProjection, whiteboardProjection, chatClient, chatWriteService, chatapi.WorkerPoolConfig{
		NumWorkers:   5,
		PollInterval: 100 * time.Millisecond,
	})
	workerPool.Start()
	defer workerPool.Stop()

	mux := http.NewServeMux()

	// Serve static assets
	mux.Handle("/assets/css/", http.StripPrefix("/assets/css/", http.FileServer(http.Dir("./assets/css"))))
	mux.Handle("/assets/js/", http.StripPrefix("/assets/js/", http.FileServer(http.Dir("./assets/js"))))
	mux.Handle("/assets/images/", http.StripPrefix("/assets/images/", http.FileServer(http.Dir("./assets/images"))))
	mux.Handle("/assets/charts/", http.StripPrefix("/assets/charts/", http.FileServer(http.Dir("./assets/charts"))))

	// Create handler with EventStore and LLMContextProjection
	handler := chatapi.NewHandler(eventLog, llmContextProjection, chatClient, queue, workerPool, chatWriteService)

	// Create AGUI projection handler
	aguiHandler := agui.NewAguiProjectionHandler(aguiProjection, whiteboardProjection)

	// Initialize Kratos client and auth handlers
	kratosClient := auth.NewKratosClient()
	authHandlers := auth.NewAuthHandlers(kratosClient)
	authMiddleware := auth.NewMiddleware(kratosClient)

	// Auth routes (public)
	mux.HandleFunc("GET /auth/login", authHandlers.LoginPage)
	mux.HandleFunc("GET /auth/registration", authHandlers.RegistrationPage)
	mux.HandleFunc("GET /auth/recovery", authHandlers.RecoveryPage)
	mux.HandleFunc("GET /auth/verification", authHandlers.VerificationPage)
	mux.HandleFunc("GET /auth/logout", authHandlers.LogoutHandler)
	mux.HandleFunc("GET /auth/error", authHandlers.ErrorPage)

	// Protected auth routes
	mux.HandleFunc("GET /auth/settings", authMiddleware.RequireAuthFunc(authHandlers.SettingsPage))

	// Protected chat routes (require authentication)
	mux.HandleFunc("/send", authMiddleware.RequireAuthFunc(handler.SendMessage))
	mux.HandleFunc("GET /api/jobs/active", authMiddleware.RequireAuthFunc(handler.GetActiveJobs))
	mux.HandleFunc("GET /api/jobs/{jobId}/stream", authMiddleware.RequireAuthFunc(handler.StreamJobEvents))
	mux.HandleFunc("DELETE /api/jobs/{jobId}", authMiddleware.RequireAuthFunc(handler.CancelJob))
	mux.HandleFunc("GET /api/threads", authMiddleware.RequireAuthFunc(handler.GetThreads))
	mux.HandleFunc("GET /api/threads/{threadId}/messages", authMiddleware.RequireAuthFunc(handler.GetMessages))
	mux.HandleFunc("DELETE /api/threads/{threadId}", authMiddleware.RequireAuthFunc(handler.DeleteThread))

	// AGUI projection query endpoints (protected)
	mux.HandleFunc("GET /api/agui/threads", authMiddleware.RequireAuthFunc(aguiHandler.ListThreads))
	mux.HandleFunc("GET /api/agui/threads/{threadId}", authMiddleware.RequireAuthFunc(aguiHandler.GetThreadState))
	mux.HandleFunc("GET /api/agui/threads/{threadId}/messages", authMiddleware.RequireAuthFunc(aguiHandler.GetMessages))
	mux.HandleFunc("GET /api/agui/threads/{threadId}/tool-calls", authMiddleware.RequireAuthFunc(aguiHandler.GetToolCalls))
	mux.HandleFunc("GET /api/agui/threads/{threadId}/agent-activity", authMiddleware.RequireAuthFunc(aguiHandler.GetAgentActivity))
	mux.HandleFunc("GET /api/agui/threads/{threadId}/active-run", authMiddleware.RequireAuthFunc(aguiHandler.GetActiveRun))

	mux.HandleFunc("/", authMiddleware.RequireAuthFunc(HandleViewChatInputScreen))
	err = http.ListenAndServe(addr, mux)
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
