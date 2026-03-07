package taskmanagement

import (
	"context"
	"errors"
	"fmt"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"time"

	"github.com/hibiken/asynq"
)

var (
	// ErrRedisHealthCheckFailed is returned when Redis health check fails.
	ErrRedisHealthCheckFailed = errors.New("redis health check failed")
	// ErrTaskNotFound is returned when trying to cancel a task that is not found or already completed.
	ErrTaskNotFound = errors.New("failed to cancel task: task not found or already completed")
)

// AsynqTaskManager wraps Asynq server and client for chat task processing.
type AsynqTaskManager struct {
	log           *logger.Logger
	server        *asynq.Server
	client        *asynq.Client
	inspector     *asynq.Inspector
	redisAddr     string // Store for metrics
	redisPassword string // Store for Inspector calls
	concurrency   int    // Store for metrics
}

// AsynqConfig contains configuration for the Asynq server.
type AsynqConfig struct {
	RedisAddr           string
	RedisPassword       string
	Concurrency         int
	RedisOptions        asynq.RedisClientOpt
	ShutdownTimeout     time.Duration // Time to wait for workers to finish
	HealthCheckInterval time.Duration // Interval for Redis health checks
	RetryDelayFunc      asynq.RetryDelayFunc
}

// NewAsynqTaskManager creates a new Asynq-based task processing server.
func NewAsynqTaskManager(
	log *logger.Logger,
	config AsynqConfig,
	orchestrator Orchestrator, // Orchestrator that emits run lifecycle events
) *AsynqTaskManager {
	// Default Redis options if not provided
	if config.RedisOptions.Addr == "" && config.RedisAddr == "" {
		config.RedisAddr = "localhost:6379"
	}

	redisOpt := config.RedisOptions
	if redisOpt.Addr == "" {
		redisOpt = asynq.RedisClientOpt{Addr: config.RedisAddr, Password: config.RedisPassword}
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

	// Create orchestration service
	orchestrationService := NewService(
		log,
		orchestrator,
	)

	// Create chat worker (thin wrapper around orchestration service)
	chatWorker := NewChatWorker(
		log,
		orchestrationService,
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
					log.Error(context.Background(), "[ASYNQ] Redis health check failed", "error", err)
				} else {
					log.Debug(context.Background(), "[ASYNQ] Redis health check passed")
				}
			},
			HealthCheckInterval: config.HealthCheckInterval,
			// Error handler
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				retried, _ := asynq.GetRetryCount(ctx)
				maxRetry, _ := asynq.GetMaxRetry(ctx)
				log.Error(ctx, "[ASYNQ] Task processing failed",
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
			log.Error(context.Background(), "Asynq server error", "error", err)
		}
	}()

	return &AsynqTaskManager{
		log:           log,
		server:        srv,
		client:        client,
		inspector:     asynq.NewInspector(redisOpt),
		redisAddr:     config.RedisAddr,
		redisPassword: config.RedisPassword,
		concurrency:   config.Concurrency,
	}
}

// EnqueueJob enqueues a chat processing job and returns task info.
func (s *AsynqTaskManager) EnqueueJob(payload ProcessChatPayload) (*asynq.TaskInfo, error) {
	task, err := NewProcessChatTask(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	info, err := s.client.Enqueue(task, asynq.TaskID(payload.JobID))
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	s.log.Info(context.Background(), "Enqueued chat task", "jobID", payload.JobID, "taskID", info.ID, "queue", info.Queue)

	return info, nil
}

// SubmitChatTaskResult contains the result of submitting a chat task.
type SubmitChatTaskResult struct {
	JobID     string          `json:"jobId"`
	ContextID string          `json:"contextId"`
	TaskInfo  *asynq.TaskInfo `json:"-"`
}

// SubmitChatTask submits a chat task for async processing.
// It handles validation, ID generation, and payload construction.
func (s *AsynqTaskManager) SubmitChatTask(ctx context.Context, userID, message, contextID, agentName, toolCallID string, attachments []FileAttachment) (*SubmitChatTaskResult, error) {
	if message == "" && len(attachments) == 0 {
		return nil, errors.New("message is required")
	}

	// Generate context ID (thread ID) if not provided
	if contextID == "" {
		contextID = fmt.Sprintf("thread_%s_%d", userID, time.Now().UnixNano())
	}

	// Generate IDs for async task
	jobID := agent.GenerateID()
	messageID := agent.GenerateID()

	// Create payload for async processing
	payload := ProcessChatPayload{
		JobID:       jobID,
		ThreadID:    contextID,
		MessageID:   messageID,
		Message:     message,
		UserID:      userID,
		AgentName:   agentName,
		ToolCallID:  toolCallID,
		Attachments: attachments,
	}

	s.log.Info(ctx, "Submitting chat task",
		"userID", userID,
		"contextID", contextID,
		"taskID", jobID,
		"messageLength", len(message))

	// Enqueue to Asynq for async processing
	taskInfo, err := s.EnqueueJob(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	s.log.Info(ctx, "Task submitted successfully",
		"jobID", jobID,
		"taskID", taskInfo.ID,
		"queue", taskInfo.Queue)

	return &SubmitChatTaskResult{
		JobID:     jobID,
		ContextID: contextID,
		TaskInfo:  taskInfo,
	}, nil
}

// GetTaskInfo retrieves task information from Asynq.
func (s *AsynqTaskManager) GetTaskInfo(queue, taskID string) (*asynq.TaskInfo, error) {
	// Use Inspector's GetTaskInfo which efficiently looks up the task
	taskInfo, err := s.inspector.GetTaskInfo(queue, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task info for queue=%s taskID=%s: %w", queue, taskID, err)
	}

	return taskInfo, nil
}

// CancelTask attempts to cancel a task by job ID.
func (s *AsynqTaskManager) CancelTask(jobID string) error {
	// Try to delete the task from the default queue first (for pending/scheduled/retry tasks)
	err := s.inspector.DeleteTask("default", jobID)
	if err == nil {
		s.log.Info(context.Background(), "Deleted pending task", "jobID", jobID)

		return nil
	}

	// If delete failed, the task might be actively processing
	// Try to send cancellation signal instead
	if err := s.inspector.CancelProcessing(jobID); err != nil {
		s.log.Warn(context.Background(), "Failed to send cancellation signal",
			"jobID", jobID,
			"error", err,
			"note", "Task may have already completed or doesn't exist")

		return ErrTaskNotFound
	}

	s.log.Info(context.Background(), "Sent cancellation signal to active task", "jobID", jobID)

	return nil
}

// Shutdown gracefully shuts down the Asynq server.
func (s *AsynqTaskManager) Shutdown() {
	s.log.Info(context.Background(), "Shutting down Asynq server")
	s.server.Shutdown()
	err := s.client.Close()
	if err != nil {
		s.log.Error(context.Background(), "Failed to close Asynq client", "error", err)
	}
	s.log.Info(context.Background(), "Asynq server shut down")
}
