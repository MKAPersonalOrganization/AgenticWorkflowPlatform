// Package gemini provides adapters that bridge Google's genai SDK to
// framework-agnostic interfaces defined in the plugin layer.
// All direct google.golang.org/genai imports are confined to this package.
package gemini

import (
	"aicompass/plugins/websearch"
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

const defaultSearchModel = "gemini-2.5-flash"

// SearchClient implements websearch.GroundedSearchClient using the Gemini API.
type SearchClient struct {
	apiKey string
	model  string
}

// NewSearchClient creates a SearchClient. If model is empty it falls back to
// the default search model. apiKey may be empty — Configured() will return
// false and the tool will surface a user-visible error on first use.
func NewSearchClient(apiKey, model string) *SearchClient {
	if model == "" {
		model = defaultSearchModel
	}
	return &SearchClient{apiKey: apiKey, model: model}
}

// Configured reports whether an API key has been provided.
func (c *SearchClient) Configured() bool { return c.apiKey != "" }

// Search calls the Gemini API with GoogleSearch grounding and returns a
// structured result with synthesised answer, sources, and citations.
func (c *SearchClient) Search(ctx context.Context, query string) (*websearch.WebSearchResult, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: c.apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Gemini client: %w", err)
	}

	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
	}

	promptText := fmt.Sprintf(
		"Search the web for: %s\n\nProvide a comprehensive, well-organized answer based on the search results. Include all relevant details.",
		query,
	)

	resp, err := client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: promptText}},
		},
	}, config)
	if err != nil {
		return nil, fmt.Errorf("gemini API call failed: %w", err)
	}

	return parseGroundedResponse(query, resp)
}

// parseGroundedResponse converts a Gemini GenerateContentResponse into a
// websearch.WebSearchResult with sources and inline citation markers.
func parseGroundedResponse(query string, resp *genai.GenerateContentResponse) (*websearch.WebSearchResult, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return noResultsResult(query, "No results found for this query."), nil
	}

	candidate := resp.Candidates[0]
	summary := extractAnswerText(candidate)
	if summary == "" {
		return noResultsResult(query, "The search returned no useful content."), nil
	}

	result := &websearch.WebSearchResult{
		Status:  "ok",
		Query:   query,
		Summary: summary,
		Sources: make([]websearch.Source, 0),
	}

	gm := candidate.GroundingMetadata
	if gm == nil {
		return result, nil
	}

	result.SearchQueries = gm.WebSearchQueries
	if gm.SearchEntryPoint != nil {
		result.SearchWidgetHTML = gm.SearchEntryPoint.RenderedContent
	}

	result.Sources = collectSources(gm.GroundingChunks)
	result.SourceCount = len(result.Sources)
	result.HasCitations = len(result.Sources) > 0

	result.Citations = collectCitations(gm.GroundingSupports, gm.GroundingChunks, result.Sources)
	result.Summary = websearch.InsertCitationMarkers(summary, result.Citations)

	return result, nil
}

func noResultsResult(query, summary string) *websearch.WebSearchResult {
	return &websearch.WebSearchResult{Status: "no_results", Query: query, Summary: summary}
}

func extractAnswerText(candidate *genai.Candidate) string {
	if candidate.Content == nil {
		return ""
	}
	var sb strings.Builder
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			sb.WriteString(part.Text)
		}
	}
	return sb.String()
}

// collectSources dedupes grounding chunks by URI and caps at maxSourcesReturned.
func collectSources(chunks []*genai.GroundingChunk) []websearch.Source {
	const maxSourcesReturned = 10
	sources := make([]websearch.Source, 0)
	seen := make(map[string]bool)
	for _, chunk := range chunks {
		if len(sources) >= maxSourcesReturned {
			break
		}
		if chunk.Web == nil || chunk.Web.URI == "" || seen[chunk.Web.URI] {
			continue
		}
		seen[chunk.Web.URI] = true
		sources = append(sources, websearch.Source{
			Index:  len(sources) + 1,
			Title:  chunk.Web.Title,
			URL:    chunk.Web.URI,
			Domain: extractDomain(chunk.Web.URI),
		})
	}
	return sources
}

func collectCitations(supports []*genai.GroundingSupport, chunks []*genai.GroundingChunk, sources []websearch.Source) []websearch.Citation {
	var citations []websearch.Citation
	for _, support := range supports {
		if support.Segment == nil || support.Segment.Text == "" {
			continue
		}
		indices, avgConfidence := citationSourceIndices(support, chunks, sources)
		citations = append(citations, websearch.Citation{
			Text:          support.Segment.Text,
			SourceIndices: indices,
			StartIndex:    int(support.Segment.StartIndex),
			EndIndex:      int(support.Segment.EndIndex),
			ConfidenceAvg: avgConfidence,
		})
	}
	return citations
}

// citationSourceIndices resolves a support's chunk references into result-space
// source indices and returns the mean confidence across the referenced chunks.
func citationSourceIndices(support *genai.GroundingSupport, chunks []*genai.GroundingChunk, sources []websearch.Source) ([]int, float32) {
	indices := make([]int, 0, len(support.GroundingChunkIndices))
	var confidenceSum float32
	for i, chunkIdx := range support.GroundingChunkIndices {
		if idx, ok := sourceIndexForChunk(int(chunkIdx), chunks, sources); ok {
			indices = append(indices, idx)
		}
		if i < len(support.ConfidenceScores) {
			confidenceSum += support.ConfidenceScores[i]
		}
	}
	var avg float32
	if len(indices) > 0 {
		avg = confidenceSum / float32(len(indices))
	}
	return indices, avg
}

// sourceIndexForChunk maps a grounding-chunk index to its result-space source
// index via URL equality. Returns (0, false) if the chunk is out of range,
// missing Web metadata, or no matching source was collected.
func sourceIndexForChunk(chunkIdx int, chunks []*genai.GroundingChunk, sources []websearch.Source) (int, bool) {
	if chunkIdx < 0 || chunkIdx >= len(chunks) {
		return 0, false
	}
	chunk := chunks[chunkIdx]
	if chunk.Web == nil {
		return 0, false
	}
	for _, s := range sources {
		if s.URL == chunk.Web.URI {
			return s.Index, true
		}
	}
	return 0, false
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

var _ websearch.GroundedSearchClient = (*SearchClient)(nil)
