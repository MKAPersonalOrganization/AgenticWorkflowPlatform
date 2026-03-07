//go:build integration
// +build integration

package shared_test

import (
	"context"
	"os"
	"testing"

	"school-with-ai/modules/knowledge/rag/shared"

	"github.com/stretchr/testify/assert"
)

func TestGetSharedConversation_NotFound(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	svc := shared.NewService(baseURL, nil)
	ctx := context.Background()

	_, err := svc.GetSharedConversation(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	t.Logf("got expected error: %v", err)
}
