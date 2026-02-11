package lightrag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	component "school-with-ai/components/lightrag"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"

	"school-with-ai/services/rag"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lightrag/lightrag-go/pkg/core/base"
	ragcore "github.com/lightrag/lightrag-go/pkg/core/lightrag"
)

// Handler provides HTTP handlers for the LightRAG dashboard.
type Handler struct {
	log        *logger.Logger
	ragService rag.Service
	rag        *ragcore.LightRAG
	adminUser  string
	adminPass  string
}

// NewHandler creates a new LightRAG dashboard handler.
func NewHandler(log *logger.Logger, ragService rag.Service, rag *ragcore.LightRAG, adminUser, adminPass string) *Handler {
	return &Handler{
		log:        log,
		ragService: ragService,
		rag:        rag,
		adminUser:  adminUser,
		adminPass:  adminPass,
	}
}

// BasicAuth wraps a handler with basic auth for the dashboard page.
func (h *Handler) BasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != h.adminUser || pass != h.adminPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="LightRAG Dashboard"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// BasicAuthMiddleware is middleware form for use with chi router groups.
func (h *Handler) BasicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != h.adminUser || pass != h.adminPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="LightRAG Dashboard"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ServeDashboard renders the LightRAG dashboard page.
func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := domain.GetUserFromContext(ctx)

	component.Dashboard(component.DashboardProps{
		User: user,
	}).Render(ctx, w)
}

// ---------- Scope Helper ----------

func (h *Handler) resolveScope(r *http.Request) (rag.Scope, string) {
	scopeStr := r.URL.Query().Get("scope")
	if scopeStr == "" {
		scopeStr = r.FormValue("scope")
	}
	if scopeStr == "private" {
		return rag.ScopePrivate, h.adminUser
	}
	return rag.ScopePublic, ""
}

// ---------- JSON API Handlers ----------

// docEntry is a flat representation of a document for the API response.
type docEntry struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	ParentPath  string  `json:"parent_path"`
	Status      string  `json:"status"`
	Summary     string  `json:"content_summary,omitempty"`
	Size        int64   `json:"size"`
	ContentType string  `json:"content_type"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	DeletedAt   *string `json:"deleted_at,omitempty"`
	IsDir       bool    `json:"is_dir"`
}

func docToEntry(d domain.Document) docEntry {
	// Extract display name (basename) from full path
	displayName := d.Name
	if d.ParentPath != "" {
		displayName = strings.TrimPrefix(d.Name, d.ParentPath)
	}
	displayName = strings.TrimSuffix(displayName, "/")
	if displayName == "" {
		displayName = d.Name
	}

	entry := docEntry{
		ID:          d.ID.String(),
		Name:        d.Name,
		DisplayName: displayName,
		ParentPath:  d.ParentPath,
		Status:      string(d.Status),
		Size:        d.Size,
		ContentType: d.ContentType,
		CreatedAt:   d.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   d.UpdatedAt.Format(time.RFC3339),
		IsDir:       d.IsDir,
	}
	if d.DeletedAt != nil {
		ts := d.DeletedAt.Format(time.RFC3339)
		entry.DeletedAt = &ts
	}
	return entry
}

// CreateFolderRequest is the request body for creating a folder.
type CreateFolderRequest struct {
	Name       string `json:"name"`
	ParentPath string `json:"parent_path"`
	Scope      string `json:"scope"`
}

// CreateFolder creates a new folder.
func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CreateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Validate folder name
	if strings.Contains(req.Name, "/") || strings.Contains(req.Name, "\\") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "folder name cannot contain slashes"})
		return
	}

	// Build full path
	parentPath := req.ParentPath
	if parentPath != "" && !strings.HasSuffix(parentPath, "/") {
		parentPath += "/"
	}
	fullPath := parentPath + req.Name + "/"

	// Determine scope/owner
	var scope rag.Scope
	ownerID := ""
	if req.Scope == "private" {
		scope = rag.ScopePrivate
		ownerID = h.adminUser
	} else {
		scope = rag.ScopePublic
	}

	if err := h.ragService.CreateDirectory(ctx, scope, ownerID, fullPath); err != nil {
		h.log.Error(ctx, "lightrag: create folder", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	doc := &domain.Document{
		OwnerID:     ownerID,
		Name:        fullPath,
		ParentPath:  parentPath,
		S3Key:       fullPath,
		ContentType: "application/x-directory",
		Size:        0,
		Status:      domain.DocumentStatusUploaded,
		IsDir:       true,
	}

	if err := h.ragService.SaveDocument(ctx, doc); err != nil {
		h.log.Error(ctx, "lightrag: save folder metadata", "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "Folder created"})
}

// ListDocuments returns a paginated list of documents for the current folder.
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	parentPath := r.URL.Query().Get("parent_path")
	_, ownerID := h.resolveScope(r)

	// Pagination
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	docs, total, err := h.ragService.ListByParentPath(ctx, ownerID, parentPath, pageSize, (page-1)*pageSize)
	if err != nil {
		h.log.Error(ctx, "lightrag: list documents", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list documents"})
		return
	}

	entries := make([]docEntry, 0, len(docs))
	for _, d := range docs {
		entries = append(entries, docToEntry(d))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"documents":   entries,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
		"parent_path": parentPath,
	})
}

// ListChildren is an alias for ListDocuments using parent_path-based listing.
func (h *Handler) ListChildren(w http.ResponseWriter, r *http.Request) {
	h.ListDocuments(w, r)
}

// UploadDocument saves an uploaded file to the RAG working directory.
func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing file field"})
		return
	}

	scopeStr := r.FormValue("scope")
	ownerID := r.FormValue("owner_id")
	parentPath := r.FormValue("parent_path")

	// Normalize parent path
	if parentPath != "" {
		if parentPath[len(parentPath)-1] != '/' {
			parentPath += "/"
		}
		if parentPath[0] == '/' {
			parentPath = parentPath[1:]
		}
	}

	var scope rag.Scope
	if scopeStr == "private" {
		scope = rag.ScopePrivate
		if ownerID == "" {
			ownerID = h.adminUser
		}
	} else {
		scope = rag.ScopePublic
	}

	successCount := 0
	var errors []string

	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to open %s: %v", header.Filename, err))
			continue
		}
		data, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to read %s: %v", header.Filename, err))
			continue
		}

		fullFileName := parentPath + header.Filename

		// Upload to MinIO
		size, err := h.ragService.Upload(ctx, scope, ownerID, fullFileName, bytes.NewReader(data))
		if err != nil {
			h.log.Error(ctx, "lightrag: upload to minio", "error", err)
			errors = append(errors, fmt.Sprintf("failed to upload %s: %v", header.Filename, err))
			continue
		}

		// Save to DB
		doc := &domain.Document{
			OwnerID:     ownerID,
			Name:        fullFileName,
			ParentPath:  parentPath,
			S3Key:       fullFileName,
			ContentType: header.Header.Get("Content-Type"),
			Size:        size,
			Status:      domain.DocumentStatusUploaded,
		}

		if err := h.ragService.SaveDocument(ctx, doc); err != nil {
			h.log.Error(ctx, "lightrag: save doc metadata", "error", err)
			errors = append(errors, fmt.Sprintf("failed to save metadata for %s: %v", header.Filename, err))
			continue
		}

		successCount++
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Uploaded %d files", successCount),
	}
	if len(errors) > 0 {
		response["status"] = "partial_success"
		response["errors"] = errors
	}
	if successCount == 0 && len(errors) > 0 {
		response["status"] = "failure"
		response["message"] = "Failed to upload any files"
	}

	writeJSON(w, http.StatusOK, response)
}

// DeleteDocument permanently deletes a single document by ID.
func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docID := chi.URLParam(r, "id")

	if docID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document ID required"})
		return
	}

	scope, ownerID := h.resolveScope(r)

	uid, err := uuid.Parse(docID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid document ID"})
		return
	}

	doc, err := h.ragService.GetDocument(ctx, uid)
	if err != nil {
		h.log.Error(ctx, "lightrag: get document", "error", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}
	if doc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	// Recursive delete if directory
	if doc.IsDir {
		prefix := doc.S3Key
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		// Delete children from S3 and DB
		pageSize := 100
		for {
			children, _, err := h.ragService.ListDocumentsDB(ctx, ownerID, prefix, pageSize, 0)
			if err != nil {
				h.log.Error(ctx, "lightrag: recursive list failed", "error", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list children for deletion"})
				return
			}
			var toDelete []domain.Document
			for _, child := range children {
				if child.ID != doc.ID {
					toDelete = append(toDelete, child)
				}
			}
			if len(toDelete) == 0 {
				break
			}
			for _, child := range toDelete {
				if err := h.ragService.Delete(ctx, scope, ownerID, child.S3Key); err != nil {
					h.log.Error(ctx, "lightrag: recursive s3 delete failed", "child", child.S3Key, "error", err)
				}
				if err := h.ragService.DeleteDocumentDB(ctx, child.ID); err != nil {
					h.log.Error(ctx, "lightrag: recursive db delete failed", "child", child.ID, "error", err)
				}
			}
		}
	}

	// Delete the document itself from S3
	if err := h.ragService.Delete(ctx, scope, ownerID, doc.S3Key); err != nil {
		h.log.Error(ctx, "lightrag: s3 delete failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete file from storage: " + err.Error()})
		return
	}

	// Delete from DB
	if err := h.ragService.DeleteDocumentDB(ctx, uid); err != nil {
		h.log.Error(ctx, "lightrag: db delete failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete metadata"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Document deleted",
	})
}

// ---------- New Drive Endpoints ----------

// RenameRequest is the request body for renaming a document.
type RenameRequest struct {
	NewName string `json:"new_name"`
}

// RenameDocument renames a document.
func (h *Handler) RenameDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docID := chi.URLParam(r, "id")
	uid, err := uuid.Parse(docID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid document ID"})
		return
	}

	var req RenameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.NewName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "new_name is required"})
		return
	}

	if strings.Contains(req.NewName, "/") || strings.Contains(req.NewName, "\\") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name cannot contain slashes"})
		return
	}

	scope, ownerID := h.resolveScope(r)

	if err := h.ragService.RenameDocument(ctx, scope, ownerID, uid, req.NewName); err != nil {
		h.log.Error(ctx, "lightrag: rename document", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "Document renamed"})
}

// MoveRequest is the request body for moving a document.
type MoveRequest struct {
	TargetFolder string `json:"target_folder"`
}

// MoveDocument moves a document to a different folder.
func (h *Handler) MoveDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docID := chi.URLParam(r, "id")
	uid, err := uuid.Parse(docID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid document ID"})
		return
	}

	var req MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	scope, ownerID := h.resolveScope(r)

	if err := h.ragService.MoveDocument(ctx, scope, ownerID, uid, req.TargetFolder); err != nil {
		h.log.Error(ctx, "lightrag: move document", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "Document moved"})
}

// TrashDocument moves a document to trash (soft delete).
func (h *Handler) TrashDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docID := chi.URLParam(r, "id")
	uid, err := uuid.Parse(docID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid document ID"})
		return
	}

	if err := h.ragService.TrashDocument(ctx, uid); err != nil {
		h.log.Error(ctx, "lightrag: trash document", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "Document moved to trash"})
}

// RestoreDocument restores a document from trash.
func (h *Handler) RestoreDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docID := chi.URLParam(r, "id")
	uid, err := uuid.Parse(docID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid document ID"})
		return
	}

	if err := h.ragService.RestoreDocument(ctx, uid); err != nil {
		h.log.Error(ctx, "lightrag: restore document", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "Document restored"})
}

// ListTrash returns trashed documents.
func (h *Handler) ListTrash(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, ownerID := h.resolveScope(r)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	docs, total, err := h.ragService.ListTrashed(ctx, ownerID, pageSize, (page-1)*pageSize)
	if err != nil {
		h.log.Error(ctx, "lightrag: list trash", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list trash"})
		return
	}

	entries := make([]docEntry, 0, len(docs))
	for _, d := range docs {
		entries = append(entries, docToEntry(d))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"documents":   entries,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
	})
}

// EmptyTrash permanently deletes all trashed documents.
func (h *Handler) EmptyTrash(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	scope, ownerID := h.resolveScope(r)

	if err := h.ragService.EmptyTrash(ctx, scope, ownerID); err != nil {
		h.log.Error(ctx, "lightrag: empty trash", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "Trash emptied"})
}

// DownloadDocument downloads a document.
func (h *Handler) DownloadDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docID := chi.URLParam(r, "id")
	uid, err := uuid.Parse(docID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid document ID"})
		return
	}

	scope, ownerID := h.resolveScope(r)

	reader, doc, err := h.ragService.DownloadDocument(ctx, scope, ownerID, uid)
	if err != nil {
		h.log.Error(ctx, "lightrag: download document", "error", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	defer reader.Close()

	// Extract filename for Content-Disposition
	filename := filepath.Base(doc.Name)

	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if doc.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(doc.Size, 10))
	}

	io.Copy(w, reader)
}

// SearchDocuments searches documents by name.
func (h *Handler) SearchDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, ownerID := h.resolveScope(r)
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query parameter 'q' is required"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	docs, total, err := h.ragService.SearchDocuments(ctx, ownerID, query, pageSize, (page-1)*pageSize)
	if err != nil {
		h.log.Error(ctx, "lightrag: search documents", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "search failed"})
		return
	}

	entries := make([]docEntry, 0, len(docs))
	for _, d := range docs {
		entries = append(entries, docToEntry(d))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"documents":   entries,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
		"query":       query,
	})
}

// ---------- Existing RAG Endpoints (kept for admin) ----------

// ClearDocuments deletes all documents.
func (h *Handler) ClearDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.rag.ClearAllDocuments(ctx); err != nil {
		h.log.Error(ctx, "lightrag: clear documents", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "All documents cleared",
	})
}

// GetStatusCounts returns document counts by status.
func (h *Handler) GetStatusCounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	statuses, err := h.rag.GetDocStatuses(ctx)
	if err != nil {
		h.log.Error(ctx, "lightrag: get status counts", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get status counts"})
		return
	}

	counts := map[string]int{
		"pending":    0,
		"processing": 0,
		"processed":  0,
		"failed":     0,
		"total":      len(statuses),
	}
	for _, data := range statuses {
		status := strVal(data, "status")
		counts[status]++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status_counts": counts})
}

// GetHealth returns the health status of the LightRAG instance.
func (h *Handler) GetHealth(w http.ResponseWriter, r *http.Request) {
	status := h.rag.GetStatus()
	cfg := h.rag.GetConfig()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "healthy",
		"storages_status": status.String(),
		"working_dir":     cfg.WorkingDir,
		"workspace":       cfg.Workspace,
	})
}

// IndexDocuments triggers indexing of all pending documents through the pipeline.
func (h *Handler) IndexDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	result, err := h.rag.IndexPendingDocuments(ctx)
	if err != nil {
		h.log.Error(ctx, "lightrag: index documents", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// QueryRequest is the request body for the query endpoint.
type QueryRequest struct {
	Query string `json:"query"`
	Mode  string `json:"mode"`
}

// QueryKnowledgeBase queries the knowledge base.
func (h *Handler) QueryKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query is required"})
		return
	}

	if req.Mode == "" {
		req.Mode = "hybrid"
	}

	param := &base.QueryParam{
		Mode: req.Mode,
	}

	result, err := h.rag.Query(ctx, req.Query, param)
	if err != nil {
		h.log.Error(ctx, "lightrag: query", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"response": result.Answer,
		"context":  result.Context,
	})
}

// ClearCache clears the LLM response cache.
func (h *Handler) ClearCache(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.rag.ClearLLMCache(ctx); err != nil {
		h.log.Error(ctx, "lightrag: clear cache", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "LLM cache cleared",
	})
}

// SyncDocuments synchronizes the S3 state to the database.
func (h *Handler) SyncDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scopes := []rag.Scope{rag.ScopePublic, rag.ScopePrivate}
	ownerID := h.adminUser

	ragStatuses, _ := h.rag.GetDocStatuses(ctx)

	syncedCount := 0

	for _, scope := range scopes {
		files, err := h.ragService.List(ctx, scope, ownerID, "")
		if err != nil {
			h.log.Error(ctx, "lightrag: sync list s3", "error", err)
			continue
		}

		for _, f := range files {
			existing, err := h.ragService.GetDocumentByS3Key(ctx, f.Name)
			if err != nil {
				h.log.Error(ctx, "lightrag: sync check db", "file", f.Name, "error", err)
				continue
			}

			if existing != nil {
				continue
			}

			status := domain.DocumentStatusUploaded
			if val, ok := ragStatuses[f.Name]; ok {
				rawStatus := strVal(val, "status")
				if rawStatus == "processed" {
					status = domain.DocumentStatusIndexed
				} else if rawStatus == "failed" {
					status = domain.DocumentStatusFailed
				} else if rawStatus == "pending" || rawStatus == "processing" {
					status = domain.DocumentStatusPipelined
				}
			}

			// Derive parent path
			parentPath := ""
			if idx := strings.LastIndex(strings.TrimSuffix(f.Name, "/"), "/"); idx >= 0 {
				parentPath = f.Name[:idx+1]
			}

			doc := &domain.Document{
				OwnerID:     ownerID,
				Name:        f.Name,
				ParentPath:  parentPath,
				S3Key:       f.Name,
				ContentType: "application/octet-stream",
				Size:        f.Size,
				Status:      status,
				IsDir:       f.IsDir,
				CreatedAt:   f.ModTime,
				UpdatedAt:   f.ModTime,
			}

			if f.IsDir {
				doc.ContentType = "application/x-directory"
			}

			if err := h.ragService.SaveDocument(ctx, doc); err != nil {
				h.log.Error(ctx, "lightrag: sync save doc", "file", f.Name, "error", err)
			} else {
				syncedCount++
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Synced %d new documents", syncedCount),
	})
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
