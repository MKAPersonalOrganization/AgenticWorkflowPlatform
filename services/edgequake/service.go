package edgequake

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/rag/documents"
	"school-with-ai/services/rag/folders"
	"school-with-ai/services/rag/query"
	"school-with-ai/services/rag/tenants"
	"school-with-ai/services/rag/workspaces"

	"github.com/google/uuid"
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
	Workspaces *workspaces.Service
	Tenants    *tenants.Service
	Folders    *folders.Service
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
		Workspaces: workspaces.NewService(baseURL, httpClient),
		Tenants:    tenants.NewService(baseURL, httpClient),
		Folders:    folders.NewService(baseURL, httpClient),
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

// ListWorkspaces retrieves a list of workspaces for a specific tenant.
func (s *Service) ListWorkspaces(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int) (*workspaces.WorkspaceListResponse, error) {
	return s.Workspaces.List(ctx, tCtx, page, pageSize)
}

// CreateWorkspace creates a new workspace for a specific tenant.
func (s *Service) CreateWorkspace(ctx context.Context, tCtx *domain.TenantContext, req *workspaces.CreateWorkspaceRequest) (*workspaces.WorkspaceResponse, error) {
	return s.Workspaces.Create(ctx, tCtx, req)
}

// GetWorkspace retrieves a specific workspace by its ID.
func (s *Service) GetWorkspace(ctx context.Context, tCtx *domain.TenantContext, workspaceID uuid.UUID) (*workspaces.WorkspaceResponse, error) {
	return s.Workspaces.Get(ctx, tCtx, workspaceID)
}

// DeleteWorkspace removes a specific workspace and its associated data.
func (s *Service) DeleteWorkspace(ctx context.Context, tCtx *domain.TenantContext, workspaceID uuid.UUID) error {
	return s.Workspaces.Delete(ctx, tCtx, workspaceID)
}

// GetWorkspaceStats retrieves statistics for a specific workspace.
func (s *Service) GetWorkspaceStats(ctx context.Context, tCtx *domain.TenantContext, workspaceID uuid.UUID) (*workspaces.WorkspaceStatsResponse, error) {
	return s.Workspaces.GetStats(ctx, tCtx, workspaceID)
}

// UpdateWorkspace modifies an existing workspace's fields.
func (s *Service) UpdateWorkspace(ctx context.Context, tCtx *domain.TenantContext, workspaceID uuid.UUID, req *workspaces.UpdateWorkspaceRequest) (*workspaces.WorkspaceResponse, error) {
	return s.Workspaces.Update(ctx, tCtx, workspaceID, req)
}

// ListTenants retrieves all tenants from Edgequake.
func (s *Service) ListTenants(ctx context.Context, page, pageSize int) (*tenants.TenantListResponse, error) {
	return s.Tenants.List(ctx, page, pageSize)
}

// CreateTenant creates a new tenant.
func (s *Service) CreateTenant(ctx context.Context, req *tenants.CreateTenantRequest) (*tenants.TenantResponse, error) {
	return s.Tenants.Create(ctx, req)
}

// GetTenant retrieves a specific tenant by ID.
func (s *Service) GetTenant(ctx context.Context, tenantID uuid.UUID) (*tenants.TenantResponse, error) {
	return s.Tenants.Get(ctx, tenantID)
}

// UpdateTenant updates an existing tenant.
func (s *Service) UpdateTenant(ctx context.Context, tenantID uuid.UUID, req *tenants.UpdateTenantRequest) (*tenants.TenantResponse, error) {
	return s.Tenants.Update(ctx, tenantID, req)
}

// DeleteTenant deletes a tenant.
func (s *Service) DeleteTenant(ctx context.Context, tenantID uuid.UUID) error {
	return s.Tenants.Delete(ctx, tenantID)
}

// ListFolders lists all folders.
func (s *Service) ListFolders(ctx context.Context, tCtx *domain.TenantContext) ([]folders.FolderResponse, error) {
	return s.Folders.List(ctx, tCtx)
}

// CreateFolder creates a new folder.
func (s *Service) CreateFolder(ctx context.Context, tCtx *domain.TenantContext, req *folders.CreateFolderRequest) (*folders.FolderResponse, error) {
	return s.Folders.Create(ctx, tCtx, req)
}

// DeleteFolder deletes a folder.
func (s *Service) DeleteFolder(ctx context.Context, tCtx *domain.TenantContext, folderID uuid.UUID) error {
	return s.Folders.Delete(ctx, tCtx, folderID)
}

// UpdateFolder updates an existing folder.
func (s *Service) UpdateFolder(ctx context.Context, tCtx *domain.TenantContext, folderID uuid.UUID, req *folders.UpdateFolderRequest) (*folders.FolderResponse, error) {
	return s.Folders.Update(ctx, tCtx, folderID, req)
}
