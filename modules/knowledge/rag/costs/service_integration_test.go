//go:build integration
// +build integration

package costs

import (
	"context"
	"os"
	"testing"

	"school-with-ai/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTenantContext() *domain.TenantContext {
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	return &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "00000000-0000-0000-0000-000000000001",
		Role:        "admin",
	}
}

func TestService_Integration_CostSummary(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testTenantContext()

	resp, err := service.Summary(ctx, tCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.WorkspaceID)
	assert.GreaterOrEqual(t, resp.TotalCost, 0.0)
	assert.GreaterOrEqual(t, resp.DocumentCount, 0)
	assert.GreaterOrEqual(t, resp.TotalTokens, 0)
	assert.NotNil(t, resp.ByOperation)
}

func TestService_Integration_CostHistory(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testTenantContext()

	resp, err := service.History(ctx, tCtx, nil)
	require.NoError(t, err)
	// Empty history is valid
	assert.NotNil(t, resp)
}

func TestService_Integration_GetBudget(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testTenantContext()

	resp, err := service.GetBudget(ctx, tCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.MonthlyBudgetUSD, 0.0)
	assert.GreaterOrEqual(t, resp.SpentUSD, 0.0)
	assert.GreaterOrEqual(t, resp.RemainingUSD, 0.0)
	assert.GreaterOrEqual(t, resp.AlertThreshold, 0.0)
}

func TestService_Integration_UpdateBudget(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testTenantContext()

	budget := &BudgetInfo{
		MonthlyBudgetUSD: 200.0,
		SpentUSD:         10.0,
		RemainingUSD:     190.0,
		AlertThreshold:   80.0,
		IsOverBudget:     false,
	}

	resp, err := service.UpdateBudget(ctx, tCtx, budget)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 200.0, resp.MonthlyBudgetUSD)
}
