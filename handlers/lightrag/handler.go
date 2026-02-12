package lightrag

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"

	component "school-with-ai/components/lightrag"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"

	"github.com/go-chi/chi/v5"
	"github.com/lightrag/lightrag-go/pkg/core/base"
	ragcore "github.com/lightrag/lightrag-go/pkg/core/lightrag"
)

// Handler provides HTTP handlers for the LightRAG dashboard.
type Handler struct {
	log       *logger.Logger
	rag       *ragcore.LightRAG
	adminUser string
	adminPass string
}

// NewHandler creates a new LightRAG dashboard handler.
func NewHandler(log *logger.Logger, rag *ragcore.LightRAG, adminUser, adminPass string) *Handler {
	return &Handler{
		log:       log,
		rag:       rag,
		adminUser: adminUser,
		adminPass: adminPass,
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
}

// ListDocuments returns a paginated list of documents.
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	statuses, err := h.rag.GetDocStatuses(ctx)
	if err != nil {
		h.log.Error(ctx, "lightrag: list documents", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list documents"})
		return
	}

	// Build flat list
	docs := make([]docEntry, 0, len(statuses))
	for id, data := range statuses {
		docs = append(docs, docEntry{
			ID:        id,
			FilePath:  strVal(data, "file_path"),
			Status:    strVal(data, "status"),
			Summary:   strVal(data, "content_summary"),
			Length:    intVal(data, "content_length"),
			CreatedAt: strVal(data, "created_at"),
			UpdatedAt: strVal(data, "updated_at"),
		})
	}

	// Sort by created_at descending (newest first)
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].CreatedAt > docs[j].CreatedAt
	})

	// Paginate
	total := len(docs)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"documents":   docs[start:end],
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
	})
}

// UploadDocument saves an uploaded file to the RAG working directory and registers it.
// The file is stored as-is (binary-safe) — no content parsing or text extraction.
func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing file field"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read upload"})
		return
	}

	if err := h.rag.RegisterFile(ctx, header.Filename, data); err != nil {
		h.log.Error(ctx, "lightrag: register file", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Document '" + header.Filename + "' uploaded",
	})
}

// DeleteDocument deletes a single document by ID.
func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docID := chi.URLParam(r, "id")

	if docID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document ID required"})
		return
	}

	if err := h.rag.Delete(ctx, docID); err != nil {
		h.log.Error(ctx, "lightrag: delete document", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
