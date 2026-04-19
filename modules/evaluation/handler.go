package evaluation

import (
	"aicompass/foundation/auth"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

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

func writeSSEEvent(w io.Writer, event any, seqID string) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	if seqID != "" {
		_, err = fmt.Fprintf(w, "id: %s\ndata: %s\n\n", seqID, eventBytes)
	} else {
		_, err = fmt.Fprintf(w, "data: %s\n\n", eventBytes)
	}
	if err != nil {
		return fmt.Errorf("failed to write SSE event: %w", err)
	}
	return nil
}

// ─── EvalSet Handlers ────────────────────────────────────────────────────────

// listEvalSets handles GET /api/eval-sets.
func (h *handler) listEvalSets(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
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
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	var req EvalSetCreate
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.UserID = user.ID

	es, err := h.svc.createEvalSet(r.Context(), user.ID, req)
	if handleCreateError(w, err, "create eval set") {
		return
	}

	writeJSON(w, http.StatusCreated, es)
}

// getEvalSet handles GET /api/eval-sets/{id}.
func (h *handler) getEvalSet(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
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
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}

	var req EvalSetUpdate
	if !decodeJSONBody(w, r, &req) {
		return
	}

	err := h.svc.updateEvalSet(r.Context(), id, user.ID, req)
	if err == ErrConflict {
		writeErrorWithField(w, http.StatusConflict, "version",
			"another user modified this eval set. Please refresh and try again.")
		return
	}
	if handleMutationError(w, err, "eval set", "update eval set") {
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteEvalSet handles DELETE /api/eval-sets/{id}.
func (h *handler) deleteEvalSet(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
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
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	evalSetID, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}

	cases, err := h.svc.listEvalCases(r.Context(), evalSetID, user.ID)
	if handleMutationError(w, err, "eval set", "list eval cases") {
		return
	}

	if cases == nil {
		cases = []EvalCase{}
	}

	writeJSON(w, http.StatusOK, cases)
}

// createEvalCase handles POST /api/eval-sets/{id}/cases.
func (h *handler) createEvalCase(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	evalSetID, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}

	var req EvalCaseCreate
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.EvalSetID = evalSetID

	ec, err := h.svc.createEvalCase(r.Context(), user.ID, req)
	if handleMutationError(w, err, "eval set", "create eval case") {
		return
	}

	writeJSON(w, http.StatusCreated, ec)
}

// updateEvalCase handles PUT /api/eval-cases/{id}.
func (h *handler) updateEvalCase(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}

	var req EvalCaseUpdate
	if !decodeJSONBody(w, r, &req) {
		return
	}

	if handleMutationError(w, h.svc.updateEvalCase(r.Context(), id, user.ID, req),
		"eval case", "update eval case") {
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteEvalCase handles DELETE /api/eval-cases/{id}.
func (h *handler) deleteEvalCase(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}

	if handleMutationError(w, h.svc.deleteEvalCase(r.Context(), id, user.ID),
		"eval case", "delete eval case") {
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// reorderEvalCases handles POST /api/eval-sets/{id}/reorder.
func (h *handler) reorderEvalCases(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	evalSetID, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}

	var req struct {
		CaseIDs []string `json:"case_ids"`
	}
	if !decodeJSONBody(w, r, &req) {
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

	if handleMutationError(w,
		h.svc.reorderEvalCases(r.Context(), evalSetID, user.ID, caseIDs),
		"eval set", "reorder eval cases") {
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

// requireUser reads the authenticated user from request context; on miss
// it writes 401 and returns ok=false so the handler can early-return.
func requireUser(w http.ResponseWriter, r *http.Request) (*auth.User, bool) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	return user, true
}

// parsePathUUID parses the named path variable as a UUID; on miss it
// writes 400 and returns ok=false. name is used in both the error
// message and the path lookup.
func parsePathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return uuid.Nil, false
	}
	return id, true
}

// decodeJSONBody decodes the request body into dst; on malformed JSON it
// writes 400 and returns ok=false.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

// handleMutationError maps a service error onto an HTTP response for the
// update/delete/mutation shape: ErrNotFound → 404, ErrUnauthorized → 403,
// "validation error:" prefix → 400, other → 500 with action-labelled log.
// Returns true when a response was written.
func handleMutationError(w http.ResponseWriter, err error, resource, action string) bool {
	if err == nil {
		return false
	}
	switch err {
	case ErrNotFound:
		writeError(w, http.StatusNotFound, resource+" not found")
	case ErrUnauthorized:
		http.Error(w, "unauthorized", http.StatusForbidden)
	default:
		if strings.HasPrefix(err.Error(), "validation error:") {
			writeError(w, http.StatusBadRequest, err.Error())
			return true
		}
		slog.Error(action, "error", err)
		http.Error(w, "Failed to "+action, http.StatusInternalServerError)
	}
	return true
}

// handleCreateError handles the create-shape error set: validation-prefix →
// 400, duplicate-name-prefix → 409 with field=name, default → 500. Returns
// true when a response was written.
func handleCreateError(w http.ResponseWriter, err error, action string) bool {
	if err == nil {
		return false
	}
	switch {
	case strings.HasPrefix(err.Error(), "validation error:"):
		writeError(w, http.StatusBadRequest, err.Error())
	case strings.HasPrefix(err.Error(), "duplicate name:"):
		writeErrorWithField(w, http.StatusConflict, "name", err.Error())
	default:
		slog.Error(action, "error", err)
		http.Error(w, "Failed to "+action, http.StatusInternalServerError)
	}
	return true
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
	user, ok := requireUser(w, r)
	if !ok {
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
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req MetricProfileCreate
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.UserID = user.ID

	p, err := h.svc.createMetricProfile(r.Context(), user.ID, req)
	if handleCreateError(w, err, "create metric profile") {
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// getMetricProfile handles GET /api/evaluation/profiles/{id}.
func (h *handler) getMetricProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
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
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}
	var req MetricProfileUpdate
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if handleMutationError(w, h.svc.updateMetricProfile(r.Context(), id, user.ID, req),
		"metric profile", "update metric profile") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteMetricProfile handles DELETE /api/evaluation/profiles/{id}.
func (h *handler) deleteMetricProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
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
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}
	if handleMutationError(w, h.svc.setDefaultProfile(r.Context(), id, user.ID),
		"metric profile", "set default profile") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Evaluation Run Handlers ─────────────────────────────────────────────────

// createEvalRun handles POST /api/evaluation/runs.
func (h *handler) createEvalRun(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req EvalRunCreate
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.UserID = user.ID

	run, err := h.svc.createEvalRun(r.Context(), user.ID, req)
	if handleMutationError(w, err, "eval set", "create evaluation run") {
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

// listEvalRuns handles GET /api/evaluation/runs.
func (h *handler) listEvalRuns(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	runs, err := h.svc.listEvalRuns(r.Context(), user.ID)
	if err != nil {
		slog.Error("listing evaluation runs", "error", err)
		http.Error(w, "Failed to list evaluation runs", http.StatusInternalServerError)
		return
	}
	if runs == nil {
		runs = []EvalRun{}
	}
	writeJSON(w, http.StatusOK, runs)
}

// getEvalRun handles GET /api/evaluation/runs/{id}.
func (h *handler) getEvalRun(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}
	run, err := h.svc.getEvalRun(r.Context(), id, user.ID)
	if err != nil {
		handleNotFound(w, err, "evaluation run")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// getEvalResults handles GET /api/evaluation/runs/{id}/results.
func (h *handler) getEvalResults(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}
	results, err := h.svc.getEvalResults(r.Context(), id, user.ID)
	if err != nil {
		handleNotFound(w, err, "evaluation run")
		return
	}
	if results == nil {
		results = []EvalResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

// cancelEvalRun handles POST /api/evaluation/runs/{id}/cancel.
func (h *handler) cancelEvalRun(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}
	if err := h.svc.cancelEvalRun(r.Context(), id, user.ID); err != nil {
		switch err {
		case ErrNotFound:
			writeError(w, http.StatusNotFound, "evaluation run not found")
		case ErrUnauthorized:
			http.Error(w, "unauthorized", http.StatusForbidden)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// streamEvalRun handles GET /api/evaluation/runs/{id}/stream.
// Streams evaluation run status changes via SSE.
func (h *handler) streamEvalRun(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}

	// Verify ownership
	run, err := h.svc.getEvalRun(r.Context(), id, user.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "evaluation run not found")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Enable full duplex for HTTP/1.1
	if f, ok := w.(interface{ EnableFullDuplex() error }); ok {
		_ = f.EnableFullDuplex()
	}
	flusher.Flush()

	// Send current status
	_ = writeSSEEvent(w, map[string]any{
		"type":   "status",
		"status": run.Status,
		"run":    run,
	}, "")
	flusher.Flush()

	// If already completed, we're done
	if run.Status != "running" {
		return
	}

	// Poll for updates until completion or error
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			run, err := h.svc.getEvalRun(r.Context(), id, user.ID)
			if err != nil || run.Status != "running" {
				_ = writeSSEEvent(w, map[string]any{
					"type":   "status",
					"status": run.Status,
					"run":    run,
				}, "")
				flusher.Flush()
				return
			}
			_ = writeSSEEvent(w, map[string]any{
				"type":   "progress",
				"status": run.Status,
			}, "")
			flusher.Flush()
		}
	}
}

// deleteEvalRun handles DELETE /api/evaluation/runs/{id}.
func (h *handler) deleteEvalRun(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}
	if err := h.svc.deleteEvalRun(r.Context(), id, user.ID); err != nil {
		handleNotFound(w, err, "evaluation run")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// exportEvalRun handles GET /api/evaluation/runs/{id}/export.
// Exports evaluation results as a JSON file.
func (h *handler) exportEvalRun(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}

	// Get run info
	run, err := h.svc.getEvalRun(r.Context(), id, user.ID)
	if err != nil {
		handleNotFound(w, err, "evaluation run")
		return
	}

	// Get eval set info
	evalSet, err := h.svc.getEvalSet(r.Context(), run.EvalSetID, user.ID)
	if err != nil {
		handleNotFound(w, err, "eval set")
		return
	}

	// Get eval cases
	cases, err := h.svc.listEvalCases(r.Context(), run.EvalSetID, user.ID)
	if err != nil {
		slog.Error("getting eval cases", "error", err)
		http.Error(w, "Failed to get eval cases", http.StatusInternalServerError)
		return
	}

	// Get eval results
	results, err := h.svc.getEvalResults(r.Context(), id, user.ID)
	if err != nil {
		slog.Error("getting eval results", "error", err)
		http.Error(w, "Failed to get eval results", http.StatusInternalServerError)
		return
	}

	// Build export structure
	caseMap := make(map[uuid.UUID]*EvalCase)
	for i, c := range cases {
		caseMap[c.ID] = &cases[i]
	}

	// Group results by case
	resultsByCase := make(map[uuid.UUID][]EvalResult)
	for _, r := range results {
		resultsByCase[r.CaseID] = append(resultsByCase[r.CaseID], r)
	}

	// Build export payload
	type ExportMetric struct {
		MetricID string          `json:"metric_id"`
		Score    *float64        `json:"score,omitempty"`
		Status   string          `json:"status"`
		Details  json.RawMessage `json:"details,omitempty"`
	}

	type ExportCase struct {
		ID      uuid.UUID      `json:"id"`
		Name    *string        `json:"name,omitempty"`
		Input   string         `json:"input"`
		Metrics []ExportMetric `json:"metrics"`
	}

	type ExportPayload struct {
		Run        *EvalRun     `json:"run"`
		EvalSet    *EvalSet     `json:"eval_set"`
		Cases      []ExportCase `json:"cases"`
		ExportedAt time.Time    `json:"exported_at"`
	}

	exportCases := make([]ExportCase, 0)
	for _, c := range cases {
		caseResults := resultsByCase[c.ID]
		metrics := make([]ExportMetric, 0)
		for _, res := range caseResults {
			metrics = append(metrics, ExportMetric{
				MetricID: res.MetricID,
				Score:    res.Score,
				Status:   res.Status,
				Details:  res.Details,
			})
		}
		exportCases = append(exportCases, ExportCase{
			ID:      c.ID,
			Name:    c.Name,
			Input:   c.Input,
			Metrics: metrics,
		})
	}

	payload := ExportPayload{
		Run:        &run,
		EvalSet:    &evalSet,
		Cases:      exportCases,
		ExportedAt: time.Now(),
	}

	// Set response headers for file download
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="eval-results-%s.json"`, id.String()[:8]))

	// Write JSON
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("encoding export", "error", err)
	}
}

// ─── Rubric Handlers ─────────────────────────────────────────────────────────

// listRubrics handles GET /api/evaluation/rubrics.
func (h *handler) listRubrics(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	rubrics, err := h.svc.listRubrics(r.Context(), user.ID)
	if err != nil {
		slog.Error("listing rubrics", "error", err)
		http.Error(w, "Failed to list rubrics", http.StatusInternalServerError)
		return
	}
	if rubrics == nil {
		rubrics = []Rubric{}
	}
	writeJSON(w, http.StatusOK, rubrics)
}

// createRubric handles POST /api/evaluation/rubrics.
func (h *handler) createRubric(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req RubricCreate
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.UserID = user.ID

	rubric, err := h.svc.createRubric(r.Context(), user.ID, req)
	if handleCreateError(w, err, "create rubric") {
		return
	}
	writeJSON(w, http.StatusCreated, rubric)
}

// getRubric handles GET /api/evaluation/rubrics/{id}.
func (h *handler) getRubric(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}
	rubric, err := h.svc.getRubric(r.Context(), id, user.ID)
	if err != nil {
		handleNotFound(w, err, "rubric")
		return
	}
	writeJSON(w, http.StatusOK, rubric)
}

// updateRubric handles PUT /api/evaluation/rubrics/{id}.
func (h *handler) updateRubric(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}
	var req RubricUpdate
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if handleMutationError(w, h.svc.updateRubric(r.Context(), id, user.ID, req),
		"rubric", "update rubric") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteRubric handles DELETE /api/evaluation/rubrics/{id}.
func (h *handler) deleteRubric(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
		return
	}
	if err := h.svc.deleteRubric(r.Context(), id, user.ID); err != nil {
		handleNotFound(w, err, "rubric")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Import / Export Handlers ────────────────────────────────────────────────

// importEvalSet handles POST /api/eval-sets/import.
// Accepts a multipart form with a JSON file.
func (h *handler) importEvalSet(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
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
	if handleCreateError(w, err, "import eval set") {
		return
	}

	writeJSON(w, http.StatusCreated, es)
}

// exportEvalSet handles GET /api/eval-sets/{id}/export.
// Returns a JSON file download.
func (h *handler) exportEvalSet(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	id, idOK := parsePathUUID(w, r, "id")
	if !idOK {
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

// ─── IDE Test Runner Endpoints ───────────────────────────────────────────────
