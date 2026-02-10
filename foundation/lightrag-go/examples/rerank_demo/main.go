// Package main demonstrates reranking functionality in LightRAG.
//
// This example shows how to:
// 1. Configure a rerank function with Cohere or other providers
// 2. Compare query results with and without reranking
// 3. Understand how reranking improves relevance
//
// Requirements:
// - Cohere API key (set COHERE_API_KEY) for reranking
// - OpenAI API key (set OPENAI_API_KEY) for LLM and embeddings
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lightrag/lightrag-go/pkg/core/base"
	"github.com/lightrag/lightrag-go/pkg/core/lightrag"
	"github.com/lightrag/lightrag-go/pkg/core/operate"
	"github.com/lightrag/lightrag-go/pkg/llm"
	"github.com/lightrag/lightrag-go/pkg/rerank"
	"github.com/lightrag/lightrag-go/pkg/storage"
	"github.com/sirupsen/logrus"
)

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// Configure logging
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Get API keys
	openaiKey := os.Getenv("OPENAI_API_KEY")
	cohereKey := os.Getenv("COHERE_API_KEY")

	if openaiKey == "" {
		logger.Fatal("Error: OPENAI_API_KEY environment variable is not set")
	}

	// Configuration
	llmModel := getEnvOrDefault("OPENAI_MODEL", "gpt-4o-mini")
	embeddingModel := getEnvOrDefault("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
	rerankModel := getEnvOrDefault("COHERE_RERANK_MODEL", "rerank-english-v2.0")
	workingDir := getEnvOrDefault("WORKING_DIR", "./rerank_demo_data")

	logger.Info("Starting LightRAG Rerank Demo")
	logger.Infof("LLM Model: %s", llmModel)
	logger.Infof("Embedding Model: %s", embeddingModel)
	if cohereKey != "" {
		logger.Infof("Rerank Model: %s", rerankModel)
	} else {
		logger.Warn("COHERE_API_KEY not set - reranking will be simulated")
	}

	// Create working directory
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		logger.Fatalf("Failed to create working directory: %v", err)
	}

	// Initialize OpenAI client
	openaiClient := llm.NewOpenAIClient(&llm.LLMConfig{
		APIKey:         openaiKey,
		Model:          llmModel,
		EmbeddingModel: embeddingModel,
		Timeout:        120,
	}, logger)

	// Create LLM function wrapper
	llmFunc := func(ctx context.Context, prompt string, opts ...operate.LLMOption) (string, error) {
		messages := []llm.Message{
			{Role: "user", Content: prompt},
		}
		resp, err := openaiClient.Complete(ctx, &llm.CompletionRequest{Messages: messages})
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	}

	// Create embedding function
	embeddingFunc := func(ctx context.Context, texts []string) ([][]float64, error) {
		resp, err := openaiClient.Embed(ctx, &llm.EmbeddingRequest{Input: texts})
		if err != nil {
			return nil, err
		}
		return resp.Embeddings, nil
	}

	// Test embedding to get dimension
	logger.Info("Testing embedding function...")
	testEmbedding, err := embeddingFunc(context.Background(), []string{"test"})
	if err != nil {
		logger.Fatalf("Failed to test embedding: %v", err)
	}
	embeddingDim := len(testEmbedding[0])
	logger.Infof("Detected embedding dimension: %d", embeddingDim)

	// Create rerank function
	var rerankFunc func(ctx context.Context, query string, documents []string) ([]float64, error)
	if cohereKey != "" {
		// Use Cohere reranker
		cohereReranker := rerank.NewCohereReranker(&rerank.RerankerConfig{
			APIKey: cohereKey,
			Model:  rerankModel,
		}, logger)

		rerankFunc = func(ctx context.Context, query string, documents []string) ([]float64, error) {
			resp, err := cohereReranker.Rerank(ctx, &rerank.RerankRequest{
				Query:     query,
				Documents: documents,
			})
			if err != nil {
				return nil, err
			}
			// Extract scores in document order
			scores := make([]float64, len(documents))
			for _, r := range resp.Results {
				if r.Index < len(scores) {
					scores[r.Index] = r.RelevanceScore
				}
			}
			return scores, nil
		}
		logger.Info("Using Cohere for reranking")
	} else {
		// Simulated rerank function for demo
		rerankFunc = func(ctx context.Context, query string, documents []string) ([]float64, error) {
			// Simple mock: score based on query word overlap
			queryWords := strings.Fields(strings.ToLower(query))
			scores := make([]float64, len(documents))
			for i, doc := range documents {
				docLower := strings.ToLower(doc)
				matchCount := 0
				for _, word := range queryWords {
					if strings.Contains(docLower, word) {
						matchCount++
					}
				}
				scores[i] = float64(matchCount) / float64(len(queryWords)+1)
			}
			return scores, nil
		}
		logger.Info("Using simulated rerank function (set COHERE_API_KEY for real reranking)")
	}

	// Create LightRAG configuration
	config := lightrag.DefaultConfig()
	config.WorkingDir = workingDir
	config.LLMModelFunc = llmFunc
	config.LLMModelName = llmModel
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         embeddingFunc,
		EmbeddingDim: embeddingDim,
		MaxTokenSize: 8192,
		ModelName:    embeddingModel,
	}
	config.RerankModelFunc = &rerankFunc
	config.MinRerankScore = 0.1

	// Create LightRAG instance
	rag, err := lightrag.New(config)
	if err != nil {
		logger.Fatalf("Failed to create LightRAG instance: %v", err)
	}

	// Create storage instances
	logger.Info("Initializing storage backends...")

	kvConfig := func(namespace string) *storage.StorageConfig {
		return storage.NewStorageConfig(namespace, workingDir)
	}

	vdbConfig := func(namespace string) *storage.StorageConfig {
		cfg := storage.NewStorageConfig(namespace, workingDir)
		cfg.EmbeddingDim = embeddingDim
		cfg.CosineThreshold = 0.2
		cfg.EmbeddingFunc = embeddingFunc
		return cfg
	}

	graphConfig := func(namespace string) *storage.StorageConfig {
		return storage.NewStorageConfig(namespace, workingDir)
	}

	// Create all storage backends
	fullDocs := storage.NewJsonKVStorage(kvConfig("full_docs"), logger)
	textChunks := storage.NewJsonKVStorage(kvConfig("text_chunks"), logger)
	fullEntities := storage.NewJsonKVStorage(kvConfig("full_entities"), logger)
	fullRelations := storage.NewJsonKVStorage(kvConfig("full_relations"), logger)
	entityChunks := storage.NewJsonKVStorage(kvConfig("entity_chunks"), logger)
	relationChunks := storage.NewJsonKVStorage(kvConfig("relation_chunks"), logger)
	entitiesVDB := storage.NewNanoVectorDBStorage(vdbConfig("entities"), logger)
	relationshipsVDB := storage.NewNanoVectorDBStorage(vdbConfig("relationships"), logger)
	chunksVDB := storage.NewNanoVectorDBStorage(vdbConfig("chunks"), logger)
	graphStorage := storage.NewNetworkXStorage(graphConfig("graph"), logger)
	llmCache := storage.NewJsonKVStorage(kvConfig("llm_cache"), logger)
	docStatus := storage.NewJsonKVStorage(kvConfig("doc_status"), logger)

	// Set all storages
	rag.SetStorages(
		fullDocs, textChunks, fullEntities, fullRelations,
		entityChunks, relationChunks,
		entitiesVDB, relationshipsVDB, chunksVDB,
		graphStorage, llmCache, docStatus,
	)

	// Initialize storages
	ctx := context.Background()
	if err := rag.InitializeStorages(ctx); err != nil {
		logger.Fatalf("Failed to initialize storages: %v", err)
	}
	defer rag.FinalizeStorages(ctx)

	// Sample documents with varying relevance
	documents := []string{
		`Machine Learning is a subset of artificial intelligence that enables systems to learn
from data and improve from experience without being explicitly programmed. It encompasses
supervised learning, unsupervised learning, and reinforcement learning paradigms.`,

		`Deep Learning is a subset of machine learning based on artificial neural networks
with multiple layers. These deep neural networks can learn complex patterns in large
amounts of data for tasks like image recognition and natural language processing.`,

		`The history of computing dates back to ancient abacuses and mechanical calculators.
Modern digital computers emerged in the mid-20th century with pioneers like Alan Turing
and John von Neumann laying the theoretical foundations.`,

		`Python is a versatile programming language widely used in data science and machine
learning. Libraries like TensorFlow, PyTorch, and scikit-learn make it the language
of choice for many AI practitioners.`,

		`Quantum computing uses quantum mechanical phenomena like superposition and entanglement
to perform calculations. It has potential applications in cryptography, optimization,
and simulating quantum systems that are intractable for classical computers.`,
	}

	// Insert documents
	logger.Info("Inserting sample documents...")
	for i, doc := range documents {
		if err := rag.Insert(ctx, doc); err != nil {
			logger.Errorf("Failed to insert document %d: %v", i, err)
		}
	}
	logger.Info("Documents inserted successfully!")

	// Test queries
	queries := []string{
		"What is machine learning?",
		"How do neural networks work?",
		"What programming language is used for AI?",
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("RERANKING COMPARISON")
	fmt.Println(strings.Repeat("=", 70))

	for _, query := range queries {
		fmt.Printf("\n📝 Query: %s\n", query)
		fmt.Println(strings.Repeat("-", 70))

		// Query without reranking
		paramNoRerank := base.NewQueryParam()
		paramNoRerank.Mode = "hybrid"
		paramNoRerank.EnableRerank = false

		fmt.Println("\n❌ Without Reranking:")
		resultNoRerank, err := rag.Query(ctx, query, paramNoRerank)
		if err != nil {
			logger.Errorf("Query without rerank failed: %v", err)
		} else {
			response := resultNoRerank.Answer
			if response == "" {
				response = resultNoRerank.Content
			}
			if len(response) > 400 {
				response = response[:400] + "..."
			}
			fmt.Printf("%s\n", response)
		}

		// Query with reranking
		paramWithRerank := base.NewQueryParam()
		paramWithRerank.Mode = "hybrid"
		paramWithRerank.EnableRerank = true

		fmt.Println("\n✅ With Reranking:")
		resultWithRerank, err := rag.Query(ctx, query, paramWithRerank)
		if err != nil {
			logger.Errorf("Query with rerank failed: %v", err)
		} else {
			response := resultWithRerank.Answer
			if response == "" {
				response = resultWithRerank.Content
			}
			if len(response) > 400 {
				response = response[:400] + "..."
			}
			fmt.Printf("%s\n", response)
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("EXPLANATION")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Print(`
Reranking improves search relevance by:
1. Taking initial vector search results
2. Using a specialized model to re-score documents for the specific query
3. Reordering results based on semantic relevance
4. Filtering out low-scoring documents

This often produces more relevant context for the LLM to generate answers.
`)

	logger.Info("Rerank demo completed successfully!")
}
