package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"school-with-ai/foundation/agent"

	"google.golang.org/genai"
)

// =========================================================================
// Configuration & constants
// =========================================================================

const (
	// defaultSearchModel is the Gemini model used for grounded search.
	defaultSearchModel = "gemini-2.5-flash"

	// maxQueryLength limits the search query to prevent abuse.
	maxQueryLength = 500

	// maxSourcesReturned caps the number of source references included.
	maxSourcesReturned = 10

	// rateLimitWindow is the sliding window for per-user rate limiting.
	rateLimitWindow = time.Minute

	// rateLimitMax is the max searches allowed per user per window.
	rateLimitMax = 10
)

// =========================================================================
// Tool schema
// =========================================================================

var webSearchSchema = &agent.Schema{
	Type:        agent.TypeObject,
	Description: "Performs a grounded Google Search to find information across the internet. Returns a synthesized answer with citations (e.g., [1]) and source URIs. Best for finding up-to-date documentation, troubleshooting obscure errors, or broad research. Use this when you don't have a specific URL.",
	Properties: map[string]*agent.Schema{
		"query": {
			Type:        agent.TypeString,
			Description: "The search query. Be specific and descriptive for best results.",
		},
	},
	Required: []string{"query"},
}

// =========================================================================
// Input / output types
// =========================================================================

// WebSearchArgs is the input schema for the web_search tool.
type WebSearchArgs struct {
	Query string `json:"query"`
}

// Source represents a single web source reference.
type Source struct {
	Index  int    `json:"index"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Domain string `json:"domain,omitempty"`
}

// Citation maps a text segment to its supporting source indices.
type Citation struct {
	Text          string  `json:"text"`
	SourceIndices []int   `json:"source_indices"`
	StartIndex    int     `json:"start_index"`
	EndIndex      int     `json:"end_index"`
	ConfidenceAvg float32 `json:"confidence_avg"`
}

// WebSearchResult is the structured result returned to the UI and LLM.
type WebSearchResult struct {
	Status           string     `json:"status"`
	Query            string     `json:"query"`
	Summary          string     `json:"summary"`
	Sources          []Source   `json:"sources"`
	Citations        []Citation `json:"citations,omitempty"`
	SearchQueries    []string   `json:"search_queries,omitempty"`
	SourceCount      int        `json:"source_count"`
	HasCitations     bool       `json:"has_citations"`
	SearchWidgetHTML string     `json:"search_widget_html,omitempty"`
}

// =========================================================================
// Rate limiter (per-user, in-memory)
// =========================================================================

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string][]time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string][]time.Time),
	}
}

// allow returns true if the user is within their rate limit.
func (rl *rateLimiter) allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rateLimitWindow)

	// Prune expired entries.
	recent := rl.buckets[userID][:0]
	for _, t := range rl.buckets[userID] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= rateLimitMax {
		rl.buckets[userID] = recent
		return false
	}

	rl.buckets[userID] = append(recent, now)
	return true
}

// global rate limiter instance shared across all WebSearchTool instances.
var globalLimiter = newRateLimiter()

// =========================================================================
// WebSearchTool
// =========================================================================

// WebSearchTool performs a grounded Google Search via the Gemini API.
// It uses the GoogleSearch grounding tool to return synthesised answers
// with inline citations and source links — matching the behaviour of
// Gemini CLI's google_web_search tool.
type WebSearchTool struct {
	apiKey string
	model  string
}

// NewWebSearchTool creates a new web search tool. If model is empty,
// it falls back to the default search model.
func NewWebSearchTool(apiKey, model string) *WebSearchTool {
	if model == "" {
		model = defaultSearchModel
	}
	return &WebSearchTool{
		apiKey: apiKey,
		model:  model,
	}
}

func (t *WebSearchTool) Name() string          { return "web_search" }
func (t *WebSearchTool) Description() string   { return webSearchSchema.Description }
func (t *WebSearchTool) Schema() *agent.Schema { return webSearchSchema }

// Execute performs a grounded web search.
func (t *WebSearchTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	// Decode arguments.
	var parsed WebSearchArgs
	if err := args.Decode(&parsed); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
	}

	// Validate query.
	query := strings.TrimSpace(parsed.Query)
	if query == "" {
		return agent.ErrorResult(fmt.Errorf("query must be a non-empty string")), nil
	}
	if len(query) > maxQueryLength {
		return agent.ErrorResult(fmt.Errorf("query too long: maximum %d characters, got %d", maxQueryLength, len(query))), nil
	}

	// Rate limit check.
	userID := args.UserID
	if userID == "" {
		userID = "anonymous"
	}
	if !globalLimiter.allow(userID) {
		return agent.ErrorResult(fmt.Errorf("rate limit exceeded: maximum %d searches per minute", rateLimitMax)), nil
	}

	// Validate API key.
	if t.apiKey == "" {
		return agent.ErrorResult(fmt.Errorf("web search is not configured: missing Gemini API key")), nil
	}

	// Execute grounded search.
	result, err := t.executeGroundedSearch(ctx, query)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("web search failed: %w", err)), nil
	}

	// Serialize for return.
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to serialize result: %w", err)), nil
	}

	return agent.Result{
		Content: []any{string(resultJSON)},
	}, nil
}

// executeGroundedSearch calls the Gemini API with GoogleSearch grounding.
func (t *WebSearchTool) executeGroundedSearch(ctx context.Context, query string) (*WebSearchResult, error) {
	// Create a fresh client for each call (stateless, matches Gemini CLI pattern).
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: t.apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Gemini client: %w", err)
	}

	// Build the request with GoogleSearch grounding tool.
	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
	}

	// Construct the prompt — instruct the model to search and synthesize.
	promptText := fmt.Sprintf(
		"Search the web for: %s\n\nProvide a comprehensive, well-organized answer based on the search results. Include all relevant details.",
		query,
	)

	// Call Gemini with grounding.
	resp, err := client.Models.GenerateContent(ctx, t.model, []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: promptText}},
		},
	}, config)
	if err != nil {
		return nil, fmt.Errorf("Gemini API call failed: %w", err)
	}

	// Parse the response.
	return t.parseGroundedResponse(query, resp)
}

// parseGroundedResponse extracts the answer text, sources, and citations
// from the Gemini response's grounding metadata — exactly as Gemini CLI does.
func (t *WebSearchTool) parseGroundedResponse(query string, resp *genai.GenerateContentResponse) (*WebSearchResult, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return &WebSearchResult{
			Status:  "no_results",
			Query:   query,
			Summary: "No results found for this query.",
		}, nil
	}

	candidate := resp.Candidates[0]

	// Extract the synthesized answer text.
	var answerText strings.Builder
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				answerText.WriteString(part.Text)
			}
		}
	}

	summary := answerText.String()
	if summary == "" {
		return &WebSearchResult{
			Status:  "no_results",
			Query:   query,
			Summary: "The search returned no useful content.",
		}, nil
	}

	result := &WebSearchResult{
		Status:  "ok",
		Query:   query,
		Summary: summary,
		Sources: make([]Source, 0),
	}

	// Parse grounding metadata (sources + citations).
	gm := candidate.GroundingMetadata
	if gm == nil {
		result.SourceCount = 0
		return result, nil
	}

	// Collect search queries used by the model.
	result.SearchQueries = gm.WebSearchQueries

	// Collect the search entry point widget HTML (for rendering in UI).
	if gm.SearchEntryPoint != nil {
		result.SearchWidgetHTML = gm.SearchEntryPoint.RenderedContent
	}

	// Build source list from grounding chunks.
	seenURLs := make(map[string]bool)
	for i, chunk := range gm.GroundingChunks {
		if i >= maxSourcesReturned {
			break
		}
		if chunk.Web != nil && chunk.Web.URI != "" && !seenURLs[chunk.Web.URI] {
			seenURLs[chunk.Web.URI] = true
			source := Source{
				Index:  len(result.Sources) + 1,
				Title:  chunk.Web.Title,
				URL:    chunk.Web.URI,
				Domain: extractDomain(chunk.Web.URI),
			}
			result.Sources = append(result.Sources, source)
		}
	}
	result.SourceCount = len(result.Sources)
	result.HasCitations = len(result.Sources) > 0

	// Build citation map from grounding supports.
	// Each GroundingSupport links a text segment to its source chunks.
	for _, support := range gm.GroundingSupports {
		if support.Segment == nil || support.Segment.Text == "" {
			continue
		}

		// Map chunk indices to our 1-based source indices.
		sourceIndices := make([]int, 0, len(support.GroundingChunkIndices))
		var confidenceSum float32
		for i, chunkIdx := range support.GroundingChunkIndices {
			if int(chunkIdx) < len(gm.GroundingChunks) {
				chunk := gm.GroundingChunks[chunkIdx]
				if chunk.Web != nil {
					for _, s := range result.Sources {
						if s.URL == chunk.Web.URI {
							sourceIndices = append(sourceIndices, s.Index)
							break
						}
					}
				}
			}
			if i < len(support.ConfidenceScores) {
				confidenceSum += support.ConfidenceScores[i]
			}
		}

		var avgConfidence float32
		if len(sourceIndices) > 0 {
			avgConfidence = confidenceSum / float32(len(sourceIndices))
		}

		citation := Citation{
			Text:          support.Segment.Text,
			SourceIndices: sourceIndices,
			StartIndex:    int(support.Segment.StartIndex),
			EndIndex:      int(support.Segment.EndIndex),
			ConfidenceAvg: avgConfidence,
		}
		result.Citations = append(result.Citations, citation)
	}

	// Insert inline citation markers into the summary text.
	result.Summary = insertCitationMarkers(summary, result.Citations)

	return result, nil
}

// insertCitationMarkers inserts [1][2] style markers into the text at segment boundaries.
// Processes segments in reverse order to preserve byte offsets (matching Gemini CLI approach).
func insertCitationMarkers(text string, citations []Citation) string {
	if len(citations) == 0 {
		return text
	}

	// Sort citations by end index descending so insertions don't shift offsets.
	sorted := make([]Citation, len(citations))
	copy(sorted, citations)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].EndIndex > sorted[i].EndIndex {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	runes := []rune(text)
	for _, cit := range sorted {
		if len(cit.SourceIndices) == 0 {
			continue
		}

		var marker strings.Builder
		for _, idx := range cit.SourceIndices {
			marker.WriteString(fmt.Sprintf("[%d]", idx))
		}

		endIdx := cit.EndIndex
		if endIdx > len(runes) {
			endIdx = len(runes)
		}
		if endIdx <= 0 {
			continue
		}

		result := make([]rune, 0, len(runes)+len([]rune(marker.String())))
		result = append(result, runes[:endIdx]...)
		result = append(result, []rune(marker.String())...)
		result = append(result, runes[endIdx:]...)
		runes = result
	}

	return string(runes)
}

// extractDomain extracts the hostname from a URL string.
func extractDomain(rawURL string) string {
	s := rawURL
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
	}
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.Index(s, "?"); idx >= 0 {
		s = s[:idx]
	}
	return s
}

var _ agent.ExecutableTool = (*WebSearchTool)(nil)
