package resources

import (
	"aicompass/foundation/auth"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// handler provides HTTP endpoints for resource management.
type handler struct {
	svc *service
}

// newHandler creates a new resources handler.
func newHandler(svc *service) *handler {
	return &handler{svc: svc}
}

// upload handles file upload requests.
// POST /api/resources/upload
func (h *handler) upload(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form. Cap the on-wire body via MaxBytesReader so
	// ParseMultipartForm's maxMemory is only about memory-vs-spill, not
	// the total upload size.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	res, err := h.svc.uploadResource(r.Context(), user.ID, header.Filename, header.Header.Get("Content-Type"), file)
	if err != nil {
		http.Error(w, "Failed to upload resource", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// list handles listing resources.
// GET /api/resources?source=public|private
func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	source := ResourceSource(r.URL.Query().Get("source"))
	if source == "" {
		source = SourcePublic
	}

	res, err := h.svc.listResources(r.Context(), user.ID, source)
	if err != nil {
		http.Error(w, "Failed to list resources", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// getContent handles resource content retrieval.
// GET /api/resources/{id}/content
func (h *handler) getContent(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
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

	res, content, err := h.svc.getResourceContent(r.Context(), id)
	if err != nil {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}
	defer func() { _ = content.Close() }()

	// Security check: private resources only for owner
	if res.Source == SourcePrivate && (res.UserID == nil || *res.UserID != user.ID) {
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

// delete handles resource deletion.
// DELETE /api/resources/{id}
func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
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

	if err := h.svc.deleteResource(r.Context(), id, user.ID); err != nil {
		http.Error(w, "Failed to delete resource", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
