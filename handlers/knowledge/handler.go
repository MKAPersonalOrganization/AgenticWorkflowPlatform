package knowledge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"school-with-ai/components/knowledge"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/edgequake"
	"school-with-ai/services/rag/documents"
	"school-with-ai/services/rag/graph"
	"school-with-ai/services/rag/workspaces"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for the RAG Knowledge Base dashboard.
type Handler struct {
	log              *logger.Logger
	edgequakeService *edgequake.Service
	tenantID         string
}

// NewHandler creates a new Knowledge Base handler.
func NewHandler(log *logger.Logger, edgequakeService *edgequake.Service, tenantID string) *Handler {
	return &Handler{
		log:              log,
		edgequakeService: edgequakeService,
		tenantID:         tenantID,
	}
}

// resolveTenantContext resolves the tenant context for the authenticated user.
func (h *Handler) resolveTenantContext(r *http.Request) (*domain.TenantContext, error) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		return nil, fmt.Errorf("no authenticated user")
	}

	// 1. Static Configuration (Required)
	if h.tenantID == "" {
		return nil, fmt.Errorf("tenant ID not configured")
	}

	// Use configured tenant ID
	tID, err := uuid.Parse(h.tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid configured tenant ID: %w", err)
	}
	return &domain.TenantContext{
		TenantID: &tID,
		UserID:   user.ID,
		Role:     string(domain.RoleAdmin), // Assume admin for static tenant
	}, nil
}

// WorkspacesPage renders the workspaces listing page.
func (h *Handler) WorkspacesPage(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())

	component := knowledge.WorkspacesPage(knowledge.WorkspacesPageProps{
		User: user,
	})
	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render workspaces page", "error", err)
	}
}

// WorkspaceDetailsPage renders the workspace details and document list.
func (h *Handler) WorkspaceDetailsPage(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		h.log.Error(r.Context(), "failed to resolve tenant context", "error", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	wsID := chi.URLParam(r, "workspaceId")
	workspaceID, err := uuid.Parse(wsID)
	if err != nil {
		http.Error(w, "Invalid workspace ID", http.StatusBadRequest)
		return
	}

	// Fetch workspace details
	workspace, err := h.edgequakeService.GetWorkspace(r.Context(), tCtx, workspaceID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get workspace details", "error", err, "workspaceId", workspaceID)
		http.Error(w, "Failed to load workspace", http.StatusInternalServerError)
		return
	}

	component := knowledge.WorkspaceDetailsPage(knowledge.WorkspaceDetailsPageProps{
		User:      user,
		Workspace: workspace,
	})
	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render workspace details page", "error", err)
	}
}

// DashboardPage renders the knowledge dashboard page.
func (h *Handler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())

	component := knowledge.DashboardPage(knowledge.DashboardPageProps{
		User: user,
	})
	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render dashboard page", "error", err)
	}
}

// ListDocuments returns documents for the current workspace as JSON.
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		h.log.Error(r.Context(), "failed to resolve tenant context", "error", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 20
	}
	status := r.URL.Query().Get("status")

	docs, err := h.edgequakeService.ListDocuments(r.Context(), tCtx, page, pageSize, status)
	if err != nil {
		h.log.Error(r.Context(), "failed to list documents", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load documents"})
		return
	}

	writeJSON(w, http.StatusOK, docs)
}

// GetDocument retrieves a single document by ID.
func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	docID := chi.URLParam(r, "documentId")
	if docID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing document ID"})
		return
	}

	doc, err := h.edgequakeService.GetDocument(r.Context(), tCtx, docID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get document", "error", err, "documentId", docID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load document"})
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

// UploadDocument handles document upload.
func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		h.log.Error(r.Context(), "failed to resolve tenant context", "error", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to parse form"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No file provided"})
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var result interface{}
	var uploadErr error

	if strings.HasSuffix(strings.ToLower(header.Filename), ".pdf") {
		result, uploadErr = h.edgequakeService.UploadPDF(r.Context(), tCtx, header.Filename, file)
	} else {
		result, uploadErr = h.edgequakeService.UploadDocument(r.Context(), tCtx, header.Filename, contentType, file)
	}

	if uploadErr != nil {
		h.log.Error(r.Context(), "failed to upload document", "error", uploadErr, "filename", header.Filename)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to upload document"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UploadPDF handles PDF upload.
func (h *Handler) UploadPDF(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to parse form"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No file provided"})
		return
	}
	defer file.Close()

	result, err := h.edgequakeService.UploadPDF(r.Context(), tCtx, header.Filename, file)
	if err != nil {
		h.log.Error(r.Context(), "failed to upload PDF", "error", err, "filename", header.Filename)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to upload PDF"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeletePDF deletes a PDF document.
func (h *Handler) DeletePDF(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	pdfID := chi.URLParam(r, "pdfId")
	if pdfID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing PDF ID"})
		return
	}

	if err := h.edgequakeService.DeletePDF(r.Context(), tCtx, pdfID); err != nil {
		h.log.Error(r.Context(), "failed to delete PDF", "error", err, "pdfId", pdfID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete PDF"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListPDFs returns PDFs for the current workspace.
func (h *Handler) ListPDFs(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 20
	}
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	pdfs, err := h.edgequakeService.ListPDFs(r.Context(), tCtx, page, pageSize, status, search)
	if err != nil {
		h.log.Error(r.Context(), "failed to list PDFs", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load PDFs"})
		return
	}

	writeJSON(w, http.StatusOK, pdfs)
}

// GetPDFProgress retrieves the progress of a PDF upload.
func (h *Handler) GetPDFProgress(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	trackID := chi.URLParam(r, "trackId")
	if trackID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing track ID"})
		return
	}

	progress, err := h.edgequakeService.GetPDFProgress(r.Context(), tCtx, trackID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get PDF progress", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get progress"})
		return
	}

	writeJSON(w, http.StatusOK, progress)
}

// GetPDFContent retrieves the content of a specific PDF.
func (h *Handler) GetPDFContent(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pdfID := chi.URLParam(r, "pdfId")
	if pdfID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing PDF ID"})
		return
	}

	content, err := h.edgequakeService.GetPDFContent(r.Context(), tCtx, pdfID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get PDF content", "error", err, "pdfId", pdfID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load PDF content"})
		return
	}

	writeJSON(w, http.StatusOK, content)
}

// DownloadPDF proxies the PDF binary download from Edgequake.
func (h *Handler) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	pdfID := chi.URLParam(r, "pdfId")
	if pdfID == "" {
		http.Error(w, "Missing PDF ID", http.StatusBadRequest)
		return
	}

	body, contentType, contentLength, err := h.edgequakeService.DownloadPDF(r.Context(), tCtx, pdfID)
	if err != nil {
		h.log.Error(r.Context(), "failed to download PDF", "error", err, "pdfId", pdfID)
		http.Error(w, "Failed to download PDF", http.StatusInternalServerError)
		return
	}
	defer body.Close()

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/pdf")
	}

	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}

	w.Header().Set("Content-Disposition", "inline; filename=\"document.pdf\"")
	io.Copy(w, body)
}

// ListWorkspaces returns workspaces for the user's tenant.
func (h *Handler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 50
	}

	wsList, err := h.edgequakeService.ListWorkspaces(r.Context(), tCtx, page, pageSize)
	if err != nil {
		h.log.Error(r.Context(), "failed to list workspaces", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load workspaces"})
		return
	}

	writeJSON(w, http.StatusOK, wsList)
}

// GetWorkspaceStats returns statistics for a specific workspace.
func (h *Handler) GetWorkspaceStats(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	wsID := chi.URLParam(r, "workspaceId")
	workspaceID, err := uuid.Parse(wsID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid workspace ID"})
		return
	}

	stats, err := h.edgequakeService.GetWorkspaceStats(r.Context(), tCtx, workspaceID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get workspace stats", "error", err, "workspaceId", wsID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load workspace stats"})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// CreateWorkspace creates a new workspace.
func (h *Handler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Name               string `json:"name"`
		Description        string `json:"description"`
		Slug               string `json:"slug"`
		LlmModel           string `json:"llm_model"`
		LlmProvider        string `json:"llm_provider"`
		EmbeddingModel     string `json:"embedding_model"`
		EmbeddingProvider  string `json:"embedding_provider"`
		EmbeddingDimension int    `json:"embedding_dimension"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Name is required"})
		return
	}

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	var slug *string
	if req.Slug != "" {
		slug = &req.Slug
	}

	var llmModel *string
	if req.LlmModel != "" {
		llmModel = &req.LlmModel
	}

	var llmProvider *string
	if req.LlmProvider != "" {
		llmProvider = &req.LlmProvider
	}

	var embeddingModel *string
	if req.EmbeddingModel != "" {
		embeddingModel = &req.EmbeddingModel
	}

	var embeddingProvider *string
	if req.EmbeddingProvider != "" {
		embeddingProvider = &req.EmbeddingProvider
	}

	var embeddingDimension *int
	if req.EmbeddingDimension > 0 {
		embeddingDimension = &req.EmbeddingDimension
	}

	ws, err := h.edgequakeService.CreateWorkspace(r.Context(), tCtx, &workspaces.CreateWorkspaceRequest{
		Name:               req.Name,
		Description:        desc,
		Slug:               slug,
		LlmModel:           llmModel,
		LlmProvider:        llmProvider,
		EmbeddingModel:     embeddingModel,
		EmbeddingProvider:  embeddingProvider,
		EmbeddingDimension: embeddingDimension,
	})
	if err != nil {
		h.log.Error(r.Context(), "failed to create workspace", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create workspace"})
		return
	}

	writeJSON(w, http.StatusCreated, ws)
}

// DeleteWorkspace deletes a workspace.
func (h *Handler) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	wsID := chi.URLParam(r, "workspaceId")
	workspaceID, err := uuid.Parse(wsID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid workspace ID"})
		return
	}

	if err := h.edgequakeService.DeleteWorkspace(r.Context(), tCtx, workspaceID); err != nil {
		h.log.Error(r.Context(), "failed to delete workspace", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete workspace"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// DeleteDocument deletes an ingested document.
func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	docID := chi.URLParam(r, "documentId")
	if docID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing document ID"})
		return
	}

	if err := h.edgequakeService.DeleteDocument(r.Context(), tCtx, docID); err != nil {
		h.log.Error(r.Context(), "failed to delete document", "error", err, "documentId", docID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete document"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ReprocessDocument triggers reprocessing for a single document.
func (h *Handler) ReprocessDocument(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	result, err := h.edgequakeService.ReprocessDocument(r.Context(), tCtx, req.DocumentID)
	if err != nil {
		h.log.Error(r.Context(), "failed to reprocess document", "error", err, "documentId", req.DocumentID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to reprocess document"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ReprocessFailed triggers reprocessing for all failed documents.
func (h *Handler) ReprocessFailed(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	result, err := h.edgequakeService.ReprocessFailedDocuments(r.Context(), tCtx)
	if err != nil {
		h.log.Error(r.Context(), "failed to reprocess failed documents", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to reprocess failed documents"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ScanDocuments triggers scanning of an input directory for new documents.
func (h *Handler) ScanDocuments(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Path == "" {
		req.Path = "./inputs"
	}

	result, err := h.edgequakeService.ScanDocuments(r.Context(), tCtx, req.Path)
	if err != nil {
		h.log.Error(r.Context(), "failed to scan documents", "error", err, "path", req.Path)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to scan documents"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteAllDocuments deletes all documents in the workspace.
func (h *Handler) DeleteAllDocuments(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	result, err := h.edgequakeService.DeleteAllDocuments(r.Context(), tCtx)
	if err != nil {
		h.log.Error(r.Context(), "failed to delete all documents", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete all documents"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// RetryTask retries a failed pipeline task.
func (h *Handler) RetryTask(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := h.edgequakeService.RetryTask(r.Context(), tCtx, req.TaskID)
	if err != nil {
		h.log.Error(r.Context(), "failed to retry task", "error", err, "taskId", req.TaskID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to retry task"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// RetryPdfProcessing retries a failed PDF processing.
func (h *Handler) RetryPdfProcessing(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pdfID := chi.URLParam(r, "pdfId")
	if pdfID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing PDF ID"})
		return
	}

	result, err := h.edgequakeService.RetryPdfProcessing(r.Context(), tCtx, pdfID)
	if err != nil {
		h.log.Error(r.Context(), "failed to retry PDF processing", "error", err, "pdfId", pdfID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to retry PDF processing"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetEnhancedPipelineStatus returns the enhanced pipeline status.
func (h *Handler) GetEnhancedPipelineStatus(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	status, err := h.edgequakeService.GetEnhancedPipelineStatus(r.Context(), tCtx)
	if err != nil {
		h.log.Error(r.Context(), "failed to get enhanced pipeline status", "error", err)
		// Fallback to task list
		tasks, taskErr := h.edgequakeService.GetPipelineTasks(r.Context(), tCtx, 1, 50)
		if taskErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load pipeline status"})
			return
		}
		writeJSON(w, http.StatusOK, tasks)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// GetQueueMetrics returns queue metrics.
func (h *Handler) GetQueueMetrics(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	metrics, err := h.edgequakeService.GetQueueMetrics(r.Context(), tCtx)
	if err != nil {
		h.log.Error(r.Context(), "failed to get queue metrics", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load queue metrics"})
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}

// CancelPipeline cancels the entire pipeline.
func (h *Handler) CancelPipeline(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	result, err := h.edgequakeService.CancelPipeline(r.Context(), tCtx)
	if err != nil {
		// 409 means no job is running — this is not an error, just nothing to cancel
		if strings.Contains(err.Error(), "409") {
			writeJSON(w, http.StatusOK, map[string]string{
				"status":  "no_job_running",
				"message": "No pipeline job is currently running.",
			})
			return
		}
		h.log.Error(r.Context(), "failed to cancel pipeline", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to cancel pipeline"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ReprocessAll triggers reprocessing for all documents.
func (h *Handler) ReprocessAll(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	wsID := r.URL.Query().Get("workspace_id")
	workspaceID, err := uuid.Parse(wsID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid workspace ID"})
		return
	}

	if err := h.edgequakeService.ReprocessAllDocuments(r.Context(), tCtx, workspaceID); err != nil {
		h.log.Error(r.Context(), "failed to reprocess workspace", "error", err, "workspaceId", wsID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to reprocess workspace"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

// RebuildWorkspace triggers knowledge graph or embeddings rebuild.
func (h *Handler) RebuildWorkspace(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	wsID := chi.URLParam(r, "workspaceId")
	workspaceID, err := uuid.Parse(wsID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid workspace ID"})
		return
	}

	var req struct {
		Type              string `json:"type"` // "graph" or "embeddings"
		LlmModel          string `json:"llm_model,omitempty"`
		LlmProvider       string `json:"llm_provider,omitempty"`
		EmbeddingModel    string `json:"embedding_model,omitempty"`
		EmbeddingProvider string `json:"embedding_provider,omitempty"`
		Force             bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Type == "graph" {
		result, err := h.edgequakeService.RebuildKnowledgeGraph(r.Context(), tCtx, workspaceID, &workspaces.RebuildKnowledgeGraphRequest{
			LlmModel:    &req.LlmModel,
			LlmProvider: &req.LlmProvider,
			Force:       req.Force,
		})
		if err != nil {
			h.log.Error(r.Context(), "failed to rebuild graph", "error", err, "workspaceId", wsID)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to rebuild knowledge graph"})
			return
		}
		writeJSON(w, http.StatusOK, result)
	} else if req.Type == "embeddings" {
		result, err := h.edgequakeService.RebuildEmbeddings(r.Context(), tCtx, workspaceID, &workspaces.RebuildEmbeddingsRequest{
			EmbeddingModel:    &req.EmbeddingModel,
			EmbeddingProvider: &req.EmbeddingProvider,
			Force:             req.Force,
		})
		if err != nil {
			h.log.Error(r.Context(), "failed to rebuild embeddings", "error", err, "workspaceId", wsID)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to rebuild embeddings"})
			return
		}
		writeJSON(w, http.StatusOK, result)
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid rebuild type"})
	}
}

// CancelPipelineTask cancels a specific task in the pipeline.
func (h *Handler) CancelPipelineTask(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if err := h.edgequakeService.CancelPipelineTask(r.Context(), tCtx, req.TaskID); err != nil {
		if errors.Is(err, documents.ErrConflict) {
			h.log.Info(r.Context(), "pipeline task already in target state", "taskId", req.TaskID)
			writeJSON(w, http.StatusConflict, map[string]string{"error": "Task is already cancelled or in a state where cancellation is not possible"})
			return
		}
		h.log.Error(r.Context(), "failed to cancel pipeline task", "error", err, "taskId", req.TaskID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to cancel task"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// GetGraph retrieves the knowledge graph.
func (h *Handler) GetGraph(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle workspace_id param
	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	params := &graph.GraphQueryParams{}
	if startNode := r.URL.Query().Get("start_node"); startNode != "" {
		params.StartNode = &startNode
	}
	if depth, err := strconv.Atoi(r.URL.Query().Get("depth")); err == nil && depth > 0 {
		params.Depth = depth
	}
	if maxNodes, err := strconv.Atoi(r.URL.Query().Get("max_nodes")); err == nil && maxNodes > 0 {
		params.MaxNodes = maxNodes
	}

	graphData, err := h.edgequakeService.GetGraph(r.Context(), tCtx, params)
	if err != nil {
		h.log.Error(r.Context(), "failed to get graph", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load knowledge graph"})
		return
	}

	writeJSON(w, http.StatusOK, graphData)
}

// GetGraphNode retrieves a specific graph node.
func (h *Handler) GetGraphNode(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	nodeID := chi.URLParam(r, "nodeId")
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing node ID"})
		return
	}

	node, err := h.edgequakeService.GetGraphNode(r.Context(), tCtx, nodeID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get graph node", "error", err, "nodeId", nodeID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load node"})
		return
	}

	writeJSON(w, http.StatusOK, node)
}

// SearchGraphNodes searches for nodes by query.
func (h *Handler) SearchGraphNodes(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing search query"})
		return
	}

	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}

	result, err := h.edgequakeService.SearchGraphNodes(r.Context(), tCtx, q, limit)
	if err != nil {
		h.log.Error(r.Context(), "failed to search graph nodes", "error", err, "query", q)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to search nodes"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetPopularLabels retrieves popular graph labels.
func (h *Handler) GetPopularLabels(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}

	result, err := h.edgequakeService.GetPopularLabels(r.Context(), tCtx, limit)
	if err != nil {
		h.log.Error(r.Context(), "failed to get popular labels", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load popular labels"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchGraphLabels searches for graph labels.
func (h *Handler) SearchGraphLabels(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	wsID := r.URL.Query().Get("workspace_id")
	if wsID != "" {
		if uid, err := uuid.Parse(wsID); err == nil {
			tCtx.WorkspaceID = &uid
		}
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing search query"})
		return
	}

	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}

	result, err := h.edgequakeService.SearchGraphLabels(r.Context(), tCtx, q, limit)
	if err != nil {
		h.log.Error(r.Context(), "failed to search graph labels", "error", err, "query", q)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to search labels"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListLLMModels returns available LLM models for workspace creation.
func (h *Handler) ListLLMModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.edgequakeService.ListLLMModels(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "failed to list LLM models", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load LLM models"})
		return
	}

	writeJSON(w, http.StatusOK, models)
}

// ListEmbeddingModels returns available embedding models for workspace creation.
func (h *Handler) ListEmbeddingModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.edgequakeService.ListEmbeddingModels(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "failed to list embedding models", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load embedding models"})
		return
	}

	writeJSON(w, http.StatusOK, models)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
