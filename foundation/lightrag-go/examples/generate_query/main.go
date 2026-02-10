// Package main demonstrates generating evaluation queries using LLM.
//
// This example shows how to:
// 1. Use an LLM to generate diverse evaluation queries
// 2. Create user personas for more realistic queries
// 3. Generate questions that test different aspects of a knowledge base
//
// This is useful for evaluating RAG system performance.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lightrag/lightrag-go/pkg/llm"
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

	// Get API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		logger.Warn("OPENAI_API_KEY not set - using mock generation")
		runMockDemo(logger)
		return
	}

	// Configuration
	llmModel := getEnvOrDefault("OPENAI_MODEL", "gpt-4o-mini")

	logger.Info("Starting Query Generation Demo")
	logger.Infof("LLM Model: %s", llmModel)

	// Initialize OpenAI client
	openaiClient := llm.NewOpenAIClient(&llm.LLMConfig{
		APIKey:  apiKey,
		Model:   llmModel,
		Timeout: 120,
	}, logger)

	ctx := context.Background()

	// Dataset description for generating queries
	datasetDescription := `
This knowledge base contains information about:
- Artificial Intelligence and Machine Learning concepts
- Deep Learning architectures and techniques
- Natural Language Processing methods
- Computer Vision applications
- Reinforcement Learning algorithms
- Large Language Models and their applications
- RAG (Retrieval-Augmented Generation) systems
- Knowledge graphs and vector databases
`

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("QUERY GENERATION DEMO")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("\nDataset Description:")
	fmt.Println(datasetDescription)

	// Generate user personas
	fmt.Println("\n" + strings.Repeat("-", 70))
	fmt.Println("GENERATING USER PERSONAS")
	fmt.Println(strings.Repeat("-", 70))

	personaPrompt := fmt.Sprintf(`Based on this dataset description:
%s

Generate 3 diverse user personas who might query this knowledge base.
For each persona, provide:
1. Name and role
2. Technical background level
3. Typical questions they might ask

Format as a numbered list.`, datasetDescription)

	resp, err := openaiClient.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: "user", Content: personaPrompt},
		},
	})
	if err != nil {
		logger.Errorf("Failed to generate personas: %v", err)
	} else {
		fmt.Println(resp.Content)
	}

	// Generate evaluation queries
	fmt.Println("\n" + strings.Repeat("-", 70))
	fmt.Println("GENERATING EVALUATION QUERIES")
	fmt.Println(strings.Repeat("-", 70))

	queryPrompt := fmt.Sprintf(`Based on this dataset description:
%s

Generate 10 diverse evaluation queries that test different aspects:
1. Simple factual questions (2 queries)
2. Comparison questions (2 queries)
3. Relationship questions (2 queries)
4. Definition/explanation questions (2 queries)
5. Complex multi-hop questions (2 queries)

Format each query with its type in brackets.`, datasetDescription)

	resp, err = openaiClient.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: "user", Content: queryPrompt},
		},
	})
	if err != nil {
		logger.Errorf("Failed to generate queries: %v", err)
	} else {
		fmt.Println(resp.Content)
	}

	// Generate edge case queries
	fmt.Println("\n" + strings.Repeat("-", 70))
	fmt.Println("GENERATING EDGE CASE QUERIES")
	fmt.Println(strings.Repeat("-", 70))

	edgeCasePrompt := fmt.Sprintf(`Based on this dataset description:
%s

Generate 5 edge case queries that might challenge a RAG system:
1. A query about a topic partially related but not fully covered
2. A query with ambiguous terminology
3. A query requiring inference from multiple facts
4. A query with negation or exclusion
5. A query that might be interpreted multiple ways

Format each with an explanation of why it's an edge case.`, datasetDescription)

	resp, err = openaiClient.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: "user", Content: edgeCasePrompt},
		},
	})
	if err != nil {
		logger.Errorf("Failed to generate edge cases: %v", err)
	} else {
		fmt.Println(resp.Content)
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Query generation demo completed!")
	fmt.Println(strings.Repeat("=", 70))
}

// runMockDemo demonstrates query generation without API access.
func runMockDemo(logger *logrus.Logger) {
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("QUERY GENERATION DEMO (Mock Mode)")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Println("\nExample User Personas:")
	personas := []string{
		"1. Sarah - ML Engineer: Advanced technical background, asks about implementation details and best practices",
		"2. Mike - Product Manager: Intermediate understanding, asks about use cases and comparisons",
		"3. Lisa - Student: Beginner level, asks for definitions and explanations",
	}
	for _, p := range personas {
		fmt.Println(p)
	}

	fmt.Println("\nExample Evaluation Queries:")
	queries := []string{
		"[Factual] What is the difference between supervised and unsupervised learning?",
		"[Factual] What are the main components of a transformer architecture?",
		"[Comparison] How does RAG compare to fine-tuning for knowledge-intensive tasks?",
		"[Comparison] What are the tradeoffs between CNN and ViT for image classification?",
		"[Relationship] How do embeddings relate to vector databases?",
		"[Relationship] What is the connection between attention mechanisms and transformers?",
		"[Definition] What is a knowledge graph and how is it used in AI?",
		"[Definition] Explain the concept of retrieval-augmented generation.",
		"[Multi-hop] How does combining knowledge graphs with LLMs improve factual accuracy?",
		"[Multi-hop] What role do embeddings play in connecting RAG systems to vector databases?",
	}
	for _, q := range queries {
		fmt.Println(q)
	}

	fmt.Println("\nExample Edge Cases:")
	edgeCases := []string{
		"1. 'What is the best AI model?' - Subjective, requires criteria definition",
		"2. 'How does ML work with quantum computing?' - Partially related topic",
		"3. 'What isn't deep learning good for?' - Negation query",
		"4. 'Explain the difference between attention and self-attention' - Subtle distinction",
		"5. 'When should I not use RAG?' - Counter-intuitive question",
	}
	for _, e := range edgeCases {
		fmt.Println(e)
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Set OPENAI_API_KEY for actual LLM-generated queries")
	fmt.Println(strings.Repeat("=", 70))
}
