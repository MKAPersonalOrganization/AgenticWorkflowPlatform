package orchestration

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite"
	"time"

	"github.com/hibiken/asynq"
)

var (
	// ErrRedisHealthCheckFailed is returned when Redis health check fails.
	ErrRedisHealthCheckFailed = errors.New("redis health check failed")
	// ErrTaskNotFound is returned when trying to cancel a task that is not found or already completed.
	ErrTaskNotFound = errors.New("failed to cancel task: task not found or already completed")
)

// AsynqServer wraps Asynq server and client for chat task processing.
type AsynqServer struct {
	server             *asynq.Server
	client             *asynq.Client
	eventBufferManager *EventBufferManager
	redisAddr          string // Store for metrics
}

// AsynqConfig contains configuration for the Asynq server.
type AsynqConfig struct {
	RedisAddr           string
	Concurrency         int
	RedisOptions        asynq.RedisClientOpt
	ShutdownTimeout     time.Duration // Time to wait for workers to finish
	HealthCheckInterval time.Duration // Interval for Redis health checks
	RetryDelayFunc      asynq.RetryDelayFunc
}

// NewAsynqServer creates a new Asynq-based task processing server.
func NewAsynqServer(
	config AsynqConfig,
	log core.EventStore,
	state chatwrite.ThreadStore,
	whiteboardProjection domain.WhiteboardStore,
	chatClient chatwrite.ChatClient,
	chatWriteService *chatwrite.ChatWriteService,
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

	// Default shutdown timeout
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = 8 * time.Second
	}

	// Default health check interval
	if config.HealthCheckInterval <= 0 {
		config.HealthCheckInterval = 15 * time.Second
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
			// Shutdown timeout
			ShutdownTimeout: config.ShutdownTimeout,
			// Health check
			HealthCheckFunc: func(err error) {
				if err != nil {
					slog.Error("[ASYNQ] Redis health check failed", "error", err)
				} else {
					slog.Debug("[ASYNQ] Redis health check passed")
				}
			},
			HealthCheckInterval: config.HealthCheckInterval,
			// Error handler
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				retried, _ := asynq.GetRetryCount(ctx)
				maxRetry, _ := asynq.GetMaxRetry(ctx)
				slog.Error("[ASYNQ] Task processing failed",
					"type", task.Type(),
					"error", err,
					"retried", retried,
					"maxRetry", maxRetry,
					"exhausted", retried >= maxRetry)
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
		err := srv.Run(mux)
		if err != nil {
			slog.Error("Asynq server error", "error", err)
		}
	}()

	return &AsynqServer{
		server:             srv,
		client:             client,
		eventBufferManager: eventBufferManager,

		redisAddr: config.RedisAddr,
	}
}

// EnqueueJob enqueues a chat processing job and returns task info.
func (s *AsynqServer) EnqueueJob(payload ProcessChatPayload) (*asynq.TaskInfo, error) {
	task, err := NewProcessChatTask(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	info, err := s.client.Enqueue(task)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	slog.Info("Enqueued chat task", "jobID", payload.JobID, "taskID", info.ID, "queue", info.Queue)

	return info, nil
}

// GetEventBuffer gets the event buffer for a job (for SSE streaming).
func (s *AsynqServer) GetEventBuffer(jobID string) (*EventBuffer, bool) {
	return s.eventBufferManager.GetEventBuffer(jobID)
}

// GetTaskInfo retrieves task information from Asynq.
func (s *AsynqServer) GetTaskInfo(queue, taskID string) (*asynq.TaskInfo, error) {
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: s.redisAddr})
	defer func() {
		err := inspector.Close()
		if err != nil {
			slog.Warn("Failed to close inspector", "error", err)
		}
	}()

	// Use Inspector's GetTaskInfo which efficiently looks up the task
	taskInfo, err := inspector.GetTaskInfo(queue, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task info for queue=%s taskID=%s: %w", queue, taskID, err)
	}

	return taskInfo, nil
}

// CancelTask attempts to cancel a task by job ID.
func (s *AsynqServer) CancelTask(jobID string) error {
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: s.redisAddr})
	defer func() {
		err := inspector.Close()
		if err != nil {
			slog.Warn("Failed to close inspector", "error", err)
		}
	}()

	// Try to delete the task from the default queue first (for pending/scheduled/retry tasks)
	err := inspector.DeleteTask("default", jobID)
	if err == nil {
		slog.Info("Deleted pending task", "jobID", jobID)

		return nil
	}

	// If delete failed, the task might be actively processing
	// Try to send cancellation signal instead
	if err := inspector.CancelProcessing(jobID); err != nil {
		slog.Warn("Failed to send cancellation signal",
			"jobID", jobID,
			"error", err,
			"note", "Task may have already completed or doesn't exist")

		return ErrTaskNotFound
	}

	slog.Info("Sent cancellation signal to active task", "jobID", jobID)

	return nil
}

// Shutdown gracefully shuts down the Asynq server.
func (s *AsynqServer) Shutdown() {
	slog.Info("Shutting down Asynq server")
	s.server.Shutdown()
	err := s.client.Close()
	if err != nil {
		slog.Error("Failed to close Asynq client", "error", err)
	}
	slog.Info("Asynq server shut down")
}
