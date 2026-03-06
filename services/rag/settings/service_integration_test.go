//go:build integration

package settings_test

import (
	"context"
	"os"
	"testing"

	"school-with-ai/services/rag/settings"

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

func TestGetStatus_Returns404(t *testing.T) {
	svc := settings.NewService(baseURL(t), nil)
	_, err := svc.GetStatus(context.Background())
	require.Error(t, err, "status endpoint should return an error (404)")
	assert.Contains(t, err.Error(), "not found")
}

func TestGetProviders(t *testing.T) {
	svc := settings.NewService(baseURL(t), nil)
	resp, err := svc.GetProviders(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, len(resp.LLMProviders) > 0 || len(resp.EmbeddingProviders) > 0,
		"should have at least one LLM or embedding provider")

	for _, p := range resp.LLMProviders {
		assert.NotEmpty(t, p.Name, "LLM provider should have a name")
		assert.NotEmpty(t, p.ID, "LLM provider should have an ID")
	}
	for _, p := range resp.EmbeddingProviders {
		assert.NotEmpty(t, p.Name, "embedding provider should have a name")
		assert.NotEmpty(t, p.ID, "embedding provider should have an ID")
	}
}
