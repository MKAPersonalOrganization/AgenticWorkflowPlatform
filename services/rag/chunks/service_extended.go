package chunks

// Extended service methods for additional chunk endpoints.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"school-with-ai/domain"
)

// ChunkLineageResponse represents lineage information for a chunk.
type ChunkLineageResponse struct {
	ChunkID string      `json:"chunk_id"`
	Lineage interface{} `json:"lineage"`
}

// GetChunkLineage retrieves lineage for a specific chunk.
func (s *Service) GetChunkLineage(ctx context.Context, tCtx *domain.TenantContext, chunkID string) (*ChunkLineageResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/chunks/%s/lineage", s.baseURL, url.PathEscape(chunkID))

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response ChunkLineageResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}
