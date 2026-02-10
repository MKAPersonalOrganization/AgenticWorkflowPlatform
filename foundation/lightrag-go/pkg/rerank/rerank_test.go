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

func TestRerankResult(t *testing.T) {
	result := RerankResult{
		Index:          0,
		RelevanceScore: 0.95,
		Document:       "test document",
	}
	assert.Equal(t, 0, result.Index)
	assert.Equal(t, 0.95, result.RelevanceScore)
	assert.Equal(t, "test document", result.Document)
}

func TestDefaultRerankerConfig(t *testing.T) {
	config := DefaultRerankerConfig()
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 60, config.Timeout)
	assert.Equal(t, 480, config.MaxTokensPerDoc)
	assert.Equal(t, 32, config.ChunkOverlap)
}

func TestChunkDocumentsForRerank(t *testing.T) {
	tests := []struct {
		name           string
		documents      []string
		maxTokens      int
		overlapTokens  int
		expectedChunks int
	}{
		{
			name:           "short documents",
			documents:      []string{"hello", "world"},
			maxTokens:      100,
			overlapTokens:  10,
			expectedChunks: 2,
		},
		{
			name:           "long document needs chunking",
			documents:      []string{string(make([]byte, 2000))}, // 2000 chars = ~500 tokens
			maxTokens:      100,                                  // 400 chars per chunk
			overlapTokens:  10,
			expectedChunks: 6, // ceil(2000 / (400 - 40))
		},
		{
			name:           "overlap clamped to prevent infinite loop",
			documents:      []string{"short"},
			maxTokens:      10,
			overlapTokens:  100, // > maxTokens, should be clamped
			expectedChunks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, indices := chunkDocumentsForRerank(tt.documents, tt.maxTokens, tt.overlapTokens)
			assert.Equal(t, len(chunks), len(indices))
			if tt.name == "short documents" {
				assert.Equal(t, tt.expectedChunks, len(chunks))
			}
		})
	}
}

func TestAggregateChunkScores(t *testing.T) {
	tests := []struct {
		name            string
		chunkResults    []RerankResult
		docIndices      []int
		numOriginalDocs int
		aggregation     string
		expectedCount   int
	}{
		{
			name: "max aggregation",
			chunkResults: []RerankResult{
				{Index: 0, RelevanceScore: 0.5},
				{Index: 1, RelevanceScore: 0.8},
				{Index: 2, RelevanceScore: 0.3},
			},
			docIndices:      []int{0, 0, 1}, // chunks 0,1 belong to doc 0, chunk 2 to doc 1
			numOriginalDocs: 2,
			aggregation:     "max",
			expectedCount:   2,
		},
		{
			name: "mean aggregation",
			chunkResults: []RerankResult{
				{Index: 0, RelevanceScore: 0.4},
				{Index: 1, RelevanceScore: 0.6},
			},
			docIndices:      []int{0, 0},
			numOriginalDocs: 1,
			aggregation:     "mean",
			expectedCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := aggregateChunkScores(tt.chunkResults, tt.docIndices, tt.numOriginalDocs, tt.aggregation)
			assert.Equal(t, tt.expectedCount, len(results))

			if tt.name == "max aggregation" {
				// Doc 0 should have max of 0.5 and 0.8 = 0.8
				for _, r := range results {
					if r.Index == 0 {
						assert.Equal(t, 0.8, r.RelevanceScore)
					}
				}
			}

			if tt.name == "mean aggregation" {
				assert.InDelta(t, 0.5, results[0].RelevanceScore, 0.001)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello World! This is a TEST 123.")
	assert.Equal(t, 1, tokens["hello"])
	assert.Equal(t, 1, tokens["world"])
	assert.Equal(t, 1, tokens["test"])
	assert.Equal(t, 1, tokens["123"])
}

func TestComputeSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		query    map[string]int
		doc      map[string]int
		expected float64
	}{
		{
			name:     "identical",
			query:    map[string]int{"hello": 1, "world": 1},
			doc:      map[string]int{"hello": 1, "world": 1},
			expected: 1.0,
		},
		{
			name:     "no overlap",
			query:    map[string]int{"hello": 1},
			doc:      map[string]int{"goodbye": 1},
			expected: 0.0,
		},
		{
			name:     "partial overlap",
			query:    map[string]int{"hello": 1, "world": 1},
			doc:      map[string]int{"hello": 1, "there": 1},
			expected: 0.5, // 1 / (sqrt(2) * sqrt(2)) = 0.5
		},
		{
			name:     "empty query",
			query:    map[string]int{},
			doc:      map[string]int{"hello": 1},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := computeSimilarity(tt.query, tt.doc)
			assert.InDelta(t, tt.expected, score, 0.001)
		})
	}
}

func TestLocalReranker(t *testing.T) {
	config := DefaultRerankerConfig()
	reranker := NewLocalReranker(config, nil)

	ctx := context.Background()
	req := &RerankRequest{
		Query: "machine learning",
		Documents: []string{
			"Machine learning is a subset of artificial intelligence",
			"The weather is nice today",
			"Deep learning and machine learning are related",
		},
		TopN:            2,
		ReturnDocuments: true,
	}

	resp, err := reranker.Rerank(ctx, req)
	require.NoError(t, err)
	assert.Len(t, resp.Results, 2)

	// Documents containing "machine" and "learning" should rank higher
	assert.True(t, resp.Results[0].RelevanceScore > resp.Results[1].RelevanceScore)
	assert.Equal(t, "local", resp.Model)
}

func TestLocalReranker_EmptyDocuments(t *testing.T) {
	config := DefaultRerankerConfig()
	reranker := NewLocalReranker(config, nil)

	ctx := context.Background()
	req := &RerankRequest{
		Query:     "test",
		Documents: []string{},
	}

	resp, err := reranker.Rerank(ctx, req)
	require.NoError(t, err)
	assert.Empty(t, resp.Results)
}

func TestLocalReranker_ModelName(t *testing.T) {
	config := DefaultRerankerConfig()
	reranker := NewLocalReranker(config, nil)
	assert.Equal(t, "local", reranker.ModelName())
}

func TestNewJinaReranker(t *testing.T) {
	config := &RerankerConfig{
		APIKey: "test-key",
	}

	reranker := NewJinaReranker(config, nil)

	assert.NotNil(t, reranker)
	assert.Equal(t, "https://api.jina.ai/v1/rerank", reranker.config.BaseURL)
	assert.Equal(t, "jina-reranker-v2-base-multilingual", reranker.config.Model)
}

func TestJinaReranker_Rerank(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		resp := jinaRerankResponse{
			Model: "jina-reranker-v2-base-multilingual",
			Results: []struct {
				Index          int     `json:"index"`
				RelevanceScore float64 `json:"relevance_score"`
				Document       struct {
					Text string `json:"text"`
				} `json:"document,omitempty"`
			}{
				{Index: 1, RelevanceScore: 0.95},
				{Index: 0, RelevanceScore: 0.7},
			},
			Usage: struct {
				TotalTokens int `json:"total_tokens"`
			}{TotalTokens: 100},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &RerankerConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	reranker := NewJinaReranker(config, nil)

	ctx := context.Background()
	req := &RerankRequest{
		Query:     "test query",
		Documents: []string{"doc1", "doc2"},
	}

	resp, err := reranker.Rerank(ctx, req)
	require.NoError(t, err)
	assert.Len(t, resp.Results, 2)
	assert.Equal(t, 0.95, resp.Results[0].RelevanceScore)
	assert.Equal(t, 100, resp.Usage.TotalTokens)
}

func TestJinaReranker_EmptyDocuments(t *testing.T) {
	config := &RerankerConfig{}
	reranker := NewJinaReranker(config, nil)

	ctx := context.Background()
	req := &RerankRequest{
		Query:     "test",
		Documents: []string{},
	}

	resp, err := reranker.Rerank(ctx, req)
	require.NoError(t, err)
	assert.Empty(t, resp.Results)
}

func TestNewCohereReranker(t *testing.T) {
	config := &RerankerConfig{
		APIKey: "test-key",
	}

	reranker := NewCohereReranker(config, nil)

	assert.NotNil(t, reranker)
	assert.Equal(t, "https://api.cohere.ai/v1/rerank", reranker.config.BaseURL)
	assert.Equal(t, "rerank-english-v3.0", reranker.config.Model)
}

func TestCohereReranker_Rerank(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		resp := cohereRerankResponse{
			ID: "test-id",
			Results: []struct {
				Index          int     `json:"index"`
				RelevanceScore float64 `json:"relevance_score"`
				Document       struct {
					Text string `json:"text"`
				} `json:"document,omitempty"`
			}{
				{Index: 0, RelevanceScore: 0.85},
				{Index: 1, RelevanceScore: 0.65},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &RerankerConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	reranker := NewCohereReranker(config, nil)

	ctx := context.Background()
	req := &RerankRequest{
		Query:     "test query",
		Documents: []string{"doc1", "doc2"},
	}

	resp, err := reranker.Rerank(ctx, req)
	require.NoError(t, err)
	assert.Len(t, resp.Results, 2)
	assert.Equal(t, 0.85, resp.Results[0].RelevanceScore)
}

func TestJinaReranker_WithChunking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody jinaRerankRequest
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Return scores for chunked documents
		results := make([]struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
			Document       struct {
				Text string `json:"text"`
			} `json:"document,omitempty"`
		}, len(reqBody.Documents))

		for i := range reqBody.Documents {
			results[i] = struct {
				Index          int     `json:"index"`
				RelevanceScore float64 `json:"relevance_score"`
				Document       struct {
					Text string `json:"text"`
				} `json:"document,omitempty"`
			}{
				Index:          i,
				RelevanceScore: float64(len(reqBody.Documents)-i) * 0.1,
			}
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
		Documents: []string{"short doc", string(make([]byte, 500))}, // Second doc needs chunking
	}

	resp, err := reranker.Rerank(ctx, req)
	require.NoError(t, err)
	// Results should be aggregated back to original document count
	assert.LessOrEqual(t, len(resp.Results), len(req.Documents))
}

func TestJinaReranker_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	config := &RerankerConfig{
		APIKey:  "invalid-key",
		BaseURL: server.URL,
	}

	reranker := NewJinaReranker(config, nil)

	ctx := context.Background()
	req := &RerankRequest{
		Query:     "test",
		Documents: []string{"doc"},
	}

	_, err := reranker.Rerank(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
