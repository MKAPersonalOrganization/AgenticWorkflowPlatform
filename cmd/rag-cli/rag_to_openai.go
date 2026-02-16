package main

//  WORKSPACE_ID=ccc18ec6-0354-48a2-820d-f4ee35bf5af5  EDGEQUAKE_MODE=hybrid EDGEQUAKE_RERANK_TOP_K=5  EDGEQUAKE_RERANK=true go run rag_to_openai.go "summarize the c4 model"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/charmbracelet/log"
)

// --- Retrieval API Types ---

type SourceReference struct {
	SourceType       string   `json:"source_type"` // chunk, entity, relationship
	ID               string   `json:"id"`
	Score            float32  `json:"score"`
	RerankScore      *float32 `json:"rerank_score,omitempty"`
	Snippet          *string  `json:"snippet,omitempty"`
	ReferenceID      *int     `json:"reference_id,omitempty"`
	DocumentID       *string  `json:"document_id,omitempty"`
	FilePath         *string  `json:"file_path,omitempty"`
	SourceDocumentID *string  `json:"source_document_id,omitempty"`
	SourceFilePath   *string  `json:"source_file_path,omitempty"`
	StartLine        *int     `json:"start_line,omitempty"`
	EndLine          *int     `json:"end_line,omitempty"`
	ChunkIndex       *int     `json:"chunk_index,omitempty"`
}

type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type QueryRequest struct {
	Query               string                `json:"query"`
	Mode                *string               `json:"mode,omitempty"` // naive, local, global, hybrid, mix
	ContextOnly         bool                  `json:"context_only"`
	PromptOnly          bool                  `json:"prompt_only,omitempty"`
	IncludeReferences   bool                  `json:"include_references"`
	ConversationHistory []ConversationMessage `json:"conversation_history,omitempty"`
	MaxResults          *int                  `json:"max_results,omitempty"`
	EnableRerank        bool                  `json:"enable_rerank,omitempty"`
	RerankModel         *string               `json:"rerank_model,omitempty"`
	RerankTopK          *int                  `json:"rerank_top_k,omitempty"`
	LLMProvider         *string               `json:"llm_provider,omitempty"`
	LLMModel            *string               `json:"llm_model,omitempty"`
}

type QueryStats struct {
	EmbeddingTimeMs  uint64   `json:"embedding_time_ms"`
	RetrievalTimeMs  uint64   `json:"retrieval_time_ms"`
	GenerationTimeMs uint64   `json:"generation_time_ms"`
	TotalTimeMs      uint64   `json:"total_time_ms"`
	SourcesRetrieved int      `json:"sources_retrieved"`
	RerankTimeMs     *uint64  `json:"rerank_time_ms,omitempty"`
	TokensUsed       *int     `json:"tokens_used,omitempty"`
	TokensPerSecond  *float32 `json:"tokens_per_second,omitempty"`
	LLMProvider      *string  `json:"llm_provider,omitempty"`
	LLMModel         *string  `json:"llm_model,omitempty"`
}

type QueryResponse struct {
	Answer         string            `json:"answer"`
	Mode           string            `json:"mode"`
	Sources        []SourceReference `json:"sources"`
	Stats          QueryStats        `json:"stats"`
	ConversationID *string           `json:"conversation_id,omitempty"`
	Reranked       bool              `json:"reranked,omitempty"`
}

// --- LLM API Types ---

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmRequest struct {
	Model    string       `json:"model"`
	Messages []llmMessage `json:"messages"`
}

type llmResponse struct {
	Choices []struct {
		Message llmMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// --- LLM Interface ---

type LLM interface {
	Generate(systemPrompt, userQuery string) (string, error)
}

// --- OpenAI LLM Implementation ---

type OpenAILLM struct {
	APIKey string
	Model  string
	Client *http.Client
}

func NewOpenAILLM(apiKey, model string) *OpenAILLM {
	return &OpenAILLM{
		APIKey: apiKey,
		Model:  model,
		Client: &http.Client{},
	}
}

func (llm *OpenAILLM) Generate(systemPrompt, userQuery string) (string, error) {
	log.Info("Execution Step: Calling LLM API with augmented context", "model", llm.Model)

	llmReq := llmRequest{
		Model: llm.Model,
		Messages: []llmMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userQuery},
		},
	}

	llmReqBody, err := json.Marshal(llmReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(llmReqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create LLM request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+llm.APIKey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := llm.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API returned error %d: %s", resp.StatusCode, string(body))
	}

	var llmResp llmResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return "", fmt.Errorf("failed to decode LLM response: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	log.Info("Final Step: AI response generated successfully")
	return llmResp.Choices[0].Message.Content, nil
}

// --- Retriever Interface ---

type Retriever interface {
	RetrieveContext(query string, opts QueryOptions) (*QueryResponse, error)
}

type QueryOptions struct {
	Mode                *string
	EnableRerank        bool
	LLMProvider         *string
	LLMModel            *string
	ConversationHistory []ConversationMessage
	MaxResults          *int
	RerankModel         *string
	RerankTopK          *int
}

// --- EdgeQuake Retriever Implementation ---

type EdgeQuakeRetriever struct {
	BaseURL     string
	WorkspaceID string
	Client      *http.Client
}

func NewEdgeQuakeRetriever(baseURL, workspaceID string) *EdgeQuakeRetriever {
	return &EdgeQuakeRetriever{
		BaseURL:     baseURL,
		WorkspaceID: workspaceID,
		Client:      &http.Client{},
	}
}

func (r *EdgeQuakeRetriever) RetrieveContext(query string, opts QueryOptions) (*QueryResponse, error) {
	log.Info("Retrieval Step: Fetching context from Retrieval API",
		"url", r.BaseURL,
		"workspace_id", r.WorkspaceID,
		"query", query,
		"mode", opts.Mode,
	)

	queryReq := QueryRequest{
		Query:               query,
		ContextOnly:         true,
		IncludeReferences:   true,
		EnableRerank:        opts.EnableRerank,
		ConversationHistory: opts.ConversationHistory,
		MaxResults:          opts.MaxResults,
		RerankModel:         opts.RerankModel,
		RerankTopK:          opts.RerankTopK,
	}

	if opts.Mode != nil {
		queryReq.Mode = opts.Mode
	}
	if opts.LLMProvider != nil {
		queryReq.LLMProvider = opts.LLMProvider
	}
	if opts.LLMModel != nil {
		queryReq.LLMModel = opts.LLMModel
	}

	reqBody, err := json.Marshal(queryReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query request: %w", err)
	}

	req, err := http.NewRequest("POST", r.BaseURL+"/api/v1/query", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if r.WorkspaceID != "" {
		req.Header.Set("X-Workspace-ID", r.WorkspaceID)
	}

	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("retrieval API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("retrieval API returned error %d: %s", resp.StatusCode, string(body))
	}

	var queryResp QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Debug("Context retrieved successfully", "source_count", len(queryResp.Sources))
	return &queryResp, nil
}

// --- Agent Type ---

type Agent struct {
	Retriever    Retriever
	LLM          LLM
	SystemPrompt string
}

// DefaultSystemPrompt provides the default system prompt template (100% Rust Parity)
const DefaultSystemPrompt = `---Role---

You are an expert AI assistant specializing in synthesizing information from a provided knowledge base. Your primary function is to answer user queries accurately by ONLY using the information within the provided **Context**.

---Goal---

Generate a comprehensive, well-structured answer to the user query.
The answer must integrate relevant facts from the Knowledge Graph and Document Chunks found in the **Context**.

---Instructions---

1. Step-by-Step Reasoning:
  - Carefully determine the user's query intent to fully understand the information need.
  - Scrutinize both Knowledge Graph Data (Entities and Relationships) and Document Chunks in the **Context**. Identify and extract all pieces of information that are directly relevant to answering the user query.
  - Weave the extracted facts into a coherent and logical response. Your own knowledge must ONLY be used to formulate fluent sentences and connect ideas, NOT to introduce any external information.

2. Content & Grounding:
  - Strictly adhere to the provided context; DO NOT invent, assume, or infer any information not explicitly stated.
  - If the answer cannot be fully determined from the **Context**, state what information IS available and note what is missing. A partial answer with specific data is better than a generic "insufficient information" response.

3. Formatting & Language:
  - The response MUST be in the same language as the user query.
  - Use Markdown formatting for clarity (headings, bold text, bullet points).

---Context---

{{.ContextText}}`

func NewAgent(retriever Retriever, llm LLM) *Agent {
	return &Agent{
		Retriever:    retriever,
		LLM:          llm,
		SystemPrompt: DefaultSystemPrompt,
	}
}

func NewAgentWithPrompt(retriever Retriever, llm LLM, systemPrompt string) *Agent {
	return &Agent{
		Retriever:    retriever,
		LLM:          llm,
		SystemPrompt: systemPrompt,
	}
}

// buildContextString builds the context string from source references (Agent-specific method)
func (a *Agent) buildContextString(sources []SourceReference) string {
	var chunks, entities, relationships []string

	for _, s := range sources {
		snippet := ""
		if s.Snippet != nil {
			snippet = *s.Snippet
		}

		switch s.SourceType {
		case "chunk":
			docRef := ""
			if s.DocumentID != nil {
				docRef = *s.DocumentID
			} else if s.FilePath != nil {
				docRef = *s.FilePath
			}
			if docRef != "" {
				chunks = append(chunks, fmt.Sprintf("[%d] (score: %.3f, doc: %s)\n%s", len(chunks)+1, s.Score, docRef, snippet))
			} else {
				chunks = append(chunks, fmt.Sprintf("[%d] (score: %.3f)\n%s", len(chunks)+1, s.Score, snippet))
			}
		case "entity":
			// In Retrieval API, ID is the Entity Name
			entities = append(entities, fmt.Sprintf("- **%s**: %s", s.ID, snippet))
		case "relationship":
			// In Retrieval API, ID is "Source->Target", Snippet is "Source Relation Target"
			relationships = append(relationships, fmt.Sprintf("- %s", snippet))
		}
	}

	var sb strings.Builder
	if len(entities) > 0 {
		sb.WriteString("### Knowledge Graph Data (Entities)\n\n")
		sb.WriteString(strings.Join(entities, "\n"))
		sb.WriteString("\n\n")
	}
	if len(relationships) > 0 {
		sb.WriteString("### Knowledge Graph Data (Relationships)\n\n")
		sb.WriteString(strings.Join(relationships, "\n"))
		sb.WriteString("\n\n")
	}
	if len(chunks) > 0 {
		sb.WriteString("### Document Chunks\n\n")
		sb.WriteString(strings.Join(chunks, "\n\n"))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (a *Agent) SendQuery(query string, opts QueryOptions) (string, error) {
	// 1. Retrieve context using the retriever
	queryResp, err := a.Retriever.RetrieveContext(query, opts)
	if err != nil {
		return "", fmt.Errorf("context retrieval failed: %w", err)
	}

	// 2. Build the system prompt with context
	contextText := a.buildContextString(queryResp.Sources)
	if contextText == "" {
		log.Warn("No context retrieved for query", "query", query)
	} else {
		log.Debug("Context Content:\n" + contextText)
	}

	tmpl, err := template.New("prompt").Parse(a.SystemPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to parse prompt template: %w", err)
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, struct {
		ContextText string
	}{
		ContextText: contextText,
	}); err != nil {
		return "", fmt.Errorf("failed to execute prompt template: %w", err)
	}

	fullPrompt := promptBuf.String()

	// 3. Generate response using LLM
	return a.LLM.Generate(fullPrompt, query)
}

func renderMarkdown(text string) {
	cmd := exec.Command("glow", "-")
	cmd.Stdin = strings.NewReader(text)
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		// Fallback to raw text if glow fails
		fmt.Println(text)
	}
}

func main() {
	// Configure logger
	log.SetReportTimestamp(true)
	log.SetLevel(log.DebugLevel)

	// Configuration
	edgequakeURL := os.Getenv("EDGEQUAKE_URL")
	if edgequakeURL == "" {
		edgequakeURL = "http://localhost:8081"
	}
	llmKey := os.Getenv("OPENAI_API_KEY")
	workspaceID := os.Getenv("WORKSPACE_ID")
	userQuery := "What are the main features mentioned in the documents?"

	if len(os.Args) > 1 {
		userQuery = strings.Join(os.Args[1:], " ")
	}

	if llmKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}

	if workspaceID == "" {
		log.Warn("WORKSPACE_ID not set, using default workspace")
	}

	// Create retriever
	retriever := NewEdgeQuakeRetriever(edgequakeURL, workspaceID)

	// Create LLM
	llm := NewOpenAILLM(llmKey, "gpt-4o")

	// Create agent
	agent := NewAgent(retriever, llm)

	// Advanced Query Options (Rust API Parity)
	mode := os.Getenv("EDGEQUAKE_MODE")
	llmProvider := os.Getenv("EDGEQUAKE_LLM_PROVIDER")
	llmModel := os.Getenv("EDGEQUAKE_LLM_MODEL")

	enableRerank := true // Default to true matching Rust API
	if os.Getenv("EDGEQUAKE_RERANK") == "false" {
		enableRerank = false
	}

	opts := QueryOptions{
		EnableRerank: enableRerank,
	}
	if mode != "" {
		opts.Mode = &mode
	}
	if llmProvider != "" {
		opts.LLMProvider = &llmProvider
	}
	if llmModel != "" {
		opts.LLMModel = &llmModel
	}

	// Send query using agent
	response, err := agent.SendQuery(userQuery, opts)
	if err != nil {
		log.Error("Query failed", "err", err)
		return
	}

	// Output result
	fmt.Println("\n--- AI Response ---")
	renderMarkdown(response)
}
