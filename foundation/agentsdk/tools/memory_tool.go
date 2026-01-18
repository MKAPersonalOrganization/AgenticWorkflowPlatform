package tools

import (
	"context"
	"fmt"
	"io"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/services/artifact"
	"strings"
)

const (
	MemoryFileName    = "SCHOOLAI.md"
	MemoryHeader      = "## SchoolAI Memories"
	MemoryHeaderCheck = "## SchoolAI Memories" // Simplified check
)

// MemoryTool provides the save_memory tool.
type MemoryTool struct {
	store artifact.Service
}

// NewMemoryTool creates a new MemoryTool instance.
func NewMemoryTool(store artifact.Service) *MemoryTool {
	return &MemoryTool{store: store}
}

// Definition returns the tool definition.
func (t *MemoryTool) Definition() []core.Tool {
	return []core.Tool{
		core.NewFunctionTool(
			"save_memory",
			"Saves a specific piece of information or fact to your long-term memory. Use this when the user explicitly asks you to remember something, or when they state a clear, concise fact that seems important to retain for future interactions.",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"fact": map[string]any{
						"type":        "string",
						"description": "The specific fact or piece of information to remember. Should be a clear, self-contained statement.",
					},
				},
				"required": []string{"fact"},
			},
			t.saveMemory,
		),
	}
}

func (t *MemoryTool) saveMemory(ctx core.ToolContext, args any) (any, error) {
	var input struct {
		Fact string `json:"fact"`
	}
	if err := core.UnmarshalArgs(args, &input); err != nil {
		return nil, err
	}

	if input.Fact == "" {
		return nil, fmt.Errorf("fact cannot be empty")
	}

	// Extract userID from context
	userID := "default_user"
	if user := domain.GetUserFromContext(ctx.Context); user != nil {
		userID = user.ID
	}
	sessionID := "user_shared"

	// 1. Load existing content
	var currentContent string
	rc, err := t.store.Load(ctx, userID, sessionID, MemoryFileName)
	if err == nil {
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("failed to read memory file: %w", err)
		}
		currentContent = string(data)
	} else if err != artifact.ErrNotFound {
		return nil, fmt.Errorf("failed to load memory file: %w", err)
	}

	// 2. Compute new content
	newContent := t.computeNewContent(currentContent, input.Fact)

	// 3. Save content
	_, err = t.store.Save(ctx, userID, sessionID, MemoryFileName, strings.NewReader(newContent))
	if err != nil {
		return nil, fmt.Errorf("failed to save memory file: %w", err)
	}

	return fmt.Sprintf("Okay, I've remembered that: %q", input.Fact), nil
}

func (t *MemoryTool) computeNewContent(currentContent, fact string) string {
	processedFact := strings.TrimSpace(fact)
	// Remove leading dashes/bullets/regex-like start if present to normalize
	// Gemini uses: processedText = processedText.replace(/^(-+\s*)+/, '').trim();
	// Go approximate equivalent:
	processedFact = strings.TrimLeft(processedFact, "- ")
	processedFact = strings.TrimSpace(processedFact)

	newEntry := fmt.Sprintf("- %s", processedFact)

	if strings.Contains(currentContent, MemoryHeaderCheck) {
		// Append to existing section
		parts := strings.SplitN(currentContent, MemoryHeaderCheck, 2)
		if len(parts) == 2 {
			// Ensure proper newline separation for the new item
			sectionContent := parts[1]
			if !strings.HasSuffix(sectionContent, "\n") {
				sectionContent += "\n"
			}
			return parts[0] + MemoryHeader + sectionContent + newEntry + "\n"
		}
		return currentContent + "\n" + newEntry + "\n"
	}

	// Create new section
	prefix := ""
	if currentContent != "" {
		if strings.HasSuffix(currentContent, "\n\n") {
			prefix = ""
		} else if strings.HasSuffix(currentContent, "\n") {
			prefix = "\n"
		} else {
			prefix = "\n\n"
		}
	}
	return currentContent + prefix + MemoryHeader + "\n" + newEntry + "\n"
}

// NewMemoryInjectorMiddleware creates a middleware that injects the user's long-term memory
// into the system prompt.
func NewMemoryInjectorMiddleware(tool *MemoryTool) core.Middleware {
	return core.MiddlewareFunc(func(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
		memories, err := tool.LoadMemory(ctx)
		if err == nil && memories != "" {
			reqCtx.AddSystemPrompt(memories)
		}
		return next(ctx, reqCtx)
	})
}

// LoadMemory returns the full content of the memory file for context injection.
func (t *MemoryTool) LoadMemory(ctx context.Context) (string, error) {
	userID := "default_user"
	if user := domain.GetUserFromContext(ctx); user != nil {
		userID = user.ID
	}
	sessionID := "user_shared"

	rc, err := t.store.Load(ctx, userID, sessionID, MemoryFileName)
	if err != nil {
		if err == artifact.ErrNotFound {
			return "", nil
		}
		return "", err
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
