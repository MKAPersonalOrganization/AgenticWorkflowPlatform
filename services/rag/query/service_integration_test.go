//go:build integration

package query

import (
	"context"
	"fmt"
	"os"
	"school-with-ai/domain"
	"testing"
	"time"

	"github.com/google/uuid"
)

// To run these tests:
// EDGEQUAKE_BASE_URL=http://localhost:8081 go test -v -tags=integration ./services/rag/query/...

func TestService_Integration_Execute(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getIntegrationContext()

	req := &QueryRequest{
		Query:       "What are the documents in this workspace?",
		Mode:        stringPtr("naive"),
		LLMProvider: stringPtr("openai"),
		LLMModel:    stringPtr("gpt-4.1-nano"),
	}

	resp, err := service.Execute(ctx, tCtx, req)
	if err != nil {
		t.Fatalf("Query execution failed: %v", err)
	}

	fmt.Printf("Query Response: %s\n", resp.Answer)
	fmt.Printf("Sources Found: %d\n", len(resp.Sources))
}

func TestService_Integration_Stream(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tCtx := getIntegrationContext()

	req := &StreamQueryRequest{
		Query: "Tell me a short story about AI.",
		Mode:  stringPtr("naive"),
	}

	textChan, errChan := service.Stream(ctx, tCtx, req)

	fmt.Print("Streaming Response: ")
	receivedAny := false
	for text := range textChan {
		fmt.Print(text)
		receivedAny = true
	}
	fmt.Println()

	if err := <-errChan; err != nil {
		if ctx.Err() != nil {
			t.Skip("Skipping: stream endpoint timed out (LLM provider may be unavailable)")
		}
		t.Fatalf("Streaming failed: %v", err)
	}

	if !receivedAny && ctx.Err() != nil {
		t.Skip("Skipping: stream endpoint timed out without sending data")
	}

	if !receivedAny {
		t.Errorf("Did not receive any tokens from stream")
	}
}

func getIntegrationContext() *domain.TenantContext {
	tenantIDStr := "00000000-0000-0000-0000-000000000002"
	workspaceIDStr := "00000000-0000-0000-0000-000000000003"
	userID := "00000000-0000-0000-0000-000000000001"
	role := "admin"

	id1, _ := uuid.Parse(tenantIDStr)
	id2, _ := uuid.Parse(workspaceIDStr)

	return &domain.TenantContext{
		TenantID:    &id1,
		WorkspaceID: &id2,
		UserID:      userID,
		Role:        role,
	}
}

func stringPtr(s string) *string {
	return &s
}
