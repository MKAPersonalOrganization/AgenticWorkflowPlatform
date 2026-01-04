package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"school-with-ai/components"
	"school-with-ai/infra"
	"school-with-ai/infra/auth"
	"school-with-ai/services/agents/advancedmath/linearalgebra"
	"school-with-ai/services/agents/chemistry"
	"school-with-ai/services/agents/diagramming"
	"school-with-ai/services/agents/math"
	"school-with-ai/services/agents/physics"
	"school-with-ai/services/agents/statistics"
	"school-with-ai/services/agents/studentfacing"
	"school-with-ai/services/llm"

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

	// Initialize EventLog and ThreadState (event sourcing architecture)
	eventLog := infra.NewInMemoryEventLog()
	threadState := infra.NewInMemoryThreadState()

	// Initialize OpenAI chat client with model
	model := string(openai.ChatModelGPT4oMini) // Use GPT-4o-mini for cost efficiency
	chatClient := llm.NewOpenAIClient(model)

	// Initialize job queue
	queue := infra.NewInMemoryJobQueue()

	// Initialize agent registry (notifier will be set per-job dynamically)
	agentRegistry := infra.NewAgentRegistry(nil)

	// Register all agents
	agentRegistry.Register("student-facing-agent", studentfacing.CreateStudentFacingAgent)
	agentRegistry.Register("math-instructor", math.CreateMathInstructor)
	agentRegistry.Register("physics-instructor", physics.CreatePhysicsInstructor)
	agentRegistry.Register("chemistry-instructor", chemistry.CreateChemistryInstructor)
	agentRegistry.Register("statistics-instructor", statistics.CreateStatisticsInstructor)
	agentRegistry.Register("linearalgebra-professor", linearalgebra.CreateLinearAlgebraProfessor)
	agentRegistry.Register("diagramming-instructor", diagramming.CreateDiagrammingInstructor)

	// Initialize and start worker pool with agent registry
	workerPool := infra.NewWorkerPool(queue, eventLog, threadState, chatClient, agentRegistry, infra.WorkerPoolConfig{
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

	// Create handler with EventLog and ThreadState
	handler := infra.NewHandler(eventLog, threadState, chatClient, queue, workerPool)

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
