package orchestration

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

// ServerMetrics contains metrics about the Asynq server.
type ServerMetrics struct {
	// Queue metrics
	Pending   int `json:"pending"`
	Active    int `json:"active"`
	Scheduled int `json:"scheduled"`
	Retry     int `json:"retry"`
	Archived  int `json:"archived"`
	Completed int `json:"completed"`

	// Server metrics
	Workers     int       `json:"workers"`
	Concurrency int       `json:"concurrency"`
	Queues      []string  `json:"queues"`
	RedisAddr   string    `json:"redisAddr"`
	StartedAt   time.Time `json:"startedAt"`

	// Health
	RedisHealthy bool   `json:"redisHealthy"`
	Status       string `json:"status"`
}

// GetMetrics returns current server and queue metrics.
func (s *AsynqServer) GetMetrics() (*ServerMetrics, error) {
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr: s.redisAddr,
	})
	defer func() {
		err := inspector.Close()
		if err != nil {
			slog.Warn("Failed to close inspector", "error", err)
		}
	}()

	// Get queue stats
	stats, err := inspector.GetQueueInfo("default")
	if err != nil {
		return nil, fmt.Errorf("failed to get queue info: %w", err)
	}

	metrics := &ServerMetrics{
		Pending:      stats.Pending,
		Active:       stats.Active,
		Scheduled:    stats.Scheduled,
		Retry:        stats.Retry,
		Archived:     stats.Archived,
		Completed:    stats.Completed,
		Queues:       []string{"default"},
		RedisAddr:    s.redisAddr,
		RedisHealthy: true,
		Status:       "running",
		Concurrency:  5,
	}

	return metrics, nil
}
