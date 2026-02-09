package lightrag_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"school-with-ai/foundation/lightrag"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	baseURL = "http://localhost:9625"
	apiKey  = "test_key"
)

func TestWorkspaceIsolation(t *testing.T) {
	// Initialize clients for Workspace A and Workspace B
	clientA := lightrag.NewClient(baseURL, apiKey, nil)
	clientA.SetWorkspace("workspace_a")

	clientB := lightrag.NewClient(baseURL, apiKey, nil)
	clientB.SetWorkspace("workspace_b")

	ctx := context.Background()

	// 1. Insert document into Workspace A
	t.Log("Inserting document into Workspace A...")
	docContent := "This is a secret document for Workspace A. Workspace B should not see this."
	uploadResp, err := clientA.UploadFile(ctx, "secret_doc.txt", strings.NewReader(docContent))
	require.NoError(t, err, "Failed to upload file to Workspace A")
	assert.Contains(t, []string{"success", "duplicated"}, uploadResp.Status, "Upload status should be success or duplicated")

	// Wait for indexing
	t.Log("Waiting for indexing...")
	time.Sleep(30 * time.Second)

	// 2. Query Workspace A (should find it)
	t.Log("Querying Workspace A...")
	queryReqA := lightrag.QueryRequest{
		Query:           "What is the content of the secret document?",
		Mode:            "naive",
		OnlyNeedContext: true,
	}
	respA, err := clientA.Query(ctx, queryReqA)
	require.NoError(t, err, "Failed to query Workspace A")

	// Verify response contains the secret content
	t.Logf("Query A Result: %v", respA.Response)
	assert.Contains(t, respA.Response, "secret document", "Workspace A should find the document")

	// 3. Query Workspace B (should NOT find it)
	t.Log("Querying Workspace B...")
	queryReqB := lightrag.QueryRequest{
		Query:           "What is the content of the secret document?",
		Mode:            "naive",
		OnlyNeedContext: true,
	}
	respB, err := clientB.Query(ctx, queryReqB)
	require.NoError(t, err, "Failed to query Workspace B")

	// Verify response does NOT contain the secret content
	t.Logf("Query B Result: %v", respB.Response)
	assert.NotContains(t, respB.Response, "Workspace A", "Workspace B should NOT find the document")
	assert.NotContains(t, respB.Response, "secret document", "Workspace B should NOT find the document")
}
