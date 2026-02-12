package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"school-with-ai/foundation/lightrag"
	"school-with-ai/foundation/logger"

	"github.com/google/uuid"
)

func main() {
	var (
		baseURL        = flag.String("url", "", "LightRAG API base URL")
		apiKey         = flag.String("api-key", "", "LightRAG API key")
		mode           = flag.String("mode", lightrag.ModeHybrid, "Query mode (local, global, hybrid, naive, mix, bypass)")
		stream         = flag.Bool("stream", false, "Enable streaming response")
		verbose        = flag.Bool("v", false, "Enable verbose logging")
		list           = flag.Bool("list", false, "List documents")
		clear          = flag.Bool("clear", false, "Clear all documents")
		scan           = flag.Bool("scan", false, "Scan for new documents")
		upload         = flag.String("upload", "", "Upload a document file")
		reprocess      = flag.Bool("reprocess", false, "Reprocess failed documents")
		cancel         = flag.Bool("cancel", false, "Cancel currently running pipeline")
		stats          = flag.Bool("stats", false, "Show document status counts")
		clearCache     = flag.Bool("clear-cache", false, "Clear LLM response cache")
		track          = flag.String("track", "", "Track status of a specific ID (or 'pipeline' for overall status)")
		status         = flag.String("status", "", "Filter by status (PENDING, PROCESSING, PREPROCESSED, PROCESSED, FAILED)")
		page           = flag.Int("page", 1, "Page number for listing")
		pageSize       = flag.Int("page-size", 50, "Page size for listing")
		deleteDoc      = flag.String("delete-doc", "", "Delete document(s) by ID (comma-separated)")
		deleteFile     = flag.Bool("delete-file", false, "Delete original file when deleting document")
		deleteLLMCache = flag.Bool("delete-llm-cache", false, "Delete LLM cache when deleting document")
	)

	flag.Parse()

	if !*list && !*clear && !*scan && *track == "" && *upload == "" && !*reprocess && !*cancel && !*stats && !*clearCache && *deleteDoc == "" && flag.NArg() < 1 {
		fmt.Println("Usage: lightrag-query [flags] <query>")
		fmt.Println("\nFlags:")
		flag.PrintDefaults()
		fmt.Println("\nExample:")
		fmt.Println("  lightrag-query -stream \"What is biology?\"")
		fmt.Println("  lightrag-query -list -status PROCESSED")
		fmt.Println("  lightrag-query -clear")
		fmt.Println("  lightrag-query -scan")
		fmt.Println("  lightrag-query -reprocess")
		fmt.Println("  lightrag-query -cancel")
		fmt.Println("  lightrag-query -stats")
		fmt.Println("  lightrag-query -clear-cache")
		fmt.Println("  lightrag-query -delete-doc doc_id1,doc_id2 [-delete-file] [-delete-llm-cache]")
		fmt.Println("  lightrag-query -upload document.pdf")
		fmt.Println("  lightrag-query -track pipeline")
		fmt.Println("  lightrag-query -track scan_20240101_000000_xxxx")
		os.Exit(1)
	}

	query := strings.Join(flag.Args(), " ")

	// Initialize logger (only active if verbose)
	logLevel := logger.LevelError
	if *verbose {
		logLevel = logger.LevelInfo
	}
	l := logger.New(os.Stdout, logLevel, "LIGHTRAG-QUERY", func(ctx context.Context) string {
		return uuid.NewString()
	})

	ctx := context.Background()

	if *baseURL == "" {
		*baseURL = os.Getenv("LIGHTRAG_URL")
		if *baseURL == "" {
			*baseURL = "http://localhost:9622"
		}
	}
	if *apiKey == "" {
		*apiKey = os.Getenv("LIGHTRAG_API_KEY")
	}

	client := lightrag.NewClient(*baseURL, *apiKey, nil)

	if *list {
		handleList(ctx, client, *status, *page, *pageSize)
		return
	}

	if *clear {
		handleClear(ctx, client)
		return
	}

	if *scan {
		handleScan(ctx, client)
		return
	}

	if *track != "" {
		handleTrack(ctx, client, *track)
		return
	}

	if *upload != "" {
		handleUpload(ctx, client, *upload)
		return
	}

	if *reprocess {
		handleReprocess(ctx, client)
		return
	}

	if *cancel {
		handleCancel(ctx, client)
		return
	}

	if *stats {
		handleStats(ctx, client)
		return
	}

	if *clearCache {
		handleClearCache(ctx, client)
		return
	}

	if *deleteDoc != "" {
		handleDeleteDoc(ctx, client, *deleteDoc, *deleteFile, *deleteLLMCache)
		return
	}

	if *verbose {
		l.Info(ctx, "sending query to lightrag", "url", *baseURL, "query", query, "mode", *mode, "stream", *stream)
	}

	req := lightrag.QueryRequest{
		Query:             query,
		Mode:              *mode,
		IncludeReferences: true,
	}

	start := time.Now()

	if *stream {
		ch, err := client.QueryStream(ctx, req)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n--- LightRAG Response (%s mode) ---\n\n", *mode)
		for chunk := range ch {
			if chunk.Error != "" {
				fmt.Printf("\n[Stream Error]: %s\n", chunk.Error)
				break
			}
			fmt.Print(chunk.Response)
		}
		fmt.Println("\n\n---------------------------------")
	} else {
		resp, err := client.Query(ctx, req)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n--- LightRAG Response (%s mode) ---\n\n", *mode)
		fmt.Println(resp.Response)
		fmt.Println("\n---------------------------------")
	}

	if *verbose {
		l.Info(ctx, "query completed", "duration", time.Since(start))
	}
}

func handleList(ctx context.Context, client *lightrag.Client, statusFilter string, page, pageSize int) {
	fmt.Printf("\n--- LightRAG Documents (Page %d, Size %d) ---\n", page, pageSize)

	req := lightrag.DocumentsRequest{
		Page:     page,
		PageSize: pageSize,
	}
	if statusFilter != "" {
		sf := lightrag.DocStatus(statusFilter)
		req.StatusFilter = &sf
	}

	resp, err := client.GetDocumentsPaginated(ctx, req)
	if err != nil {
		fmt.Printf("Error listing documents: %v\n", err)
		os.Exit(1)
	}

	if len(resp.Documents) == 0 {
		fmt.Println("No documents found.")
	} else {
		for _, doc := range resp.Documents {
			fmt.Printf("[%s] %s (ID: %s, Chunks: %d)\n", doc.Status, doc.FilePath, doc.ID, zeroIfNil(doc.ChunksCount))
		}
	}

	fmt.Printf("\nSummary: Total: %d, Pages: %d, HasNext: %v\n",
		resp.Pagination.TotalCount, resp.Pagination.TotalPages, resp.Pagination.HasNext)
	fmt.Println("Counts by status:")
	for s, count := range resp.StatusCounts {
		fmt.Printf("  %s: %d\n", s, count)
	}
	fmt.Println("------------------------------------------")
}

func handleClear(ctx context.Context, client *lightrag.Client) {
	fmt.Println("\nClearing all documents...")
	resp, err := client.ClearDocuments(ctx)
	if err != nil {
		fmt.Printf("Error clearing documents: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Message: %s\n", resp.Message)
}

func handleScan(ctx context.Context, client *lightrag.Client) {
	fmt.Println("\nScanning for new documents...")
	resp, err := client.ScanDocuments(ctx)
	if err != nil {
		fmt.Printf("Error scanning documents: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Message: %s\n", resp.Message)
	fmt.Printf("Track ID: %s\n", resp.TrackID)
}

func handleClearCache(ctx context.Context, client *lightrag.Client) {
	fmt.Println("\nClearing LLM response cache...")
	resp, err := client.ClearCache(ctx)
	if err != nil {
		fmt.Printf("Error clearing cache: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Message: %s\n", resp.Message)
}

func handleDeleteDoc(ctx context.Context, client *lightrag.Client, docIDsStr string, deleteFile, deleteLLMCache bool) {
	docIDs := strings.Split(docIDsStr, ",")
	for i := range docIDs {
		docIDs[i] = strings.TrimSpace(docIDs[i])
	}

	fmt.Printf("\nDeleting %d document(s)...\n", len(docIDs))
	req := lightrag.DeleteDocRequest{
		DocIDs:         docIDs,
		DeleteFile:     deleteFile,
		DeleteLLMCache: deleteLLMCache,
	}

	resp, err := client.DeleteDocument(ctx, req)
	if err != nil {
		fmt.Printf("Error deleting document(s): %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Message: %s\n", resp.Message)
	fmt.Printf("Doc IDs: %s\n", resp.DocID)
}

func handleStats(ctx context.Context, client *lightrag.Client) {
	fmt.Println("\nRetrieving document status counts...")
	resp, err := client.GetStatusCounts(ctx)
	if err != nil {
		fmt.Printf("Error retrieving status counts: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Counts by status:")
	for s, count := range resp.StatusCounts {
		fmt.Printf("  %s: %d\n", s, count)
	}
}

func handleReprocess(ctx context.Context, client *lightrag.Client) {
	fmt.Println("\nInitiating reprocessing of failed documents...")
	resp, err := client.ReprocessFailedDocuments(ctx)
	if err != nil {
		fmt.Printf("Error reprocessing documents: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Message: %s\n", resp.Message)
}

func handleCancel(ctx context.Context, client *lightrag.Client) {
	fmt.Println("\nRequesting pipeline cancellation...")
	resp, err := client.CancelPipeline(ctx)
	if err != nil {
		fmt.Printf("Error cancelling pipeline: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Message: %s\n", resp.Message)
}

func handleUpload(ctx context.Context, client *lightrag.Client, filePath string) {
	fmt.Printf("\nUploading document: %s...\n", filePath)
	f, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	resp, err := client.UploadFile(ctx, filepath.Base(filePath), f)
	if err != nil {
		fmt.Printf("Error uploading document: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Message: %s\n", resp.Message)
	fmt.Printf("Track ID: %s\n", resp.TrackID)
}

func handleTrack(ctx context.Context, client *lightrag.Client, trackID string) {
	if trackID == "pipeline" {
		fmt.Println("\nRetrieving pipeline status...")
		resp, err := client.GetPipelineStatus(ctx)
		if err != nil {
			fmt.Printf("Error retrieving pipeline status: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Busy: %v\n", resp.Busy)
		if resp.JobName != "" {
			fmt.Printf("Job Name: %s\n", resp.JobName)
			fmt.Printf("Job Start: %s\n", resp.JobStart)
			fmt.Printf("Progress: %d/%d (Batch %d)\n", resp.Docs, resp.Batchs, resp.CurBatch)
		}
		fmt.Printf("Latest Message: %s\n", resp.LatestMessage)
		if len(resp.HistoryMessages) > 0 {
			fmt.Println("\nRecent History:")
			// Print latest 5 messages
			start := len(resp.HistoryMessages) - 5
			if start < 0 {
				start = 0
			}
			for i := start; i < len(resp.HistoryMessages); i++ {
				fmt.Printf("  - %s\n", resp.HistoryMessages[i])
			}
		}
		return
	}

	fmt.Printf("\nTracking status for ID: %s...\n", trackID)
	resp, err := client.GetTrackStatus(ctx, trackID)
	if err != nil {
		fmt.Printf("Error tracking status: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total Documents: %d\n", resp.TotalCount)
	fmt.Println("\nStatus Summary:")
	for s, count := range resp.StatusSummary {
		fmt.Printf("  %s: %d\n", s, count)
	}

	if len(resp.Documents) > 0 {
		fmt.Println("\nDocuments:")
		for _, doc := range resp.Documents {
			fmt.Printf("  - [%s] %s (%s)\n", doc.Status, doc.ID, doc.FilePath)
			if doc.ErrorMsg != nil && *doc.ErrorMsg != "" {
				fmt.Printf("    Error: %s\n", *doc.ErrorMsg)
			}
		}
	}
}

func zeroIfNil(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
