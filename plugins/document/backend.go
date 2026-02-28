package document

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/artifact"
)

// CreateMarkdownTool allows agents to create new markdown files.
type CreateMarkdownTool struct {
	ArtifactStore artifact.Service
}

func (t *CreateMarkdownTool) Name() string {
	return "create_markdown"
}

func (t *CreateMarkdownTool) Description() string {
	return "Create a new markdown file for the student."
}

func (t *CreateMarkdownTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type:        agent.TypeObject,
		Description: "Create a new markdown file for the student.",
		Properties: map[string]*agent.Schema{
			"filename": {
				Type:        agent.TypeString,
				Description: "Name of the file (e.g., 'notes.md'). Must end in .md",
			},
			"content": {
				Type:        agent.TypeString,
				Description: "Markdown content to write.",
			},
		},
		Required: []string{"filename", "content"},
	}
}

func (t *CreateMarkdownTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var input struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(args.Raw, &input); err != nil {
		return agent.NewResultError(fmt.Errorf("invalid arguments: %w", err)), nil
	}

	if !strings.HasSuffix(input.Filename, ".md") {
		return agent.NewResultError(fmt.Errorf("filename must end with .md")), nil
	}

	if args.UserID == "" || args.SessionID == "" {
		return agent.NewResultError(fmt.Errorf("missing user or session context")), nil
	}

	reader := strings.NewReader(input.Content)
	size, err := t.ArtifactStore.Save(ctx, args.UserID, args.SessionID, input.Filename, "text/markdown", reader)
	if err != nil {
		return agent.NewResultError(fmt.Errorf("failed to save artifact: %w", err)), nil
	}

	return agent.TextResult(fmt.Sprintf("Successfully created %s (%d bytes)", input.Filename, size)), nil
}

// UpdateMarkdownTool allows agents to update existing markdown files.
type UpdateMarkdownTool struct {
	ArtifactStore artifact.Service
}

func (t *UpdateMarkdownTool) Name() string {
	return "update_markdown"
}

func (t *UpdateMarkdownTool) Description() string {
	return "Update an existing markdown file. Overwrites entire content."
}

func (t *UpdateMarkdownTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type:        agent.TypeObject,
		Description: "Update an existing markdown file. Overwrites entire content.",
		Properties: map[string]*agent.Schema{
			"filename": {
				Type:        agent.TypeString,
				Description: "Name of the file to update.",
			},
			"content": {
				Type:        agent.TypeString,
				Description: "New markdown content.",
			},
		},
		Required: []string{"filename", "content"},
	}
}

func (t *UpdateMarkdownTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var input struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(args.Raw, &input); err != nil {
		return agent.NewResultError(fmt.Errorf("invalid arguments: %w", err)), nil
	}

	if !strings.HasSuffix(input.Filename, ".md") {
		return agent.NewResultError(fmt.Errorf("filename must end with .md")), nil
	}

	if args.UserID == "" || args.SessionID == "" {
		return agent.NewResultError(fmt.Errorf("missing user or session context")), nil
	}

	reader := strings.NewReader(input.Content)
	size, err := t.ArtifactStore.Save(ctx, args.UserID, args.SessionID, input.Filename, "text/markdown", reader)
	if err != nil {
		return agent.NewResultError(fmt.Errorf("failed to save artifact: %w", err)), nil
	}

	return agent.TextResult(fmt.Sprintf("Successfully updated %s (%d bytes)", input.Filename, size)), nil
}

// DeleteMarkdownTool allows agents to delete markdown files.
type DeleteMarkdownTool struct {
	ArtifactStore artifact.Service
}

func (t *DeleteMarkdownTool) Name() string {
	return "delete_markdown"
}

func (t *DeleteMarkdownTool) Description() string {
	return "Delete a markdown file."
}

func (t *DeleteMarkdownTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type:        agent.TypeObject,
		Description: "Delete a markdown file.",
		Properties: map[string]*agent.Schema{
			"filename": {
				Type:        agent.TypeString,
				Description: "Name of the file to delete.",
			},
		},
		Required: []string{"filename"},
	}
}

func (t *DeleteMarkdownTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var input struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(args.Raw, &input); err != nil {
		return agent.NewResultError(fmt.Errorf("invalid arguments: %w", err)), nil
	}

	if args.UserID == "" || args.SessionID == "" {
		return agent.NewResultError(fmt.Errorf("missing user or session context")), nil
	}

	err := t.ArtifactStore.Delete(ctx, args.UserID, args.SessionID, input.Filename)
	if err != nil {
		return agent.NewResultError(fmt.Errorf("failed to delete artifact: %w", err)), nil
	}

	return agent.TextResult(fmt.Sprintf("Successfully deleted %s", input.Filename)), nil
}
