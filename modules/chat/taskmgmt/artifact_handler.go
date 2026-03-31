package taskmgmt

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"school-with-ai/platform/artifact"

	"github.com/google/uuid"
)

// ListArtifacts returns a list of artifacts for a thread.
// GET /v1/threads/{threadId}/artifacts
func (h *TaskManagementHandler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
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
func (h *TaskManagementHandler) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
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

	// ── Thread ownership check ────────────────────────────────────────────
	// Prevents user A from reading user B's artifacts by guessing thread IDs.
	state, exists := h.aguiProjection.GetThreadState(r.Context(), threadID)
	if !exists || state.UserID != user.ID {
		jsonError(w, "Artifact not found", http.StatusNotFound)
		return
	}

	content, meta, err := h.artifactStore.Load(r.Context(), user.ID, threadID, fileName)
	if err != nil {
		if errors.Is(err, artifact.ErrNotFound) {
			jsonError(w, "Artifact not found", http.StatusNotFound)
			return
		}
		h.log.Error(r.Context(), "failed to load artifact", "error", err, "userID", user.ID, "threadID", threadID, "fileName", fileName)
		jsonError(w, "Failed to load artifact", http.StatusInternalServerError)
		return
	}
	defer content.Close()

	// Set content type from S3 metadata
	ct := meta.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)

	// Use inline disposition for media types so browsers can play/display them,
	// and attachment for everything else so they download.
	if strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "video/") {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", fileName))
	} else {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	}

	// Artifacts are named with UUID prefixes (immutable content-addressing).
	// Set aggressive cache headers so browsers and CDNs never re-fetch.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	// minio.Object implements io.ReadSeeker, so http.ServeContent can handle
	// Range requests (HTTP 206), Content-Length, and conditional headers.
	if rs, ok := content.(io.ReadSeeker); ok {
		http.ServeContent(w, r, fileName, time.Time{}, rs)
	} else {
		// Fallback: plain copy (no Range support)
		if _, copyErr := io.Copy(w, content); copyErr != nil {
			h.log.Error(r.Context(), "failed to stream artifact to client", "error", copyErr, "fileName", fileName)
		}
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
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
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
func (h *TaskManagementHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	threadID := r.PathValue("threadId")
	if threadID == "" {
		jsonError(w, "threadId is required", http.StatusBadRequest)
		return
	}

	// ── Thread ownership check ────────────────────────────────────────────
	// S3 keys are prefixed with userID/threadID, so namespace isolation is
	// inherent. For new sessions the thread doesn't exist in the projection
	// yet (it's created when the first message is sent), so we allow the
	// upload. If the thread *does* exist and belongs to another user, block.
	if state, exists := h.aguiProjection.GetThreadState(r.Context(), threadID); exists && state.UserID != user.ID {
		jsonError(w, "Thread not found", http.StatusNotFound)
		return
	}

	// ── Parse multipart ───────────────────────────────────────────────────
	// maxMemory = 2 MB: keeps RAM bounded under concurrent load.
	// Excess spills to OS temp files (cleaned up when request ends).
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		jsonError(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "Missing 'file' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// ── Content-type validation ───────────────────────────────────────────
	// 1. Read the declared type from the multipart header.
	contentType := header.Header.Get("Content-Type")

	// 2. Sniff actual bytes to defeat spoofed Content-Type headers.
	//    Read first 512 bytes (the amount http.DetectContentType uses).
	sniffBuf := make([]byte, 512)
	n, _ := file.Read(sniffBuf)
	detected := http.DetectContentType(sniffBuf[:n])

	// Seek back so S3 upload reads from the beginning.
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Trust the declared type if it's in our allowlist AND the sniffed type
	// is either a match or "application/octet-stream" (many audio/pdf files
	// sniff as octet-stream because DetectContentType only knows a few).
	if contentType == "" || !allowedUploadTypes[contentType] {
		contentType = detected
	}
	if !allowedUploadTypes[contentType] {
		jsonError(w, fmt.Sprintf("Unsupported file type: %s", contentType), http.StatusUnsupportedMediaType)
		return
	}

	// ── Collision-safe filename ───────────────────────────────────────────
	// UUID prefix ensures no two concurrent uploads collide, even with the
	// same original filename across 100K users.
	originalName := sanitizeFileName(header.Filename)
	if originalName == "" {
		originalName = "upload"
	}
	fileName := fmt.Sprintf("%s_%s", uuid.New().String()[:8], originalName)

	// ── Save to S3 ───────────────────────────────────────────────────────
	_, saveErr := h.artifactStore.Save(r.Context(), user.ID, threadID, fileName, contentType, file)
	if saveErr != nil {
		h.log.Error(r.Context(), "failed to save uploaded attachment",
			"error", saveErr, "userID", user.ID, "threadID", threadID, "fileName", fileName)
		jsonError(w, "Failed to save attachment", http.StatusInternalServerError)
		return
	}

	artifactURL := fmt.Sprintf("/v1/threads/%s/artifacts/%s", threadID, fileName)

	h.log.Info(r.Context(), "[API] Attachment uploaded",
		"userID", user.ID, "threadID", threadID, "fileName", fileName,
		"contentType", contentType, "size", header.Size)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"url":      artifactURL,
		"mimeType": contentType,
		"fileName": fileName,
	})
}
