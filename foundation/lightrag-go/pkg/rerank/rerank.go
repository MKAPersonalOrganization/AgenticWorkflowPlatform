// Package rerank provides document reranking functionality.
package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
)

// RerankResult represents a single rerank result.
type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	Document       string  `json:"document,omitempty"`
}

// RerankRequest represents a rerank request.
type RerankRequest struct {
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	Model           string   `json:"model,omitempty"`
	TopN            int      `json:"top_n,omitempty"`
	ReturnDocuments bool     `json:"return_documents,omitempty"`
}

// RerankResponse represents a rerank response.
type RerankResponse struct {
	Results []RerankResult `json:"results"`
	Model   string         `json:"model,omitempty"`
	Usage   *UsageInfo     `json:"usage,omitempty"`
}

// UsageInfo represents token usage.
type UsageInfo struct {
	TotalTokens int `json:"total_tokens"`
}

// Reranker is the interface for document reranking.
type Reranker interface {
	// Rerank reranks documents based on the query.
	Rerank(ctx context.Context, req *RerankRequest) (*RerankResponse, error)

	// ModelName returns the model name.
	ModelName() string

	// Close releases resources.
	Close() error
}

// RerankerConfig holds configuration for rerankers.
type RerankerConfig struct {
	APIKey          string
	BaseURL         string
	Model           string
	MaxRetries      int
	Timeout         int
	EnableChunking  bool
	MaxTokensPerDoc int
	ChunkOverlap    int
}

// DefaultRerankerConfig returns default configuration.
func DefaultRerankerConfig() *RerankerConfig {
	return &RerankerConfig{
		MaxRetries:      3,
		Timeout:         60,
		EnableChunking:  false,
		MaxTokensPerDoc: 480,
		ChunkOverlap:    32,
	}
}

// JinaReranker implements Reranker using Jina AI API.
type JinaReranker struct {
	config     *RerankerConfig
	httpClient *http.Client
	logger     *logrus.Logger
}

// NewJinaReranker creates a new Jina reranker.
func NewJinaReranker(config *RerankerConfig, logger *logrus.Logger) *JinaReranker {
	if logger == nil {
		logger = logrus.New()
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.jina.ai/v1/rerank"
	}
	if config.Model == "" {
		config.Model = "jina-reranker-v2-base-multilingual"
	}
	if config.Timeout == 0 {
		config.Timeout = 60
	}

	return &JinaReranker{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		logger: logger,
	}
}

// jinaRerankRequest represents the Jina rerank request format.
type jinaRerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            int      `json:"top_n,omitempty"`
	ReturnDocuments bool     `json:"return_documents,omitempty"`
}

// jinaRerankResponse represents the Jina rerank response format.
type jinaRerankResponse struct {
	Model   string `json:"model"`
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
		Document       struct {
			Text string `json:"text"`
		} `json:"document,omitempty"`
	} `json:"results"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// Rerank performs document reranking using Jina API.
func (r *JinaReranker) Rerank(ctx context.Context, req *RerankRequest) (*RerankResponse, error) {
	if len(req.Documents) == 0 {
		return &RerankResponse{Results: []RerankResult{}}, nil
	}

	documents := req.Documents
	docIndices := make([]int, len(documents))
	for i := range docIndices {
		docIndices[i] = i
	}

	// Apply chunking if enabled
	if r.config.EnableChunking {
		documents, docIndices = chunkDocumentsForRerank(documents, r.config.MaxTokensPerDoc, r.config.ChunkOverlap)
	}

	model := req.Model
	if model == "" {
		model = r.config.Model
	}

	jinaReq := jinaRerankRequest{
		Model:           model,
		Query:           req.Query,
		Documents:       documents,
		TopN:            req.TopN,
		ReturnDocuments: req.ReturnDocuments,
	}

	body, err := json.Marshal(jinaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", r.config.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.config.APIKey)
	}

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jina API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var jinaResp jinaRerankResponse
	if err := json.Unmarshal(respBody, &jinaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert results
	chunkResults := make([]RerankResult, len(jinaResp.Results))
	for i, res := range jinaResp.Results {
		chunkResults[i] = RerankResult{
			Index:          res.Index,
			RelevanceScore: res.RelevanceScore,
			Document:       res.Document.Text,
		}
	}

	// Aggregate if chunking was applied
	var results []RerankResult
	if r.config.EnableChunking && len(docIndices) != len(req.Documents) {
		results = aggregateChunkScores(chunkResults, docIndices, len(req.Documents), "max")
	} else {
		results = chunkResults
	}

	return &RerankResponse{
		Results: results,
		Model:   jinaResp.Model,
		Usage: &UsageInfo{
			TotalTokens: jinaResp.Usage.TotalTokens,
		},
	}, nil
}

// ModelName returns the model name.
func (r *JinaReranker) ModelName() string {
	return r.config.Model
}

// Close closes the reranker.
func (r *JinaReranker) Close() error {
	r.httpClient.CloseIdleConnections()
	return nil
}

// CohereReranker implements Reranker using Cohere API.
type CohereReranker struct {
	config     *RerankerConfig
	httpClient *http.Client
	logger     *logrus.Logger
}

// NewCohereReranker creates a new Cohere reranker.
func NewCohereReranker(config *RerankerConfig, logger *logrus.Logger) *CohereReranker {
	if logger == nil {
		logger = logrus.New()
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.cohere.ai/v1/rerank"
	}
	if config.Model == "" {
		config.Model = "rerank-english-v3.0"
	}
	if config.Timeout == 0 {
		config.Timeout = 60
	}

	return &CohereReranker{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		logger: logger,
	}
}

// cohereRerankRequest represents the Cohere rerank request format.
type cohereRerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            int      `json:"top_n,omitempty"`
	ReturnDocuments bool     `json:"return_documents,omitempty"`
}

// cohereRerankResponse represents the Cohere rerank response format.
type cohereRerankResponse struct {
	ID      string `json:"id"`
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
		Document       struct {
			Text string `json:"text"`
		} `json:"document,omitempty"`
	} `json:"results"`
	Meta struct {
		BilledUnits struct {
			SearchUnits int `json:"search_units"`
		} `json:"billed_units"`
	} `json:"meta"`
}

// Rerank performs document reranking using Cohere API.
func (r *CohereReranker) Rerank(ctx context.Context, req *RerankRequest) (*RerankResponse, error) {
	if len(req.Documents) == 0 {
		return &RerankResponse{Results: []RerankResult{}}, nil
	}

	documents := req.Documents
	docIndices := make([]int, len(documents))
	for i := range docIndices {
		docIndices[i] = i
	}

	if r.config.EnableChunking {
		documents, docIndices = chunkDocumentsForRerank(documents, r.config.MaxTokensPerDoc, r.config.ChunkOverlap)
	}

	model := req.Model
	if model == "" {
		model = r.config.Model
	}

	cohereReq := cohereRerankRequest{
		Model:           model,
		Query:           req.Query,
		Documents:       documents,
		TopN:            req.TopN,
		ReturnDocuments: req.ReturnDocuments,
	}

	body, err := json.Marshal(cohereReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", r.config.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.config.APIKey)
	}

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Cohere API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var cohereResp cohereRerankResponse
	if err := json.Unmarshal(respBody, &cohereResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	chunkResults := make([]RerankResult, len(cohereResp.Results))
	for i, res := range cohereResp.Results {
		chunkResults[i] = RerankResult{
			Index:          res.Index,
			RelevanceScore: res.RelevanceScore,
			Document:       res.Document.Text,
		}
	}

	var results []RerankResult
	if r.config.EnableChunking && len(docIndices) != len(req.Documents) {
		results = aggregateChunkScores(chunkResults, docIndices, len(req.Documents), "max")
	} else {
		results = chunkResults
	}

	return &RerankResponse{
		Results: results,
	}, nil
}

// ModelName returns the model name.
func (r *CohereReranker) ModelName() string {
	return r.config.Model
}

// Close closes the reranker.
func (r *CohereReranker) Close() error {
	r.httpClient.CloseIdleConnections()
	return nil
}

// chunkDocumentsForRerank splits documents that exceed token limit.
func chunkDocumentsForRerank(documents []string, maxTokens, overlapTokens int) ([]string, []int) {
	// Ensure overlap is less than max tokens
	if overlapTokens >= maxTokens {
		overlapTokens = maxTokens - 1
		if overlapTokens < 0 {
			overlapTokens = 0
		}
	}

	// Use character-based approximation (1 token ≈ 4 characters)
	maxChars := maxTokens * 4
	overlapChars := overlapTokens * 4

	var chunkedDocs []string
	var docIndices []int

	for idx, doc := range documents {
		if len(doc) <= maxChars {
			chunkedDocs = append(chunkedDocs, doc)
			docIndices = append(docIndices, idx)
		} else {
			// Split into overlapping chunks
			start := 0
			for start < len(doc) {
				end := start + maxChars
				if end > len(doc) {
					end = len(doc)
				}
				chunk := doc[start:end]
				chunkedDocs = append(chunkedDocs, chunk)
				docIndices = append(docIndices, idx)

				if end >= len(doc) {
					break
				}
				start = end - overlapChars
			}
		}
	}

	return chunkedDocs, docIndices
}

// aggregateChunkScores aggregates scores from chunks back to original documents.
func aggregateChunkScores(chunkResults []RerankResult, docIndices []int, numOriginalDocs int, aggregation string) []RerankResult {
	// Group scores by original document index
	docScores := make(map[int][]float64)

	for _, result := range chunkResults {
		chunkIdx := result.Index
		if chunkIdx >= 0 && chunkIdx < len(docIndices) {
			originalDocIdx := docIndices[chunkIdx]
			docScores[originalDocIdx] = append(docScores[originalDocIdx], result.RelevanceScore)
		}
	}

	// Aggregate scores
	var results []RerankResult
	for docIdx, scores := range docScores {
		if len(scores) == 0 {
			continue
		}

		var finalScore float64
		switch aggregation {
		case "mean":
			sum := 0.0
			for _, s := range scores {
				sum += s
			}
			finalScore = sum / float64(len(scores))
		case "first":
			finalScore = scores[0]
		default: // "max"
			finalScore = scores[0]
			for _, s := range scores[1:] {
				if s > finalScore {
					finalScore = s
				}
			}
		}

		results = append(results, RerankResult{
			Index:          docIdx,
			RelevanceScore: finalScore,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	return results
}

// LocalReranker implements Reranker using local similarity computation.
type LocalReranker struct {
	config *RerankerConfig
	logger *logrus.Logger
}

// NewLocalReranker creates a local reranker using basic similarity.
func NewLocalReranker(config *RerankerConfig, logger *logrus.Logger) *LocalReranker {
	if logger == nil {
		logger = logrus.New()
	}
	return &LocalReranker{
		config: config,
		logger: logger,
	}
}

// Rerank performs local reranking using TF-IDF like similarity.
func (r *LocalReranker) Rerank(ctx context.Context, req *RerankRequest) (*RerankResponse, error) {
	if len(req.Documents) == 0 {
		return &RerankResponse{Results: []RerankResult{}}, nil
	}

	queryTerms := tokenize(req.Query)
	results := make([]RerankResult, len(req.Documents))

	for i, doc := range req.Documents {
		docTerms := tokenize(doc)
		score := computeSimilarity(queryTerms, docTerms)
		results[i] = RerankResult{
			Index:          i,
			RelevanceScore: score,
		}
		if req.ReturnDocuments {
			results[i].Document = doc
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	// Apply topN
	if req.TopN > 0 && req.TopN < len(results) {
		results = results[:req.TopN]
	}

	return &RerankResponse{
		Results: results,
		Model:   "local",
	}, nil
}

// ModelName returns the model name.
func (r *LocalReranker) ModelName() string {
	return "local"
}

// Close closes the reranker.
func (r *LocalReranker) Close() error {
	return nil
}

// tokenize splits text into lowercase tokens.
func tokenize(text string) map[string]int {
	tokens := make(map[string]int)
	current := ""

	for _, ch := range text {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			if ch >= 'A' && ch <= 'Z' {
				ch = ch + 32 // lowercase
			}
			current += string(ch)
		} else {
			if current != "" {
				tokens[current]++
				current = ""
			}
		}
	}
	if current != "" {
		tokens[current]++
	}

	return tokens
}

// computeSimilarity computes a simple TF-based similarity score.
func computeSimilarity(queryTerms, docTerms map[string]int) float64 {
	if len(queryTerms) == 0 || len(docTerms) == 0 {
		return 0
	}

	// Compute dot product and magnitudes
	var dotProduct, queryMag, docMag float64

	for term, qCount := range queryTerms {
		qf := float64(qCount)
		queryMag += qf * qf
		if dCount, ok := docTerms[term]; ok {
			df := float64(dCount)
			dotProduct += qf * df
		}
	}

	for _, dCount := range docTerms {
		df := float64(dCount)
		docMag += df * df
	}

	if queryMag == 0 || docMag == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(queryMag) * math.Sqrt(docMag))
}
