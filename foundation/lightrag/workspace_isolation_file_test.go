package lightrag_test

import (
	"context"
	"os"
	"testing"
	"time"

	"school-with-ai/foundation/lightrag"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceIsolationWithFiles(t *testing.T) {
	baseURL := "http://localhost:9625"
	apiKey := "test_key"

	// Initialize Clients for two different workspaces
	clientA := lightrag.NewClient(baseURL, apiKey, nil)
	clientA.SetWorkspace("workspace_file_a")

	clientB := lightrag.NewClient(baseURL, apiKey, nil)
	clientB.SetWorkspace("workspace_file_b")

	// Ensure clean state (optional, but good for repeatability)
	// In a real scenario, we might want to clear documents first, but for now we rely on unique workspaces or overwrite
	// For this test, we'll just proceed with upload.

	// 1. Upload doc_a.txt to Workspace A
	t.Log("Uploading doc_a.txt to Workspace A...")
	fileA, err := os.Open("test_data/doc_a.txt")
	require.NoError(t, err)
	defer fileA.Close()

	respA, err := clientA.UploadFile(context.Background(), fileA.Name(), fileA)
	require.NoError(t, err, "Failed to upload file to Workspace A")
	assert.Contains(t, []string{"success", "duplicated"}, respA.Status)
	t.Logf("Upload A Response: %+v", respA)

	// 2. Upload doc_b.txt to Workspace B
	t.Log("Uploading doc_b.txt to Workspace B...")
	fileB, err := os.Open("test_data/doc_b.txt")
	require.NoError(t, err)
	defer fileB.Close()

	respB, err := clientB.UploadFile(context.Background(), fileB.Name(), fileB)
	require.NoError(t, err, "Failed to upload file to Workspace B")
	assert.Contains(t, []string{"success", "duplicated"}, respB.Status)
	t.Logf("Upload B Response: %+v", respB)

	// Wait for indexing
	t.Log("Waiting for indexing...")
	time.Sleep(30 * time.Second)

	// 3. Query Workspace A for content in doc_a -> Should Find
	t.Log("Querying Workspace A for 'Project Alpha'...")
	queryReqA := lightrag.QueryRequest{
		Query: "What is the launch code for Project Alpha?",
		Mode:  "naive", // Using naive mode for direct retrieval verification
	}
	queryRespA, err := clientA.Query(context.Background(), queryReqA)
	require.NoError(t, err, "Failed to query Workspace A")
	t.Logf("Query A Result: %s", queryRespA.Response)

	assert.Contains(t, queryRespA.Response, "1234-ALPHA", "Workspace A should know about Project Alpha")

	// 4. Query Workspace A for content in doc_b -> Should NOT Find
	t.Log("Querying Workspace A for 'Project Beta'...")
	queryReqA_Beta := lightrag.QueryRequest{
		Query: "What is the repository for Project Beta?",
		Mode:  "naive",
	}
	queryRespA_Beta, err := clientA.Query(context.Background(), queryReqA_Beta)
	require.NoError(t, err, "Failed to query Workspace A for Beta")
	t.Logf("Query A (Beta) Result: %s", queryRespA_Beta.Response)

	assert.NotContains(t, queryRespA_Beta.Response, "github.com/beta", "Workspace A should NOT know about Project Beta")

	// 5. Query Workspace B for content in doc_b -> Should Find
	t.Log("Querying Workspace B for 'Project Beta'...")
	queryReqB := lightrag.QueryRequest{
		Query: "What is the repository for Project Beta?",
		Mode:  "naive",
	}
	queryRespB, err := clientB.Query(context.Background(), queryReqB)
	require.NoError(t, err, "Failed to query Workspace B")
	t.Logf("Query B Result: %s", queryRespB.Response)

	assert.Contains(t, queryRespB.Response, "github.com/beta", "Workspace B should know about Project Beta")

	// 6. Query Workspace B for content in doc_a -> Should NOT Find
	t.Log("Querying Workspace B for 'Project Alpha'...")
	queryReqB_Alpha := lightrag.QueryRequest{
		Query: "What is the launch code for Project Alpha?",
		Mode:  "naive",
	}
	queryRespB_Alpha, err := clientB.Query(context.Background(), queryReqB_Alpha)
	require.NoError(t, err, "Failed to query Workspace B for Alpha")
	t.Logf("Query B (Alpha) Result: %s", queryRespB_Alpha.Response)

	assert.NotContains(t, queryRespB_Alpha.Response, "1234-ALPHA", "Workspace B should NOT know about Project Alpha")
}
