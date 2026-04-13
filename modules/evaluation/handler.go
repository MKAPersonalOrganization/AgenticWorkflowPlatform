package evaluation

import (
	"aicompass/shared"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// handler provides HTTP endpoints for evaluation management.
type handler struct {
	svc *service
}

// newHandler creates a new evaluation handler.
func newHandler(svc *service) *handler {
	return &handler{svc: svc}
}

// ─── EvalSet Handlers ────────────────────────────────────────────────────────

// listEvalSets handles GET /api/eval-sets.
func (h *handler) listEvalSets(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sets, err := h.svc.listEvalSets(r.Context(), user.ID)
	if err != nil {
		slog.Error("listing eval_sets", "error", err)
		http.Error(w, "Failed to list eval sets", http.StatusInternalServerError)
		return
	}

	if sets == nil {
		sets = []EvalSet{}
	}

	writeJSON(w, http.StatusOK, sets)
}

// createEvalSet handles POST /api/eval-sets.
func (h *handler) createEvalSet(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req EvalSetCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.UserID = user.ID

	es, err := h.svc.createEvalSet(r.Context(), user.ID, req)
	if err != nil {
		switch {
		case err.Error() == "validation error: name is required" ||
			err.Error() == "validation error: name must be at most 255 characters" ||
			err.Error() == "validation error: name contains invalid characters":
			writeError(w, http.StatusBadRequest, err.Error())
		case strings.HasPrefix(err.Error(), "duplicate name:"):
			writeErrorWithField(w, http.StatusConflict, "name", err.Error())
		default:
			slog.Error("creating eval_set", "error", err)
			http.Error(w, "Failed to create eval set", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusCreated, es)
}

// getEvalSet handles GET /api/eval-sets/{id}.
func (h *handler) getEvalSet(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	es, err := h.svc.getEvalSet(r.Context(), id, user.ID)
	if err != nil {
		handleNotFound(w, err, "eval set")
		return
	}

	writeJSON(w, http.StatusOK, es)
}

// updateEvalSet handles PUT /api/eval-sets/{id}.
func (h *handler) updateEvalSet(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req EvalSetUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.updateEvalSet(r.Context(), id, user.ID, req); err != nil {
		switch err {
		case ErrNotFound:
			writeError(w, http.StatusNotFound, "eval set not found")
		case ErrUnauthorized:
			http.Error(w, "unauthorized", http.StatusForbidden)
		case ErrConflict:
			writeErrorWithField(w, http.StatusConflict, "version", "another user modified this eval set. Please refresh and try again.")
		default:
			if strings.HasPrefix(err.Error(), "validation error:") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			slog.Error("updating eval_set", "error", err)
			http.Error(w, "Failed to update eval set", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteEvalSet handles DELETE /api/eval-sets/{id}.
func (h *handler) deleteEvalSet(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.deleteEvalSet(r.Context(), id, user.ID); err != nil {
		handleNotFound(w, err, "eval set")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── EvalCase Handlers ───────────────────────────────────────────────────────

// listEvalCases handles GET /api/eval-sets/{id}/cases.
func (h *handler) listEvalCases(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	evalSetID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	cases, err := h.svc.listEvalCases(r.Context(), evalSetID, user.ID)
	if err != nil {
		switch err {
		case ErrNotFound:
			writeError(w, http.StatusNotFound, "eval set not found")
		case ErrUnauthorized:
			http.Error(w, "unauthorized", http.StatusForbidden)
		default:
			slog.Error("listing eval_cases", "error", err)
			http.Error(w, "Failed to list eval cases", http.StatusInternalServerError)
		}
		return
	}

	if cases == nil {
		cases = []EvalCase{}
	}

	writeJSON(w, http.StatusOK, cases)
}

// createEvalCase handles POST /api/eval-sets/{id}/cases.
func (h *handler) createEvalCase(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	evalSetID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req EvalCaseCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.EvalSetID = evalSetID

	ec, err := h.svc.createEvalCase(r.Context(), user.ID, req)
	if err != nil {
		switch err {
		case ErrNotFound:
			writeError(w, http.StatusNotFound, "eval set not found")
		case ErrUnauthorized:
			http.Error(w, "unauthorized", http.StatusForbidden)
		default:
			if strings.HasPrefix(err.Error(), "validation error:") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			slog.Error("creating eval_case", "error", err)
			http.Error(w, "Failed to create eval case", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusCreated, ec)
}

// updateEvalCase handles PUT /api/eval-cases/{id}.
func (h *handler) updateEvalCase(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req EvalCaseUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.updateEvalCase(r.Context(), id, user.ID, req); err != nil {
		switch err {
		case ErrNotFound:
			writeError(w, http.StatusNotFound, "eval case not found")
		case ErrUnauthorized:
			http.Error(w, "unauthorized", http.StatusForbidden)
		default:
			if strings.HasPrefix(err.Error(), "validation error:") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			slog.Error("updating eval_case", "error", err)
			http.Error(w, "Failed to update eval case", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteEvalCase handles DELETE /api/eval-cases/{id}.
func (h *handler) deleteEvalCase(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.deleteEvalCase(r.Context(), id, user.ID); err != nil {
		switch err {
		case ErrNotFound:
			writeError(w, http.StatusNotFound, "eval case not found")
		case ErrUnauthorized:
			http.Error(w, "unauthorized", http.StatusForbidden)
		default:
			slog.Error("deleting eval_case", "error", err)
			http.Error(w, "Failed to delete eval case", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// reorderEvalCases handles POST /api/eval-sets/{id}/reorder.
func (h *handler) reorderEvalCases(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	evalSetID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req struct {
		CaseIDs []string `json:"case_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	caseIDs := make([]uuid.UUID, 0, len(req.CaseIDs))
	for _, s := range req.CaseIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid case_id: "+s)
			return
		}
		caseIDs = append(caseIDs, id)
	}

	if err := h.svc.reorderEvalCases(r.Context(), evalSetID, user.ID, caseIDs); err != nil {
		switch err {
		case ErrNotFound:
			writeError(w, http.StatusNotFound, "eval set not found")
		case ErrUnauthorized:
			http.Error(w, "unauthorized", http.StatusForbidden)
		default:
			if strings.HasPrefix(err.Error(), "validation error:") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			slog.Error("reordering eval_cases", "error", err)
			http.Error(w, "Failed to reorder eval cases", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Response helpers ────────────────────────────────────────────────────────

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writing json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}

func writeErrorWithField(w http.ResponseWriter, status int, field, message string) {
	writeJSON(w, status, errorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Field:   field,
	})
}

func handleNotFound(w http.ResponseWriter, err error, resource string) {
	switch err {
	case ErrNotFound:
		writeError(w, http.StatusNotFound, resource+" not found")
	case ErrUnauthorized:
		http.Error(w, "unauthorized", http.StatusForbidden)
	default:
		slog.Error("getting "+resource, "error", err)
		http.Error(w, "Failed to get "+resource, http.StatusInternalServerError)
	}
}

// ─── Metric Catalog Handlers ─────────────────────────────────────────────────

// listMetrics handles GET /api/evaluation/metrics.
func (h *handler) listMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := GetMetricCatalog()
	if metrics == nil {
		metrics = []MetricDefinition{}
	}
	writeJSON(w, http.StatusOK, metrics)
}

// getMetric handles GET /api/evaluation/metrics/{id}.
func (h *handler) getMetric(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, ok := GetMetricByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "metric not found: "+id)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// ─── Metric Profile Handlers ─────────────────────────────────────────────────

// listMetricProfiles handles GET /api/evaluation/profiles.
func (h *handler) listMetricProfiles(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	profiles, err := h.svc.listMetricProfiles(r.Context(), user.ID)
	if err != nil {
		slog.Error("listing metric profiles", "error", err)
		http.Error(w, "Failed to list metric profiles", http.StatusInternalServerError)
		return
	}
	if profiles == nil {
		profiles = []MetricProfile{}
	}
	writeJSON(w, http.StatusOK, profiles)
}

// createMetricProfile handles POST /api/evaluation/profiles.
func (h *handler) createMetricProfile(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req MetricProfileCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.UserID = user.ID

	p, err := h.svc.createMetricProfile(r.Context(), user.ID, req)
	if err != nil {
		switch {
		case strings.HasPrefix(err.Error(), "validation error:"):
			writeError(w, http.StatusBadRequest, err.Error())
		case strings.HasPrefix(err.Error(), "duplicate name:"):
			writeErrorWithField(w, http.StatusConflict, "name", err.Error())
		default:
			slog.Error("creating metric profile", "error", err)
			http.Error(w, "Failed to create metric profile", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// getMetricProfile handles GET /api/evaluation/profiles/{id}.
func (h *handler) getMetricProfile(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	p, err := h.svc.getMetricProfile(r.Context(), id, user.ID)
	if err != nil {
		handleNotFound(w, err, "metric profile")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// updateMetricProfile handles PUT /api/evaluation/profiles/{id}.
func (h *handler) updateMetricProfile(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req MetricProfileUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.updateMetricProfile(r.Context(), id, user.ID, req); err != nil {
		switch err {
		case ErrNotFound:
			writeError(w, http.StatusNotFound, "metric profile not found")
		case ErrUnauthorized:
			http.Error(w, "unauthorized", http.StatusForbidden)
		default:
			if strings.HasPrefix(err.Error(), "validation error:") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			slog.Error("updating metric profile", "error", err)
			http.Error(w, "Failed to update metric profile", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteMetricProfile handles DELETE /api/evaluation/profiles/{id}.
func (h *handler) deleteMetricProfile(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.deleteMetricProfile(r.Context(), id, user.ID); err != nil {
		handleNotFound(w, err, "metric profile")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setDefaultProfile handles POST /api/evaluation/profiles/{id}/set-default.
func (h *handler) setDefaultProfile(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.setDefaultProfile(r.Context(), id, user.ID); err != nil {
		switch err {
		case ErrNotFound:
			writeError(w, http.StatusNotFound, "metric profile not found")
		case ErrUnauthorized:
			http.Error(w, "unauthorized", http.StatusForbidden)
		default:
			slog.Error("setting default profile", "error", err)
			http.Error(w, "Failed to set default profile", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Import / Export Handlers ────────────────────────────────────────────────

// importEvalSet handles POST /api/eval-sets/import.
// Accepts a multipart form with a JSON file.
func (h *handler) importEvalSet(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Limit upload to 10 MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form: file too large or invalid")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field is required in multipart form")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, 10<<20))
	if err != nil {
		slog.Error("reading upload file", "error", err)
		http.Error(w, "Failed to read upload", http.StatusInternalServerError)
		return
	}

	es, err := h.svc.importEvalSet(r.Context(), user.ID, data)
	if err != nil {
		switch {
		case strings.HasPrefix(err.Error(), "validation error:"):
			writeError(w, http.StatusBadRequest, err.Error())
		case strings.HasPrefix(err.Error(), "duplicate name:"):
			writeErrorWithField(w, http.StatusConflict, "name", err.Error())
		default:
			slog.Error("importing eval_set", "error", err)
			http.Error(w, "Failed to import eval set", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusCreated, es)
}

// exportEvalSet handles GET /api/eval-sets/{id}/export.
// Returns a JSON file download.
func (h *handler) exportEvalSet(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	export, err := h.svc.exportEvalSet(r.Context(), id, user.ID)
	if err != nil {
		handleNotFound(w, err, "eval set")
		return
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		slog.Error("marshaling export", "error", err)
		http.Error(w, "Failed to marshal export", http.StatusInternalServerError)
		return
	}

	// Sanitize filename for Content-Disposition
	safeName := strings.Map(func(r rune) rune {
		if r >= 32 && r != '/' && r != '\\' && r != ':' && r != '*' && r != '?' && r != '"' && r != '<' && r != '>' && r != '|' {
			return r
		}
		return '_'
	}, export.Name)
	if safeName == "" {
		safeName = "evalset"
	}
	filename := fmt.Sprintf("%s.json", safeName)

	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", disposition)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		slog.Error("writing export response", "error", err)
	}
}
