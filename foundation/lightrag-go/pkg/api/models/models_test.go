package models

import (
	"testing"

	"github.com/lightrag/lightrag-go/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestQueryRequest_ToQueryParams(t *testing.T) {
	t.Run("with defaults", func(t *testing.T) {
		req := &QueryRequest{
			Query: "test query",
		}

		params := req.ToQueryParams(false)

		assert.Equal(t, types.QueryModeMix, params.Mode)
		assert.False(t, params.Stream)
		assert.Equal(t, 40, params.TopK)
		assert.Equal(t, 20, params.ChunkTopK)
	})

	t.Run("with custom values", func(t *testing.T) {
		topK := 100
		chunkTopK := 50
		maxEntityTokens := 8000
		enableRerank := false
		includeRefs := true
		responseType := "Single Paragraph"
		userPrompt := "Custom prompt"

		req := &QueryRequest{
			Query:             "test query",
			Mode:              types.QueryModeLocal,
			TopK:              &topK,
			ChunkTopK:         &chunkTopK,
			MaxEntityTokens:   &maxEntityTokens,
			EnableRerank:      &enableRerank,
			IncludeReferences: &includeRefs,
			ResponseType:      &responseType,
			UserPrompt:        &userPrompt,
			HLKeywords:        []string{"keyword1", "keyword2"},
			LLKeywords:        []string{"detail1"},
		}

		params := req.ToQueryParams(true)

		assert.Equal(t, types.QueryModeLocal, params.Mode)
		assert.True(t, params.Stream)
		assert.Equal(t, 100, params.TopK)
		assert.Equal(t, 50, params.ChunkTopK)
		assert.Equal(t, 8000, params.MaxEntityTokens)
		assert.False(t, params.EnableRerank)
		assert.True(t, params.IncludeReferences)
		assert.Equal(t, "Single Paragraph", params.ResponseType)
		assert.Equal(t, "Custom prompt", *params.UserPrompt)
		assert.Equal(t, []string{"keyword1", "keyword2"}, params.HLKeywords)
		assert.Equal(t, []string{"detail1"}, params.LLKeywords)
	})

	t.Run("with conversation history", func(t *testing.T) {
		req := &QueryRequest{
			Query: "follow up question",
			ConversationHistory: []types.ConversationMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			},
		}

		params := req.ToQueryParams(false)

		assert.Len(t, params.ConversationHistory, 2)
		assert.Equal(t, "user", params.ConversationHistory[0].Role)
		assert.Equal(t, "Hello", params.ConversationHistory[0].Content)
	})
}

func TestReferenceItem(t *testing.T) {
	ref := ReferenceItem{
		ReferenceID: "ref-1",
		FilePath:    "/docs/test.pdf",
		Content:     []string{"chunk 1", "chunk 2"},
	}

	assert.Equal(t, "ref-1", ref.ReferenceID)
	assert.Equal(t, "/docs/test.pdf", ref.FilePath)
	assert.Len(t, ref.Content, 2)
}

func TestQueryResponse(t *testing.T) {
	resp := QueryResponse{
		Response: "This is the generated response.",
		References: []ReferenceItem{
			{ReferenceID: "1", FilePath: "/doc1.pdf"},
			{ReferenceID: "2", FilePath: "/doc2.txt"},
		},
	}

	assert.Equal(t, "This is the generated response.", resp.Response)
	assert.Len(t, resp.References, 2)
}

func TestInsertTextRequest(t *testing.T) {
	req := InsertTextRequest{
		Text:       "Sample text to insert",
		FileSource: "source.txt",
	}

	assert.Equal(t, "Sample text to insert", req.Text)
	assert.Equal(t, "source.txt", req.FileSource)
}

func TestInsertTextsRequest(t *testing.T) {
	req := InsertTextsRequest{
		Texts:       []string{"Text 1", "Text 2"},
		FileSources: []string{"source1.txt", "source2.txt"},
	}

	assert.Len(t, req.Texts, 2)
	assert.Len(t, req.FileSources, 2)
}

func TestInsertResponse(t *testing.T) {
	resp := InsertResponse{
		Status:  "success",
		Message: "Document inserted successfully",
		TrackID: "track_123456",
	}

	assert.Equal(t, "success", resp.Status)
	assert.NotEmpty(t, resp.TrackID)
}

func TestEntityUpdateRequest(t *testing.T) {
	req := EntityUpdateRequest{
		EntityName: "TestEntity",
		UpdatedData: map[string]interface{}{
			"description": "Updated description",
			"entity_type": "Person",
		},
		AllowRename: true,
		AllowMerge:  false,
	}

	assert.Equal(t, "TestEntity", req.EntityName)
	assert.True(t, req.AllowRename)
	assert.False(t, req.AllowMerge)
}

func TestEntityMergeRequest(t *testing.T) {
	req := EntityMergeRequest{
		EntitiesToChange:   []string{"Elon Msk", "Ellon Musk"},
		EntityToChangeInto: "Elon Musk",
	}

	assert.Len(t, req.EntitiesToChange, 2)
	assert.Equal(t, "Elon Musk", req.EntityToChangeInto)
}

func TestRelationCreateRequest(t *testing.T) {
	req := RelationCreateRequest{
		SourceEntity: "Elon Musk",
		TargetEntity: "Tesla",
		RelationData: map[string]interface{}{
			"description": "CEO of",
			"weight":      1.0,
		},
	}

	assert.Equal(t, "Elon Musk", req.SourceEntity)
	assert.Equal(t, "Tesla", req.TargetEntity)
	assert.NotNil(t, req.RelationData)
}

func TestOllamaModels(t *testing.T) {
	t.Run("OllamaChatRequest", func(t *testing.T) {
		req := OllamaChatRequest{
			Model: "lightrag:latest",
			Messages: []OllamaMessage{
				{Role: "user", Content: "Hello"},
			},
			Stream: true,
		}

		assert.Equal(t, "lightrag:latest", req.Model)
		assert.Len(t, req.Messages, 1)
		assert.True(t, req.Stream)
	})

	t.Run("OllamaGenerateRequest", func(t *testing.T) {
		system := "You are a helpful assistant"
		req := OllamaGenerateRequest{
			Model:  "lightrag:latest",
			Prompt: "Generate something",
			System: &system,
			Stream: false,
		}

		assert.Equal(t, "lightrag:latest", req.Model)
		assert.Equal(t, "Generate something", req.Prompt)
		assert.Equal(t, "You are a helpful assistant", *req.System)
		assert.False(t, req.Stream)
	})

	t.Run("OllamaTagResponse", func(t *testing.T) {
		resp := OllamaTagResponse{
			Models: []OllamaModel{
				{
					Name:   "lightrag:latest",
					Model:  "lightrag:latest",
					Size:   7365960935,
					Digest: "test-digest",
					Details: OllamaModelDetails{
						Format: "gguf",
						Family: "lightrag",
					},
				},
			},
		}

		assert.Len(t, resp.Models, 1)
		assert.Equal(t, "lightrag:latest", resp.Models[0].Name)
	})
}

func TestHealthResponse(t *testing.T) {
	resp := HealthResponse{
		Status:  "healthy",
		Version: "1.0.0",
		Config: map[string]interface{}{
			"llm_binding":       "ollama",
			"embedding_binding": "ollama",
		},
	}

	assert.Equal(t, "healthy", resp.Status)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.NotNil(t, resp.Config)
}

func TestLoginRequest(t *testing.T) {
	req := LoginRequest{
		Username: "admin",
		Password: "secret",
	}

	assert.Equal(t, "admin", req.Username)
	assert.Equal(t, "secret", req.Password)
}

func TestLoginResponse(t *testing.T) {
	resp := LoginResponse{
		AccessToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
		TokenType:   "bearer",
		ExpiresIn:   172800,
	}

	assert.NotEmpty(t, resp.AccessToken)
	assert.Equal(t, "bearer", resp.TokenType)
	assert.Equal(t, 172800, resp.ExpiresIn)
}

func TestErrorResponse(t *testing.T) {
	resp := ErrorResponse{
		Detail: "Something went wrong",
	}

	assert.Equal(t, "Something went wrong", resp.Detail)
}

func TestValidationErrorResponse(t *testing.T) {
	resp := ValidationErrorResponse{
		Detail: []ValidationErrorDetail{
			{
				Loc:  []string{"body", "query"},
				Msg:  "field required",
				Type: "value_error.missing",
			},
		},
	}

	assert.Len(t, resp.Detail, 1)
	assert.Equal(t, "field required", resp.Detail[0].Msg)
}

func TestPaginatedDocumentsResponse(t *testing.T) {
	resp := PaginatedDocumentsResponse{
		Total:      100,
		Page:       1,
		PageSize:   10,
		TotalPages: 10,
		Documents: []DocumentStatus{
			{
				ID:       "doc1",
				FileName: "test.pdf",
				Status:   types.DocStatusCompleted,
			},
		},
	}

	assert.Equal(t, 100, resp.Total)
	assert.Equal(t, 1, resp.Page)
	assert.Len(t, resp.Documents, 1)
}

func TestPipelineStatusResponse(t *testing.T) {
	resp := PipelineStatusResponse{
		IsBusy:         true,
		DocsInProgress: 2,
		DocsRemaining:  5,
		CurrentTrackID: "track_123",
		JobsInQueue:    3,
	}

	assert.True(t, resp.IsBusy)
	assert.Equal(t, 2, resp.DocsInProgress)
	assert.Equal(t, "track_123", resp.CurrentTrackID)
}

func TestOperationSummary(t *testing.T) {
	summary := OperationSummary{
		Merged:          true,
		MergeStatus:     "success",
		OperationStatus: "success",
		FinalEntity:     "Elon Musk",
		Renamed:         true,
	}

	assert.True(t, summary.Merged)
	assert.Equal(t, "success", summary.MergeStatus)
	assert.Equal(t, "Elon Musk", summary.FinalEntity)
}
