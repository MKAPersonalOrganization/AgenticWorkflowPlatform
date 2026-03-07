package memory

import (
	"context"
	"fmt"
	"strings"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/chat/memory"
	"school-with-ai/shared"
)

const (
	MemoryHeader      = "## SchoolAI Memories"
	MemoryHeaderCheck = "## SchoolAI Memories"
)

// MemoryTool saves individual facts to the user's long-term memory.
// This tool persists important information across all conversations, such as:
// - User preferences, learning goals, and interests
// - Key facts about the user (name, background, learning style)
// - Important insights or progress milestones
// Use this tool when the user shares information that should be remembered permanently.
type MemoryTool struct {
	service memory.Service
}

func NewMemoryTool(service memory.Service) *MemoryTool {
	return &MemoryTool{service: service}
}

func (t *MemoryTool) Name() string { return "save_memory" }
func (t *MemoryTool) Description() string {
	return "Saves a single important fact about the user to long-term memory. Use this to remember key information like the user's name, preferences, learning goals, or important insights that should persist across all future conversations. Each fact is stored as a separate item."
}

func (t *MemoryTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: agent.TypeObject,
		Properties: map[string]*agent.Schema{
			"fact": {
				Type:        agent.TypeString,
				Description: "A clear, concise fact to remember (e.g., 'User's name is Sarah', 'Prefers visual learning', 'Working towards calculus proficiency'). Avoid duplicates of existing memories.",
			},
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

	userID := getUserID(ctx)

	// Debug logging
	fmt.Printf("[DEBUG save_memory] UserID: %s\n", userID)
	if userID == "" {
		return agent.ErrorResult(fmt.Errorf("no authenticated user found in context - cannot save memory")), nil
	}

	content, err := t.service.Load(ctx, userID)
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

	if err := t.service.Save(ctx, userID, content); err != nil {
		return agent.ErrorResult(err), nil
	}

	return agent.TextResult(fmt.Sprintf("Saved memory: %q", input.Fact)), nil
}

// ForgetMemoryTool removes a specific fact from the user's long-term memory.
// This tool searches for and deletes a single memory entry that matches the provided text.
// Use this when the user explicitly asks to forget something, or when information becomes
// outdated or incorrect. The search is case-insensitive and matches partial text.
type ForgetMemoryTool struct {
	service memory.Service
}

func NewForgetMemoryTool(service memory.Service) *ForgetMemoryTool {
	return &ForgetMemoryTool{service: service}
}

func (t *ForgetMemoryTool) Name() string { return "forget_memory" }
func (t *ForgetMemoryTool) Description() string {
	return "Removes a single specific fact from long-term memory. Searches for memories containing the provided text and deletes the first match found. Use this when the user asks to forget something or when correcting outdated information. If you need to delete all memories, use clear_memories instead."
}

func (t *ForgetMemoryTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: agent.TypeObject,
		Properties: map[string]*agent.Schema{
			"fact": {
				Type:        agent.TypeString,
				Description: "The fact to remove from memory. Can be partial text - will match any memory containing this text (case-insensitive). Example: 'name is' would match 'User's name is John'.",
			},
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

	userID := getUserID(ctx)

	// Require authenticated user
	if userID == "" {
		return agent.ErrorResult(fmt.Errorf("no authenticated user found in context - cannot modify memory")), nil
	}

	content, err := t.service.Load(ctx, userID)
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

	if err := t.service.Save(ctx, userID, strings.Join(newLines, "\n")); err != nil {
		return agent.ErrorResult(err), nil
	}

	return agent.TextResult(fmt.Sprintf("Forgotten memory: %q", input.Fact)), nil
}

// ListMemoriesTool retrieves and displays all stored memories for the user.
// This is a read-only operation that shows what information is currently persisted.
// Use this when:
// - The user asks what you remember about them
// - Before saving a new memory to check for duplicates
// - To verify memory contents before deletion
// Note: Memories are automatically injected into your context, so you don't need
// to call this to access them during normal conversation.
type ListMemoriesTool struct {
	service memory.Service
}

func NewListMemoriesTool(service memory.Service) *ListMemoriesTool {
	return &ListMemoriesTool{service: service}
}

func (t *ListMemoriesTool) Name() string { return "list_memories" }
func (t *ListMemoriesTool) Description() string {
	return "Retrieves and displays all facts currently stored in long-term memory. Use this when the user asks what you remember about them, or before saving new memories to avoid duplicates. This is read-only and does not modify any data. Note: Memories are already available in your context automatically."
}

func (t *ListMemoriesTool) Schema() *agent.Schema {
	return nil
}

func (t *ListMemoriesTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	userID := getUserID(ctx)

	// Require authenticated user
	if userID == "" {
		return agent.ErrorResult(fmt.Errorf("no authenticated user found in context - cannot list memories")), nil
	}

	// Debug: log userID
	fmt.Printf("[DEBUG list_memories] UserID: %s\n", userID)

	content, err := t.service.Load(ctx, userID)
	if err != nil {
		return agent.ErrorResult(err), nil
	}

	// Debug: log content
	fmt.Printf("[DEBUG list_memories] Content length: %d, Content: %q\n", len(content), content)

	if content == "" {
		return agent.TextResult("No memories found."), nil
	}

	// Extract just the memory items (lines starting with "- ")
	lines := strings.Split(content, "\n")
	var memories []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			memories = append(memories, strings.TrimPrefix(trimmed, "- "))
		}
	}

	if len(memories) == 0 {
		return agent.TextResult("No memories found."), nil
	}

	result := fmt.Sprintf("Found %d memories:\n", len(memories))
	for i, mem := range memories {
		result += fmt.Sprintf("%d. %s\n", i+1, mem)
	}

	return agent.TextResult(result), nil
}

// ClearMemoriesTool permanently deletes ALL memories from the user's long-term storage.
// WARNING: This is a DESTRUCTIVE operation that cannot be undone.
// This removes every single stored fact about the user across all conversations.
// Use ONLY when:
// - The user explicitly requests to delete all their memories
// - The user wants to start completely fresh
// Always confirm with the user before executing this tool.
type ClearMemoriesTool struct {
	service memory.Service
}

func NewClearMemoriesTool(service memory.Service) *ClearMemoriesTool {
	return &ClearMemoriesTool{service: service}
}

func (t *ClearMemoriesTool) Name() string { return "clear_memories" }
func (t *ClearMemoriesTool) Description() string {
	return "DESTRUCTIVE: Permanently deletes ALL facts from long-term memory. This cannot be undone and will erase every stored memory about the user. Use ONLY when the user explicitly requests to delete all memories or wants to reset everything. Always confirm before using this tool. For removing single facts, use forget_memory instead."
}

func (t *ClearMemoriesTool) Schema() *agent.Schema {
	return nil
}

func (t *ClearMemoriesTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	userID := getUserID(ctx)

	// Require authenticated user
	if userID == "" {
		return agent.ErrorResult(fmt.Errorf("no authenticated user found in context - cannot clear memories")), nil
	}

	// Save empty content to clear all memories
	if err := t.service.Save(ctx, userID, ""); err != nil {
		return agent.ErrorResult(err), nil
	}

	return agent.TextResult("All memories have been cleared."), nil
}

// NewMemoryInjectorMiddleware returns a Chat-level Middleware that prepends
// the user's long-term memories as a system message before each LLM call.
func NewMemoryInjectorMiddleware(service memory.Service) agent.Middleware {
	return func(next agent.Handler) agent.Handler {
		return agent.HandlerFunc(func(c *agent.Context) error {
			if !c.IsChat() {
				return next.Handle(c)
			}

			userID := getUserID(c.Context)
			if userID == "" {
				return next.Handle(c)
			}

			content, err := service.Load(c.Context, userID)
			if err == nil && content != "" {
				memMsg := agent.Message{
					Role:    agent.RoleSystem,
					Content: []agent.Content{{Type: agent.ContentTypeText, Text: content}},
				}
				c.Chat.Messages = append([]agent.Message{memMsg}, c.Chat.Messages...)
			}

			return next.Handle(c)
		})
	}
}

func getUserID(ctx context.Context) string {
	if user := shared.GetUserFromContext(ctx); user != nil {
		return user.ID
	}
	// Return empty string if no authenticated user - this should not happen in production
	// as memory operations should only occur for authenticated users
	return ""
}
