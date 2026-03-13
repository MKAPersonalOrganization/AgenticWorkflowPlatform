package agent

import (
	"fmt"
	"log/slog"
	"reflect"
	"time"
)

// =============================================================================
// Function-Level Middleware
//
// These middlewares wrap individual tool executions and are bridged to
// ADK-Go's BeforeToolCallback / AfterToolCallback via the middleware bridge
// in adapters/adkgo/runner.go.
// =============================================================================

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

			agentName, _ := c.Metadata["agent_name"].(string)
			if agentName == "" {
				agentName = "<unknown>"
			}
			threadID, _ := c.Metadata["thread_id"].(string)
			messageID, _ := c.Metadata["message_id"].(string)

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
				logResult := c.Result
				if logResult != nil {
					v := reflect.ValueOf(logResult)
					if v.Kind() == reflect.Ptr {
						v = v.Elem()
					}
					if v.Kind() == reflect.Struct {
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
