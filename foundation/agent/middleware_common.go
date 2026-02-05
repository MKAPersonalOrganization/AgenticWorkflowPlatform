package agent

import (
	"fmt"
	"log/slog"
	"reflect"
	"time"
)

// =============================================================================
// Agent-Level Middleware
// =============================================================================

// AgentLogging logs detailed agent operations including agent identity, session, and message context.
func AgentLogging(logger *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.DryRun || !c.IsAgent() {
				return next.Handle(c)
			}

			start := time.Now()

			// Extract agent details from typed fields, fall back to metadata
			agentName := "<unknown>"
			agentID := "<unknown>"
			if c.Agent != nil && c.Agent.Agent != nil {
				agentName = c.Agent.Agent.Name()
				agentID = string(c.Agent.Agent.ID())
			}
			if agentName == "<unknown>" {
				if v, ok := c.Metadata["agent_name"].(string); ok && v != "" {
					agentName = v
				}
			}

			// Extract session details from typed fields, fall back to metadata
			sessionID := ""
			userID := ""
			if c.Agent != nil && c.Agent.Session != nil {
				sessionID = c.Agent.Session.ID()
				userID = c.Agent.Session.UserID()
			}
			if sessionID == "" {
				if v, ok := c.Metadata["session_id"].(string); ok {
					sessionID = v
				}
			}
			if userID == "" {
				if v, ok := c.Metadata["user_id"].(string); ok {
					userID = v
				}
			}

			// Extract thread ID from metadata
			threadID, _ := c.Metadata["thread_id"].(string)

			// Extract message count and streaming from typed fields, fall back to metadata
			msgCount := 0
			streaming := false
			if c.Agent != nil {
				msgCount = len(c.Agent.Messages)
				streaming = c.Agent.Streaming
			}
			if msgCount == 0 {
				if v, ok := c.Metadata["message_count"].(int); ok {
					msgCount = v
				}
			}

			// Log agent run started
			logger.Info("agent run started",
				"agent", agentName,
				"agent_id", agentID,
				"thread_id", threadID,
				"user_id", userID,
				"message_count", msgCount,
				"streaming", streaming)

			err := next.Handle(c)

			duration := time.Since(start)
			if err != nil {
				logger.Error("agent run failed",
					"agent", agentName,
					"agent_id", agentID,
					"thread_id", threadID,
					"user_id", userID,
					"message_count", msgCount,
					"streaming", streaming,
					"duration", duration,
					"error", err)
			} else {
				logger.Info("agent run completed",
					"agent", agentName,
					"agent_id", agentID,
					"thread_id", threadID,
					"user_id", userID,
					"message_count", msgCount,
					"streaming", streaming,
					"duration", duration)
			}

			return err
		})
	}
}

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
			if c.DryRun {
				return next.Handle(c)
			}
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

// ToolLogging logs detailed tool calls including arguments and results.
func ToolLogging(logger *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.DryRun || !c.IsFunction() {
				return next.Handle(c)
			}

			start := time.Now()
			toolName, _ := c.Metadata["tool_name"].(string)
			if toolName == "" && c.Function != nil && c.Function.Tool != nil {
				toolName = c.Function.Tool.Name()
			}
			if toolName == "" {
				toolName = "<unknown>"
			}

			if c.Function == nil || c.Function.Tool == nil {
				logger.Warn("tool call without resolved tool object",
					"tool", toolName)
			}

			// Extract metadata for enriched logging
			agentName, _ := c.Metadata["agent_name"].(string)
			if agentName == "" {
				agentName = "<unknown>"
			}
			threadID, _ := c.Metadata["thread_id"].(string)
			messageID, _ := c.Metadata["message_id"].(string)

			// Log tool call started
			logger.Info("tool call started",
				"agent", agentName,
				"thread_id", threadID,
				"message_id", messageID,
				"tool", toolName,
				"arguments", c.Function.Arguments)

			err := next.Handle(c)

			duration := time.Since(start)
			if err != nil {
				logger.Error("tool call failed",
					"agent", agentName,
					"thread_id", threadID,
					"message_id", messageID,
					"tool", toolName,
					"duration", duration,
					"error", err)
			} else {
				// Format the result for better readability in logs.
				// If it's a struct (like core.ToolExecResult), we try to extract its Result field.
				logResult := c.Result
				if logResult != nil {
					v := reflect.ValueOf(logResult)
					// Handle pointer to struct
					if v.Kind() == reflect.Ptr {
						v = v.Elem()
					}
					if v.Kind() == reflect.Struct {
						// Look for a field named 'Result' (common in tool result structures)
						if f := v.FieldByName("Result"); f.IsValid() && f.Kind() == reflect.String {
							logResult = f.String()
						}
					}
				}

				logger.Info("tool call completed",
					"agent", agentName,
					"thread_id", threadID,
					"message_id", messageID,
					"tool", toolName,
					"duration", duration,
					"result", logResult)
			}

			return err
		})
	}
}

// =============================================================================
// Chat-Level Middleware
// =============================================================================

// ChatLogging logs detailed LLM/chat calls including model, messages, and options.
func ChatLogging(logger *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.DryRun || !c.IsChat() {
				return next.Handle(c)
			}

			start := time.Now()

			// Extract metadata for enriched logging
			agentName, _ := c.Metadata["agent_name"].(string)
			if agentName == "" {
				agentName = "<unknown>"
			}
			threadID, _ := c.Metadata["thread_id"].(string)

			// Extract chat operation details from typed fields, fall back to metadata
			provider := "<unknown>"
			if c.Chat != nil && c.Chat.Model != nil {
				provider = c.Chat.Model.Provider()
			}
			if provider == "<unknown>" {
				if v, ok := c.Metadata["provider"].(string); ok && v != "" {
					provider = v
				}
			}

			model := ""
			if v, ok := c.Metadata["model"].(string); ok {
				model = v
			}

			msgCount := 0
			if c.Chat != nil {
				msgCount = len(c.Chat.Messages)
			}
			if msgCount == 0 {
				if v, ok := c.Metadata["message_count"].(int); ok {
					msgCount = v
				}
			}

			// Extract options from typed fields, fall back to metadata
			var temperature float64
			var maxTokens int
			if c.Chat != nil && c.Chat.Options != nil {
				temperature = c.Chat.Options.Temperature
				maxTokens = c.Chat.Options.MaxTokens
			}
			if temperature == 0 {
				if v, ok := c.Metadata["temperature"].(float64); ok {
					temperature = v
				}
			}
			if maxTokens == 0 {
				if v, ok := c.Metadata["max_tokens"].(int); ok {
					maxTokens = v
				}
			}

			// Log chat call started
			logger.Info("chat call started",
				"agent", agentName,
				"thread_id", threadID,
				"provider", provider,
				"model", model,
				"message_count", msgCount,
				"temperature", temperature,
				"max_tokens", maxTokens)

			err := next.Handle(c)

			duration := time.Since(start)
			if err != nil {
				logger.Error("chat call failed",
					"agent", agentName,
					"thread_id", threadID,
					"provider", provider,
					"model", model,
					"message_count", msgCount,
					"duration", duration,
					"error", err)
			} else {
				logger.Info("chat call completed",
					"agent", agentName,
					"thread_id", threadID,
					"provider", provider,
					"model", model,
					"message_count", msgCount,
					"duration", duration)
			}

			return err
		})
	}
}

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
