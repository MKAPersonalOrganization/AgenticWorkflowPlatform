package filesystem

import (
	"aicompass/shared"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type handler struct {
	svc *service
}

func newHandler(svc *service) *handler {
	return &handler{svc: svc}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func userFromRequest(r *http.Request) (*shared.User, bool) {
	u := shared.GetUserFromContext(r.Context())
	return u, u != nil
}

// ── Filesystem CRUD ─────────────────────────────────────────────────────────

func (h *handler) createFilesystem(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateFilesystemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fs, err := h.svc.createFilesystem(r.Context(), user.ID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, fs)
}

func (h *handler) listFilesystems(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	list, err := h.svc.listFilesystems(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list filesystems")
		return
	}
	if list == nil {
		list = []FileSystem{}
	}

	writeJSON(w, http.StatusOK, list)
}

func (h *handler) getFilesystem(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	fs, err := h.svc.getFilesystem(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "filesystem not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get filesystem")
		return
	}

	writeJSON(w, http.StatusOK, fs)
}

func (h *handler) updateFilesystem(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	var req UpdateFilesystemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fs, err := h.svc.updateFilesystem(r.Context(), id, user.ID, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "filesystem not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update filesystem")
		return
	}

	writeJSON(w, http.StatusOK, fs)
}

func (h *handler) deleteFilesystem(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	if err := h.svc.deleteFilesystem(r.Context(), id, user.ID); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "filesystem not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete filesystem")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── File/Directory Operations ───────────────────────────────────────────────

func (h *handler) listDirectory(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		dirPath = "/"
	}

	entries, err := h.svc.listDir(r.Context(), id, user.ID, dirPath)
	if err != nil {
		status := errorToStatus(err)
		writeError(w, status, err.Error())
		return
	}
	if entries == nil {
		entries = []DirEntry{}
	}

	writeJSON(w, http.StatusOK, entries)
}

func (h *handler) createDirectory(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	var req MkdirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.mkdir(r.Context(), id, user.ID, req.Path); err != nil {
		status := errorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (h *handler) uploadFile(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	// Parse multipart form (32MB limit).
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	filePath := r.FormValue("path")
	if filePath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	stat, err := h.svc.uploadFile(r.Context(), id, user.ID, filePath, file, contentType, header.Size)
	if err != nil {
		status := errorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, stat)
}

func (h *handler) readFile(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	rc, stat, err := h.svc.readFile(r.Context(), id, user.ID, filePath)
	if err != nil {
		status := errorToStatus(err)
		writeError(w, status, err.Error())
		return
	}
	defer rc.Close()

	if stat.ContentType != "" {
		w.Header().Set("Content-Type", stat.ContentType)
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, stat.Name))
	if stat.Size > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size))
	}

	io.Copy(w, rc)
}

func (h *handler) removeEntry(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	entryPath := r.URL.Query().Get("path")
	if entryPath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	if err := h.svc.removeEntry(r.Context(), id, user.ID, entryPath); err != nil {
		status := errorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) renameEntry(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	var req RenameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.renameEntry(r.Context(), id, user.ID, req); err != nil {
		status := errorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "renamed"})
}

// ── Permissions ─────────────────────────────────────────────────────────────

func (h *handler) grantPermission(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	var req PermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.grantPermission(r.Context(), id, user.ID, req.AgentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "granted"})
}

func (h *handler) revokePermission(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	agentID := chi.URLParam(r, "agentId")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	if err := h.svc.revokePermission(r.Context(), id, user.ID, agentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *handler) listPermissions(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filesystem id")
		return
	}

	perms, err := h.svc.listPermissions(r.Context(), id, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if perms == nil {
		perms = []FSPermission{}
	}

	writeJSON(w, http.StatusOK, perms)
}

func (h *handler) listAgentFilesystems(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	agentID := chi.URLParam(r, "agentId")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	list, err := h.svc.listAgentFilesystems(r.Context(), user.ID, agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agent filesystems")
		return
	}

	writeJSON(w, http.StatusOK, list)
}

func (h *handler) toggleAgentPermission(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	agentID := chi.URLParam(r, "agentId")
	fsID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil || agentID == "" {
		writeError(w, http.StatusBadRequest, "invalid agent or filesystem id")
		return
	}

	var req ToggleAgentPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Enabled {
		err = h.svc.grantPermission(r.Context(), fsID, user.ID, agentID)
	} else {
		err = h.svc.revokePermission(r.Context(), fsID, user.ID, agentID)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Error mapping ───────────────────────────────────────────────────────────

func errorToStatus(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, ErrNotDirectory):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotFile):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotEmpty):
		return http.StatusConflict
	case errors.Is(err, ErrReadOnly):
		return http.StatusForbidden
	case errors.Is(err, ErrInvalidPath):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
