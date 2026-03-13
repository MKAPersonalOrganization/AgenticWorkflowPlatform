package websearch

import "context"

// GroundedSearchClient performs a grounded web search and returns structured results.
// The concrete implementation (e.g. Gemini API) lives in the adapter layer.
type GroundedSearchClient interface {
	// Search executes a grounded web search for the given query and returns a
	// structured result with a synthesized answer, sources, and citations.
	Search(ctx context.Context, query string) (*WebSearchResult, error)

	// Configured reports whether the client has the credentials needed to
	// execute a search (e.g. an API key has been provided).
	Configured() bool
}
