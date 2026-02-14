//go:build integration
// +build integration

package conversations

import (
	"context"
	"os"
	"school-with-ai/domain"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestContext() *domain.TenantContext {
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	return &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000001").String(),
		Role:        "user",
	}
}

func TestService_Integration_ConversationLifecycle(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getTestContext()

	// 1. Create a conversation
	title := "Integration Test Conv " + uuid.New().String()[:8]
	mode := "local"
	createReq := &CreateConversationRequest{
		Title: &title,
		Mode:  &mode,
	}
	conv, err := service.Create(ctx, tCtx, createReq)
	require.NoError(t, err)
	require.NotNil(t, conv)
	assert.Equal(t, title, conv.Title)
	assert.Equal(t, mode, conv.Mode)
	assert.False(t, conv.IsPinned)
	assert.False(t, conv.IsArchived)
	convID := conv.ID

	// 2. List conversations and verify presence
	listResp, err := service.List(ctx, tCtx, nil)
	require.NoError(t, err)
	require.NotNil(t, listResp)
	found := false
	for _, item := range listResp.Items {
		if item.ID == convID {
			found = true
			break
		}
	}
	assert.True(t, found, "Conversation should be present in list")

	// 3. Get conversation by ID
	getResp, err := service.Get(ctx, tCtx, convID)
	require.NoError(t, err)
	require.NotNil(t, getResp)
	assert.Equal(t, convID, getResp.Conversation.ID)
	assert.Equal(t, title, getResp.Conversation.Title)

	// 4. Update the conversation
	updatedTitle := "Updated Conv " + uuid.New().String()[:8]
	pinned := true
	updateReq := &UpdateConversationRequest{
		Title:    &updatedTitle,
		IsPinned: &pinned,
	}
	updateResp, err := service.Update(ctx, tCtx, convID, updateReq)
	require.NoError(t, err)
	require.NotNil(t, updateResp)
	assert.Equal(t, updatedTitle, updateResp.Title)
	assert.True(t, updateResp.IsPinned)

	// 5. Delete the conversation
	err = service.Delete(ctx, tCtx, convID)
	require.NoError(t, err)

	// 6. Verify deletion
	_, err = service.Get(ctx, tCtx, convID)
	assert.Error(t, err)
}

func TestService_Integration_ListConversationsEmpty(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	tenantID := uuid.New()
	workspaceID := uuid.New()
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user-empty",
		Role:        "user",
	}

	resp, err := service.List(ctx, tCtx, nil)
	if err == nil {
		assert.NotNil(t, resp)
		assert.Empty(t, resp.Items)
	}
}

func TestService_Integration_ListConversationsWithPagination(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getTestContext()

	limit := 5
	params := &ListConversationsParams{
		Limit: &limit,
	}

	resp, err := service.List(ctx, tCtx, params)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.LessOrEqual(t, len(resp.Items), limit)
}

func TestService_Integration_ConversationMessages(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getTestContext()

	// Create a conversation
	title := "Message Test Conv " + uuid.New().String()[:8]
	conv, err := service.Create(ctx, tCtx, &CreateConversationRequest{Title: &title})
	require.NoError(t, err)
	require.NotNil(t, conv)

	// Create a user message (non-streaming)
	msgReq := &CreateMessageRequest{
		Content: "Hello, this is a test message",
		Role:    "user",
		Stream:  false,
	}
	msg, err := service.CreateMessage(ctx, tCtx, conv.ID, msgReq)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "Hello, this is a test message", msg.Content)
	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, conv.ID, msg.ConversationID)

	// List messages
	msgsResp, err := service.ListMessages(ctx, tCtx, conv.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, msgsResp)
	assert.GreaterOrEqual(t, len(msgsResp.Items), 1)

	foundMsg := false
	for _, m := range msgsResp.Items {
		if m.ID == msg.ID {
			foundMsg = true
			assert.Equal(t, "user", m.Role)
			break
		}
	}
	assert.True(t, foundMsg, "Message should be in message list")

	// Cleanup
	service.Delete(ctx, tCtx, conv.ID)
}

func TestService_Integration_UpdateMessage(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getTestContext()

	title := "Update Msg Test " + uuid.New().String()[:8]
	conv, err := service.Create(ctx, tCtx, &CreateConversationRequest{Title: &title})
	require.NoError(t, err)

	msg, err := service.CreateMessage(ctx, tCtx, conv.ID, &CreateMessageRequest{
		Content: "Original content",
		Role:    "user",
		Stream:  false,
	})
	require.NoError(t, err)

	updatedContent := "Updated content"
	tokensUsed := int32(42)
	updateResp, err := service.UpdateMessage(ctx, tCtx, msg.ID, &UpdateMessageRequest{
		Content:    &updatedContent,
		TokensUsed: &tokensUsed,
	})
	require.NoError(t, err)
	require.NotNil(t, updateResp)
	assert.Equal(t, updatedContent, updateResp.Content)

	// Cleanup
	service.Delete(ctx, tCtx, conv.ID)
}

func TestService_Integration_DeleteMessage(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getTestContext()

	title := "Delete Msg Test " + uuid.New().String()[:8]
	conv, err := service.Create(ctx, tCtx, &CreateConversationRequest{Title: &title})
	require.NoError(t, err)

	msg, err := service.CreateMessage(ctx, tCtx, conv.ID, &CreateMessageRequest{
		Content: "Message to delete",
		Role:    "user",
		Stream:  false,
	})
	require.NoError(t, err)

	err = service.DeleteMessage(ctx, tCtx, msg.ID)
	require.NoError(t, err)

	msgsResp, err := service.ListMessages(ctx, tCtx, conv.ID, nil)
	require.NoError(t, err)
	for _, m := range msgsResp.Items {
		assert.NotEqual(t, msg.ID, m.ID, "Deleted message should not appear in list")
	}

	// Cleanup
	service.Delete(ctx, tCtx, conv.ID)
}

func TestService_Integration_GetNonExistentConversation(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getTestContext()

	_, err := service.Get(ctx, tCtx, uuid.New())
	assert.Error(t, err)
}

func TestService_Integration_DeleteNonExistentConversation(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getTestContext()

	err := service.Delete(ctx, tCtx, uuid.New())
	assert.Error(t, err)
}

func TestService_Integration_BulkDelete(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getTestContext()

	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		title := "Bulk Delete " + uuid.New().String()[:8]
		conv, err := service.Create(ctx, tCtx, &CreateConversationRequest{Title: &title})
		require.NoError(t, err)
		ids = append(ids, conv.ID)
	}

	bulkResp, err := service.BulkDelete(ctx, tCtx, &BulkOperationRequest{
		ConversationIDs: ids,
	})
	require.NoError(t, err)
	require.NotNil(t, bulkResp)
	assert.Equal(t, 3, bulkResp.Affected)
}

func TestService_Integration_BulkArchive(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getTestContext()

	var ids []uuid.UUID
	for i := 0; i < 2; i++ {
		title := "Bulk Archive " + uuid.New().String()[:8]
		conv, err := service.Create(ctx, tCtx, &CreateConversationRequest{Title: &title})
		require.NoError(t, err)
		ids = append(ids, conv.ID)
	}

	archiveResp, err := service.BulkArchive(ctx, tCtx, &BulkArchiveRequest{
		ConversationIDs: ids,
		Archive:         true,
	})
	require.NoError(t, err)
	require.NotNil(t, archiveResp)
	assert.Equal(t, 2, archiveResp.Affected)

	for _, id := range ids {
		getResp, err := service.Get(ctx, tCtx, id)
		require.NoError(t, err)
		assert.True(t, getResp.Conversation.IsArchived)
	}

	// Cleanup
	service.BulkDelete(ctx, tCtx, &BulkOperationRequest{ConversationIDs: ids})
}

func TestService_Integration_ConversationTimestamps(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getTestContext()

	title := "Timestamp Conv " + uuid.New().String()[:8]
	conv, err := service.Create(ctx, tCtx, &CreateConversationRequest{Title: &title})
	require.NoError(t, err)

	assert.NotEmpty(t, conv.CreatedAt)
	assert.NotEmpty(t, conv.UpdatedAt)

	// Cleanup
	service.Delete(ctx, tCtx, conv.ID)
}
