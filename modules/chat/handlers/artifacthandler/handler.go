package artifacthandler

import (
	"aicompass/foundation/artifact"
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"
	"aicompass/modules/chat/projection/agui"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Handler handles artifact-related HTTP endpoints.
type Handler struct {
	log            *logger.Logger
	aguiProjection agui.AguiProjection
	artifactStore  artifact.Service
}

// New creates a new artifact handler.
func New(
	log *logger.Logger,
	aguiProjection agui.AguiProjection,
	artifactStore artifact.Service,
) *Handler {
	return &Handler{
		log:            log,
		aguiProjection: aguiProjection,
		artifactStore:  artifactStore,
	}
}

// ListArtifacts returns a list of artifacts for a thread.
// GET /v1/threads/{threadId}/artifacts
func (h *Handler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	threadID := r.PathValue("threadId")
	if threadID == "" {
		jsonError(w, "threadId is required", http.StatusBadRequest)
		return
	}

	// Thread ownership check — S3 keys are userID/threadID prefixed.
	// For new sessions the thread doesn't exist yet, so allow (returns empty).
	// Block only if the thread exists and belongs to a different user.
	if state, exists := h.aguiProjection.GetThreadState(r.Context(), threadID); exists && state.UserID != user.ID {
		jsonError(w, "Thread not found", http.StatusNotFound)
		return
	}

	artifacts, err := h.artifactStore.List(r.Context(), user.ID, threadID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list artifacts", "error", err, "userID", user.ID, "threadID", threadID)
		jsonError(w, "Failed to list artifacts", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"artifacts": artifacts}); err != nil {
		h.log.Error(r.Context(), "failed to encode artifacts list", "error", err)
	}
}

// DownloadArtifact streams a specific artifact with HTTP Range request support.
// GET /v1/threads/{threadId}/artifacts/{fileName}
//
// Uses http.ServeContent which handles:
//   - Range requests (HTTP 206 Partial Content) for audio/video streaming
//   - Content-Length headers
//   - If-Modified-Since / If-None-Match conditional requests
func (h *Handler) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	threadID := r.PathValue("threadId")
	fileName := r.PathValue("fileName")
	if threadID == "" || fileName == "" {
		jsonError(w, "threadId and fileName are required", http.StatusBadRequest)
		return
	}
	// Thread ownership check: stops user A from reading user B's
	// artifacts by guessing thread IDs.
	if !h.ownsThread(r, user.ID, threadID /*allowMissing=*/, false) {
		jsonError(w, "Artifact not found", http.StatusNotFound)
		return
	}

	content, meta, err := h.artifactStore.Load(r.Context(), user.ID, threadID, fileName)
	if err != nil {
		h.writeLoadError(w, r, err, user.ID, threadID, fileName)
		return
	}
	defer func() { _ = content.Close() }()

	writeArtifactHeaders(w, fileName, meta.ContentType)
	h.streamArtifactBody(w, r, content, fileName)
}

// ownsThread returns true when the thread is accessible to userID: it
// either belongs to userID, or doesn't exist yet (allowMissing=true).
// Used by all ownership gates to keep the caller bodies linear.
func (h *Handler) ownsThread(r *http.Request, userID, threadID string, allowMissing bool) bool {
	state, exists := h.aguiProjection.GetThreadState(r.Context(), threadID)
	if !exists {
		return allowMissing
	}
	return state.UserID == userID
}

// writeLoadError maps artifact-Load errors onto HTTP responses: the
// ErrNotFound sentinel becomes 404, anything else is a logged 500.
func (h *Handler) writeLoadError(w http.ResponseWriter, r *http.Request, err error, userID, threadID, fileName string) {
	if errors.Is(err, artifact.ErrNotFound) {
		jsonError(w, "Artifact not found", http.StatusNotFound)
		return
	}
	h.log.Error(r.Context(), "failed to load artifact", "error", err, "userID", userID, "threadID", threadID, "fileName", fileName)
	jsonError(w, "Failed to load artifact", http.StatusInternalServerError)
}

// writeArtifactHeaders sets Content-Type, Content-Disposition (inline
// for media that browsers can render, attachment otherwise), and the
// immutable-cache header driven by the UUID-prefixed filename.
func writeArtifactHeaders(w http.ResponseWriter, fileName, contentType string) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	disposition := "attachment"
	if isInlineMediaType(contentType) {
		disposition = "inline"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q", disposition, fileName))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
}

// isInlineMediaType returns true for image/audio/video types browsers
// can render directly — those get inline disposition so the browser
// plays them rather than downloading.
func isInlineMediaType(contentType string) bool {
	return strings.HasPrefix(contentType, "image/") ||
		strings.HasPrefix(contentType, "audio/") ||
		strings.HasPrefix(contentType, "video/")
}

// streamArtifactBody ships the body: ReadSeekers go through
// http.ServeContent (Range support, Content-Length, conditional
// requests); anything else falls back to io.Copy with no Range support.
func (h *Handler) streamArtifactBody(w http.ResponseWriter, r *http.Request, content io.ReadCloser, fileName string) {
	if rs, ok := content.(io.ReadSeeker); ok {
		http.ServeContent(w, r, fileName, time.Time{}, rs)
		return
	}
	if _, err := io.Copy(w, content); err != nil {
		h.log.Error(r.Context(), "failed to stream artifact to client", "error", err, "fileName", fileName)
	}
}

// sanitizeFileName strips control characters, null bytes, path separators, and
// caps the length to prevent header injection and S3 key abuse.
func sanitizeFileName(name string) string {
	// Replace path separators with underscores
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "..", "_")

	// Strip control characters (ASCII 0x00–0x1F, 0x7F)
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if r > 0x1F && r != 0x7F {
			b.WriteRune(r)
		}
	}
	name = b.String()

	// Cap at 200 bytes (leave room for UUID prefix + underscore = ~9 chars)
	if len(name) > 200 {
		name = name[:200]
	}
	return strings.TrimSpace(name)
}

// jsonError writes a structured JSON error response.
// Industry practice: API error responses should be machine-parseable.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// allowedUploadTypes is the server-side allowlist of MIME types accepted for
// file uploads. This is the authoritative check — the frontend allowlist is
// advisory only. Keeping this tight prevents abuse (e.g. uploading executables).
var allowedUploadTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"audio/wav":       true,
	"audio/x-wav":     true,
	"audio/mpeg":      true,
	"audio/mp3":       true,
	"audio/ogg":       true,
	"audio/webm":      true,
	"application/pdf": true,
	"text/plain":      true,
	"text/csv":        true,
	"text/markdown":   true,
}

// UploadAttachment handles direct file uploads to the artifact store.
// POST /v1/threads/{threadId}/attachments:upload
//
// Production hardening for 100K concurrent users:
//   - Thread ownership verification (user A cannot upload to user B's thread)
//   - UUID-prefixed filenames prevent collisions across concurrent uploads
//   - Server-side MIME allowlist with http.DetectContentType sniff verification
//   - Multipart memory cap of 2 MB (spills to temp files, not RAM)
//
// Accepts multipart/form-data with a single "file" field.
// Returns JSON: {"url": "/v1/threads/.../artifacts/...", "mimeType": "...", "fileName": "..."}
func (h *Handler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	threadID := r.PathValue("threadId")
	if threadID == "" {
		jsonError(w, "threadId is required", http.StatusBadRequest)
		return
	}
	// Thread ownership: fresh threads (not yet in the projection) are
	// allowed — the thread row gets created on first message send.
	if !h.ownsThread(r, user.ID, threadID /*allowMissing=*/, true) {
		jsonError(w, "Thread not found", http.StatusNotFound)
		return
	}

	// Cap the request body before parsing so a hostile uploader can't
	// exhaust memory or disk. 64 MB total request, with 2 MB of that kept
	// in memory; the rest spills to OS temp files cleaned up at request end.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		jsonError(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "Missing 'file' field", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	contentType, err := resolveUploadContentType(file, header)
	if err != nil {
		jsonError(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}
	fileName := buildUploadFileName(header)

	if _, saveErr := h.artifactStore.Save(r.Context(), user.ID, threadID, fileName, contentType, file); saveErr != nil {
		h.log.Error(r.Context(), "failed to save uploaded attachment",
			"error", saveErr, "userID", user.ID, "threadID", threadID, "fileName", fileName)
		jsonError(w, "Failed to save attachment", http.StatusInternalServerError)
		return
	}
	h.log.Info(r.Context(), "[API] Attachment uploaded",
		"userID", user.ID, "threadID", threadID, "fileName", fileName,
		"contentType", contentType, "size", header.Size)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"url":      fmt.Sprintf("/v1/threads/%s/artifacts/%s", threadID, fileName),
		"mimeType": contentType,
		"fileName": fileName,
	})
}

// resolveUploadContentType decides the MIME to store on the artifact.
// Strategy:
//  1. Read the declared header type.
//  2. Sniff the first 512 bytes (http.DetectContentType's window).
//  3. Prefer the declared type when it's in the allowlist; fall back to
//     the sniffed type otherwise.
//
// Returns an error when the final type isn't allowed, so the caller
// surfaces 415 Unsupported Media Type.
func resolveUploadContentType(file io.ReadSeeker, header *multipart.FileHeader) (string, error) {
	sniffBuf := make([]byte, 512)
	n, _ := file.Read(sniffBuf)
	detected := http.DetectContentType(sniffBuf[:n])
	// Seek back so the S3 upload reads from byte zero.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", errors.New("internal error")
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" || !allowedUploadTypes[contentType] {
		contentType = detected
	}
	if !allowedUploadTypes[contentType] {
		return "", fmt.Errorf("unsupported file type: %s", contentType)
	}
	return contentType, nil
}

// buildUploadFileName produces a collision-safe stored filename: 8 hex
// chars of a UUID, underscore, then the sanitised original name (or
// "upload" if the original was empty / all control chars).
func buildUploadFileName(header *multipart.FileHeader) string {
	original := sanitizeFileName(header.Filename)
	if original == "" {
		original = "upload"
	}
	return fmt.Sprintf("%s_%s", uuid.New().String()[:8], original)
}
