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

// ToolLogging logs tool calls including arguments and results.
//
// When used with the ADK-Go middleware bridge (which sets ToolPhaseKey in
// metadata), this middleware:
//   - In the "before" phase: logs "tool call started" and records the start time
//     in metadata so the "after" phase can compute duration accurately.
//   - In the "after" phase: logs "tool call completed" or "tool call failed"
//     with the result and duration.
//
// When ToolPhaseKey is absent (direct / single-pass use), the full before+after
// logic runs inline in a single call, preserving backward compatibility.
func ToolLogging(logger *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.DryRun || !c.IsTool() {
				return next.Handle(c)
			}

			toolName, agentName, threadID, messageID := extractLoggingMetadata(c)

			phase, _ := c.Metadata[ToolPhaseKey].(string)

			if phase != ToolPhaseAfter {
				// Before phase (or single-pass): log started and record the start time.
				// In single-pass mode this runs immediately before next.Handle(c).
				// In phased mode it runs in the BeforeToolCallback; the start time is
				// carried forward to the AfterToolCallback via the shared Context.
				c.Metadata[ToolStartTimeKey] = time.Now()

				logger.Info("tool call started",
					"agent", agentName,
					"thread_id", threadID,
					"message_id", messageID,
					"tool", toolName,
					"arguments", c.Tool.Arguments)
			}

			err := next.Handle(c)

			if phase != ToolPhaseBefore {
				// After phase (or single-pass): log completed / failed.
				start, hasStart := c.Metadata[ToolStartTimeKey].(time.Time)
				if !hasStart {
					start = time.Now()
				}
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
					logger.Info("tool call completed",
						"agent", agentName,
						"thread_id", threadID,
						"message_id", messageID,
						"tool", toolName,
						"duration", duration,
						"result", summariseResult(c.Result))
				}
			}

			return err
		})
	}
}

// extractLoggingMetadata pulls the common logging fields from a middleware Context.
func extractLoggingMetadata(c *Context) (toolName, agentName, threadID, messageID string) {
	toolName, _ = c.Metadata["tool_name"].(string)
	if toolName == "" && c.Tool != nil && c.Tool.Tool != nil {
		toolName = c.Tool.Tool.Name()
	}
	if toolName == "" {
		toolName = "<unknown>"
	}

	agentName, _ = c.Metadata["agent_name"].(string)
	if agentName == "" {
		agentName = "<unknown>"
	}
	threadID, _ = c.Metadata["thread_id"].(string)
	messageID, _ = c.Metadata["message_id"].(string)
	return
}

// summariseResult extracts a loggable representation of the tool result.
func summariseResult(result any) any {
	if result == nil {
		return nil
	}
	v := reflect.ValueOf(result)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		if f := v.FieldByName("Result"); f.IsValid() && f.Kind() == reflect.String {
			return f.String()
		}
	}
	return result
}

// Validator is the interface for validating function arguments.
type Validator interface {
	Validate(tool Tool, arguments map[string]any) error
}

// FunctionValidation validates function arguments before execution.
func FunctionValidation(validator Validator) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if c.Tool == nil {
				return next.Handle(c)
			}

			if err := validator.Validate(c.Tool.Tool, c.Tool.Arguments); err != nil {
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
			if c.Tool == nil {
				return next.Handle(c)
			}

			key := keyFunc(c.Tool.Tool, c.Tool.Arguments)
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
			if c.Tool == nil {
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
				if c.Tool.Tool != nil {
					toolName = c.Tool.Tool.Name()
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
