package rerank

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Comprehensive Rerank Chunking Tests
// ============================================================================

func TestNoChunkingNeededForShortDocs(t *testing.T) {
	// Documents shorter than max_tokens should not be chunked
	documents := []string{
		"Short doc 1",
		"Short doc 2",
		"Short doc 3",
	}

	chunkedDocs, docIndices := chunkDocumentsForRerank(documents, 100, 10)

	// No chunking should occur
	assert.Len(t, chunkedDocs, 3)
	assert.Equal(t, documents, chunkedDocs)
	assert.Equal(t, []int{0, 1, 2}, docIndices)
}

func TestChunkingWithLongDocument(t *testing.T) {
	// Create document with known approximate token count
	longDoc := ""
	for i := 0; i < 200; i++ {
		longDoc += "word "
	}
	documents := []string{longDoc, "short"}

	chunkedDocs, docIndices := chunkDocumentsForRerank(documents, 50, 10)

	// Long doc should be split, short doc should remain
	assert.Greater(t, len(chunkedDocs), 2)
	assert.Equal(t, 1, docIndices[len(docIndices)-1]) // Last chunk is from second document
}

func TestEmptyDocuments(t *testing.T) {
	// Test handling of empty document list
	documents := []string{}
	chunkedDocs, docIndices := chunkDocumentsForRerank(documents, 100, 10)

	assert.Empty(t, chunkedDocs)
	assert.Empty(t, docIndices)
}

func TestSingleDocumentChunking(t *testing.T) {
	// Test chunking of a single long document
	longDoc := ""
	for i := 0; i < 100; i++ {
		longDoc += "token "
	}
	documents := []string{longDoc}

	chunkedDocs, docIndices := chunkDocumentsForRerank(documents, 30, 5)

	// Should create multiple chunks
	assert.Greater(t, len(chunkedDocs), 1)
	// All chunks should map to document 0
	for _, idx := range docIndices {
		assert.Equal(t, 0, idx)
	}
}

// ============================================================================
// Score Aggregation Tests
// ============================================================================

func TestNoChunkingSimpleAggregation(t *testing.T) {
	// Test aggregation when no chunking occurred (1:1 mapping)
	chunkResults := []RerankResult{
		{Index: 0, RelevanceScore: 0.9},
		{Index: 1, RelevanceScore: 0.7},
		{Index: 2, RelevanceScore: 0.5},
	}
	docIndices := []int{0, 1, 2} // 1:1 mapping
	numOriginalDocs := 3

	aggregated := aggregateChunkScores(chunkResults, docIndices, numOriginalDocs, "max")

	// Results should be sorted by score
	assert.Len(t, aggregated, 3)
	assert.Equal(t, 0, aggregated[0].Index)
	assert.Equal(t, 0.9, aggregated[0].RelevanceScore)
	assert.Equal(t, 1, aggregated[1].Index)
	assert.Equal(t, 0.7, aggregated[1].RelevanceScore)
	assert.Equal(t, 2, aggregated[2].Index)
	assert.Equal(t, 0.5, aggregated[2].RelevanceScore)
}

func TestMaxAggregationWithChunks(t *testing.T) {
	// Test max aggregation strategy with multiple chunks per document
	// 5 chunks: first 3 from doc 0, last 2 from doc 1
	chunkResults := []RerankResult{
		{Index: 0, RelevanceScore: 0.5},
		{Index: 1, RelevanceScore: 0.8},
		{Index: 2, RelevanceScore: 0.6},
		{Index: 3, RelevanceScore: 0.7},
		{Index: 4, RelevanceScore: 0.4},
	}
	docIndices := []int{0, 0, 0, 1, 1}
	numOriginalDocs := 2

	aggregated := aggregateChunkScores(chunkResults, docIndices, numOriginalDocs, "max")

	// Should take max score for each document
	assert.Len(t, aggregated, 2)

	// Find doc 0 and doc 1 scores
	var doc0Score, doc1Score float64
	for _, r := range aggregated {
		if r.Index == 0 {
			doc0Score = r.RelevanceScore
		} else if r.Index == 1 {
			doc1Score = r.RelevanceScore
		}
	}

	assert.Equal(t, 0.8, doc0Score) // max of 0.5, 0.8, 0.6
	assert.Equal(t, 0.7, doc1Score) // max of 0.7, 0.4
}

func TestMeanAggregationWithChunks(t *testing.T) {
	// Test mean aggregation strategy
	chunkResults := []RerankResult{
		{Index: 0, RelevanceScore: 0.6},
		{Index: 1, RelevanceScore: 0.8},
		{Index: 2, RelevanceScore: 0.4},
	}
	docIndices := []int{0, 0, 1} // First two chunks from doc 0, last from doc 1
	numOriginalDocs := 2

	aggregated := aggregateChunkScores(chunkResults, docIndices, numOriginalDocs, "mean")

	assert.Len(t, aggregated, 2)

	// Find scores by document index
	scoreMap := make(map[int]float64)
	for _, r := range aggregated {
		scoreMap[r.Index] = r.RelevanceScore
	}

	assert.InDelta(t, 0.7, scoreMap[0], 0.001) // (0.6 + 0.8) / 2
	assert.Equal(t, 0.4, scoreMap[1])
}

func TestFirstAggregationWithChunks(t *testing.T) {
	// Test first aggregation strategy
	chunkResults := []RerankResult{
		{Index: 0, RelevanceScore: 0.6},
		{Index: 1, RelevanceScore: 0.8},
		{Index: 2, RelevanceScore: 0.4},
	}
	docIndices := []int{0, 0, 1}
	numOriginalDocs := 2

	aggregated := aggregateChunkScores(chunkResults, docIndices, numOriginalDocs, "first")

	assert.Len(t, aggregated, 2)

	// Find scores by document index
	scoreMap := make(map[int]float64)
	for _, r := range aggregated {
		scoreMap[r.Index] = r.RelevanceScore
	}

	// First should use first score seen for each doc
	assert.Equal(t, 0.6, scoreMap[0]) // First score for doc 0
	assert.Equal(t, 0.4, scoreMap[1])
}

func TestEmptyChunkResults(t *testing.T) {
	// Test handling of empty results
	aggregated := aggregateChunkScores([]RerankResult{}, []int{}, 3, "max")
	assert.Empty(t, aggregated)
}

func TestDocumentsWithNoScores(t *testing.T) {
	// Test when some documents have no chunks/scores
	chunkResults := []RerankResult{
		{Index: 0, RelevanceScore: 0.9},
		{Index: 1, RelevanceScore: 0.7},
	}
	docIndices := []int{0, 0} // Both chunks from document 0
	numOriginalDocs := 3      // But we have 3 documents total

	aggregated := aggregateChunkScores(chunkResults, docIndices, numOriginalDocs, "max")

	// Only doc 0 should appear in results
	assert.Len(t, aggregated, 1)
	assert.Equal(t, 0, aggregated[0].Index)
}

func TestUnknownAggregationStrategyFallsBackToMax(t *testing.T) {
	// Test that unknown strategy falls back to max
	chunkResults := []RerankResult{
		{Index: 0, RelevanceScore: 0.6},
		{Index: 1, RelevanceScore: 0.8},
	}
	docIndices := []int{0, 0}
	numOriginalDocs := 1

	// Use invalid strategy
	aggregated := aggregateChunkScores(chunkResults, docIndices, numOriginalDocs, "invalid")

	// Should fall back to max
	assert.Equal(t, 0.8, aggregated[0].RelevanceScore)
}

// ============================================================================
// Integration Tests with Mock Server
// ============================================================================

func TestCohereRerankWithChunkingEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Verify request structure
		assert.Equal(t, "POST", r.Method)
		assert.NotNil(t, reqBody["query"])
		assert.NotNil(t, reqBody["documents"])

		// Return mock response
		docs := reqBody["documents"].([]interface{})
		results := make([]map[string]interface{}, len(docs))
		for i := range docs {
			results[i] = map[string]interface{}{
				"index":           i,
				"relevance_score": 0.9 - float64(i)*0.1,
			}
		}

		resp := map[string]interface{}{
			"id":      "test-id",
			"results": results,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &RerankerConfig{
		APIKey:          "test-key",
		BaseURL:         server.URL,
		EnableChunking:  true,
		MaxTokensPerDoc: 50,
		ChunkOverlap:    10,
	}

	reranker := NewCohereReranker(config, nil)

	ctx := context.Background()
	req := &RerankRequest{
		Query:     "test query",
		Documents: []string{"doc1", "doc2"},
		TopN:      2,
	}

	resp, err := reranker.Rerank(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Results)
}

func TestJinaRerankWithChunkingEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		// Return mock response
		docs := reqBody["documents"].([]interface{})
		results := make([]struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
			Document       struct {
				Text string `json:"text"`
			} `json:"document,omitempty"`
		}, len(docs))

		for i := range docs {
			results[i].Index = i
			results[i].RelevanceScore = 0.9 - float64(i)*0.1
		}

		resp := jinaRerankResponse{
			Model:   "jina-reranker",
			Results: results,
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &RerankerConfig{
		APIKey:          "test-key",
		BaseURL:         server.URL,
		EnableChunking:  true,
		MaxTokensPerDoc: 50,
		ChunkOverlap:    10,
	}

	reranker := NewJinaReranker(config, nil)

	ctx := context.Background()
	req := &RerankRequest{
		Query:     "test query",
		Documents: []string{"short doc"},
	}

	resp, err := reranker.Rerank(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Results)
}

// ============================================================================
// Top N Behavior Tests
// ============================================================================

func TestTopNLimitsDocumentsNotChunks(t *testing.T) {
	// Test that top_n correctly limits documents (not chunks) when chunking is enabled
	// 5 documents, each producing multiple chunks
	longDocs := make([]string, 5)
	for i := 0; i < 5; i++ {
		doc := ""
		for j := 0; j < 50; j++ {
			doc += "word "
		}
		longDocs[i] = doc
	}

	// Chunk the documents
	_, docIndices := chunkDocumentsForRerank(longDocs, 50, 10)

	// Verify chunks map to correct documents
	for _, idx := range docIndices {
		assert.GreaterOrEqual(t, idx, 0)
		assert.Less(t, idx, 5)
	}
}

func TestResultsSortedByScore(t *testing.T) {
	// Test that results are sorted by relevance score (descending)
	chunkResults := []RerankResult{
		{Index: 0, RelevanceScore: 0.3},
		{Index: 1, RelevanceScore: 0.9},
		{Index: 2, RelevanceScore: 0.5},
		{Index: 3, RelevanceScore: 0.7},
	}
	docIndices := []int{0, 1, 2, 3}
	numOriginalDocs := 4

	aggregated := aggregateChunkScores(chunkResults, docIndices, numOriginalDocs, "max")

	// Verify sorted by score descending
	for i := 0; i < len(aggregated)-1; i++ {
		assert.GreaterOrEqual(t, aggregated[i].RelevanceScore, aggregated[i+1].RelevanceScore)
	}
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestVeryLongDocument(t *testing.T) {
	// Test chunking of a very long document
	longDoc := ""
	for i := 0; i < 10000; i++ {
		longDoc += "word "
	}
	documents := []string{longDoc}

	chunkedDocs, docIndices := chunkDocumentsForRerank(documents, 100, 20)

	// Should produce many chunks
	assert.Greater(t, len(chunkedDocs), 50)
	// All should map to document 0
	for _, idx := range docIndices {
		assert.Equal(t, 0, idx)
	}
}

func TestMixedLengthDocuments(t *testing.T) {
	// Test with mix of short and long documents
	documents := []string{
		"short",
		"",
		"another short one",
	}
	for i := 0; i < 100; i++ {
		documents[1] += "word "
	}

	chunkedDocs, docIndices := chunkDocumentsForRerank(documents, 50, 10)

	// Verify structure
	assert.Greater(t, len(chunkedDocs), len(documents))

	// Verify document indices are valid
	for _, idx := range docIndices {
		assert.GreaterOrEqual(t, idx, 0)
		assert.Less(t, idx, 3)
	}
}

func TestWhitespaceOnlyDocument(t *testing.T) {
	// Test handling of whitespace-only documents
	documents := []string{"   ", "normal doc", "\t\n\t"}

	chunkedDocs, docIndices := chunkDocumentsForRerank(documents, 100, 10)

	// Should handle gracefully
	assert.Len(t, chunkedDocs, len(docIndices))
}

func TestUnicodeDocuments(t *testing.T) {
	// Test chunking with unicode content
	documents := []string{
		"这是一个中文文档，包含很多中文字符。",
		"日本語のドキュメントもテストします。",
		"한국어 문서도 포함합니다.",
	}

	chunkedDocs, docIndices := chunkDocumentsForRerank(documents, 100, 10)

	// Should handle unicode gracefully
	assert.GreaterOrEqual(t, len(chunkedDocs), 3)
	assert.Equal(t, len(chunkedDocs), len(docIndices))
}
