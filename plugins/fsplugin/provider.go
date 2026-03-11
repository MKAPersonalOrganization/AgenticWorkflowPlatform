package fsplugin

import (
	"context"
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/platform/toolregistry"

	"github.com/a-h/templ"
)

// Provider implements the toolregistry.PluginProvider interface for agent filesystem tools.
type Provider struct {
	FS AgentFilesystem
}

func (p *Provider) ID() string { return "filesystem" }
func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(`<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2z"/>`)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "filesystem-set",
			Name:        "Filesystem",
			Description: "Tools for reading and writing files in user-attached virtual filesystems",
			Category:    "Storage",
			ToolIDs:     []string{"list_filesystems", "list_directory", "read_file", "write_file", "create_directory", "remove_entry", "file_info", "rename_entry"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{ID: "list_filesystems", Name: "List Filesystems", Description: "List virtual filesystems the agent can access", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "list_directory", Name: "List Directory", Description: "List files and subdirectories in a directory", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "read_file", Name: "Read File", Description: "Read the contents of a file", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "write_file", Name: "Write File", Description: "Create or overwrite a file with content", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "create_directory", Name: "Create Directory", Description: "Create a new directory", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "remove_entry", Name: "Remove", Description: "Delete a file or empty directory", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "file_info", Name: "File Info", Description: "Get metadata (size, type, modified time) for a path", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "rename_entry", Name: "Rename / Move", Description: "Rename or move a file or directory", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
	}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

func (p *Provider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	if p.FS == nil {
		return nil, nil // filesystem module not available
	}

	switch toolID {
	case "list_filesystems":
		return &listFilesystemsTool{fs: p.FS}, nil
	case "list_directory":
		return &listDirTool{fs: p.FS}, nil
	case "read_file":
		return &readFileTool{fs: p.FS}, nil
	case "write_file":
		return &writeFileTool{fs: p.FS}, nil
	case "create_directory":
		return &mkdirTool{fs: p.FS}, nil
	case "remove_entry":
		return &removeTool{fs: p.FS}, nil
	case "file_info":
		return &statTool{fs: p.FS}, nil
	case "rename_entry":
		return &renameTool{fs: p.FS}, nil
	default:
		return nil, nil
	}
}

func (p *Provider) GetUITemplate() templ.Component { return nil }
func (p *Provider) UIHandledTools() []string       { return nil }
