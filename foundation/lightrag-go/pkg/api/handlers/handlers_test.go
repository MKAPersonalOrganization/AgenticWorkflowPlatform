package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lightrag/lightrag-go/pkg/api/models"
	"github.com/lightrag/lightrag-go/pkg/config"
	"github.com/lightrag/lightrag-go/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHealthHandler(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewHealthHandler(cfg)

	router := gin.New()
	router.GET("/health", handler.Health)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp.Status)
	assert.Equal(t, APIVersion, resp.Version)
	assert.NotNil(t, resp.Config)
}

func TestQueryHandler_Query(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewQueryHandler(cfg)

	router := gin.New()
	router.POST("/query", handler.Query)

	t.Run("valid request", func(t *testing.T) {
		reqBody := models.QueryRequest{
			Query: "What is artificial intelligence?",
			Mode:  types.QueryModeMix,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/query", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.QueryResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Response)
	})

	t.Run("with include_references", func(t *testing.T) {
		includeRefs := true
		reqBody := models.QueryRequest{
			Query:             "What is AI?",
			IncludeReferences: &includeRefs,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/query", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.QueryResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotNil(t, resp.References)
	})

	t.Run("invalid request - missing query", func(t *testing.T) {
		reqBody := map[string]interface{}{}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/query", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid request - query too short", func(t *testing.T) {
		reqBody := models.QueryRequest{
			Query: "ab", // Less than 3 characters
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/query", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestQueryHandler_QueryData(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewQueryHandler(cfg)

	router := gin.New()
	router.POST("/query/data", handler.QueryData)

	reqBody := models.QueryRequest{
		Query:      "test query for data",
		Mode:       types.QueryModeLocal,
		HLKeywords: []string{"AI", "ML"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/query/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.QueryDataResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.NotNil(t, resp.Data)
	assert.NotNil(t, resp.Metadata)
}

func TestDocumentHandler_InsertText(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewDocumentHandler(cfg)

	router := gin.New()
	router.POST("/documents/text", handler.InsertText)

	t.Run("valid request", func(t *testing.T) {
		reqBody := models.InsertTextRequest{
			Text:       "This is sample text to insert into the RAG system.",
			FileSource: "test_source.txt",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/documents/text", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.InsertResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "success", resp.Status)
		assert.NotEmpty(t, resp.TrackID)
	})

	t.Run("invalid request - empty text", func(t *testing.T) {
		reqBody := models.InsertTextRequest{
			Text: "",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/documents/text", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestDocumentHandler_InsertTexts(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewDocumentHandler(cfg)

	router := gin.New()
	router.POST("/documents/texts", handler.InsertTexts)

	reqBody := models.InsertTextsRequest{
		Texts:       []string{"First text", "Second text", "Third text"},
		FileSources: []string{"source1.txt", "source2.txt"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/documents/texts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.InsertResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}

func TestDocumentHandler_Scan(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewDocumentHandler(cfg)

	router := gin.New()
	router.POST("/documents/scan", handler.Scan)

	req := httptest.NewRequest("POST", "/documents/scan", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ScanResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "scanning_started", resp.Status)
	assert.NotEmpty(t, resp.TrackID)
}

func TestDocumentHandler_GetPaginatedDocuments(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewDocumentHandler(cfg)

	router := gin.New()
	router.GET("/documents/paginated", handler.GetPaginatedDocuments)

	req := httptest.NewRequest("GET", "/documents/paginated?page=1&page_size=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.PaginatedDocumentsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Page)
	assert.NotNil(t, resp.Documents)
}

func TestDocumentHandler_GetPipelineStatus(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewDocumentHandler(cfg)

	router := gin.New()
	router.GET("/documents/pipeline_status", handler.GetPipelineStatus)

	req := httptest.NewRequest("GET", "/documents/pipeline_status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.PipelineStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	// Status should be valid
	assert.GreaterOrEqual(t, resp.DocsInProgress, 0)
}

func TestDocumentHandler_DeleteDocument(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewDocumentHandler(cfg)

	router := gin.New()
	router.DELETE("/documents/:doc_id", handler.DeleteDocument)

	t.Run("valid request", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/documents/doc123", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.DeleteDocumentResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "success", resp.Status)
	})
}

func TestGraphHandler_GetLabels(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewGraphHandler(cfg)

	router := gin.New()
	router.GET("/graph/label/list", handler.GetLabels)

	req := httptest.NewRequest("GET", "/graph/label/list", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.GraphLabelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Labels)
}

func TestGraphHandler_UpdateEntity(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewGraphHandler(cfg)

	router := gin.New()
	router.POST("/graph/entity/edit", handler.UpdateEntity)

	t.Run("valid request", func(t *testing.T) {
		reqBody := models.EntityUpdateRequest{
			EntityName: "TestEntity",
			UpdatedData: map[string]interface{}{
				"description": "Updated description",
			},
			AllowRename: false,
			AllowMerge:  false,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/graph/entity/edit", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.EntityUpdateResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "success", resp.Status)
		assert.Equal(t, "not_attempted", resp.OperationSummary.MergeStatus)
	})
}

func TestGraphHandler_MergeEntities(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewGraphHandler(cfg)

	router := gin.New()
	router.POST("/graph/entity/merge", handler.MergeEntities)

	reqBody := models.EntityMergeRequest{
		EntitiesToChange:   []string{"Entity1", "Entity2"},
		EntityToChangeInto: "MainEntity",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/graph/entity/merge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGraphHandler_CreateRelation(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewGraphHandler(cfg)

	router := gin.New()
	router.POST("/graph/relation/create", handler.CreateRelation)

	reqBody := models.RelationCreateRequest{
		SourceEntity: "Person A",
		TargetEntity: "Organization B",
		RelationData: map[string]interface{}{
			"description": "Works at",
			"weight":      1.0,
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/graph/relation/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGenerateTrackID(t *testing.T) {
	id1 := generateTrackID()
	id2 := generateTrackID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.Contains(t, id1, "track_")
}
