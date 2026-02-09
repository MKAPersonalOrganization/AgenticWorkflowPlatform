package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"school-with-ai/foundation/lightrag"
	"school-with-ai/foundation/logger"

	"github.com/google/uuid"
)

func main() {
	var (
		baseURL  = flag.String("url", "", "LightRAG API base URL")
		apiKey   = flag.String("api-key", "", "LightRAG API key")
		mode     = flag.String("mode", lightrag.ModeHybrid, "Query mode (local, global, hybrid, naive, mix, bypass)")
		stream   = flag.Bool("stream", false, "Enable streaming response")
		verbose  = flag.Bool("v", false, "Enable verbose logging")
		list     = flag.Bool("list", false, "List documents")
		clear    = flag.Bool("clear", false, "Clear all documents")
		status   = flag.String("status", "", "Filter by status (PENDING, PROCESSING, PREPROCESSED, PROCESSED, FAILED)")
		page     = flag.Int("page", 1, "Page number for listing")
		pageSize = flag.Int("page-size", 50, "Page size for listing")
	)

	flag.Parse()

	if !*list && !*clear && flag.NArg() < 1 {
		fmt.Println("Usage: lightrag-query [flags] <query>")
		fmt.Println("\nFlags:")
		flag.PrintDefaults()
		fmt.Println("\nExample:")
		fmt.Println("  lightrag-query -stream \"What is biology?\"")
		fmt.Println("  lightrag-query -list -status PROCESSED")
		fmt.Println("  lightrag-query -clear")
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

func zeroIfNil(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
