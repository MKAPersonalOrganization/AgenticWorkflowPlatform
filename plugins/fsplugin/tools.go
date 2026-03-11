package fsplugin

import (
	"context"
	"fmt"
	"io"
	"strings"

	"school-with-ai/foundation/agent"

	"github.com/google/uuid"
)

// ─── Helpers ────────────────────────────────────────────────────────────────

// requirePermission resolves the user/agent from args, validates the filesystem_id,
// and checks that the agent has been granted access to the requested filesystem.
func requirePermission(ctx context.Context, fs AgentFilesystem, args agent.Arguments, fsIDStr string) (uuid.UUID, string, error) {
	userID := args.UserID
	if userID == "" {
		return uuid.Nil, "", fmt.Errorf("user context is missing")
	}

	agentID := ""
	if args.Metadata != nil {
		if aid, ok := args.Metadata["agent_id"].(string); ok {
			agentID = aid
		}
	}
	if agentID == "" {
		return uuid.Nil, "", fmt.Errorf("agent context is required for filesystem access")
	}

	fsID, err := uuid.Parse(fsIDStr)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("invalid filesystem_id: %w", err)
	}

	ok, err := fs.HasAgentPermission(ctx, fsID, userID, agentID)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("checking permission: %w", err)
	}
	if !ok {
		return uuid.Nil, "", fmt.Errorf("agent does not have permission to access this filesystem")
	}

	return fsID, userID, nil
}

func getAgentID(args agent.Arguments) string {
	if args.Metadata != nil {
		if aid, ok := args.Metadata["agent_id"].(string); ok {
			return aid
		}
	}
	return ""
}

// ─── list_filesystems ───────────────────────────────────────────────────────

type listFilesystemsTool struct{ fs AgentFilesystem }

func (t *listFilesystemsTool) Name() string { return "list_filesystems" }
func (t *listFilesystemsTool) Description() string {
	return "List virtual filesystems this agent has access to. Call this first to discover available filesystem_id values."
}

func (t *listFilesystemsTool) Schema() *agent.Schema {
	return &agent.Schema{Type: "object", Properties: map[string]*agent.Schema{}}
}

func (t *listFilesystemsTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	userID := args.UserID
	agentID := getAgentID(args)
	if userID == "" || agentID == "" {
		return agent.ErrorResult(fmt.Errorf("user and agent context required")), nil
	}

	fss, err := t.fs.ListPermittedFilesystems(ctx, userID, agentID)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("listing filesystems: %w", err)), nil
	}

	type entry struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Desc string `json:"description"`
		Type string `json:"type"`
	}
	out := make([]entry, 0, len(fss))
	for _, f := range fss {
		out = append(out, entry{
			ID:   f.ID.String(),
			Name: f.Name,
			Desc: f.Description,
			Type: string(f.Type),
		})
	}
	return agent.JSONResult(map[string]any{"filesystems": out, "count": len(out)}), nil
}

// ─── list_directory ─────────────────────────────────────────────────────────

type listDirArgs struct {
	FilesystemID string `json:"filesystem_id"`
	Path         string `json:"path"`
}

type listDirTool struct{ fs AgentFilesystem }

func (t *listDirTool) Name() string { return "list_directory" }
func (t *listDirTool) Description() string {
	return "List files and subdirectories at a path in a virtual filesystem."
}

func (t *listDirTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: "object",
		Properties: map[string]*agent.Schema{
			"filesystem_id": {Type: "string", Description: "UUID of the filesystem (from list_filesystems)"},
			"path":          {Type: "string", Description: "Directory path to list, e.g. \"/\" or \"/docs\""},
		},
		Required: []string{"filesystem_id", "path"},
	}
}

func (t *listDirTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var p listDirArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid args: %w", err)), nil
	}

	fsID, userID, err := requirePermission(ctx, t.fs, args, p.FilesystemID)
	if err != nil {
		return agent.ErrorResult(err), nil
	}

	entries, err := t.fs.ListDir(ctx, fsID, userID, p.Path)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("listing directory: %w", err)), nil
	}

	return agent.JSONResult(map[string]any{"path": p.Path, "entries": entries, "count": len(entries)}), nil
}

// ─── read_file ──────────────────────────────────────────────────────────────

type readFileArgs struct {
	FilesystemID string `json:"filesystem_id"`
	Path         string `json:"path"`
}

type readFileTool struct{ fs AgentFilesystem }

func (t *readFileTool) Name() string { return "read_file" }
func (t *readFileTool) Description() string {
	return "Read the text contents of a file in a virtual filesystem. Returns up to 100 KB."
}

func (t *readFileTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: "object",
		Properties: map[string]*agent.Schema{
			"filesystem_id": {Type: "string", Description: "UUID of the filesystem"},
			"path":          {Type: "string", Description: "File path to read, e.g. \"/docs/README.md\""},
		},
		Required: []string{"filesystem_id", "path"},
	}
}

const maxReadBytes = 100 * 1024 // 100 KB limit for agent reads

func (t *readFileTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var p readFileArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid args: %w", err)), nil
	}

	fsID, userID, err := requirePermission(ctx, t.fs, args, p.FilesystemID)
	if err != nil {
		return agent.ErrorResult(err), nil
	}

	rc, stat, err := t.fs.ReadFile(ctx, fsID, userID, p.Path)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("reading file: %w", err)), nil
	}
	defer rc.Close()

	limited := io.LimitReader(rc, maxReadBytes+1)
	buf := new(strings.Builder)
	n, err := io.Copy(buf, limited)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("reading file content: %w", err)), nil
	}

	truncated := n > maxReadBytes
	content := buf.String()
	if truncated {
		content = content[:maxReadBytes]
	}

	return agent.JSONResult(map[string]any{
		"path":         p.Path,
		"content":      content,
		"size":         stat.Size,
		"content_type": stat.ContentType,
		"truncated":    truncated,
	}), nil
}

// ─── write_file ─────────────────────────────────────────────────────────────

type writeFileArgs struct {
	FilesystemID string `json:"filesystem_id"`
	Path         string `json:"path"`
	Content      string `json:"content"`
	ContentType  string `json:"content_type"`
}

type writeFileTool struct{ fs AgentFilesystem }

func (t *writeFileTool) Name() string { return "write_file" }
func (t *writeFileTool) Description() string {
	return "Create or overwrite a file with text content in a virtual filesystem."
}

func (t *writeFileTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: "object",
		Properties: map[string]*agent.Schema{
			"filesystem_id": {Type: "string", Description: "UUID of the filesystem"},
			"path":          {Type: "string", Description: "File path to write, e.g. \"/docs/notes.md\""},
			"content":       {Type: "string", Description: "Text content to write to the file"},
			"content_type":  {Type: "string", Description: "MIME type (default: text/plain)"},
		},
		Required: []string{"filesystem_id", "path", "content"},
	}
}

func (t *writeFileTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var p writeFileArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid args: %w", err)), nil
	}
	if p.ContentType == "" {
		p.ContentType = "text/plain"
	}

	fsID, userID, err := requirePermission(ctx, t.fs, args, p.FilesystemID)
	if err != nil {
		return agent.ErrorResult(err), nil
	}

	reader := strings.NewReader(p.Content)
	stat, err := t.fs.WriteFile(ctx, fsID, userID, p.Path, reader, p.ContentType, int64(len(p.Content)))
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("writing file: %w", err)), nil
	}

	return agent.JSONResult(map[string]any{
		"path":    p.Path,
		"size":    stat.Size,
		"written": true,
	}), nil
}

// ─── mkdir ──────────────────────────────────────────────────────────────────

type mkdirArgs struct {
	FilesystemID string `json:"filesystem_id"`
	Path         string `json:"path"`
}

type mkdirTool struct{ fs AgentFilesystem }

func (t *mkdirTool) Name() string        { return "create_directory" }
func (t *mkdirTool) Description() string { return "Create a directory in a virtual filesystem." }
func (t *mkdirTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: "object",
		Properties: map[string]*agent.Schema{
			"filesystem_id": {Type: "string", Description: "UUID of the filesystem"},
			"path":          {Type: "string", Description: "Directory path to create, e.g. \"/projects/new-folder\""},
		},
		Required: []string{"filesystem_id", "path"},
	}
}

func (t *mkdirTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var p mkdirArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid args: %w", err)), nil
	}

	fsID, userID, err := requirePermission(ctx, t.fs, args, p.FilesystemID)
	if err != nil {
		return agent.ErrorResult(err), nil
	}

	if err := t.fs.Mkdir(ctx, fsID, userID, p.Path); err != nil {
		return agent.ErrorResult(fmt.Errorf("creating directory: %w", err)), nil
	}

	return agent.JSONResult(map[string]any{"path": p.Path, "created": true}), nil
}

// ─── remove ─────────────────────────────────────────────────────────────────

type removeArgs struct {
	FilesystemID string `json:"filesystem_id"`
	Path         string `json:"path"`
}

type removeTool struct{ fs AgentFilesystem }

func (t *removeTool) Name() string { return "remove_entry" }
func (t *removeTool) Description() string {
	return "Delete a file or an empty directory from a virtual filesystem."
}

func (t *removeTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: "object",
		Properties: map[string]*agent.Schema{
			"filesystem_id": {Type: "string", Description: "UUID of the filesystem"},
			"path":          {Type: "string", Description: "Path of the file or empty directory to delete"},
		},
		Required: []string{"filesystem_id", "path"},
	}
}

func (t *removeTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var p removeArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid args: %w", err)), nil
	}

	fsID, userID, err := requirePermission(ctx, t.fs, args, p.FilesystemID)
	if err != nil {
		return agent.ErrorResult(err), nil
	}

	if err := t.fs.Remove(ctx, fsID, userID, p.Path); err != nil {
		return agent.ErrorResult(fmt.Errorf("removing entry: %w", err)), nil
	}

	return agent.JSONResult(map[string]any{"path": p.Path, "removed": true}), nil
}

// ─── stat ───────────────────────────────────────────────────────────────────

type statArgs struct {
	FilesystemID string `json:"filesystem_id"`
	Path         string `json:"path"`
}

type statTool struct{ fs AgentFilesystem }

func (t *statTool) Name() string { return "file_info" }
func (t *statTool) Description() string {
	return "Get metadata (size, type, modified time) for a file or directory."
}

func (t *statTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: "object",
		Properties: map[string]*agent.Schema{
			"filesystem_id": {Type: "string", Description: "UUID of the filesystem"},
			"path":          {Type: "string", Description: "Path of the file or directory"},
		},
		Required: []string{"filesystem_id", "path"},
	}
}

func (t *statTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var p statArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid args: %w", err)), nil
	}

	fsID, userID, err := requirePermission(ctx, t.fs, args, p.FilesystemID)
	if err != nil {
		return agent.ErrorResult(err), nil
	}

	stat, err := t.fs.Stat(ctx, fsID, userID, p.Path)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("getting file info: %w", err)), nil
	}

	return agent.JSONResult(map[string]any{
		"path":         p.Path,
		"name":         stat.Name,
		"is_dir":       stat.IsDir,
		"size":         stat.Size,
		"content_type": stat.ContentType,
		"mod_time":     stat.ModTime,
	}), nil
}

// ─── rename ─────────────────────────────────────────────────────────────────

type renameArgs struct {
	FilesystemID string `json:"filesystem_id"`
	OldPath      string `json:"old_path"`
	NewPath      string `json:"new_path"`
}

type renameTool struct{ fs AgentFilesystem }

func (t *renameTool) Name() string { return "rename_entry" }
func (t *renameTool) Description() string {
	return "Rename or move a file or directory within a virtual filesystem."
}

func (t *renameTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: "object",
		Properties: map[string]*agent.Schema{
			"filesystem_id": {Type: "string", Description: "UUID of the filesystem"},
			"old_path":      {Type: "string", Description: "Current path of the entry"},
			"new_path":      {Type: "string", Description: "New path for the entry"},
		},
		Required: []string{"filesystem_id", "old_path", "new_path"},
	}
}

func (t *renameTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var p renameArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid args: %w", err)), nil
	}

	fsID, userID, err := requirePermission(ctx, t.fs, args, p.FilesystemID)
	if err != nil {
		return agent.ErrorResult(err), nil
	}

	if err := t.fs.Rename(ctx, fsID, userID, p.OldPath, p.NewPath); err != nil {
		return agent.ErrorResult(fmt.Errorf("renaming entry: %w", err)), nil
	}

	return agent.JSONResult(map[string]any{
		"old_path": p.OldPath,
		"new_path": p.NewPath,
		"renamed":  true,
	}), nil
}
