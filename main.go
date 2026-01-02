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
	"school-with-ai/services/agent"
	"school-with-ai/services/llm"
	"school-with-ai/services/tools"

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

	// Create tools using the Tool interface
	whiteboardTool := tools.NewWriteWhiteboardTool()
	readWhiteboardTool := tools.NewReadWhiteboardTool()
	calculatorTool := tools.NewCalculatorTool()
	weatherTool := tools.NewWeatherTool()
	graphingTool := tools.NewGraphingTool()
	unitConversionTool := tools.NewUnitConversionTool()
	periodicTableTool := tools.NewPeriodicTableTool()
	equationSolverTool := tools.NewEquationSolverTool()
	plotTool := tools.NewPlotTool()
	animateTool := tools.NewAnimateTool()

	// Event emission tools for agent communication
	emitMathQuestionTool := tools.NewEmitMathQuestionTool()
	emitPhysicsQuestionTool := tools.NewEmitPhysicsQuestionTool()
	emitChemistryQuestionTool := tools.NewEmitChemistryQuestionTool()
	emitQuestionAnsweredTool := tools.NewEmitQuestionAnsweredTool()

	// Initialize instructor registry with tools
	instructorRegistry := agent.NewInstructorRegistry(
		whiteboardTool,
		readWhiteboardTool,
		calculatorTool,
		weatherTool,
		graphingTool,
		unitConversionTool,
		periodicTableTool,
		equationSolverTool,
		plotTool,
		animateTool,
		emitMathQuestionTool,
		emitPhysicsQuestionTool,
		emitChemistryQuestionTool,
		emitQuestionAnsweredTool,
	)

	// Initialize and start worker pool with instructor registry
	workerPool := infra.NewWorkerPool(queue, eventLog, threadState, provider, instructorRegistry, infra.WorkerPoolConfig{
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
