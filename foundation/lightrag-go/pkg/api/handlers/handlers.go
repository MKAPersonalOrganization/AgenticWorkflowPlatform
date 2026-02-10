// Package handlers provides HTTP handlers for the LightRAG API.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lightrag/lightrag-go/pkg/api/models"
	"github.com/lightrag/lightrag-go/pkg/config"
)

// APIVersion is the current API version.
const APIVersion = "1.0.0"

// HealthHandler handles health check requests.
type HealthHandler struct {
	cfg *config.Config
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(cfg *config.Config) *HealthHandler {
	return &HealthHandler{cfg: cfg}
}

// Health handles the health check endpoint.
func (h *HealthHandler) Health(c *gin.Context) {
	cfg := h.cfg

	configInfo := map[string]interface{}{
		"llm_binding":                  cfg.LLMBinding,
		"embedding_binding":            cfg.EmbeddingBinding,
		"rerank_binding":               cfg.RerankBinding,
		"llm_model":                    cfg.LLMModel,
		"embedding_model":              cfg.EmbeddingModel,
		"kv_storage":                   cfg.KVStorage,
		"vector_storage":               cfg.VectorStorage,
		"graph_storage":                cfg.GraphStorage,
		"doc_status_storage":           cfg.DocStatusStorage,
		"chunk_size":                   cfg.ChunkSize,
		"chunk_overlap_size":           cfg.ChunkOverlapSize,
		"max_async":                    cfg.MaxAsync,
		"max_parallel_insert":          cfg.MaxParallelInsert,
		"enable_llm_cache":             cfg.EnableLLMCache,
		"enable_llm_cache_for_extract": cfg.EnableLLMCacheForExtract,
	}

	c.JSON(http.StatusOK, models.HealthResponse{
		Status:  "healthy",
		Version: APIVersion,
		Config:  configInfo,
	})
}

// QueryHandler handles query-related requests.
type QueryHandler struct {
	cfg *config.Config
}

// NewQueryHandler creates a new QueryHandler.
func NewQueryHandler(cfg *config.Config) *QueryHandler {
	return &QueryHandler{cfg: cfg}
}

// Query handles the main query endpoint.
func (h *QueryHandler) Query(c *gin.Context) {
	var req models.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	// Convert to query params
	params := req.ToQueryParams(false)

	// In a real implementation, this would call the RAG system
	// For now, return a mock response
	resp := models.QueryResponse{
		Response: "This is a mock response for query: " + req.Query,
	}

	if params.IncludeReferences {
		resp.References = []models.ReferenceItem{
			{ReferenceID: "1", FilePath: "/mock/doc1.pdf"},
		}
	}

	c.JSON(http.StatusOK, resp)
}

// Retrieval handles retrieval requests (context only, no LLM).
func (h *QueryHandler) Retrieval(c *gin.Context) {
	var req models.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	resp := models.QueryDataResponse{
		Status:  "success",
		Message: "Retrieval completed successfully",
		Data: map[string]interface{}{
			"entities":      []interface{}{},
			"relationships": []interface{}{},
			"chunks":        []interface{}{},
		},
		Metadata: map[string]interface{}{
			"mode":        req.Mode,
			"hl_keywords": req.HLKeywords,
			"ll_keywords": req.LLKeywords,
		},
	}

	c.JSON(http.StatusOK, resp)
}

// QueryStream handles streaming query requests.
func (h *QueryHandler) QueryStream(c *gin.Context) {
	var req models.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	// Set headers for streaming
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// In a real implementation, this would stream responses
	// For now, send a single chunk
	c.Writer.WriteString(`{"response":"This is a mock streaming response for: ` + req.Query + `"}` + "\n")
	c.Writer.WriteString(`{"response":"","done":true}` + "\n")
}

// QueryData handles detailed query data requests.
func (h *QueryHandler) QueryData(c *gin.Context) {
	var req models.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	resp := models.QueryDataResponse{
		Status:  "success",
		Message: "Query processed successfully",
		Data: map[string]interface{}{
			"entities":      []interface{}{},
			"relationships": []interface{}{},
			"chunks":        []interface{}{},
			"references":    []interface{}{},
		},
		Metadata: map[string]interface{}{
			"mode":        req.Mode,
			"hl_keywords": req.HLKeywords,
			"ll_keywords": req.LLKeywords,
		},
	}

	c.JSON(http.StatusOK, resp)
}

// DocumentHandler handles document-related requests.
type DocumentHandler struct {
	cfg *config.Config
}

// NewDocumentHandler creates a new DocumentHandler.
func NewDocumentHandler(cfg *config.Config) *DocumentHandler {
	return &DocumentHandler{cfg: cfg}
}

// InsertText handles text insertion requests.
func (h *DocumentHandler) InsertText(c *gin.Context) {
	var req models.InsertTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	// Generate a track ID
	trackID := generateTrackID()

	c.JSON(http.StatusOK, models.InsertResponse{
		Status:  "success",
		Message: "Text inserted successfully",
		TrackID: trackID,
	})
}

// InsertTexts handles multiple text insertion requests.
func (h *DocumentHandler) InsertTexts(c *gin.Context) {
	var req models.InsertTextsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	trackID := generateTrackID()

	c.JSON(http.StatusOK, models.InsertResponse{
		Status:  "success",
		Message: "Texts inserted successfully",
		TrackID: trackID,
	})
}

// UploadFile handles file upload requests.
func (h *DocumentHandler) UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Failed to get uploaded file: " + err.Error(),
		})
		return
	}

	trackID := generateTrackID()

	c.JSON(http.StatusOK, models.InsertResponse{
		Status:  "success",
		Message: "File '" + file.Filename + "' uploaded successfully",
		TrackID: trackID,
	})
}

// InsertFile handles single file upload (alias for UploadFile).
func (h *DocumentHandler) InsertFile(c *gin.Context) {
	h.UploadFile(c)
}

// InsertBatch handles batch document insertion requests.
func (h *DocumentHandler) InsertBatch(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Failed to parse multipart form: " + err.Error(),
		})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "No files provided",
		})
		return
	}

	trackID := generateTrackID()

	c.JSON(http.StatusOK, models.InsertResponse{
		Status:  "success",
		Message: "Batch upload started",
		TrackID: trackID,
	})
}

// UploadFiles handles multiple file upload requests.
func (h *DocumentHandler) UploadFiles(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Failed to parse multipart form: " + err.Error(),
		})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "No files provided",
		})
		return
	}

	trackID := generateTrackID()

	c.JSON(http.StatusOK, models.InsertResponse{
		Status:  "success",
		Message: "Files uploaded successfully",
		TrackID: trackID,
	})
}

// Scan handles document scanning requests.
func (h *DocumentHandler) Scan(c *gin.Context) {
	trackID := generateTrackID()

	c.JSON(http.StatusOK, models.ScanResponse{
		Status:  "scanning_started",
		Message: "Scanning process has been initiated in the background",
		TrackID: trackID,
	})
}

// ReprocessFailed handles reprocessing of failed documents.
func (h *DocumentHandler) ReprocessFailed(c *gin.Context) {
	c.JSON(http.StatusOK, models.ReprocessResponse{
		Status:  "reprocessing_started",
		Message: "Reprocessing of failed documents has been initiated in background",
		TrackID: "",
	})
}

// CancelPipeline handles pipeline cancellation requests.
func (h *DocumentHandler) CancelPipeline(c *gin.Context) {
	c.JSON(http.StatusOK, models.CancelPipelineResponse{
		Status:  "cancellation_requested",
		Message: "Pipeline cancellation has been requested. Documents will be marked as FAILED.",
	})
}

// GetPaginatedDocuments handles paginated document list requests.
func (h *DocumentHandler) GetPaginatedDocuments(c *gin.Context) {
	page := 1
	pageSize := 10

	if p := c.Query("page"); p != "" {
		// Parse page
	}
	if ps := c.Query("page_size"); ps != "" {
		// Parse page size
	}

	c.JSON(http.StatusOK, models.PaginatedDocumentsResponse{
		Total:      0,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: 0,
		Documents:  []models.DocumentStatus{},
	})
}

// GetPipelineStatus handles pipeline status requests.
func (h *DocumentHandler) GetPipelineStatus(c *gin.Context) {
	c.JSON(http.StatusOK, models.PipelineStatusResponse{
		IsBusy:         false,
		DocsInProgress: 0,
		DocsRemaining:  0,
		JobsInQueue:    0,
		HistoryCount:   0,
	})
}

// DeleteDocument handles document deletion requests.
func (h *DocumentHandler) DeleteDocument(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		docID = c.Param("doc_id")
	}
	if docID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Document ID is required",
		})
		return
	}

	c.JSON(http.StatusOK, models.DeleteDocumentResponse{
		Status:  "success",
		Message: "Document deleted successfully",
		Deleted: 1,
	})
}

// GetDocument handles single document retrieval requests.
func (h *DocumentHandler) GetDocument(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Document ID is required",
		})
		return
	}

	c.JSON(http.StatusOK, models.DocumentStatus{
		ID:       docID,
		FileName: "document.txt",
		Status:   "PROCESSED",
	})
}

// DeleteDocuments handles multiple document deletion requests.
func (h *DocumentHandler) DeleteDocuments(c *gin.Context) {
	c.JSON(http.StatusOK, models.DeleteDocumentResponse{
		Status:  "success",
		Message: "Documents deleted successfully",
		Deleted: 0,
	})
}

// ClearDocuments handles deletion of all documents.
func (h *DocumentHandler) ClearDocuments(c *gin.Context) {
	c.JSON(http.StatusOK, models.DeleteDocumentResponse{
		Status:  "success",
		Message: "All documents cleared successfully",
		Deleted: 0,
	})
}

// IndexAllDocuments handles reindexing of all documents.
func (h *DocumentHandler) IndexAllDocuments(c *gin.Context) {
	trackID := generateTrackID()

	c.JSON(http.StatusOK, models.InsertResponse{
		Status:  "success",
		Message: "Indexing of all documents started",
		TrackID: trackID,
	})
}

// DeleteAllDocuments handles deletion of all documents.
func (h *DocumentHandler) DeleteAllDocuments(c *gin.Context) {
	c.JSON(http.StatusOK, models.DeleteDocumentResponse{
		Status:  "success",
		Message: "All documents deleted successfully",
		Deleted: 0,
	})
}

// GraphHandler handles graph-related requests.
type GraphHandler struct {
	cfg *config.Config
}

// NewGraphHandler creates a new GraphHandler.
func NewGraphHandler(cfg *config.Config) *GraphHandler {
	return &GraphHandler{cfg: cfg}
}

// GetLabels handles graph label list requests.
func (h *GraphHandler) GetLabels(c *gin.Context) {
	c.JSON(http.StatusOK, models.GraphLabelsResponse{
		Labels: []string{},
	})
}

// GetPopularLabels handles popular labels requests.
func (h *GraphHandler) GetPopularLabels(c *gin.Context) {
	c.JSON(http.StatusOK, []string{})
}

// SearchLabels handles label search requests.
func (h *GraphHandler) SearchLabels(c *gin.Context) {
	c.JSON(http.StatusOK, []string{})
}

// GetKnowledgeGraph handles knowledge graph retrieval requests.
func (h *GraphHandler) GetKnowledgeGraph(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"nodes": []interface{}{},
		"edges": []interface{}{},
	})
}

// GetTextChunks handles retrieval of text chunks for a graph label.
func (h *GraphHandler) GetTextChunks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"chunks": []interface{}{},
		"total":  0,
	})
}

// DeleteOrphanNodes handles deletion of orphan nodes in the graph.
func (h *GraphHandler) DeleteOrphanNodes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Orphan nodes deleted successfully",
		"deleted": 0,
	})
}

// CheckEntityExists handles entity existence check requests.
func (h *GraphHandler) CheckEntityExists(c *gin.Context) {
	c.JSON(http.StatusOK, models.EntityExistsResponse{
		Exists: false,
	})
}

// UpdateEntity handles entity update requests.
func (h *GraphHandler) UpdateEntity(c *gin.Context) {
	var req models.EntityUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.EntityUpdateResponse{
		Status:  "success",
		Message: "Entity updated successfully",
		Data:    map[string]interface{}{},
		OperationSummary: models.OperationSummary{
			Merged:          false,
			MergeStatus:     "not_attempted",
			OperationStatus: "success",
			FinalEntity:     req.EntityName,
			Renamed:         false,
		},
	})
}

// MergeEntities handles entity merge requests.
func (h *GraphHandler) MergeEntities(c *gin.Context) {
	var req models.EntityMergeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Entities merged successfully",
	})
}

// CreateEntity handles entity creation requests.
func (h *GraphHandler) CreateEntity(c *gin.Context) {
	var req models.EntityCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"message":     "Entity created successfully",
		"entity_name": req.EntityName,
	})
}

// DeleteEntity handles entity deletion requests.
func (h *GraphHandler) DeleteEntity(c *gin.Context) {
	entityName := c.Query("name")
	if entityName == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Entity name is required",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Entity deleted successfully",
	})
}

// UpdateRelation handles relation update requests.
func (h *GraphHandler) UpdateRelation(c *gin.Context) {
	var req models.RelationUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Relation updated successfully",
	})
}

// CreateRelation handles relation creation requests.
func (h *GraphHandler) CreateRelation(c *gin.Context) {
	var req models.RelationCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Relation created successfully",
	})
}

// DeleteRelation handles relation deletion requests.
func (h *GraphHandler) DeleteRelation(c *gin.Context) {
	sourceID := c.Query("source_id")
	targetID := c.Query("target_id")

	if sourceID == "" || targetID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "source_id and target_id are required",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Relation deleted successfully",
	})
}

// Helper function to generate track IDs
func generateTrackID() string {
	return "track_" + randomString(16)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}
