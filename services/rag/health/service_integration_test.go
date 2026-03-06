//go:build integration

package health_test

import (
	"context"
	"os"
	"testing"

	"school-with-ai/services/rag/health"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("EDGEQUAKE_BASE_URL")
	if u == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}
	return u
}

func TestHealthCheck(t *testing.T) {
	svc := health.NewService(baseURL(t), nil)
	resp, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "healthy", resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.NotEmpty(t, resp.StorageMode)
	assert.NotEmpty(t, resp.WorkspaceID)

	// Components
	assert.True(t, resp.Components.KVStorage)
	assert.True(t, resp.Components.VectorStorage)

	// Build info should be present
	if resp.BuildInfo != nil {
		assert.NotEmpty(t, resp.BuildInfo.BuildTimestamp)
	}

	// Providers should be present
	if resp.Providers != nil {
		assert.NotEmpty(t, resp.Providers.LLM.Name)
		assert.NotEmpty(t, resp.Providers.LLM.Model)
		assert.NotEmpty(t, resp.Providers.Embedding.Name)
		assert.NotEmpty(t, resp.Providers.Embedding.Model)
		assert.Greater(t, resp.Providers.Embedding.Dimension, 0)
	}

	// Schema should be present
	if resp.Schema != nil {
		assert.Greater(t, resp.Schema.MigrationsApplied, 0)
	}
}

func TestReady(t *testing.T) {
	svc := health.NewService(baseURL(t), nil)
	err := svc.Ready(context.Background())
	require.NoError(t, err)
}

func TestLive(t *testing.T) {
	svc := health.NewService(baseURL(t), nil)
	err := svc.Live(context.Background())
	require.NoError(t, err)
}
