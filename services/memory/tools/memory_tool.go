package tools

import (
	"context"
	"fmt"
	"io"
	"strings"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/services/artifact"
)

const (
	MemoryFileName    = "SCHOOLAI.md"
	MemoryHeader      = "## SchoolAI Memories"
	MemoryHeaderCheck = "## SchoolAI Memories"
)

// memoryManager handles the persistence and manipulation of the memory file.
type memoryManager struct {
	store artifact.Service
}

func (m *memoryManager) load(ctx context.Context) (string, error) {
	userID, sessionID := m.getIDs(ctx)
	rc, err := m.store.Load(ctx, userID, sessionID, MemoryFileName)
	if err != nil {
		if err == artifact.ErrNotFound {
			return "", nil
		}
		return "", fmt.Errorf("failed to load memory file: %w", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("failed to read memory file: %w", err)
	}
	return string(data), nil
}

func (m *memoryManager) save(ctx context.Context, content string) error {
	userID, sessionID := m.getIDs(ctx)
	_, err := m.store.Save(ctx, userID, sessionID, MemoryFileName, strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("failed to save memory file: %w", err)
	}
	return nil
}

func (m *memoryManager) getIDs(ctx context.Context) (string, string) {
	userID := "default_user"
	if user := domain.GetUserFromContext(ctx); user != nil {
		userID = user.ID
	}
	return userID, "user_shared"
}

// MemoryTool saves facts to long-term memory.
type MemoryTool struct {
	mgr *memoryManager
}

func NewMemoryTool(store artifact.Service) *MemoryTool {
	return &MemoryTool{mgr: &memoryManager{store: store}}
}

func (t *MemoryTool) Name() string        { return "save_memory" }
func (t *MemoryTool) Description() string { return "Saves a specific fact to your long-term memory." }
func (t *MemoryTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: agent.TypeObject,
		Properties: map[string]*agent.Schema{
			"fact": {Type: agent.TypeString, Description: "The specific fact to remember."},
		},
		Required: []string{"fact"},
	}
}

func (t *MemoryTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var input struct {
		Fact string `json:"fact"`
	}
	if err := args.Decode(&input); err != nil {
		return agent.ErrorResult(err), nil
	}

	content, err := t.mgr.load(ctx)
	if err != nil {
		return agent.ErrorResult(err), nil
	}

	normalized := strings.TrimSpace(strings.TrimLeft(input.Fact, "- "))
	newEntry := fmt.Sprintf("- %s", normalized)

	if !strings.Contains(content, MemoryHeaderCheck) {
		prefix := ""
		if content != "" {
			if !strings.HasSuffix(content, "\n\n") {
				prefix = "\n\n"
			}
		}
		content += prefix + MemoryHeader + "\n"
	}

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += newEntry + "\n"

	if err := t.mgr.save(ctx, content); err != nil {
		return agent.ErrorResult(err), nil
	}

	return agent.TextResult(fmt.Sprintf("Saved memory: %q", input.Fact)), nil
}

// ForgetMemoryTool removes facts from long-term memory.
type ForgetMemoryTool struct {
	mgr *memoryManager
}

func NewForgetMemoryTool(store artifact.Service) *ForgetMemoryTool {
	return &ForgetMemoryTool{mgr: &memoryManager{store: store}}
}

func (t *ForgetMemoryTool) Name() string { return "forget_memory" }
func (t *ForgetMemoryTool) Description() string {
	return "Removes a specific fact from your long-term memory."
}

func (t *ForgetMemoryTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: agent.TypeObject,
		Properties: map[string]*agent.Schema{
			"fact": {Type: agent.TypeString, Description: "The specific fact to forget."},
		},
		Required: []string{"fact"},
	}
}

func (t *ForgetMemoryTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var input struct {
		Fact string `json:"fact"`
	}
	if err := args.Decode(&input); err != nil {
		return agent.ErrorResult(err), nil
	}

	content, err := t.mgr.load(ctx)
	if err != nil || content == "" {
		return agent.TextResult("No memories found."), nil
	}

	normalizedFact := strings.ToLower(strings.TrimSpace(strings.TrimLeft(input.Fact, "- ")))
	lines := strings.Split(content, "\n")
	var newLines []string
	found := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			normalizedLine := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
			if strings.Contains(normalizedLine, normalizedFact) || strings.Contains(normalizedFact, normalizedLine) {
				found = true
				continue
			}
		}
		newLines = append(newLines, line)
	}

	if !found {
		return agent.TextResult(fmt.Sprintf("Could not find memory: %q", input.Fact)), nil
	}

	if err := t.mgr.save(ctx, strings.Join(newLines, "\n")); err != nil {
		return agent.ErrorResult(err), nil
	}

	return agent.TextResult(fmt.Sprintf("Forgotten memory: %q", input.Fact)), nil
}

// NewMemoryInjectorMiddleware injects the user's memories into the system prompt.
func NewMemoryInjectorMiddleware(store artifact.Service) agent.RequestMiddleware {
	mgr := &memoryManager{store: store}
	return agent.RequestMiddlewareFunc(func(ctx context.Context, reqCtx *agent.RequestContext, next agent.RequestNextHandler) error {
		content, err := mgr.load(ctx)
		if err == nil && content != "" {
			reqCtx.AddSystemPrompt(content)
		}
		return next(ctx, reqCtx)
	})
}
