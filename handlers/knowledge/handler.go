package knowledge

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"school-with-ai/components/knowledge"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/edgequake"
	eqperms "school-with-ai/services/edgequakepermissions"
	"school-with-ai/services/rag/folders"
	"school-with-ai/services/rag/workspaces"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for the RAG Knowledge Base dashboard.
type Handler struct {
	log              *logger.Logger
	edgequakeService *edgequake.Service
	eqPermsService   *eqperms.Service
}

// NewHandler creates a new Knowledge Base handler.
func NewHandler(log *logger.Logger, edgequakeService *edgequake.Service, eqPermsService *eqperms.Service) *Handler {
	return &Handler{
		log:              log,
		edgequakeService: edgequakeService,
		eqPermsService:   eqPermsService,
	}
}

// resolveTenantContext resolves the tenant context for the authenticated user.
func (h *Handler) resolveTenantContext(r *http.Request) (*domain.TenantContext, error) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		return nil, fmt.Errorf("no authenticated user")
	}
	return h.eqPermsService.GetTenantContext(r.Context(), user.ID)
}

// DashboardPage renders the main Knowledge Base dashboard page.
func (h *Handler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())

	component := knowledge.DashboardPage(knowledge.DashboardPageProps{
		User: user,
	})
	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render knowledge dashboard", "error", err)
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

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 20
	}

	docs, err := h.edgequakeService.ListDocuments(r.Context(), tCtx, page, pageSize)
	if err != nil {
		h.log.Error(r.Context(), "failed to list documents", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load documents"})
		return
	}

	writeJSON(w, http.StatusOK, docs)
}

// UploadDocument handles document upload.
func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		h.log.Error(r.Context(), "failed to resolve tenant context", "error", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
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

	result, err := h.edgequakeService.UploadDocument(r.Context(), tCtx, header.Filename, contentType, file)
	if err != nil {
		h.log.Error(r.Context(), "failed to upload document", "error", err, "filename", header.Filename)
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

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 20
	}
	status := r.URL.Query().Get("status")

	pdfs, err := h.edgequakeService.ListPDFs(r.Context(), tCtx, page, pageSize, status)
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
		Name        string `json:"name"`
		Description string `json:"description"`
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

	ws, err := h.edgequakeService.CreateWorkspace(r.Context(), tCtx, &workspaces.CreateWorkspaceRequest{
		Name:        req.Name,
		Description: desc,
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

// ListFolders returns folders for the current workspace.
func (h *Handler) ListFolders(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	foldersList, err := h.edgequakeService.ListFolders(r.Context(), tCtx)
	if err != nil {
		h.log.Error(r.Context(), "failed to list folders", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load folders"})
		return
	}

	writeJSON(w, http.StatusOK, foldersList)
}

// CreateFolder creates a new folder.
func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Name is required"})
		return
	}

	folder, err := h.edgequakeService.CreateFolder(r.Context(), tCtx, &folders.CreateFolderRequest{Name: req.Name})
	if err != nil {
		h.log.Error(r.Context(), "failed to create folder", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create folder"})
		return
	}

	writeJSON(w, http.StatusCreated, folder)
}

// DeleteFolder deletes a folder.
func (h *Handler) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fID := chi.URLParam(r, "folderId")
	folderID, err := uuid.Parse(fID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid folder ID"})
		return
	}

	if err := h.edgequakeService.DeleteFolder(r.Context(), tCtx, folderID); err != nil {
		h.log.Error(r.Context(), "failed to delete folder", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete folder"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetDashboardData returns aggregated dashboard data.
func (h *Handler) GetDashboardData(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		h.log.Warn(r.Context(), "failed to resolve tenant context for dashboard", "error", err)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"documents": map[string]interface{}{"documents": []interface{}{}, "total": 0, "page": 1, "page_size": 10, "total_pages": 0},
			"pdfs":      map[string]interface{}{"items": []interface{}{}, "pagination": map[string]interface{}{"total_count": 0}},
			"stats":     nil,
		})
		return
	}

	type dashboardData struct {
		Documents interface{} `json:"documents"`
		PDFs      interface{} `json:"pdfs"`
		Stats     interface{} `json:"stats"`
	}

	data := dashboardData{}

	docs, err := h.edgequakeService.ListDocuments(r.Context(), tCtx, 1, 10)
	if err != nil {
		h.log.Warn(r.Context(), "failed to list documents for dashboard", "error", err)
		data.Documents = map[string]interface{}{"documents": []interface{}{}, "total": 0}
	} else {
		data.Documents = docs
	}

	pdfs, err := h.edgequakeService.ListPDFs(r.Context(), tCtx, 1, 10, "")
	if err != nil {
		h.log.Warn(r.Context(), "failed to list PDFs for dashboard", "error", err)
		data.PDFs = map[string]interface{}{"items": []interface{}{}, "pagination": map[string]interface{}{"total_count": 0}}
	} else {
		data.PDFs = pdfs
	}

	if tCtx.WorkspaceID != nil {
		stats, err := h.edgequakeService.GetWorkspaceStats(r.Context(), tCtx, *tCtx.WorkspaceID)
		if err != nil {
			h.log.Warn(r.Context(), "failed to get workspace stats for dashboard", "error", err)
		} else {
			data.Stats = stats
		}
	}

	writeJSON(w, http.StatusOK, data)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
