package agent

import (
	"fmt"
	"time"
)

// =============================================================================
// Tool-Level Middlewares
//
// These middlewares can be used with buildBeforeOnlyCallbacks in the adapter
// to apply cross-cutting concerns before tool execution.
//
// Note: Tool call logging is handled natively by ADK-Go callbacks in the
// adapter layer (adapters/adkgo/callbacks.go), not by foundation middleware.
// =============================================================================

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
