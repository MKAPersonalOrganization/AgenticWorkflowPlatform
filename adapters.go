package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"school-with-ai/modules/chat/chatcontracts"
	"school-with-ai/modules/knowledge/edgequake"
	"school-with-ai/modules/permissions"
	permworkspace "school-with-ai/modules/permissions/workspace"
	"school-with-ai/modules/userprofile/identity"
	"school-with-ai/shared"
)

// adkgoProfileAdapter bridges identity.StudentProfileRepository to
// adkgo.UserContextProvider (GetProfile → map[string]any).
type adkgoProfileAdapter struct {
	repo identity.StudentProfileRepository
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
