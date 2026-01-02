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
	"school-with-ai/services/agents/chemistry"
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

	// Initialize OpenAI provider with model
	model := string(openai.ChatModelGPT4oMini) // Use GPT-4o-mini for cost efficiency
	provider := llm.NewOpenAIProvider(model)

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

	// Initialize and start worker pool with agent registry
	workerPool := infra.NewWorkerPool(queue, eventLog, threadState, provider, agentRegistry, infra.WorkerPoolConfig{
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
	handler := infra.NewHandler(eventLog, threadState, provider, queue, workerPool)

	// Chat routes
	mux.HandleFunc("/send", handler.SendMessage)
	mux.HandleFunc("GET /api/jobs/active", handler.GetActiveJobs)
	mux.HandleFunc("GET /api/jobs/{jobId}/stream", handler.StreamJobEvents)
	mux.HandleFunc("DELETE /api/jobs/{jobId}", handler.CancelJob)
	mux.HandleFunc("GET /api/threads", handler.GetThreads)
	mux.HandleFunc("GET /api/threads/{threadId}/messages", handler.GetMessages)
	mux.HandleFunc("DELETE /api/threads/{threadId}", handler.DeleteThread)
	mux.HandleFunc("/", HandleViewChatInputScreen)
	err = http.ListenAndServe(addr, mux)
	if err != nil {
		panic(err)
	}
}

func HandleViewChatInputScreen(w http.ResponseWriter, r *http.Request) {
	component := components.ChatInputScreen()
	err := component.Render(r.Context(), w)
	if err != nil {
		slog.Error("Failed to render chat screen", "error", err)
		// Don't call http.Error after Render has already written headers
		// Just log the error - the partial response will be sent
	}
}
