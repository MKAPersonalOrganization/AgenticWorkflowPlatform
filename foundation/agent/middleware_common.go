package agent

import (
	"fmt"
	"log/slog"
	"time"
)

// =============================================================================
// Agent-Level Middleware
// =============================================================================

// Retry retries agent operations on transient failures.
func Retry(attempts int) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.Agent == nil {
				return next.Handle(c) // Pass through non-agent ops
			}

			var lastErr error
			for i := 0; i < attempts; i++ {
				if err := next.Handle(c); err == nil {
					return nil
				} else {
					lastErr = err
					// Could add exponential backoff here
				}
			}
			return fmt.Errorf("failed after %d attempts: %w", attempts, lastErr)
		})
	}
}

// Timeout adds a timeout to agent operations.
func Timeout(d time.Duration) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.Agent == nil {
				return next.Handle(c)
			}

			done := make(chan error, 1)
			go func() {
				done <- next.Handle(c)
			}()

			select {
			case err := <-done:
				return err
			case <-time.After(d):
				return fmt.Errorf("operation timed out after %v", d)
			case <-c.Done():
				return c.Err()
			}
		})
	}
}

// Recovery recovers from panics in the handler.
func Recovery() Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic recovered: %v", r)
				}
			}()
			return next.Handle(c)
		})
	}
}

// Logging logs operations using slog.
func Logging(logger *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			start := time.Now()

			// Log operation start
			switch c.Kind {
			case OperationAgent:
				agentName := "<unknown>"
				if c.Agent != nil && c.Agent.Agent != nil {
					agentName = c.Agent.Agent.Name()
				}
				msgCount := 0
				if c.Agent != nil {
					msgCount = len(c.Agent.Messages)
				}
				logger.Info("agent operation started",
					"agent", agentName,
					"message_count", msgCount)
			case OperationChat:
				msgCount := 0
				if c.Chat != nil {
					msgCount = len(c.Chat.Messages)
				}
				logger.Info("chat operation started",
					"message_count", msgCount)
			case OperationFunction:
				toolName := "<unknown>"
				if c.Function != nil && c.Function.Tool != nil {
					toolName = c.Function.Tool.Name()
				}
				logger.Info("function operation started",
					"tool", toolName)
			}

			err := next.Handle(c)

			// Log operation end
			duration := time.Since(start)
			if err != nil {
				logger.Error("operation failed",
					"duration", duration,
					"error", err)
			} else {
				logger.Info("operation completed",
					"duration", duration)
			}

			return err
		})
	}
}

// =============================================================================
// Chat-Level Middleware
// =============================================================================

// TokenCounter is the interface for counting tokens.
type TokenCounter interface {
	CountInput(messages []Message)
	CountOutput(result any)
}

// TokenCounting counts tokens for chat operations.
func TokenCounting(counter TokenCounter) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.Chat == nil {
				return next.Handle(c)
			}

			counter.CountInput(c.Chat.Messages)
			err := next.Handle(c)
			if err == nil {
				counter.CountOutput(c.Result)
			}
			return err
		})
	}
}

// RateLimiter is the interface for rate limiting.
type RateLimiter interface {
	// Allow returns true if the operation is allowed, false if rate limited.
	Allow() bool
	// Wait blocks until the operation is allowed.
	Wait() error
}

// RateLimiting rate-limits chat operations.
func RateLimiting(limiter RateLimiter) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.Chat == nil {
				return next.Handle(c)
			}

			if err := limiter.Wait(); err != nil {
				return fmt.Errorf("rate limit wait failed: %w", err)
			}

			return next.Handle(c)
		})
	}
}

// =============================================================================
// Function-Level Middleware
// =============================================================================

// Validator is the interface for validating function arguments.
type Validator interface {
	Validate(tool Tool, arguments map[string]any) error
}

// FunctionValidation validates function arguments before execution.
func FunctionValidation(validator Validator) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.Function == nil {
				return next.Handle(c)
			}

			if err := validator.Validate(c.Function.Tool, c.Function.Arguments); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			return next.Handle(c)
		})
	}
}

// Cache is the interface for caching function results.
type Cache interface {
	Get(key string) (any, bool)
	Set(key string, value any)
}

// FunctionCaching caches function results.
func FunctionCaching(cache Cache, keyFunc func(Tool, map[string]any) string) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.Function == nil {
				return next.Handle(c)
			}

			key := keyFunc(c.Function.Tool, c.Function.Arguments)
			if cached, ok := cache.Get(key); ok {
				c.Result = cached
				return nil
			}

			if err := next.Handle(c); err != nil {
				return err
			}

			cache.Set(key, c.Result)
			return nil
		})
	}
}

// FunctionTimeout adds a timeout to function operations.
func FunctionTimeout(d time.Duration) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.Function == nil {
				return next.Handle(c)
			}

			done := make(chan error, 1)
			go func() {
				done <- next.Handle(c)
			}()

			select {
			case err := <-done:
				return err
			case <-time.After(d):
				toolName := "<unknown>"
				if c.Function.Tool != nil {
					toolName = c.Function.Tool.Name()
				}
				return fmt.Errorf("function %s timed out after %v", toolName, d)
			case <-c.Done():
				return c.Err()
			}
		})
	}
}

// =============================================================================
// Testing Helpers
// =============================================================================

// NoOpHandler is a Handler that does nothing.
var NoOpHandler = HandlerFunc(func(c *Context) error {
	return nil
})

// ErrorHandler creates a handler that always returns an error.
func ErrorHandler(err error) Handler {
	return HandlerFunc(func(c *Context) error {
		return err
	})
}

// ResultHandler creates a handler that sets a result.
func ResultHandler(result any) Handler {
	return HandlerFunc(func(c *Context) error {
		c.Result = result
		return nil
	})
}
