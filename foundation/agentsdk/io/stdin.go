package io

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// StdinHandler implements Handler for CLI / local development contexts.
//
// It blocks on stdin until the user responds, mirroring BeeAI's
// ConsoleIOHandler behavior. This is suitable for CLI tools, scripts,
// and local development where the process has direct terminal access.
//
// For web/SSE contexts, use PauseHandler instead.
type StdinHandler struct {
	reader *bufio.Reader
	writer *os.File
}

// NewStdinHandler creates a new StdinHandler that reads from os.Stdin
// and writes prompts to os.Stderr (to avoid mixing with stdout).
func NewStdinHandler() *StdinHandler {
	return &StdinHandler{
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stderr,
	}
}

// Confirm prints a prompt and waits for y/n input from stdin.
// Returns true for "y"/"yes", false for "n"/"no".
func (h *StdinHandler) Confirm(ctx context.Context, prompt string, _ any) (bool, error) {
	fmt.Fprintf(h.writer, "\n🔒 %s [y/N]: ", prompt)

	// Check context before blocking
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	line, err := h.reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("io: failed to read confirmation: %w", err)
	}

	answer := strings.TrimSpace(strings.ToLower(line))
	switch answer {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// Read prints a prompt and waits for free-form input from stdin.
// Returns the user's text response (trimmed).
func (h *StdinHandler) Read(ctx context.Context, prompt string, _ map[string]any) (string, error) {
	fmt.Fprintf(h.writer, "\n📝 %s: ", prompt)

	// Check context before blocking
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	line, err := h.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("io: failed to read input: %w", err)
	}

	return strings.TrimSpace(line), nil
}

// Verify StdinHandler implements Handler.
var _ Handler = (*StdinHandler)(nil)
