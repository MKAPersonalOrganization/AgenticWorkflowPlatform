package io

import (
	"context"
)

// CachedReadHandler wraps PauseHandler and serves a pre-loaded response on the
// first Read() call. Subsequent Read() calls fall through to PauseHandler
// (returning ErrInputRequired). Confirm() always delegates to PauseHandler.
//
// This is used when resuming execution after a custom input pause: the user's
// text response is pre-loaded into the handler so the tool's io.Read() call
// returns the cached value instead of pausing again.
//
// Usage:
//
//	handler := io.NewCachedReadHandler("John Doe")
//	ctx = io.WithHandler(ctx, handler)
//	// Tool calls io.Read(ctx, "What's your name?", nil) → returns "John Doe"
type CachedReadHandler struct {
	input string
	used  bool
}

// NewCachedReadHandler creates a CachedReadHandler with the given input value.
// The first Read() call returns this value; subsequent calls return ErrInputRequired.
func NewCachedReadHandler(input string) *CachedReadHandler {
	return &CachedReadHandler{input: input}
}

// Confirm delegates to PauseHandler (returns ErrAuthRequired).
func (h *CachedReadHandler) Confirm(ctx context.Context, prompt string, data any) (bool, error) {
	return (&PauseHandler{}).Confirm(ctx, prompt, data)
}

// Read returns the cached input on the first call. Subsequent calls delegate
// to PauseHandler (returning ErrInputRequired).
func (h *CachedReadHandler) Read(ctx context.Context, prompt string, schema map[string]any) (string, error) {
	if !h.used {
		h.used = true
		return h.input, nil
	}
	return (&PauseHandler{}).Read(ctx, prompt, schema)
}

// Verify CachedReadHandler implements Handler.
var _ Handler = (*CachedReadHandler)(nil)
