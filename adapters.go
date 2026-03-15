package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/agentstore"
	"school-with-ai/modules/chat/chatcontracts"
	"school-with-ai/modules/filesystem"
	"school-with-ai/modules/knowledge/edgequake"
	"school-with-ai/modules/permissions"
	permworkspace "school-with-ai/modules/permissions/workspace"
	"school-with-ai/modules/userprofile/identity"
	"school-with-ai/plugins/fsplugin"
	"school-with-ai/shared"

	"github.com/google/uuid"
)

// adkgoProfileAdapter bridges identity.UserProfileRepository to
// adkgo.UserContextProvider (GetProfile → map[string]any).
type adkgoProfileAdapter struct {
	repo identity.UserProfileRepository
}

func (a *adkgoProfileAdapter) GetProfile(ctx context.Context, userID string) (map[string]any, error) {
	profile, err := a.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, nil
	}
	m := map[string]any{
		"user_id":        profile.UserID,
		"learning_style": profile.LearningStyle,
		"struggles":      profile.Struggles,
		"strengths":      profile.Strengths,
		"goals":          profile.Goals,
	}
	return m, nil
}

// DocumentProviderAdapter adapts edgequake.Service to chatcontracts.DocumentProvider
type DocumentProviderAdapter struct {
	svc *edgequake.Service
}

func (a *DocumentProviderAdapter) ListDocuments(ctx context.Context, tCtx *shared.TenantContext, page, pageSize int, status string) (any, error) {
	resp, err := a.svc.ListDocuments(ctx, tCtx, page, pageSize, status)
	if err != nil {
		if err == edgequake.ErrEdgequakeUnauthorized {
			return nil, chatcontracts.ErrKnowledgeUnauthorized
		}
		if err == edgequake.ErrEdgequakeForbidden {
			return nil, chatcontracts.ErrKnowledgeForbidden
		}
		return nil, err
	}
	return resp, nil
}

func (a *DocumentProviderAdapter) UploadDocument(ctx context.Context, tCtx *shared.TenantContext, fileName string, contentType string, content io.Reader) (any, error) {
	resp, err := a.svc.UploadDocument(ctx, tCtx, fileName, contentType, content)
	if err != nil {
		if err == edgequake.ErrEdgequakeUnauthorized {
			return nil, chatcontracts.ErrKnowledgeUnauthorized
		}
		if err == edgequake.ErrEdgequakeForbidden {
			return nil, chatcontracts.ErrKnowledgeForbidden
		}
		return nil, err
	}
	return resp, nil
}

// WorkspaceProviderAdapter adapts edgequake.Service to permissions.WorkspaceProvider
type WorkspaceProviderAdapter struct {
	svc *edgequake.Service
}

func (a *WorkspaceProviderAdapter) ListWorkspaces(ctx context.Context, tCtx *shared.TenantContext, page, pageSize int) (*permissions.WorkspaceList, error) {
	resp, err := a.svc.ListWorkspaces(ctx, tCtx, page, pageSize)
	if err != nil {
		return nil, err
	}

	workspaces := make([]permissions.Workspace, len(resp.Items))
	for i, w := range resp.Items {
		workspaces[i] = permissions.Workspace{
			ID:          w.ID,
			Name:        w.Name,
			Description: w.Description,
			Slug:        w.Slug,
			IsActive:    w.IsActive,
		}
	}

	return &permissions.WorkspaceList{Items: workspaces}, nil
}

// knowledgeBasePromptInjector implements adkgo.PromptInjector.
// It resolves which knowledge-base workspaces a user has enabled for the current
// agent and injects instructions into the system prompt so the LLM knows it can
// query those workspaces via the query_knowledge_base tool.
//
// This lives in the composition root — the adapter layer stays tool-agnostic.
type knowledgeBasePromptInjector struct {
	workspaceSvc *permworkspace.Service
}

func (k *knowledgeBasePromptInjector) BuildPromptAddition(ctx context.Context, agentName, _ string) string {
	user := shared.GetUserFromContext(ctx)
	if user == nil {
		return ""
	}
	enabledWorkspaceIDs, err := k.workspaceSvc.GetEnabledWorkspaces(ctx, user.ID, agentName)
	if err != nil || len(enabledWorkspaceIDs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Knowledge Bases\n\nYou have access to the following knowledge base workspaces. Use the query_knowledge_base tool to search them:\n")
	for _, wsID := range enabledWorkspaceIDs {
		fmt.Fprintf(&b, "- Workspace ID: %s\n", wsID)
	}
	b.WriteString("\nWhen the user asks about topics that may be covered by these knowledge bases, use the query_knowledge_base tool with the appropriate namespace (workspace ID) and a relevant query to retrieve information.")
	return b.String()
}

// ──────────────────────────────────────────────────────────────────────────────
// chatProfileAdapter bridges identity.UserProfileRepository to
// chatcontracts.UserProfileRepository so the chat module does not import
// modules/userprofile/identity directly.
// ──────────────────────────────────────────────────────────────────────────────

type chatProfileAdapter struct {
	repo identity.UserProfileRepository
}

func (a *chatProfileAdapter) GetByUserID(ctx context.Context, userID string) (*chatcontracts.UserProfile, error) {
	p, err := a.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &chatcontracts.UserProfile{
		UserID:        p.UserID,
		LearningStyle: p.LearningStyle,
		Struggles:     p.Struggles,
		Strengths:     p.Strengths,
		Goals:         p.Goals,
	}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// agentFSAdapter bridges modules/filesystem.AgentFS → fsplugin.AgentFilesystem.
// ──────────────────────────────────────────────────────────────────────────────

type agentFSAdapter struct {
	source filesystem.AgentFS
}

func (a *agentFSAdapter) ListPermittedFilesystems(ctx context.Context, userID, agentID string) ([]fsplugin.FileSystem, error) {
	fss, err := a.source.ListPermittedFilesystems(ctx, userID, agentID)
	if err != nil {
		return nil, err
	}
	out := make([]fsplugin.FileSystem, len(fss))
	for i, f := range fss {
		out[i] = fsplugin.FileSystem{
			ID:          f.ID,
			UserID:      f.UserID,
			Name:        f.Name,
			Description: f.Description,
			Type:        string(f.Type),
		}
	}
	return out, nil
}

func (a *agentFSAdapter) HasAgentPermission(ctx context.Context, fsID uuid.UUID, userID, agentID string) (bool, error) {
	return a.source.HasAgentPermission(ctx, fsID, userID, agentID)
}

func (a *agentFSAdapter) ListDir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) ([]fsplugin.DirEntry, error) {
	entries, err := a.source.ListDir(ctx, fsID, userID, dirPath)
	if err != nil {
		return nil, err
	}
	out := make([]fsplugin.DirEntry, len(entries))
	for i, e := range entries {
		out[i] = fsplugin.DirEntry{
			Name:        e.Name,
			IsDir:       e.IsDir,
			Size:        e.Size,
			ContentType: e.ContentType,
			ModTime:     e.ModTime,
		}
	}
	return out, nil
}

func (a *agentFSAdapter) Mkdir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) error {
	return a.source.Mkdir(ctx, fsID, userID, dirPath)
}

func (a *agentFSAdapter) ReadFile(ctx context.Context, fsID uuid.UUID, userID, filePath string) (io.ReadCloser, *fsplugin.FileStat, error) {
	rc, stat, err := a.source.ReadFile(ctx, fsID, userID, filePath)
	if err != nil {
		return nil, nil, err
	}
	return rc, mapFileStat(stat), nil
}

func (a *agentFSAdapter) WriteFile(ctx context.Context, fsID uuid.UUID, userID, filePath string, content io.Reader, contentType string, size int64) (*fsplugin.FileStat, error) {
	stat, err := a.source.WriteFile(ctx, fsID, userID, filePath, content, contentType, size)
	if err != nil {
		return nil, err
	}
	return mapFileStat(stat), nil
}

func (a *agentFSAdapter) Remove(ctx context.Context, fsID uuid.UUID, userID, entryPath string) error {
	return a.source.Remove(ctx, fsID, userID, entryPath)
}

func (a *agentFSAdapter) Stat(ctx context.Context, fsID uuid.UUID, userID, filePath string) (*fsplugin.FileStat, error) {
	stat, err := a.source.Stat(ctx, fsID, userID, filePath)
	if err != nil {
		return nil, err
	}
	return mapFileStat(stat), nil
}

func (a *agentFSAdapter) Rename(ctx context.Context, fsID uuid.UUID, userID, oldPath, newPath string) error {
	return a.source.Rename(ctx, fsID, userID, oldPath, newPath)
}

func mapFileStat(s *filesystem.FileStat) *fsplugin.FileStat {
	if s == nil {
		return nil
	}
	return &fsplugin.FileStat{
		Name:        s.Name,
		IsDir:       s.IsDir,
		Size:        s.Size,
		ContentType: s.ContentType,
		ModTime:     s.ModTime,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// userAwareAgentProvider implements agent.AgentProvider by loading all agents
// from the database. In the unified model, every agent — simple or composed —
// is an AgentDefinition. Composed agents (those with a graph) implement
// agent.WorkflowAgent so the ADK-Go adapter can detect them.
// ──────────────────────────────────────────────────────────────────────────────

type userAwareAgentProvider struct {
	repo                agentstore.AgentDefinitionRepository
	functionMiddlewares []agent.Middleware
}

func newUserAwareAgentProvider(
	repo agentstore.AgentDefinitionRepository,
	funcMw []agent.Middleware,
) *userAwareAgentProvider {
	return &userAwareAgentProvider{
		repo:                repo,
		functionMiddlewares: funcMw,
	}
}

func (p *userAwareAgentProvider) Get(name string) (agent.Agent, error) {
	return nil, fmt.Errorf("agent %q requires user context; use GetForUser", name)
}

func (p *userAwareAgentProvider) GetForUser(ctx context.Context, name, userID string) (agent.Agent, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required to resolve agent %q", name)
	}
	if name == "" {
		return nil, fmt.Errorf("an agent ID must be provided")
	}

	agentDef, err := p.repo.GetByID(ctx, name, userID)
	if err != nil {
		return nil, fmt.Errorf("agent %q not found for user %s: %w", name, userID, err)
	}
	return p.agentFromDefinition(agentDef), nil
}

func (p *userAwareAgentProvider) List() []agent.Agent {
	return []agent.Agent{}
}

func (p *userAwareAgentProvider) ListForUser(ctx context.Context, userID string) ([]agent.Agent, error) {
	if userID == "" {
		return nil, nil
	}

	definitions, err := p.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}
	agents := make([]agent.Agent, 0, len(definitions))
	for i := range definitions {
		agents = append(agents, p.agentFromDefinition(&definitions[i]))
	}
	return agents, nil
}

func (p *userAwareAgentProvider) agentFromDefinition(def *agentstore.AgentDefinition) agent.Agent {
	cfg := agent.Config{
		Name:                def.ID,
		Description:         def.Description,
		SystemPrompt:        def.SystemPrompt,
		Tools:               []agent.Tool{},
		Requirements:        []agent.Requirement{},
		FunctionMiddlewares: p.functionMiddlewares,
		TransferPolicy: agent.TransferPolicy{
			DisallowTransferToParent: def.DisallowTransferToParent,
			DisallowTransferToPeers:  def.DisallowTransferToPeers,
		},
		OutputKey:         def.OutputKey,
		IncludeContents:   agent.IncludeContents(def.IncludeContents),
		GlobalInstruction: def.GlobalInstruction,
	}
	a := newConfigAgent(cfg)

	// Composed agents (those with a graph) also implement WorkflowAgent.
	if def.IsComposed() {
		return &composedAgentAdapter{
			dbAgent: dbAgent{
				Agent:       a,
				displayName: def.Name,
				id:          def.ID,
				icon:        def.Icon,
			},
			graph: convertCompositionNode(def.Graph),
		}
	}

	return &dbAgent{
		Agent:       a,
		displayName: def.Name,
		id:          def.ID,
		icon:        def.Icon,
	}
}

// dbAgent wraps a configAgent to carry display metadata from the DB row.
type dbAgent struct {
	agent.Agent
	displayName string
	id          string
	icon        string
}

func (a *dbAgent) ID() agent.ID             { return a.id }
func (a *dbAgent) Name() string             { return a.displayName }
func (a *dbAgent) Description() string      { return a.Agent.Description() }
func (a *dbAgent) Icon() string             { return a.icon }
func (a *dbAgent) SubAgents() []agent.Agent { return a.Agent.SubAgents() }
func (a *dbAgent) Config() agent.Config     { return a.Agent.(agent.Configurable).Config() }

// ──────────────────────────────────────────────────────────────────────────────
// composedAgentAdapter extends dbAgent with the WorkflowAgent interface.
// In the unified model, a composed agent IS an agent with a composition graph.
// ──────────────────────────────────────────────────────────────────────────────

type composedAgentAdapter struct {
	dbAgent
	graph agent.WorkflowNodeInfo
}

func (a *composedAgentAdapter) WorkflowGraph() agent.WorkflowNodeInfo { return a.graph }

var _ agent.WorkflowAgent = (*composedAgentAdapter)(nil)

// convertCompositionNode recursively maps agentstore.CompositionNode to agent.WorkflowNodeInfo.
func convertCompositionNode(n *agentstore.CompositionNode) agent.WorkflowNodeInfo {
	if n == nil {
		return agent.WorkflowNodeInfo{}
	}
	info := agent.WorkflowNodeInfo{
		ID:                       n.ID,
		Type:                     string(n.Type),
		Label:                    n.Label,
		AgentID:                  n.AgentID,
		OutputKey:                n.OutputKey,
		MaxIterations:            n.MaxIterations,
		DisallowTransferToParent: n.DisallowTransferToParent,
		DisallowTransferToPeers:  n.DisallowTransferToPeers,
		IncludeContents:          n.IncludeContents,
	}
	if len(n.Children) > 0 {
		info.Children = make([]agent.WorkflowNodeInfo, len(n.Children))
		for i := range n.Children {
			info.Children[i] = convertCompositionNode(&n.Children[i])
		}
	}
	return info
}

// ──────────────────────────────────────────────────────────────────────────────
// configAgent is a concrete agent.Agent that holds an agent.Config.
// ──────────────────────────────────────────────────────────────────────────────

type configAgent struct {
	config agent.Config
}

func newConfigAgent(cfg agent.Config) agent.Agent {
	return &configAgent{config: cfg}
}

func (a *configAgent) ID() agent.ID             { return a.config.Name }
func (a *configAgent) Name() string             { return a.config.Name }
func (a *configAgent) Description() string      { return a.config.Description }
func (a *configAgent) SubAgents() []agent.Agent { return a.config.SubAgents }
func (a *configAgent) Config() agent.Config     { return a.config }

var _ agent.Configurable = (*configAgent)(nil)
