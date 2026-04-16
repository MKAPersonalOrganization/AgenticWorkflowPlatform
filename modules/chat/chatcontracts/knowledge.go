package chatcontracts

import (
	"aicompass/modules/knowledge/tenant"
	"context"
	"errors"
	"io"
)

var (
	ErrKnowledgeUnauthorized = errors.New("unauthorized by knowledge provider")
	ErrKnowledgeForbidden    = errors.New("access forbidden by knowledge provider")
)

// DocumentProvider abstracts the knowledge/document module.
type DocumentProvider interface {
	ListDocuments(ctx context.Context, tCtx *tenant.TenantContext, page, pageSize int, status string) (any, error)
	UploadDocument(ctx context.Context, tCtx *tenant.TenantContext, fileName string, contentType string, content io.Reader) (any, error)
}
