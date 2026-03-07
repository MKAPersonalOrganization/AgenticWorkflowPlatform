package knowledge

// Extended handler methods for all additional Edgequake API endpoints.
// These endpoints cover: workspace updates, document lineage/metadata,
// graph entities/relationships, tasks, costs, lineage, chunks, provenance,
// settings, and models.

import (
	"encoding/json"
	"net/http"
	"school-with-ai/modules/knowledge/rag/costs"
	"school-with-ai/modules/knowledge/rag/entities"
	"school-with-ai/modules/knowledge/rag/pipeline"
	"school-with-ai/modules/knowledge/rag/relationships"
	"school-with-ai/modules/knowledge/rag/tasks"
	"school-with-ai/modules/knowledge/rag/workspaces"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ============================================================================
// Workspaces — Update
// ============================================================================

// UpdateWorkspace updates an existing workspace.
func (h *handler) UpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid workspace ID"})
		return
	}

	var req workspaces.UpdateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := h.edgequakeService.UpdateWorkspace(r.Context(), tCtx, workspaceID, &req)
	if err != nil {
		h.log.Error(r.Context(), "failed to update workspace", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update workspace"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetWorkspaceMetricsHistory returns metrics history for a workspace.
func (h *handler) GetWorkspaceMetricsHistory(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid workspace ID"})
		return
	}

	result, err := h.edgequakeService.Workspaces.GetMetricsHistory(r.Context(), tCtx, workspaceID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get metrics history", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get metrics history"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// TriggerMetricsSnapshot triggers a manual metrics snapshot.
func (h *handler) TriggerMetricsSnapshot(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid workspace ID"})
		return
	}

	result, err := h.edgequakeService.Workspaces.TriggerMetricsSnapshot(r.Context(), tCtx, workspaceID)
	if err != nil {
		h.log.Error(r.Context(), "failed to trigger metrics snapshot", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to trigger metrics snapshot"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetWorkspaceBySlug retrieves a workspace by its slug.
func (h *handler) GetWorkspaceBySlug(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing slug"})
		return
	}

	result, err := h.edgequakeService.Workspaces.GetBySlug(r.Context(), tCtx, slug)
	if err != nil {
		h.log.Error(r.Context(), "failed to get workspace by slug", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get workspace"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Documents — Extended
// ============================================================================

// GetTrackStatus returns the status of a document tracking operation.
func (h *handler) GetTrackStatus(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	trackID := chi.URLParam(r, "trackId")
	result, err := h.edgequakeService.Documents.GetTrackStatus(r.Context(), tCtx, trackID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get track status", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get track status"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// RecoverStuckDocuments recovers documents stuck in processing.
func (h *handler) RecoverStuckDocuments(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	result, err := h.edgequakeService.Documents.RecoverStuck(r.Context(), tCtx)
	if err != nil {
		h.log.Error(r.Context(), "failed to recover stuck documents", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to recover stuck documents"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetDeletionImpact analyzes the impact of deleting a document.
func (h *handler) GetDeletionImpact(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	documentID := chi.URLParam(r, "documentId")
	result, err := h.edgequakeService.Documents.GetDeletionImpact(r.Context(), tCtx, documentID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get deletion impact", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get deletion impact"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// RetryFailedChunks retries failed chunks for a document.
func (h *handler) RetryFailedChunks(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	documentID := chi.URLParam(r, "documentId")
	result, err := h.edgequakeService.Documents.RetryFailedChunks(r.Context(), tCtx, documentID)
	if err != nil {
		h.log.Error(r.Context(), "failed to retry failed chunks", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to retry chunks"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListFailedChunks lists failed chunks for a document.
func (h *handler) ListFailedChunks(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	documentID := chi.URLParam(r, "documentId")
	result, err := h.edgequakeService.Documents.ListFailedChunks(r.Context(), tCtx, documentID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list failed chunks", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to list failed chunks"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetDocumentFullLineage retrieves full lineage for a document.
func (h *handler) GetDocumentFullLineage(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	documentID := chi.URLParam(r, "documentId")
	result, err := h.edgequakeService.Documents.GetFullLineage(r.Context(), tCtx, documentID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get document lineage", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get document lineage"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetDocumentMetadata retrieves metadata for a document.
func (h *handler) GetDocumentMetadata(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	documentID := chi.URLParam(r, "documentId")
	result, err := h.edgequakeService.Documents.GetMetadata(r.Context(), tCtx, documentID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get document metadata", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get document metadata"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ExportDocumentLineage exports lineage for a document.
func (h *handler) ExportDocumentLineage(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	documentID := chi.URLParam(r, "documentId")
	format := r.URL.Query().Get("format")
	result, err := h.edgequakeService.Documents.ExportLineage(r.Context(), tCtx, documentID, format)
	if err != nil {
		h.log.Error(r.Context(), "failed to export document lineage", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to export lineage"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetPDFStatus retrieves PDF processing status.
func (h *handler) GetPDFStatus(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pdfID := chi.URLParam(r, "pdfId")
	result, err := h.edgequakeService.GetPDFStatus(r.Context(), tCtx, pdfID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get PDF status", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get PDF status"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CancelPDFProcessing cancels PDF processing.
func (h *handler) CancelPDFProcessing(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pdfID := chi.URLParam(r, "pdfId")
	result, err := h.edgequakeService.Documents.CancelPdfProcessing(r.Context(), tCtx, pdfID)
	if err != nil {
		h.log.Error(r.Context(), "failed to cancel PDF processing", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to cancel PDF processing"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Graph — Degrees Batch
// ============================================================================

// GetDegreesBatch retrieves degrees for multiple nodes.
func (h *handler) GetDegreesBatch(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		NodeIDs []string `json:"node_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := h.edgequakeService.Graph.GetDegreesBatch(r.Context(), *tCtx, req.NodeIDs)
	if err != nil {
		h.log.Error(r.Context(), "failed to get degrees batch", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get degrees"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Graph Entities
// ============================================================================

// ListEntities returns a list of entities.
func (h *handler) ListEntities(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	params := &entities.ListEntitiesParams{
		Page:       intFromQuery(r, "page", 1),
		PageSize:   intFromQuery(r, "page_size", 50),
		EntityType: r.URL.Query().Get("entity_type"),
		Search:     r.URL.Query().Get("search"),
	}

	result, err := h.edgequakeService.Entities.ListEntities(r.Context(), *tCtx, params)
	if err != nil {
		h.log.Error(r.Context(), "failed to list entities", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to list entities"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateEntity creates a new entity.
func (h *handler) CreateEntity(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req entities.CreateEntityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := h.edgequakeService.Entities.CreateEntity(r.Context(), *tCtx, req)
	if err != nil {
		h.log.Error(r.Context(), "failed to create entity", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create entity"})
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// EntityExists checks if an entity exists.
func (h *handler) EntityExists(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := r.URL.Query().Get("name")
	result, err := h.edgequakeService.Entities.EntityExists(r.Context(), *tCtx, name)
	if err != nil {
		h.log.Error(r.Context(), "failed to check entity existence", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to check entity"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// MergeEntities merges two entities.
func (h *handler) MergeEntities(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req entities.MergeEntitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := h.edgequakeService.Entities.MergeEntities(r.Context(), *tCtx, req)
	if err != nil {
		h.log.Error(r.Context(), "failed to merge entities", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to merge entities"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetEntity retrieves a specific entity.
func (h *handler) GetEntity(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := chi.URLParam(r, "entityName")
	result, err := h.edgequakeService.Entities.GetEntity(r.Context(), *tCtx, name)
	if err != nil {
		h.log.Error(r.Context(), "failed to get entity", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get entity"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateEntity updates an entity.
func (h *handler) UpdateEntity(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := chi.URLParam(r, "entityName")
	var req entities.UpdateEntityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := h.edgequakeService.Entities.UpdateEntity(r.Context(), *tCtx, name, req)
	if err != nil {
		h.log.Error(r.Context(), "failed to update entity", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update entity"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteEntity deletes an entity.
func (h *handler) DeleteEntity(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := chi.URLParam(r, "entityName")
	confirm := r.URL.Query().Get("confirm") == "true"
	result, err := h.edgequakeService.Entities.DeleteEntity(r.Context(), *tCtx, name, confirm)
	if err != nil {
		h.log.Error(r.Context(), "failed to delete entity", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete entity"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetEntityNeighborhood retrieves the neighborhood of an entity.
func (h *handler) GetEntityNeighborhood(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := chi.URLParam(r, "entityName")
	depth := intFromQuery(r, "depth", 1)
	result, err := h.edgequakeService.Entities.GetEntityNeighborhood(r.Context(), *tCtx, name, depth)
	if err != nil {
		h.log.Error(r.Context(), "failed to get entity neighborhood", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get neighborhood"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Graph Relationships
// ============================================================================

// ListRelationships returns a list of relationships.
func (h *handler) ListRelationships(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	params := &relationships.ListRelationshipsParams{
		Page:             intFromQuery(r, "page", 1),
		PageSize:         intFromQuery(r, "page_size", 50),
		RelationshipType: r.URL.Query().Get("relationship_type"),
	}

	result, err := h.edgequakeService.Relationships.ListRelationships(r.Context(), *tCtx, params)
	if err != nil {
		h.log.Error(r.Context(), "failed to list relationships", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to list relationships"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateRelationship creates a new relationship.
func (h *handler) CreateRelationship(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req relationships.CreateRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := h.edgequakeService.Relationships.CreateRelationship(r.Context(), *tCtx, req)
	if err != nil {
		h.log.Error(r.Context(), "failed to create relationship", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create relationship"})
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// GetRelationship retrieves a specific relationship.
func (h *handler) GetRelationship(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	relID := chi.URLParam(r, "relationshipId")
	result, err := h.edgequakeService.Relationships.GetRelationship(r.Context(), *tCtx, relID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get relationship", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get relationship"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateRelationship updates a relationship.
func (h *handler) UpdateRelationship(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	relID := chi.URLParam(r, "relationshipId")
	var req relationships.UpdateRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := h.edgequakeService.Relationships.UpdateRelationship(r.Context(), *tCtx, relID, req)
	if err != nil {
		h.log.Error(r.Context(), "failed to update relationship", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update relationship"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteRelationship deletes a relationship.
func (h *handler) DeleteRelationship(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	relID := chi.URLParam(r, "relationshipId")
	result, err := h.edgequakeService.Relationships.DeleteRelationship(r.Context(), *tCtx, relID)
	if err != nil {
		h.log.Error(r.Context(), "failed to delete relationship", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete relationship"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Tasks
// ============================================================================

// GetTask retrieves a task by track ID.
func (h *handler) GetTask(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	trackID := chi.URLParam(r, "trackId")
	result, err := h.edgequakeService.Tasks.Get(r.Context(), tCtx, trackID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get task", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get task"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListTasks returns a list of tasks.
func (h *handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	page := uint32(intFromQuery(r, "page", 1))
	pageSize := uint32(intFromQuery(r, "page_size", 50))
	status := r.URL.Query().Get("status")
	taskType := r.URL.Query().Get("task_type")
	sort := r.URL.Query().Get("sort")
	order := r.URL.Query().Get("order")

	params := &tasks.ListTasksParams{}
	if page > 0 {
		params.Page = &page
	}
	if pageSize > 0 {
		params.PageSize = &pageSize
	}
	if status != "" {
		params.Status = &status
	}
	if taskType != "" {
		params.TaskType = &taskType
	}
	if sort != "" {
		params.Sort = &sort
	}
	if order != "" {
		params.Order = &order
	}
	if tCtx.WorkspaceID != nil {
		params.WorkspaceID = tCtx.WorkspaceID
	}

	result, err := h.edgequakeService.Tasks.List(r.Context(), tCtx, params)
	if err != nil {
		h.log.Error(r.Context(), "failed to list tasks", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to list tasks"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Costs
// ============================================================================

// GetCostSummary returns the cost summary.
func (h *handler) GetCostSummary(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	result, err := h.edgequakeService.Costs.Summary(r.Context(), tCtx)
	if err != nil {
		h.log.Error(r.Context(), "failed to get cost summary", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get cost summary"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetCostHistory returns cost history.
func (h *handler) GetCostHistory(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	params := &costs.CostHistoryParams{}
	if v := r.URL.Query().Get("start_date"); v != "" {
		params.StartDate = &v
	}
	if v := r.URL.Query().Get("end_date"); v != "" {
		params.EndDate = &v
	}
	if v := r.URL.Query().Get("granularity"); v != "" {
		params.Granularity = &v
	}

	result, err := h.edgequakeService.Costs.History(r.Context(), tCtx, params)
	if err != nil {
		h.log.Error(r.Context(), "failed to get cost history", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get cost history"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetBudget returns the budget status.
func (h *handler) GetBudget(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	result, err := h.edgequakeService.Costs.GetBudget(r.Context(), tCtx)
	if err != nil {
		h.log.Error(r.Context(), "failed to get budget", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get budget"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateBudget updates the budget.
func (h *handler) UpdateBudget(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var budget costs.BudgetInfo
	if err := json.NewDecoder(r.Body).Decode(&budget); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := h.edgequakeService.Costs.UpdateBudget(r.Context(), tCtx, &budget)
	if err != nil {
		h.log.Error(r.Context(), "failed to update budget", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update budget"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetPipelinePricing returns pipeline cost pricing.
func (h *handler) GetPipelinePricing(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	result, err := h.edgequakeService.Pipeline.GetPricing(r.Context(), tCtx)
	if err != nil {
		h.log.Error(r.Context(), "failed to get pipeline pricing", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get pricing"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// EstimatePipelineCost estimates the cost of a pipeline operation.
func (h *handler) EstimatePipelineCost(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req pipeline.EstimateCostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := h.edgequakeService.Pipeline.EstimateCost(r.Context(), tCtx, &req)
	if err != nil {
		h.log.Error(r.Context(), "failed to estimate cost", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to estimate cost"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Lineage
// ============================================================================

// GetEntityLineage retrieves lineage for an entity.
func (h *handler) GetEntityLineage(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	entityName := chi.URLParam(r, "entityName")
	result, err := h.edgequakeService.Lineage.GetEntityLineage(r.Context(), tCtx, entityName)
	if err != nil {
		h.log.Error(r.Context(), "failed to get entity lineage", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get entity lineage"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetDocumentLineage retrieves graph lineage for a document.
func (h *handler) GetDocumentLineage(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	documentID := chi.URLParam(r, "documentId")
	result, err := h.edgequakeService.Lineage.GetDocumentLineage(r.Context(), tCtx, documentID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get document lineage", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get document lineage"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Chunks
// ============================================================================

// GetChunkDetail retrieves a specific chunk.
func (h *handler) GetChunkDetail(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chunkID := chi.URLParam(r, "chunkId")
	result, err := h.edgequakeService.Chunks.GetChunk(r.Context(), tCtx, chunkID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get chunk", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get chunk"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetChunkLineage retrieves lineage for a specific chunk.
func (h *handler) GetChunkLineage(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chunkID := chi.URLParam(r, "chunkId")
	result, err := h.edgequakeService.Chunks.GetChunkLineage(r.Context(), tCtx, chunkID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get chunk lineage", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get chunk lineage"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Entity Provenance
// ============================================================================

// GetEntityProvenance retrieves provenance for an entity.
func (h *handler) GetEntityProvenance(w http.ResponseWriter, r *http.Request) {
	tCtx, err := h.resolveTenantContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	entityID := chi.URLParam(r, "entityId")
	result, err := h.edgequakeService.Provenance.GetEntityProvenance(r.Context(), tCtx, entityID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get entity provenance", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get entity provenance"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Settings
// ============================================================================

// GetProviderStatus returns provider status.
func (h *handler) GetProviderStatus(w http.ResponseWriter, r *http.Request) {
	result, err := h.edgequakeService.Settings.GetStatus(r.Context())
	if err != nil {
		// If settings are not configured yet, return a 404 instead of 500
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Provider settings not configured"})
			return
		}
		h.log.Error(r.Context(), "failed to get provider status", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get provider status"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListAvailableProviders returns available providers.
func (h *handler) ListAvailableProviders(w http.ResponseWriter, r *http.Request) {
	result, err := h.edgequakeService.Settings.GetProviders(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "failed to list providers", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to list providers"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Models — Extended
// ============================================================================

// ListAllModels returns all models.
func (h *handler) ListAllModels(w http.ResponseWriter, r *http.Request) {
	result, err := h.edgequakeService.Models.List(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "failed to list models", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to list models"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CheckModelsHealth returns model provider health.
func (h *handler) CheckModelsHealth(w http.ResponseWriter, r *http.Request) {
	result, err := h.edgequakeService.Models.Health(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "failed to check model health", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to check model health"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetProvider returns a specific provider.
func (h *handler) GetProvider(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	result, err := h.edgequakeService.Models.GetProvider(r.Context(), provider)
	if err != nil {
		h.log.Error(r.Context(), "failed to get provider", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get provider"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetModel returns a specific model.
func (h *handler) GetModel(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	model := chi.URLParam(r, "model")
	result, err := h.edgequakeService.Models.GetModel(r.Context(), provider, model)
	if err != nil {
		h.log.Error(r.Context(), "failed to get model", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get model"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ============================================================================
// Helpers
// ============================================================================

func intFromQuery(r *http.Request, param string, defaultVal int) int {
	v := r.URL.Query().Get(param)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}
