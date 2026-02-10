package base

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDocStatus_IsValid(t *testing.T) {
	tests := []struct {
		status   DocStatus
		expected bool
	}{
		{DocStatusPending, true},
		{DocStatusProcessing, true},
		{DocStatusPreprocessed, true},
		{DocStatusProcessed, true},
		{DocStatusFailed, true},
		{DocStatus("invalid"), false},
		{DocStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsValid())
		})
	}
}

func TestStoragesStatus(t *testing.T) {
	assert.Equal(t, StoragesStatus("not_created"), StoragesStatusNotCreated)
	assert.Equal(t, StoragesStatus("created"), StoragesStatusCreated)
	assert.Equal(t, StoragesStatus("initialized"), StoragesStatusInitialized)
	assert.Equal(t, StoragesStatus("finalized"), StoragesStatusFinalized)
}

func TestDocProcessingStatus(t *testing.T) {
	chunksCount := 5
	status := DocProcessingStatus{
		ContentSummary: "Test document summary",
		ContentLength:  1000,
		FilePath:       "/path/to/doc.txt",
		Status:         DocStatusProcessed,
		CreatedAt:      "2024-01-01T00:00:00Z",
		UpdatedAt:      "2024-01-01T01:00:00Z",
		TrackID:        "track-123",
		ChunksCount:    &chunksCount,
		ChunksList:     []string{"chunk-1", "chunk-2"},
	}

	assert.Equal(t, "Test document summary", status.ContentSummary)
	assert.Equal(t, 1000, status.ContentLength)
	assert.Equal(t, DocStatusProcessed, status.Status)
	assert.Equal(t, 5, *status.ChunksCount)
	assert.Len(t, status.ChunksList, 2)
}

func TestDeletionResult(t *testing.T) {
	result := DeletionResult{
		Status:     "success",
		DocID:      "doc-123",
		Message:    "Document deleted successfully",
		StatusCode: 200,
		FilePath:   "/path/to/doc.txt",
	}

	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "doc-123", result.DocID)
	assert.Equal(t, 200, result.StatusCode)
}

func TestQueryResult_GetReferenceList(t *testing.T) {
	t.Run("with references", func(t *testing.T) {
		result := QueryResult{
			Content: "Test response",
			RawData: map[string]interface{}{
				"data": map[string]interface{}{
					"references": []map[string]string{
						{"reference_id": "1", "file_path": "/path/to/file1.pdf"},
						{"reference_id": "2", "file_path": "/path/to/file2.pdf"},
					},
				},
			},
		}

		refs := result.GetReferenceList()
		assert.Len(t, refs, 2)
		assert.Equal(t, "1", refs[0]["reference_id"])
	})

	t.Run("without raw data", func(t *testing.T) {
		result := QueryResult{Content: "Test"}
		refs := result.GetReferenceList()
		assert.Nil(t, refs)
	})

	t.Run("without data field", func(t *testing.T) {
		result := QueryResult{
			RawData: map[string]interface{}{},
		}
		refs := result.GetReferenceList()
		assert.Nil(t, refs)
	})
}

func TestQueryResult_GetMetadata(t *testing.T) {
	t.Run("with metadata", func(t *testing.T) {
		result := QueryResult{
			RawData: map[string]interface{}{
				"metadata": map[string]interface{}{
					"query_mode": "mix",
					"top_k":      10,
				},
			},
		}

		meta := result.GetMetadata()
		assert.Equal(t, "mix", meta["query_mode"])
		assert.Equal(t, 10, meta["top_k"])
	})

	t.Run("without metadata", func(t *testing.T) {
		result := QueryResult{}
		meta := result.GetMetadata()
		assert.Nil(t, meta)
	})
}

func TestKnowledgeGraph(t *testing.T) {
	kg := KnowledgeGraph{
		Nodes: []KnowledgeGraphNode{
			{
				ID:          "node-1",
				Labels:      []string{"Person"},
				Description: "A test person",
			},
			{
				ID:          "node-2",
				Labels:      []string{"Organization"},
				Description: "A test organization",
			},
		},
		Edges: []KnowledgeGraphEdge{
			{
				Source:      "node-1",
				Target:      "node-2",
				Description: "Works at",
				Keywords:    "employment",
			},
		},
		IsTruncated: false,
	}

	assert.Len(t, kg.Nodes, 2)
	assert.Len(t, kg.Edges, 1)
	assert.False(t, kg.IsTruncated)
	assert.Equal(t, "node-1", kg.Edges[0].Source)
	assert.Equal(t, "node-2", kg.Edges[0].Target)
}

func TestTextChunkSchema(t *testing.T) {
	chunk := TextChunkSchema{
		Tokens:          100,
		Content:         "This is a test chunk content",
		FullDocID:       "doc-123",
		ChunkOrderIndex: 0,
	}

	assert.Equal(t, 100, chunk.Tokens)
	assert.Equal(t, "This is a test chunk content", chunk.Content)
	assert.Equal(t, "doc-123", chunk.FullDocID)
	assert.Equal(t, 0, chunk.ChunkOrderIndex)
}

func TestEmbeddingFunc(t *testing.T) {
	t.Run("with valid function", func(t *testing.T) {
		ef := &EmbeddingFunc{
			Func: func(ctx context.Context, texts []string) ([][]float64, error) {
				result := make([][]float64, len(texts))
				for i := range texts {
					result[i] = []float64{0.1, 0.2, 0.3}
				}
				return result, nil
			},
			EmbeddingDim: 3,
			MaxTokenSize: 8192,
			ModelName:    "test-model",
		}

		embeddings, err := ef.Embed(context.Background(), []string{"hello", "world"})
		assert.NoError(t, err)
		assert.Len(t, embeddings, 2)
		assert.Len(t, embeddings[0], 3)
	})

	t.Run("with nil function", func(t *testing.T) {
		ef := &EmbeddingFunc{}
		_, err := ef.Embed(context.Background(), []string{"test"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "embedding function not set")
	})
}

func TestQueryContextResult_GetReferenceList(t *testing.T) {
	result := QueryContextResult{
		Context: "Test context",
		RawData: map[string]interface{}{
			"data": map[string]interface{}{
				"references": []map[string]string{
					{"reference_id": "1", "file_path": "/file.pdf"},
				},
			},
		},
	}

	refs := result.GetReferenceList()
	assert.Len(t, refs, 1)
}
