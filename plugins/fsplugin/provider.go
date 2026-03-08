package fsplugin

import (
	"context"
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/filesystem"
	"school-with-ai/platform/toolregistry"

	"github.com/a-h/templ"
)

// Provider implements the toolregistry.PluginProvider interface for agent filesystem tools.
type Provider struct {
	FS filesystem.AgentFS
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
			ToolIDs:     []string{"fs-list-filesystems", "fs-list-dir", "fs-read-file", "fs-write-file", "fs-mkdir", "fs-remove", "fs-stat", "fs-rename"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{ID: "fs-list-filesystems", Name: "List Filesystems", Description: "List virtual filesystems the agent can access", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "fs-list-dir", Name: "List Directory", Description: "List files and subdirectories in a directory", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "fs-read-file", Name: "Read File", Description: "Read the contents of a file", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "fs-write-file", Name: "Write File", Description: "Create or overwrite a file with content", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "fs-mkdir", Name: "Create Directory", Description: "Create a new directory", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "fs-remove", Name: "Remove", Description: "Delete a file or empty directory", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "fs-stat", Name: "File Info", Description: "Get metadata (size, type, modified time) for a path", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
		{ID: "fs-rename", Name: "Rename / Move", Description: "Rename or move a file or directory", Category: "Filesystem", ToolSetID: "filesystem-set", CreatedAt: now, UpdatedAt: now},
	}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

func (p *Provider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	if p.FS == nil {
		return nil, nil // filesystem module not available
	}

	switch toolID {
	case "fs-list-filesystems":
		return &listFilesystemsTool{fs: p.FS}, nil
	case "fs-list-dir":
		return &listDirTool{fs: p.FS}, nil
	case "fs-read-file":
		return &readFileTool{fs: p.FS}, nil
	case "fs-write-file":
		return &writeFileTool{fs: p.FS}, nil
	case "fs-mkdir":
		return &mkdirTool{fs: p.FS}, nil
	case "fs-remove":
		return &removeTool{fs: p.FS}, nil
	case "fs-stat":
		return &statTool{fs: p.FS}, nil
	case "fs-rename":
		return &renameTool{fs: p.FS}, nil
	default:
		return nil, nil
	}
}

func (p *Provider) GetUITemplate() templ.Component { return nil }
func (p *Provider) UIHandledTools() []string       { return nil }
