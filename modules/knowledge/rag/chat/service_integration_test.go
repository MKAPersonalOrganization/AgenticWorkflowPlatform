//go:build integration
// +build integration

package chat_test

import (
	"context"
	"os"
	"testing"

	"aicompass/modules/knowledge/tenant"
	"aicompass/modules/knowledge/rag/chat"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func testTenantContext() *tenant.TenantContext {
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	return &tenant.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "00000000-0000-0000-0000-000000000001",
		Role:        "admin",
	}
}

func TestComplete_NoLLM(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	svc := chat.NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testTenantContext()

	_, err := svc.Complete(ctx, tCtx, chat.ChatCompletionRequest{
		Message: "test",
		Stream:  false,
	})
	assert.Error(t, err, "expected error because no LLM backend is available")
	t.Logf("got expected error: %v", err)
}
