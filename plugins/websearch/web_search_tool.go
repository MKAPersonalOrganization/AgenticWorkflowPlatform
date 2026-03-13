package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"school-with-ai/foundation/agent"
)

// =========================================================================
// Configuration & constants
// =========================================================================

const (
	// maxQueryLength limits the search query to prevent abuse.
	maxQueryLength = 500

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
// Deprecated: kept as a fallback; new tools receive their own limiter.
var globalLimiter = newRateLimiter()

// =========================================================================
// WebSearchTool
// =========================================================================

// WebSearchTool performs a grounded Google Search via a GroundedSearchClient.
// It is decoupled from any specific AI SDK — the concrete client (e.g. Gemini)
// is injected at construction time from the adapter layer.
type WebSearchTool struct {
	client  GroundedSearchClient
	limiter *rateLimiter
}

// WebSearchToolOption configures optional parameters for NewWebSearchTool.
type WebSearchToolOption func(*WebSearchTool)

// WithRateLimiter sets a custom rate limiter for the tool.
// Useful for tests or per-instance isolation.
func WithRateLimiter(rl *rateLimiter) WebSearchToolOption {
	return func(t *WebSearchTool) {
		t.limiter = rl
	}
}

// NewWebSearchTool creates a new web search tool backed by the given client.
func NewWebSearchTool(client GroundedSearchClient, opts ...WebSearchToolOption) *WebSearchTool {
	t := &WebSearchTool{
		client:  client,
		limiter: newRateLimiter(),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *WebSearchTool) Name() string          { return "web_search" }
func (t *WebSearchTool) Description() string   { return webSearchSchema.Description }
func (t *WebSearchTool) Schema() *agent.Schema { return webSearchSchema }

// Execute performs a grounded web search.
func (t *WebSearchTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var parsed WebSearchArgs
	if err := args.Decode(&parsed); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
	}

	query := strings.TrimSpace(parsed.Query)
	if query == "" {
		return agent.ErrorResult(fmt.Errorf("query must be a non-empty string")), nil
	}
	if len(query) > maxQueryLength {
		return agent.ErrorResult(fmt.Errorf("query too long: maximum %d characters, got %d", maxQueryLength, len(query))), nil
	}

	userID := args.UserID
	if userID == "" {
		userID = "anonymous"
	}
	if !t.limiter.allow(userID) {
		return agent.ErrorResult(fmt.Errorf("rate limit exceeded: maximum %d searches per minute", rateLimitMax)), nil
	}

	if !t.client.Configured() {
		return agent.ErrorResult(fmt.Errorf("web search is not configured: missing API key")), nil
	}

	result, err := t.client.Search(ctx, query)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("web search failed: %w", err)), nil
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to serialize result: %w", err)), nil
	}

	return agent.Result{
		Content: []any{string(resultJSON)},
	}, nil
}

// =========================================================================
// Helpers (shared with adapter layer)
// =========================================================================

// InsertCitationMarkers inserts [1][2] style markers into the text at segment
// boundaries. Exported so the adapter layer can use it when parsing responses.
// Processes segments in reverse order to preserve byte offsets (matching the
// Gemini CLI approach).
func InsertCitationMarkers(text string, citations []Citation) string {
	if len(citations) == 0 {
		return text
	}

	// Sort citations by end index descending so insertions don't shift offsets.
	sorted := make([]Citation, len(citations))
	copy(sorted, citations)
	for i := range len(sorted) {
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
			fmt.Fprintf(&marker, "[%d]", idx)
		}

		endIdx := min(cit.EndIndex, len(runes))
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

var _ agent.ExecutableTool = (*WebSearchTool)(nil)
