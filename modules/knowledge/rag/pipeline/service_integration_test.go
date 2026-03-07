//go:build integration
// +build integration

package pipeline

import (
	"context"
	"os"
	"testing"

	"school-with-ai/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTenantContext() *shared.TenantContext {
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	return &shared.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "00000000-0000-0000-0000-000000000001",
		Role:        "admin",
	}
}

func TestService_Integration_PipelineStatus(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testTenantContext()

	resp, err := service.Status(ctx, tCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.PendingTasks, 0)
	assert.GreaterOrEqual(t, resp.ProcessingTasks, 0)
	assert.GreaterOrEqual(t, resp.CompletedTasks, 0)
	assert.GreaterOrEqual(t, resp.FailedTasks, 0)
}

func TestService_Integration_PipelineMetrics_NoActiveJob(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testTenantContext()

	// Queue metrics are always available (returns zeroed metrics when idle)
	resp, err := service.Metrics(ctx, tCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.PendingCount, uint64(0))
	assert.GreaterOrEqual(t, resp.ProcessingCount, uint64(0))
	assert.GreaterOrEqual(t, resp.MaxWorkers, uint32(0))
	assert.NotEmpty(t, resp.Timestamp)
}

func TestService_Integration_PipelineCancel_NoActiveJob(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testTenantContext()

	// Cancel returns 409 when no job is running
	_, err := service.Cancel(ctx, tCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "409")
}

func TestService_Integration_PipelineHistory_NoHistory(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testTenantContext()

	// History returns 404 when no cost history exists
	_, err := service.History(ctx, tCtx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_Integration_PipelineCostEstimate(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testTenantContext()

	estReq := &EstimateCostRequest{
		Model:        "gpt-4",
		InputTokens:  1000,
		OutputTokens: 500,
	}

	resp, err := service.EstimateCost(ctx, tCtx, estReq)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "gpt-4", resp.Model)
	assert.Equal(t, 1000, resp.InputTokens)
	assert.Equal(t, 500, resp.OutputTokens)
	assert.GreaterOrEqual(t, resp.EstimatedCostUSD, 0.0)
	assert.NotEmpty(t, resp.FormattedCost)
}
