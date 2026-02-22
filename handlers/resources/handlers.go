package resources

import (
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"school-with-ai/domain"
	"school-with-ai/services/resources"

	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for resource management.
type Handler struct {
	service *resources.Service
}

// NewHandler creates a new Resources Handler.
func NewHandler(service *resources.Service) *Handler {
	return &Handler{
		service: service,
	}
}

// Upload handles file upload requests.
// POST /api/resources/upload
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		http.Error(w, "failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	res, err := h.service.UploadResource(r.Context(), user.ID, header.Filename, header.Header.Get("Content-Type"), file)
	if err != nil {
		http.Error(w, "Failed to upload resource", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// List handles listing resources.
// GET /api/resources?source=public|private
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	source := resources.ResourceSource(r.URL.Query().Get("source"))
	if source == "" {
		source = resources.SourcePublic
	}

	res, err := h.service.ListResources(r.Context(), user.ID, source)
	if err != nil {
		http.Error(w, "Failed to list resources", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// GetContent handles resource content retrieval.
// GET /api/resources/{id}/content
func (h *Handler) GetContent(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	res, content, err := h.service.GetResourceContent(r.Context(), id)
	if err != nil {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}
	defer content.Close()

	// Security check: private resources only for owner
	if res.Source == resources.SourcePrivate && (res.UserID == nil || *res.UserID != user.ID) {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}

	// Sanitize Content-Type: force download for potentially dangerous types
	contentType := res.FileType
	unsafeTypes := []string{"text/html", "application/xhtml", "application/javascript", "text/javascript"}
	for _, ut := range unsafeTypes {
		if strings.HasPrefix(strings.ToLower(contentType), ut) {
			contentType = "application/octet-stream"
			break
		}
	}
	w.Header().Set("Content-Type", contentType)

	// Sanitize filename for Content-Disposition to prevent header injection.
	// Use only the base name and pass through mime.FormatMediaType.
	safeFilename := filepath.Base(res.Title)
	safeFilename = strings.ReplaceAll(safeFilename, "\"", "_")
	safeFilename = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return '_'
		}
		return r
	}, safeFilename)
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": safeFilename})
	w.Header().Set("Content-Disposition", disposition)
	if _, err := io.Copy(w, content); err != nil {
		slog.Error("failed to stream resource content", "error", err)
	}
}

// Delete handles resource deletion.
// DELETE /api/resources/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteResource(r.Context(), id, user.ID); err != nil {
		http.Error(w, "Failed to delete resource", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
