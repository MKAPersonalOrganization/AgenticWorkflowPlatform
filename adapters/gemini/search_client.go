// Package gemini provides adapters that bridge Google's genai SDK to
// framework-agnostic interfaces defined in the plugin layer.
// All direct google.golang.org/genai imports are confined to this package.
package gemini

import (
	"context"
	"fmt"
	"strings"

	"school-with-ai/plugins/websearch"

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
		return nil, fmt.Errorf("Gemini API call failed: %w", err)
	}

	return parseGroundedResponse(query, resp)
}

// parseGroundedResponse converts a Gemini GenerateContentResponse into a
// websearch.WebSearchResult with sources and inline citation markers.
func parseGroundedResponse(query string, resp *genai.GenerateContentResponse) (*websearch.WebSearchResult, error) {
	const maxSourcesReturned = 10

	if resp == nil || len(resp.Candidates) == 0 {
		return &websearch.WebSearchResult{
			Status:  "no_results",
			Query:   query,
			Summary: "No results found for this query.",
		}, nil
	}

	candidate := resp.Candidates[0]

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
		return &websearch.WebSearchResult{
			Status:  "no_results",
			Query:   query,
			Summary: "The search returned no useful content.",
		}, nil
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

	seenURLs := make(map[string]bool)
	for i, chunk := range gm.GroundingChunks {
		if i >= maxSourcesReturned {
			break
		}
		if chunk.Web != nil && chunk.Web.URI != "" && !seenURLs[chunk.Web.URI] {
			seenURLs[chunk.Web.URI] = true
			result.Sources = append(result.Sources, websearch.Source{
				Index:  len(result.Sources) + 1,
				Title:  chunk.Web.Title,
				URL:    chunk.Web.URI,
				Domain: extractDomain(chunk.Web.URI),
			})
		}
	}
	result.SourceCount = len(result.Sources)
	result.HasCitations = len(result.Sources) > 0

	var citations []websearch.Citation
	for _, support := range gm.GroundingSupports {
		if support.Segment == nil || support.Segment.Text == "" {
			continue
		}

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

		citations = append(citations, websearch.Citation{
			Text:          support.Segment.Text,
			SourceIndices: sourceIndices,
			StartIndex:    int(support.Segment.StartIndex),
			EndIndex:      int(support.Segment.EndIndex),
			ConfidenceAvg: avgConfidence,
		})
	}

	result.Citations = citations
	result.Summary = websearch.InsertCitationMarkers(summary, citations)

	return result, nil
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
