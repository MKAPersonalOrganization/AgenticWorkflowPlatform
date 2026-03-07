package main

import (
	"context"
	"io"

	"school-with-ai/modules/chat/chatcontracts"
	"school-with-ai/modules/knowledge/edgequake"
	"school-with-ai/modules/permissions"
	"school-with-ai/shared"
)

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
