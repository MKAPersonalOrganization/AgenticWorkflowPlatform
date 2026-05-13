package chunks

// Extended service methods for additional chunk endpoints.

import (
	"aicompass/modules/knowledge/tenant"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// GetChunkLineage retrieves lineage for a specific chunk.
func (s *Service) GetChunkLineage(ctx context.Context, tCtx *tenant.TenantContext, chunkID string) (*ChunkLineageResponse, error) {
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
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response ChunkLineageResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}
