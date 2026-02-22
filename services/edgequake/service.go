package edgequake

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/rag/chunks"
	"school-with-ai/services/rag/costs"
	"school-with-ai/services/rag/documents"
	"school-with-ai/services/rag/entities"
	"school-with-ai/services/rag/folders"
	"school-with-ai/services/rag/graph"
	"school-with-ai/services/rag/lineage"
	"school-with-ai/services/rag/models"
	"school-with-ai/services/rag/pipeline"
	"school-with-ai/services/rag/provenance"
	"school-with-ai/services/rag/query"
	"school-with-ai/services/rag/relationships"
	"school-with-ai/services/rag/settings"
	"school-with-ai/services/rag/tasks"
	"school-with-ai/services/rag/tenants"
	"school-with-ai/services/rag/users"
	"school-with-ai/services/rag/workspaces"

	"github.com/google/uuid"
)

var (
	ErrEdgequakeUnauthorized = fmt.Errorf("edgequake: unauthorized")
	ErrEdgequakeForbidden    = fmt.Errorf("edgequake: access forbidden")
)

// Service provides access to the Edgequake REST API.
type Service struct {
	baseURL       string
	httpClient    *http.Client
	Documents     *documents.Service
	Query         *query.Service
	Workspaces    *workspaces.Service
	Tenants       *tenants.Service
	Folders       *folders.Service
	Users         *users.Service
	Models        *models.Service
	Pipeline      *pipeline.Service
	Graph         *graph.Service
	Entities      *entities.Service
	Relationships *relationships.Service
	Lineage       *lineage.Service
	Chunks        *chunks.Service
	Costs         *costs.Service
	Provenance    *provenance.Service
	Settings      *settings.Service
	Tasks         *tasks.Service
}

// NewService creates a new Edgequake service.
func NewService(baseURL string) *Service {
	httpClient := &http.Client{
		Timeout: 120 * time.Second, // Extended timeout to handle LLM cold starts (e.g. Ollama model loading)
	}
	return &Service{
		baseURL:       baseURL,
		httpClient:    httpClient,
		Documents:     documents.NewService(baseURL, httpClient),
		Query:         query.NewService(baseURL, httpClient),
		Workspaces:    workspaces.NewService(baseURL, httpClient),
		Tenants:       tenants.NewService(baseURL, httpClient),
		Folders:       folders.NewService(baseURL, httpClient),
		Users:         users.NewService(baseURL, httpClient),
		Models:        models.NewService(baseURL, httpClient),
		Pipeline:      pipeline.NewService(baseURL, httpClient),
		Graph:         graph.NewService(baseURL, httpClient),
		Entities:      entities.NewService(baseURL, httpClient),
		Relationships: relationships.NewService(baseURL, httpClient),
		Lineage:       lineage.NewService(baseURL, httpClient),
		Chunks:        chunks.NewService(baseURL, httpClient),
		Costs:         costs.NewService(baseURL, httpClient),
		Provenance:    provenance.NewService(baseURL, httpClient),
		Settings:      settings.NewService(baseURL, httpClient),
		Tasks:         tasks.NewService(baseURL, httpClient),
	}
}

// ListDocuments retrieves a paginated list of documents from Edgequake.
func (s *Service) ListDocuments(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int, status string) (*documents.ListDocumentsResponse, error) {
	return s.Documents.List(ctx, tCtx, page, pageSize, status)
}

// GetDocument retrieves a single document by ID.
func (s *Service) GetDocument(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*documents.DocumentDetail, error) {
	return s.Documents.Get(ctx, tCtx, documentID)
}

// UploadDocument uploads a document to Edgequake.
func (s *Service) UploadDocument(ctx context.Context, tCtx *domain.TenantContext, fileName string, contentType string, content io.Reader) (*documents.FileUploadResponse, error) {
	return s.Documents.Upload(ctx, tCtx, fileName, contentType, content)
}

// ListPDFs retrieves a paginated list of PDFs from Edgequake.
func (s *Service) ListPDFs(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int, status string, search string) (*documents.ListPdfsResponse, error) {
	return s.Documents.ListPDFs(ctx, tCtx, page, pageSize, status, search)
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

// DownloadPDF retrieves the raw PDF binary from Edgequake.
func (s *Service) DownloadPDF(ctx context.Context, tCtx *domain.TenantContext, pdfID string) (io.ReadCloser, string, int64, error) {
	return s.Documents.DownloadPDF(ctx, tCtx, pdfID)
}

// ReprocessDocument triggers reprocessing for a single document.
func (s *Service) ReprocessDocument(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*documents.ReprocessResponse, error) {
	return s.Documents.Reprocess(ctx, tCtx, documentID)
}

// DeleteDocument deletes an ingested document.
func (s *Service) DeleteDocument(ctx context.Context, tCtx *domain.TenantContext, documentID string) error {
	return s.Documents.Delete(ctx, tCtx, documentID)
}

// ReprocessFailedDocuments triggers reprocessing for all failed documents.
func (s *Service) ReprocessFailedDocuments(ctx context.Context, tCtx *domain.TenantContext) (*documents.ReprocessFailedResponse, error) {
	return s.Documents.ReprocessFailed(ctx, tCtx)
}

// ScanDocuments triggers scanning of the input directory for new documents.
func (s *Service) ScanDocuments(ctx context.Context, tCtx *domain.TenantContext, path string) (*documents.ScanDocumentsResponse, error) {
	return s.Documents.ScanDocuments(ctx, tCtx, documents.ScanDocumentsRequest{
		Path:            path,
		Recursive:       true,
		MaxFiles:        1000,
		AsyncProcessing: true,
	})
}

// DeleteAllDocuments deletes all documents from the workspace.
func (s *Service) DeleteAllDocuments(ctx context.Context, tCtx *domain.TenantContext) (*documents.DeleteAllDocumentsResponse, error) {
	return s.Documents.DeleteAll(ctx, tCtx)
}

// RetryTask retries a failed pipeline task.
func (s *Service) RetryTask(ctx context.Context, tCtx *domain.TenantContext, taskID string) (*documents.TaskRetryResponse, error) {
	return s.Documents.RetryTask(ctx, tCtx, taskID)
}

// RetryPdfProcessing retries a failed PDF processing.
func (s *Service) RetryPdfProcessing(ctx context.Context, tCtx *domain.TenantContext, pdfID string) (*documents.PdfRetryResponse, error) {
	return s.Documents.RetryPdfProcessing(ctx, tCtx, pdfID)
}

// GetEnhancedPipelineStatus retrieves the enhanced pipeline status.
func (s *Service) GetEnhancedPipelineStatus(ctx context.Context, tCtx *domain.TenantContext) (*pipeline.EnhancedPipelineStatusResponse, error) {
	return s.Pipeline.Status(ctx, tCtx)
}

// GetQueueMetrics retrieves queue metrics.
func (s *Service) GetQueueMetrics(ctx context.Context, tCtx *domain.TenantContext) (*pipeline.QueueMetricsResponse, error) {
	return s.Pipeline.Metrics(ctx, tCtx)
}

// CancelPipeline requests cancellation of the entire pipeline.
func (s *Service) CancelPipeline(ctx context.Context, tCtx *domain.TenantContext) (*pipeline.CancelPipelineResponse, error) {
	return s.Pipeline.Cancel(ctx, tCtx)
}

// GetPipelineTasks retrieves the current tasks in the pipeline.
func (s *Service) GetPipelineTasks(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int) (*tasks.TaskListResponse, error) {
	return s.Documents.GetPipelineTasks(ctx, tCtx, page, pageSize)
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

// RebuildKnowledgeGraph clears the knowledge graph and triggers a rebuild.
func (s *Service) RebuildKnowledgeGraph(ctx context.Context, tCtx *domain.TenantContext, workspaceID uuid.UUID, req *workspaces.RebuildKnowledgeGraphRequest) (*workspaces.RebuildKnowledgeGraphResponse, error) {
	return s.Workspaces.RebuildGraph(ctx, tCtx, workspaceID, req)
}

// RebuildEmbeddings clears all embeddings and triggers a re-indexing.
func (s *Service) RebuildEmbeddings(ctx context.Context, tCtx *domain.TenantContext, workspaceID uuid.UUID, req *workspaces.RebuildEmbeddingsRequest) (*workspaces.RebuildEmbeddingsResponse, error) {
	return s.Workspaces.RebuildEmbeddings(ctx, tCtx, workspaceID, req)
}

// ReprocessAllDocuments triggers reprocessing for all documents in the workspace.
func (s *Service) ReprocessAllDocuments(ctx context.Context, tCtx *domain.TenantContext, workspaceID uuid.UUID) error {
	req := &workspaces.ReprocessAllRequest{
		IncludeCompleted: true,
		MaxDocuments:     1000,
	}
	_, err := s.Workspaces.ReprocessAllDocuments(ctx, tCtx, workspaceID, req)
	return err
}

// ListTenants retrieves all tenants from Edgequake.
func (s *Service) ListTenants(ctx context.Context, offset, limit int) (*tenants.TenantListResponse, error) {
	return s.Tenants.List(ctx, offset, limit)
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

// CancelPipelineTask cancels a document processing task.
func (s *Service) CancelPipelineTask(ctx context.Context, tCtx *domain.TenantContext, taskID string) error {
	return s.Documents.CancelTask(ctx, tCtx, taskID)
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

// ListLLMModels retrieving available LLM models
func (s *Service) ListLLMModels(ctx context.Context) (*models.LlmModelsResponse, error) {
	return s.Models.ListLLM(ctx)
}

// ListEmbeddingModels retrieving available embedding models
func (s *Service) ListEmbeddingModels(ctx context.Context) (*models.EmbeddingModelsResponse, error) {
	return s.Models.ListEmbedding(ctx)
}

// GetGraph retrieves the knowledge graph.
func (s *Service) GetGraph(ctx context.Context, tCtx *domain.TenantContext, params *graph.GraphQueryParams) (*graph.KnowledgeGraphResponse, error) {
	tc := *tCtx
	return s.Graph.GetGraph(ctx, tc, params)
}

// GetGraphNode retrieves a specific graph node by ID.
func (s *Service) GetGraphNode(ctx context.Context, tCtx *domain.TenantContext, nodeID string) (*graph.GraphNodeResponse, error) {
	tc := *tCtx
	return s.Graph.GetNode(ctx, tc, nodeID)
}

// SearchGraphNodes searches for nodes matching a query.
func (s *Service) SearchGraphNodes(ctx context.Context, tCtx *domain.TenantContext, q string, limit int) (*graph.SearchNodesResponse, error) {
	tc := *tCtx
	return s.Graph.SearchNodes(ctx, tc, q, limit)
}

// GetPopularLabels retrieves popular graph labels.
func (s *Service) GetPopularLabels(ctx context.Context, tCtx *domain.TenantContext, limit int) (*graph.PopularLabelsResponse, error) {
	tc := *tCtx
	return s.Graph.GetPopularLabels(ctx, tc, limit)
}

// SearchGraphLabels searches for graph labels matching a query.
func (s *Service) SearchGraphLabels(ctx context.Context, tCtx *domain.TenantContext, q string, limit int) (*graph.SearchLabelsResponse, error) {
	tc := *tCtx
	return s.Graph.SearchLabels(ctx, tc, q, limit)
}
