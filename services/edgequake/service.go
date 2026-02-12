package edgequake

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/rag/documents"
	"school-with-ai/services/rag/query"
)

var (
	ErrEdgequakeUnauthorized = fmt.Errorf("edgequake: unauthorized")
	ErrEdgequakeForbidden    = fmt.Errorf("edgequake: access forbidden")
)

// Service provides access to the Edgequake REST API.
type Service struct {
	baseURL    string
	httpClient *http.Client
	Documents  *documents.Service
	Query      *query.Service
}

// NewService creates a new Edgequake service.
func NewService(baseURL string) *Service {
	httpClient := &http.Client{
		Timeout: 30 * time.Second, // Increased timeout for large uploads
	}
	return &Service{
		baseURL:    baseURL,
		httpClient: httpClient,
		Documents:  documents.NewService(baseURL, httpClient),
		Query:      query.NewService(baseURL, httpClient),
	}
}

// ListDocuments retrieves a paginated list of documents from Edgequake.
func (s *Service) ListDocuments(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int) (*documents.ListDocumentsResponse, error) {
	return s.Documents.List(ctx, tCtx, page, pageSize)
}

// UploadDocument uploads a document to Edgequake.
func (s *Service) UploadDocument(ctx context.Context, tCtx *domain.TenantContext, fileName string, contentType string, content io.Reader) (*documents.FileUploadResponse, error) {
	return s.Documents.Upload(ctx, tCtx, fileName, contentType, content)
}

// ListPDFs retrieves a paginated list of PDFs from Edgequake.
func (s *Service) ListPDFs(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int, status string) (*documents.ListPdfsResponse, error) {
	return s.Documents.ListPDFs(ctx, tCtx, page, pageSize, status)
}

// UploadPDF uploads a PDF document to Edgequake.
func (s *Service) UploadPDF(ctx context.Context, tCtx *domain.TenantContext, fileName string, content io.Reader) (*documents.PdfUploadResponse, error) {
	return s.Documents.UploadPDF(ctx, tCtx, fileName, content)
}

// DeletePDF deletes a PDF document from Edgequake.
func (s *Service) DeletePDF(ctx context.Context, tCtx *domain.TenantContext, pdfID string) error {
	return s.Documents.DeletePDF(ctx, tCtx, pdfID)
}

// GetPDFProgress retrieves the progress of a PDF upload by track ID.
func (s *Service) GetPDFProgress(ctx context.Context, tCtx *domain.TenantContext, trackID string) (*documents.PdfUploadProgress, error) {
	return s.Documents.GetPDFProgress(ctx, tCtx, trackID)
}

// GetPDFStatus retrieves the status of a specific PDF.
func (s *Service) GetPDFStatus(ctx context.Context, tCtx *domain.TenantContext, pdfID string) (*documents.PdfStatusResponse, error) {
	return s.Documents.GetPDFStatus(ctx, tCtx, pdfID)
}

// GetPDFContent retrieves the content of a specific PDF.
func (s *Service) GetPDFContent(ctx context.Context, tCtx *domain.TenantContext, pdfID string) (*documents.PdfContentResponse, error) {
	return s.Documents.GetPDFContent(ctx, tCtx, pdfID)
}

// ExecuteQuery performs a standard RAG query.
func (s *Service) ExecuteQuery(ctx context.Context, tCtx *domain.TenantContext, req *query.QueryRequest) (*query.QueryResponse, error) {
	return s.Query.Execute(ctx, tCtx, req)
}

// StreamQuery performs a streaming RAG query.
func (s *Service) StreamQuery(ctx context.Context, tCtx *domain.TenantContext, req *query.StreamQueryRequest) (<-chan string, <-chan error) {
	return s.Query.Stream(ctx, tCtx, req)
}
