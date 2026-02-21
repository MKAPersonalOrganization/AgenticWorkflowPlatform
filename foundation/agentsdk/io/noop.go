package io

import (
	"context"
)

// NoopHandler auto-approves all confirmations and rejects all reads.
// Use this for testing and non-interactive contexts where tool execution
// should proceed without user interaction.
//
// Behavior:
//   - Confirm: always returns (true, nil) — auto-approve
//   - Read: always returns ("", error) — no input available
type NoopHandler struct{}

// NewNoopHandler creates a new NoopHandler.
func NewNoopHandler() *NoopHandler {
	return &NoopHandler{}
}

// Confirm auto-approves all requests.
func (h *NoopHandler) Confirm(_ context.Context, _ string, _ any) (bool, error) {
	return true, nil
}

// Read returns an error because no input source is available.
func (h *NoopHandler) Read(_ context.Context, prompt string, _ map[string]any) (string, error) {
	return "", &ErrInputRequired{
		Question: prompt,
	}
}

// DenyHandler denies all confirmations and rejects all reads.
// Use this for testing scenarios where tools should be blocked.
type DenyHandler struct{}

// NewDenyHandler creates a new DenyHandler.
func NewDenyHandler() *DenyHandler {
	return &DenyHandler{}
}

// Confirm denies all requests.
func (h *DenyHandler) Confirm(_ context.Context, _ string, _ any) (bool, error) {
	return false, nil
}

// Read returns an error because no input source is available.
func (h *DenyHandler) Read(_ context.Context, prompt string, _ map[string]any) (string, error) {
	return "", &ErrInputRequired{
		Question: prompt,
	}
}

// Verify interface compliance.
var (
	_ Handler = (*NoopHandler)(nil)
	_ Handler = (*DenyHandler)(nil)
)
