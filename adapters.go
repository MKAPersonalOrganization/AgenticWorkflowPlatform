package main

import (
	"aicompass/adapters/adkgo"
	"aicompass/modules/a2a"
	"aicompass/modules/chat/chatcontracts"
	"aicompass/modules/knowledge/edgequake"
	"aicompass/modules/knowledge/tenant"
	"aicompass/modules/permissions"
	"aicompass/modules/userprofile/identity"
	"context"
	"io"
	"iter"
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
	return map[string]any{
		"user_id":        profile.UserID,
		"learning_style": profile.LearningStyle,
		"struggles":      profile.Struggles,
		"strengths":      profile.Strengths,
		"goals":          profile.Goals,
	}, nil
}

// DocumentProviderAdapter adapts edgequake.Service to chatcontracts.DocumentProvider.
type DocumentProviderAdapter struct {
	svc *edgequake.Service
}

func (a *DocumentProviderAdapter) ListDocuments(ctx context.Context, tCtx *tenant.TenantContext, page, pageSize int, status string) (any, error) {
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

func (a *DocumentProviderAdapter) UploadDocument(ctx context.Context, tCtx *tenant.TenantContext, fileName string, contentType string, content io.Reader) (any, error) {
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

// WorkspaceProviderAdapter adapts edgequake.Service to permissions.WorkspaceProvider.
type WorkspaceProviderAdapter struct {
	svc *edgequake.Service
}

func (a *WorkspaceProviderAdapter) ListWorkspaces(ctx context.Context, tCtx *tenant.TenantContext, page, pageSize int) (*permissions.WorkspaceList, error) {
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

// chatProfileAdapter bridges identity.UserProfileRepository to
// chatcontracts.UserProfileRepository so the chat module does not import
// modules/userprofile/identity directly.
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

// a2aSubAgentAdapter bridges a2a.Service to adkgo.A2ASubAgentProvider.
// The two packages use structurally identical but distinct types for
// ConnectorInfo, so a thin adapter is needed at the composition root.
type a2aSubAgentAdapter struct {
	svc *a2a.Service
}

func (a *a2aSubAgentAdapter) GetConnectorInfo(ctx context.Context, connectorID, userID string) (adkgo.A2AConnectorInfo, error) {
	info, err := a.svc.GetConnectorInfo(ctx, connectorID, userID)
	if err != nil {
		return adkgo.A2AConnectorInfo{}, err
	}
	return adkgo.A2AConnectorInfo{
		ID:          info.ID,
		Name:        info.Name,
		Description: info.Description,
		AgentURL:    info.AgentURL,
	}, nil
}

func (a *a2aSubAgentAdapter) SendMessage(ctx context.Context, connectorID, userID, message string) (string, error) {
	return a.svc.SendMessage(ctx, connectorID, userID, message)
}

func (a *a2aSubAgentAdapter) SendStreamingMessage(ctx context.Context, connectorID, userID, message string) iter.Seq2[adkgo.A2AStreamChunk, error] {
	return func(yield func(adkgo.A2AStreamChunk, error) bool) {
		for chunk, err := range a.svc.SendStreamingMessage(ctx, connectorID, userID, message) {
			if err != nil {
				yield(adkgo.A2AStreamChunk{}, err)
				return
			}
			if !yield(adkgo.A2AStreamChunk{
				Delta:      chunk.Delta,
				IsArtifact: chunk.IsArtifact,
				ArtifactID: chunk.ArtifactID,
				Title:      chunk.Title,
				Content:    chunk.Content,
				IsFile:     chunk.IsFile,
				FileURL:    chunk.FileURL,
				FileName:   chunk.FileName,
				MimeType:   chunk.MimeType,
			}, nil) {
				return
			}
		}
	}
}

var _ adkgo.A2ASubAgentProvider = (*a2aSubAgentAdapter)(nil)
