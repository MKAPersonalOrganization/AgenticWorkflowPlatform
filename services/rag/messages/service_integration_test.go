//go:build integration
// +build integration

package messages_test

import (
"context"
"os"
"testing"

"school-with-ai/domain"
"school-with-ai/services/rag/messages"

"github.com/google/uuid"
"github.com/stretchr/testify/assert"
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

func TestUpdateMessage_NotFound(t *testing.T) {
baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
if baseURL == "" {
t.Skip("EDGEQUAKE_BASE_URL not set")
}

svc := messages.NewService(baseURL, nil)
ctx := context.Background()
tCtx := testTenantContext()

_, err := svc.UpdateMessage(ctx, tCtx, "00000000-0000-0000-0000-000000000099", messages.UpdateMessageRequest{})
assert.Error(t, err, "expected error for nonexistent message")
t.Logf("got expected error: %v", err)
}

func TestDeleteMessage_NotFound(t *testing.T) {
baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
if baseURL == "" {
t.Skip("EDGEQUAKE_BASE_URL not set")
}

svc := messages.NewService(baseURL, nil)
ctx := context.Background()
tCtx := testTenantContext()

err := svc.DeleteMessage(ctx, tCtx, "00000000-0000-0000-0000-000000000099")
assert.Error(t, err, "expected error for nonexistent message")
t.Logf("got expected error: %v", err)
}
