package types

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryMode_IsValid(t *testing.T) {
	tests := []struct {
		mode     QueryMode
		expected bool
	}{
		{QueryModeLocal, true},
		{QueryModeGlobal, true},
		{QueryModeHybrid, true},
		{QueryModeNaive, true},
		{QueryModeMix, true},
		{QueryModeBypass, true},
		{QueryMode("invalid"), false},
		{QueryMode(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.IsValid())
		})
	}
}

func TestNewQueryParam(t *testing.T) {
	// Clear relevant env vars
	os.Unsetenv("TOP_K")
	os.Unsetenv("CHUNK_TOP_K")
	os.Unsetenv("MAX_ENTITY_TOKENS")
	os.Unsetenv("MAX_RELATION_TOKENS")
	os.Unsetenv("MAX_TOTAL_TOKENS")
	os.Unsetenv("HISTORY_TURNS")
	os.Unsetenv("RERANK_BY_DEFAULT")

	param := NewQueryParam()

	assert.Equal(t, QueryModeMix, param.Mode)
	assert.False(t, param.OnlyNeedContext)
	assert.False(t, param.OnlyNeedPrompt)
	assert.Equal(t, "Multiple Paragraphs", param.ResponseType)
	assert.False(t, param.Stream)
	assert.Equal(t, 40, param.TopK)
	assert.Equal(t, 20, param.ChunkTopK)
	assert.Equal(t, 6000, param.MaxEntityTokens)
	assert.Equal(t, 8000, param.MaxRelationTokens)
	assert.Equal(t, 30000, param.MaxTotalTokens)
	assert.Empty(t, param.HLKeywords)
	assert.Empty(t, param.LLKeywords)
	assert.Empty(t, param.ConversationHistory)
	assert.Equal(t, 0, param.HistoryTurns)
	assert.True(t, param.EnableRerank)
	assert.False(t, param.IncludeReferences)
}

func TestNewQueryParam_WithEnvVars(t *testing.T) {
	// Set env vars
	os.Setenv("TOP_K", "100")
	os.Setenv("CHUNK_TOP_K", "50")
	os.Setenv("MAX_ENTITY_TOKENS", "8000")
	os.Setenv("RERANK_BY_DEFAULT", "false")
	defer func() {
		os.Unsetenv("TOP_K")
		os.Unsetenv("CHUNK_TOP_K")
		os.Unsetenv("MAX_ENTITY_TOKENS")
		os.Unsetenv("RERANK_BY_DEFAULT")
	}()

	param := NewQueryParam()

	assert.Equal(t, 100, param.TopK)
	assert.Equal(t, 50, param.ChunkTopK)
	assert.Equal(t, 8000, param.MaxEntityTokens)
	assert.False(t, param.EnableRerank)
}

func TestKnowledgeGraph(t *testing.T) {
	// Test creating and manipulating a knowledge graph
	kg := KnowledgeGraph{
		Nodes: []KnowledgeGraphNode{
			{
				ID:     "node1",
				Labels: []string{"Person"},
				Properties: map[string]interface{}{
					"name": "Alice",
					"age":  30,
				},
			},
			{
				ID:     "node2",
				Labels: []string{"Organization"},
				Properties: map[string]interface{}{
					"name": "TechCorp",
				},
			},
		},
		Edges: []KnowledgeGraphEdge{
			{
				ID:     "edge1",
				Type:   stringPtr("WORKS_AT"),
				Source: "node1",
				Target: "node2",
				Properties: map[string]interface{}{
					"since": "2020",
				},
			},
		},
		IsTruncated: false,
	}

	assert.Len(t, kg.Nodes, 2)
	assert.Len(t, kg.Edges, 1)
	assert.Equal(t, "node1", kg.Nodes[0].ID)
	assert.Equal(t, "Person", kg.Nodes[0].Labels[0])
	assert.Equal(t, "edge1", kg.Edges[0].ID)
	assert.Equal(t, "WORKS_AT", *kg.Edges[0].Type)
	assert.False(t, kg.IsTruncated)
}

func TestTextChunkSchema(t *testing.T) {
	chunk := TextChunkSchema{
		Tokens:          150,
		Content:         "This is test content.",
		FullDocID:       "doc123",
		ChunkOrderIndex: 0,
	}

	assert.Equal(t, 150, chunk.Tokens)
	assert.Equal(t, "This is test content.", chunk.Content)
	assert.Equal(t, "doc123", chunk.FullDocID)
	assert.Equal(t, 0, chunk.ChunkOrderIndex)
}

func TestDocStatus(t *testing.T) {
	assert.Equal(t, DocStatus("PENDING"), DocStatusPending)
	assert.Equal(t, DocStatus("PROCESSING"), DocStatusProcessing)
	assert.Equal(t, DocStatus("COMPLETED"), DocStatusCompleted)
	assert.Equal(t, DocStatus("FAILED"), DocStatusFailed)
}

func TestDocProcessingStatus(t *testing.T) {
	status := DocProcessingStatus{
		ID:              "doc1",
		FileName:        "test.pdf",
		FilePath:        "/docs/test.pdf",
		Status:          DocStatusProcessing,
		CreatedAt:       "2024-01-01T00:00:00Z",
		UpdatedAt:       "2024-01-01T01:00:00Z",
		ChunksTotal:     10,
		ChunksProcessed: 5,
		TrackID:         "track123",
	}

	assert.Equal(t, "doc1", status.ID)
	assert.Equal(t, "test.pdf", status.FileName)
	assert.Equal(t, DocStatusProcessing, status.Status)
	assert.Equal(t, 10, status.ChunksTotal)
	assert.Equal(t, 5, status.ChunksProcessed)
}

func TestDeletionResult(t *testing.T) {
	result := DeletionResult{
		Status:  "success",
		Message: "Deleted 5 documents",
		Deleted: 5,
	}

	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "Deleted 5 documents", result.Message)
	assert.Equal(t, 5, result.Deleted)
}

func TestGPTKeywordExtractionFormat(t *testing.T) {
	format := GPTKeywordExtractionFormat{
		HighLevelKeywords: []string{"AI", "Machine Learning"},
		LowLevelKeywords:  []string{"neural network", "training"},
	}

	assert.Len(t, format.HighLevelKeywords, 2)
	assert.Len(t, format.LowLevelKeywords, 2)
	assert.Contains(t, format.HighLevelKeywords, "AI")
	assert.Contains(t, format.LowLevelKeywords, "neural network")
}

func TestSearchMode(t *testing.T) {
	assert.Equal(t, SearchMode("naive"), SearchModeNaive)
	assert.Equal(t, SearchMode("local"), SearchModeLocal)
	assert.Equal(t, SearchMode("global"), SearchModeGlobal)
	assert.Equal(t, SearchMode("hybrid"), SearchModeHybrid)
	assert.Equal(t, SearchMode("mix"), SearchModeMix)
	assert.Equal(t, SearchMode("bypass"), SearchModeBypass)
	assert.Equal(t, SearchMode("context"), SearchModeContext)
}

func TestConversationMessage(t *testing.T) {
	msg := ConversationMessage{
		Role:    "user",
		Content: "Hello, how are you?",
	}

	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "Hello, how are you?", msg.Content)
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
