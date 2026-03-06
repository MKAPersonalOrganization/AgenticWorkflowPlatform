//go:build integration
// +build integration

package models

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Integration_ListModels(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	resp, err := service.List(ctx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Providers)
	assert.NotEmpty(t, resp.DefaultLLMProvider)
	assert.NotEmpty(t, resp.DefaultLLMModel)
	assert.NotEmpty(t, resp.DefaultEmbeddingProvider)
	assert.NotEmpty(t, resp.DefaultEmbeddingModel)
}

func TestService_Integration_ListLLMModels(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	resp, err := service.ListLLM(ctx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Models)
	assert.NotEmpty(t, resp.DefaultProvider)
	assert.NotEmpty(t, resp.DefaultModel)
}

func TestService_Integration_ListEmbeddingModels(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	resp, err := service.ListEmbedding(ctx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Models)
	assert.NotEmpty(t, resp.DefaultProvider)
	assert.NotEmpty(t, resp.DefaultModel)
}

func TestService_Integration_Health(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	resp, err := service.Health(ctx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	// Should have at least one provider health entry
	assert.NotEmpty(t, resp)
}

func TestService_Integration_GetProvider_NotFound(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	_, err := service.GetProvider(ctx, "nonexistent-provider")
	assert.Error(t, err)
}

func TestService_Integration_GetModel_NotFound(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	_, err := service.GetModel(ctx, "openai", "nonexistent-model")
	assert.Error(t, err)
}
