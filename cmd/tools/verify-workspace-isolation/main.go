package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"school-with-ai/foundation/lightrag"
)

const (
	baseURL = "http://localhost:9625"
	apiKey  = "test_key"
)

func main() {
	log.Println("Starting Workspace Isolation Verification Script...")

	// Initialize Clients
	clientA := lightrag.NewClient(baseURL, apiKey, nil)
	clientA.SetWorkspace("workspace_script_a")

	clientB := lightrag.NewClient(baseURL, apiKey, nil)
	clientB.SetWorkspace("workspace_script_b")

	// Test files are expected to be in cmd/tools/verify-workspace-isolation/test_data/
	docA := "cmd/tools/verify-workspace-isolation/test_data/doc_a.txt"
	docB := "cmd/tools/verify-workspace-isolation/test_data/doc_b.txt"

	// 1. Upload to Workspace A
	log.Printf("Uploading %s to Workspace A...", docA)
	fileA, err := os.Open(docA)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", docA, err)
	}
	defer fileA.Close()

	respA, err := clientA.UploadFile(context.Background(), "doc_a.txt", fileA)
	if err != nil {
		log.Fatalf("Failed to upload to Workspace A: %v", err)
	}
	log.Printf("Upload A Success: %s (TrackID: %s)", respA.Status, respA.TrackID)

	// 2. Upload to Workspace B
	log.Printf("Uploading %s to Workspace B...", docB)
	fileB, err := os.Open(docB)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", docB, err)
	}
	defer fileB.Close()

	respB, err := clientB.UploadFile(context.Background(), "doc_b.txt", fileB)
	if err != nil {
		log.Fatalf("Failed to upload to Workspace B: %v", err)
	}
	log.Printf("Upload B Success: %s (TrackID: %s)", respB.Status, respB.TrackID)

	// Wait for indexing
	log.Println("Waiting 30 seconds for indexing...")
	time.Sleep(30 * time.Second)

	// 3. Verify Workspace A finds content A
	log.Println("Verifying Workspace A access to its own data...")
	queryReqA := lightrag.QueryRequest{Query: "What is the code for Script Workspace A?", Mode: "naive"}
	respQA, err := clientA.Query(context.Background(), queryReqA)
	if err != nil {
		log.Fatalf("Query A failed: %v", err)
	}
	if !strings.Contains(respQA.Response, "SECRET-SCRIPT-A") {
		log.Fatalf("FAIL: Workspace A did not retrieve its own data. Response: %s", respQA.Response)
	}
	log.Println("PASS: Workspace A retrieved its data.")

	// 4. Verify Workspace A DOES NOT find content B
	log.Println("Verifying Workspace A isolation from Workspace B...")
	queryReqA_B := lightrag.QueryRequest{Query: "What is the repo for Script Workspace B?", Mode: "naive"}
	respQA_B, err := clientA.Query(context.Background(), queryReqA_B)
	if err != nil {
		log.Fatalf("Query A (for B) failed: %v", err)
	}
	if strings.Contains(respQA_B.Response, "open-source-script") {
		log.Fatalf("FAIL: Workspace A retrieved Workspace B data! Response: %s", respQA_B.Response)
	}
	log.Println("PASS: Workspace A did not see Workspace B data.")

	// 5. Verify Workspace B finds content B
	log.Println("Verifying Workspace B access to its own data...")
	queryReqB := lightrag.QueryRequest{Query: "What is the repo for Script Workspace B?", Mode: "naive"}
	respQB, err := clientB.Query(context.Background(), queryReqB)
	if err != nil {
		log.Fatalf("Query B failed: %v", err)
	}
	if !strings.Contains(respQB.Response, "open-source-script") {
		log.Fatalf("FAIL: Workspace B did not retrieve its own data. Response: %s", respQB.Response)
	}
	log.Println("PASS: Workspace B retrieved its data.")

	// 6. Verify Workspace B DOES NOT find content A
	log.Println("Verifying Workspace B isolation from Workspace A...")
	queryReqB_A := lightrag.QueryRequest{Query: "What is the code for Script Workspace A?", Mode: "naive"}
	respQB_A, err := clientB.Query(context.Background(), queryReqB_A)
	if err != nil {
		log.Fatalf("Query B (for A) failed: %v", err)
	}
	if strings.Contains(respQB_A.Response, "SECRET-SCRIPT-A") {
		log.Fatalf("FAIL: Workspace B retrieved Workspace A data! Response: %s", respQB_A.Response)
	}
	log.Println("PASS: Workspace B did not see Workspace A data.")

	log.Println("✅ ALL TESTS PASSED: Workspace Isolation Verified Successfully!")
}
