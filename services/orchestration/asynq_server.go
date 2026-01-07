package orchestration

import (
	"context"
	"fmt"
	"log/slog"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite"

	"github.com/hibiken/asynq"
)

// AsynqServer wraps Asynq server and client for chat task processing
type AsynqServer struct {
	server             *asynq.Server
	client             *asynq.Client
	eventBufferManager *EventBufferManager
	queue              domain.JobQueue
}

// AsynqConfig contains configuration for the Asynq server
type AsynqConfig struct {
	RedisAddr    string
	Concurrency  int
	RedisOptions asynq.RedisClientOpt
}

// NewAsynqServer creates a new Asynq-based task processing server
func NewAsynqServer(
	config AsynqConfig,
	log core.EventStore,
	state chatwrite.ThreadStore,
	whiteboardProjection interface{ Get(threadID string) string },
	chatClient chatwrite.ChatClient,
	chatWriteService *chatwrite.ChatWriteService,
	queue domain.JobQueue,
) *AsynqServer {
	// Default Redis options if not provided
	if config.RedisOptions.Addr == "" && config.RedisAddr == "" {
		config.RedisAddr = "localhost:6379"
	}

	redisOpt := config.RedisOptions
	if redisOpt.Addr == "" {
		redisOpt = asynq.RedisClientOpt{Addr: config.RedisAddr}
	}

	// Default concurrency
	if config.Concurrency <= 0 {
		config.Concurrency = 5
	}

	// Create event buffer manager
	eventBufferManager := NewEventBufferManager()

	// Create chat worker
	chatWorker := NewChatWorker(
		log,
		state,
		whiteboardProjection,
		chatClient,
		chatWriteService,
		queue,
		eventBufferManager,
	)

	// Create Asynq server with dynamic queue configuration
	// We use a special queue name pattern for thread isolation
	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: config.Concurrency,
			// Process all queues with equal priority
			// Thread-specific queues will be created dynamically
			Queues: map[string]int{
				"default": 1,
			},
			// Log level
			LogLevel: asynq.InfoLevel,
			// Enable strict priority to ensure thread isolation
			StrictPriority: false,
			// Error handler
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				slog.Error("Task processing failed",
					"type", task.Type(),
					"payload", string(task.Payload()),
					"error", err)
			}),
		},
	)

	// Create Asynq client for enqueueing tasks
	client := asynq.NewClient(redisOpt)

	// Register task handlers
	mux := asynq.NewServeMux()
	mux.Handle(TypeProcessChat, chatWorker)

	// Start server in background
	go func() {
		if err := srv.Run(mux); err != nil {
			slog.Error("Asynq server error", "error", err)
		}
	}()

	return &AsynqServer{
		server:             srv,
		client:             client,
		eventBufferManager: eventBufferManager,
		queue:              queue,
	}
}

// EnqueueJob enqueues a chat processing job
func (s *AsynqServer) EnqueueJob(job *domain.Job) error {
	task, err := NewProcessChatTask(job)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	info, err := s.client.Enqueue(task)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	slog.Info("Enqueued chat task", "jobID", job.ID, "taskID", info.ID, "queue", info.Queue)
	return nil
}

// GetEventBuffer gets the event buffer for a job (for SSE streaming)
func (s *AsynqServer) GetEventBuffer(jobID string) (*EventBuffer, bool) {
	return s.eventBufferManager.GetEventBuffer(jobID)
}

// Shutdown gracefully shuts down the Asynq server
func (s *AsynqServer) Shutdown() {
	slog.Info("Shutting down Asynq server")
	s.server.Shutdown()
	s.client.Close()
	slog.Info("Asynq server shut down")
}
