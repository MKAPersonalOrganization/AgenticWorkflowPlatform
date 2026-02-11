package lightrag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// ---------- JSON API Handlers ----------

// docEntry is a flat representation of a document for the API response.
type docEntry struct {
	ID        string `json:"id"`
	FilePath  string `json:"file_path"`
	Status    string `json:"status"`
	Summary   string `json:"content_summary"`
	Length    int    `json:"content_length"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	// IsDir     bool   `json:"is_dir"` // Deprecated in favor of flat list? Or keep for folders?
	// For now, let's keep folders if we support them, but DB is flat for files.
	IsDir bool `json:"is_dir"`
}

// CreateFolderRequest is the request body for creating a folder.
type CreateFolderRequest struct {
	Path  string `json:"path"`
	Scope string `json:"scope"`
}

// CreateFolder creates a new folder.
func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CreateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}

	// Determine scope/owner
	var scope rag.Scope
	ownerID := ""
	if req.Scope == "private" {
		scope = rag.ScopePrivate
		ownerID = h.adminUser // or from context if multi-user
	} else {
		scope = rag.ScopePublic
	}

	if err := h.ragService.CreateDirectory(ctx, scope, ownerID, req.Path); err != nil {
		h.log.Error(ctx, "lightrag: create folder", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Save to DB
	// Normalize path
	if !strings.HasSuffix(req.Path, "/") {
		req.Path += "/"
	}

	doc := &domain.Document{
		OwnerID:     ownerID,
		Name:        req.Path,
		S3Key:       req.Path,
		ContentType: "application/x-directory",
		Size:        0,
		Status:      domain.DocumentStatusUploaded,
		IsDir:       true,
	}

	if err := h.ragService.SaveDocument(ctx, doc); err != nil {
		h.log.Error(ctx, "lightrag: save folder metadata", "error", err)
		// We could rollback S3, but folder objects are lightweight.
		// For now just log usage.
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "Folder created"})
}

// ListDocuments returns a paginated list of documents.
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parameters
	prefix := r.URL.Query().Get("prefix")
	scopeStr := r.URL.Query().Get("scope")

	// Pagination
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var ownerID string
	if scopeStr == "private" {
		ownerID = h.adminUser
	}

	// 1. Get documents from DB (Source of Truth for Metadata & ID)
	// Pagination is now done in DB
	docs, total, err := h.ragService.ListDocumentsDB(ctx, ownerID, prefix, pageSize, (page-1)*pageSize)
	if err != nil {
		h.log.Error(ctx, "lightrag: list documents db", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list documents"})
		return
	}

	// 2. Get statuses from LightRAG (Source of Truth for processing status)
	// We might want to optimize this by only querying for the specific IDs?
	// But LightRAG status is keyed by filename/path usually.
	// But LightRAG status is keyed by filename/path usually.
	ragStatuses, err := h.rag.GetDocStatuses(ctx)
	if err != nil {
		h.log.Error(ctx, "lightrag: get doc statuses", "error", err)
		ragStatuses = make(map[string]map[string]interface{})
	}

	// 3. Merge State and Filter
	var responseDocs []docEntry

	// Normalize prefix for filtering
	searchPrefix := prefix
	if searchPrefix != "" && !strings.HasSuffix(searchPrefix, "/") {
		searchPrefix += "/"
	}

	for _, d := range docs {
		// Filter out files in subdirectories (naive implementation: list all prefixed, filter in application)
		// NOTE: This breaks pagination if many files are in subdirectories.
		// Ideally the DB query should exclude them.
		// For now, let's filter what we got.

		relPath := strings.TrimPrefix(d.Name, searchPrefix)
		if relPath == "" {
			continue // Skip the directory itself if returned
		}

		// If remaining path contains separator, it's a sub-item, skip it
		// (unless we are listing recursive? No, standard UI is folder view)
		// But wait, if d.IsDir is true, it might have trailing slash?
		// e.g. foo/bar/ -> relPath = bar/
		// e.g. foo/bar.txt -> relPath = bar.txt

		// Check for nested slashes.
		// If relPath has slash, and it's NOT just the trailing slash:
		// e.g. bar/baz.txt -> has slash, skip.
		// e.g. bar/ -> has slash at end. OK.

		trimmedRel := strings.TrimSuffix(relPath, "/")
		if strings.Contains(trimmedRel, "/") {
			continue
		}

		// Status inference:
		ragStatusData, inRag := ragStatuses[d.S3Key]
		finalStatus := string(d.Status)

		if inRag {
			rawRagStatus := strVal(ragStatusData, "status")
			if rawRagStatus == "processed" {
				finalStatus = "indexed"
			} else if rawRagStatus == "failed" {
				finalStatus = "failed"
			}
		}

		entry := docEntry{
			ID:        d.ID.String(),
			FilePath:  relPath, // Display relative path
			Status:    finalStatus,
			Length:    int(d.Size),
			CreatedAt: d.CreatedAt.Format(time.RFC3339),
			UpdatedAt: d.UpdatedAt.Format(time.RFC3339),
			IsDir:     d.IsDir,
		}
		if inRag {
			entry.Summary = strVal(ragStatusData, "content_summary")
		}
		responseDocs = append(responseDocs, entry)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"documents":   responseDocs,
		"total":       total, // Note: Total is still DB total, not filtered total :(
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
	})
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

	// Parse scope, ownerID, path
	scopeStr := r.FormValue("scope")
	ownerID := r.FormValue("owner_id")
	targetPath := r.FormValue("path") // target folder

	// Ensure path ends with / and doesn't start with / if not empty
	if targetPath != "" {
		if targetPath[len(targetPath)-1] != '/' {
			targetPath += "/"
		}
		if targetPath[0] == '/' {
			targetPath = targetPath[1:]
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

		fullFileName := targetPath + header.Filename

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
			Name:        fullFileName, // Use full path for valid hierarchy
			S3Key:       fullFileName,
			ContentType: header.Header.Get("Content-Type"),
			Size:        size,
			Status:      domain.DocumentStatusUploaded,
		}

		if err := h.ragService.SaveDocument(ctx, doc); err != nil {
			h.log.Error(ctx, "lightrag: save doc metadata", "error", err)
			// TODO: Rollback S3?
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

// DeleteDocument deletes a single document by ID.
func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docID := chi.URLParam(r, "id")

	if docID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document ID required"})
		return
	}

	// Parse scope/owner from Query (files are effectively scoped in S3)
	// If not provided, we might default to Public or try both?
	// Better to require it or try to deduce.
	scopeStr := r.URL.Query().Get("scope")
	var scope rag.Scope
	ownerID := ""

	if scopeStr == "private" {
		scope = rag.ScopePrivate
		ownerID = h.adminUser
	} else {
		scope = rag.ScopePublic
	}

	// 1. Get Doc from DB to find S3 Key
	// The docID is now a UUID from the URL wildcard which matched the UUID pattern?
	// But our route is /documents/*, and standard UUID is just /documents/{id}.
	// We fixed nested files by making it wildcard to capture paths.
	// But now we are moving to opaque IDs (UUIDs).
	// So we should expect a UUID.

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

	// 2. Recursive Delete if Directory
	if doc.IsDir {
		prefix := doc.S3Key
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}

		// Delete children
		pageSize := 100
		for {
			// Always fetch page 0 because deletion shifts the results
			children, _, err := h.ragService.ListDocumentsDB(ctx, ownerID, prefix, pageSize, 0)
			if err != nil {
				h.log.Error(ctx, "lightrag: recursive list failed", "error", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list children for deletion"})
				return
			}

			// Filter out self if returned
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
				// Delete from S3
				if err := h.ragService.Delete(ctx, scope, ownerID, child.S3Key); err != nil {
					h.log.Error(ctx, "lightrag: recursive s3 delete failed", "child", child.S3Key, "error", err)
					// Continue trying to delete others
				}

				// Delete from DB
				if err := h.ragService.DeleteDocumentDB(ctx, child.ID); err != nil {
					h.log.Error(ctx, "lightrag: recursive db delete failed", "child", child.ID, "error", err)
				}
			}
		}
	}

	// 3. Delete the document itself (or directory placeholder) from S3
	if err := h.ragService.Delete(ctx, scope, ownerID, doc.S3Key); err != nil {
		h.log.Error(ctx, "lightrag: s3 delete failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete file from storage: " + err.Error()})
		return
	}

	// 4. Delete from DB
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

// IngestRequest is the request body for ingesting documents.
type IngestRequest struct {
	Files []string `json:"files"`
	Scope string   `json:"scope"`
}

// IngestDocuments triggers ingestion of specific documents from storage into the RAG pipeline.
func (h *Handler) IngestDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.Files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "files list is required"})
		return
	}

	// Parse scope/owner
	var scope rag.Scope
	ownerID := ""
	if req.Scope == "private" {
		scope = rag.ScopePrivate
		ownerID = h.adminUser
	} else {
		scope = rag.ScopePublic
	}

	successCount := 0
	var errors []string

	for _, idStr := range req.Files {
		// idStr is now UUID
		uid, err := uuid.Parse(idStr)
		if err != nil {
			errors = append(errors, fmt.Sprintf("invalid ID %s: %v", idStr, err))
			continue
		}

		doc, err := h.ragService.GetDocument(ctx, uid)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to find doc %s: %v", idStr, err))
			continue
		}

		// 1. Load from Storage (S3) using Doc's S3Key
		reader, err := h.ragService.Load(ctx, scope, ownerID, doc.S3Key)
		if err != nil {
			h.log.Error(ctx, "lightrag: load from storage", "file", doc.S3Key, "error", err)
			errors = append(errors, fmt.Sprintf("failed to load %s: %v", doc.Name, err))
			continue
		}

		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			h.log.Error(ctx, "lightrag: read from storage", "file", doc.S3Key, "error", err)
			errors = append(errors, fmt.Sprintf("failed to read %s: %v", doc.Name, err))
			continue
		}

		// 2. Register file in LightRAG for indexing
		// We use the S3Key as the ID/Subject in RAG
		if err := h.rag.RegisterFile(ctx, doc.S3Key, data); err != nil {
			h.log.Error(ctx, "lightrag: register file failed", "file", doc.S3Key, "error", err)
			errors = append(errors, fmt.Sprintf("failed to register %s: %v", doc.Name, err))
			continue
		}

		successCount++
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Queued %d files for ingestion", successCount),
	}
	if len(errors) > 0 {
		response["status"] = "partial_success"
		response["errors"] = errors
	}
	if successCount == 0 && len(errors) > 0 {
		response["status"] = "failure"
		response["message"] = "Failed to queue any files"
	}

	writeJSON(w, http.StatusOK, response)
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

	// 1. List all files from S3 (Public and Private)
	// For simplicity, let's sync public first, or iterate all users if needed.
	// Admin user context?
	// Let's iterate both scopes for the admin user.

	scopes := []rag.Scope{rag.ScopePublic, rag.ScopePrivate}
	ownerID := h.adminUser

	// 2. Get RAG Statuses for status inference
	ragStatuses, _ := h.rag.GetDocStatuses(ctx) // changes map[string]interface{}

	syncedCount := 0

	for _, scope := range scopes {
		files, err := h.ragService.List(ctx, scope, ownerID, "")
		if err != nil {
			h.log.Error(ctx, "lightrag: sync list s3", "error", err)
			continue
		}

		for _, f := range files {
			// Check if exists in DB by S3Key
			// S3 Key in our system is currently the full path (f.Name)
			existing, err := h.ragService.GetDocumentByS3Key(ctx, f.Name)
			if err != nil {
				h.log.Error(ctx, "lightrag: sync check db", "file", f.Name, "error", err)
				continue
			}

			if existing != nil {
				continue // Already exists
			}

			// Determine status
			status := domain.DocumentStatusUploaded
			if val, ok := ragStatuses[f.Name]; ok {
				// val is map[string]interface{}
				rawStatus := strVal(val, "status")
				if rawStatus == "processed" {
					status = domain.DocumentStatusIndexed
				} else if rawStatus == "failed" {
					status = domain.DocumentStatusFailed
				} else if rawStatus == "pending" || rawStatus == "processing" {
					status = domain.DocumentStatusPipelined
				}
			}

			// Create new document
			doc := &domain.Document{
				OwnerID:     ownerID,
				Name:        f.Name, // Using f.Name as Name AND S3Key
				S3Key:       f.Name,
				ContentType: "application/octet-stream", // Unknown/Default
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

func intVal(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		}
	}
	return 0
}
