package pipeline

// Extended service methods for additional pipeline endpoints.

import (
	"aicompass/modules/knowledge/tenant"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// PricingResponse represents pipeline cost pricing.
type PricingResponse struct {
	Pricing interface{} `json:"pricing"`
}

// GetPricing retrieves pipeline cost pricing.
func (s *Service) GetPricing(ctx context.Context, tCtx *tenant.TenantContext) (*PricingResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/pipeline/costs/pricing", s.baseURL)

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

	var response PricingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}
