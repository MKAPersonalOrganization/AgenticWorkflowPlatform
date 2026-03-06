package document

import (
	"context"
	"school-with-ai/foundation/agent"
	"school-with-ai/services/artifact"
	"school-with-ai/services/toolregistry"
	"time"

	"github.com/a-h/templ"
)

// Provider implements the toolregistry.PluginProvider interface for the Document toolset.
type Provider struct{}

func (p *Provider) ID() string { return "document" }
func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(`<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M10 9H8"/><path d="M16 13H8"/><path d="M16 17H8"/>`)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "document-set",
			Name:        "Document Management",
			Description: "Tools for managing documents and artifacts",
			Category:    "Productivity",
			ToolIDs:     []string{"doc-create", "doc-update", "doc-delete"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{ID: "doc-create", Name: "Create Document", Description: "Create a new markdown document", Category: "Documents", ToolSetID: "document-set", CreatedAt: now, UpdatedAt: now},
		{ID: "doc-update", Name: "Update Document", Description: "Update an existing document", Category: "Documents", ToolSetID: "document-set", CreatedAt: now, UpdatedAt: now},
		{ID: "doc-delete", Name: "Delete Document", Description: "Delete a document", Category: "Documents", ToolSetID: "document-set", CreatedAt: now, UpdatedAt: now},
	}
}

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	artifactStore, ok := deps.ArtifactStore.(artifact.Service)
	if !ok {
		// Provide fallback or panic if strictly required
	}

	switch toolID {
	case "doc-create":
		return &CreateMarkdownTool{ArtifactStore: artifactStore}, nil
	case "doc-update":
		return &UpdateMarkdownTool{ArtifactStore: artifactStore}, nil
	case "doc-delete":
		return &DeleteMarkdownTool{ArtifactStore: artifactStore}, nil
	default:
		return nil, nil
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"create_markdown", "update_markdown", "delete_markdown"}
}

func (p *Provider) UIHandledElicitations() []string {
	return nil
}
