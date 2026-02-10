// Package operate provides the core operation functions for LightRAG.
// This is a direct translation of Python lightrag/operate.py to Go.
package operate

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lightrag/lightrag-go/pkg/core/base"
	"github.com/lightrag/lightrag-go/pkg/core/coreconstants"
	"github.com/lightrag/lightrag-go/pkg/core/prompt"
	"github.com/lightrag/lightrag-go/pkg/core/utils"
)

// ChunkResult represents a chunked piece of text
type ChunkResult struct {
	Tokens          int    `json:"tokens"`
	Content         string `json:"content"`
	ChunkOrderIndex int    `json:"chunk_order_index"`
}

// EntityData represents extracted entity information
type EntityData struct {
	EntityName  string `json:"entity_name"`
	EntityType  string `json:"entity_type"`
	Description string `json:"description"`
	SourceID    string `json:"source_id"`
	FilePath    string `json:"file_path"`
	Timestamp   int64  `json:"timestamp"`
}

// RelationshipData represents extracted relationship information
type RelationshipData struct {
	SrcID       string  `json:"src_id"`
	TgtID       string  `json:"tgt_id"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
	Keywords    string  `json:"keywords"`
	SourceID    string  `json:"source_id"`
	FilePath    string  `json:"file_path"`
	Timestamp   int64   `json:"timestamp"`
}

// ExtractionResult contains extracted entities and relationships
type ExtractionResult struct {
	Entities      map[string][]EntityData
	Relationships map[string][]RelationshipData // key format: "src_id||tgt_id"
}

// GlobalConfig holds configuration for operations
type GlobalConfig struct {
	Tokenizer                      utils.TokenizerInterface
	SummaryContextSize             int
	SummaryMaxTokens               int
	ForceLLMSummaryOnMerge         int
	SummaryLengthRecommended       int
	LLMModelFunc                   LLMFunc
	LLMModelMaxAsync               int
	MaxSourceIDsPerEntity          int
	MaxSourceIDsPerRelation        int
	SourceIDsLimitMethod           string
	MaxFilePaths                   int
	FilePathMorePlaceholder        string
	AddonParams                    map[string]interface{}
	Workspace                      string
	EnableLLMCache                 bool
	EnableLLMCacheForEntityExtract bool
	EmbeddingTokenLimit            int
}

// LLMFunc is the function signature for LLM calls
type LLMFunc func(ctx context.Context, userPrompt string, opts ...LLMOption) (string, error)

// LLMOption represents options for LLM calls
type LLMOption func(*LLMOptions)

// LLMOptions holds options for LLM calls
type LLMOptions struct {
	SystemPrompt    string
	MaxTokens       int
	HistoryMessages []map[string]string
	Priority        int
}

// WithSystemPrompt sets the system prompt
func WithSystemPrompt(prompt string) LLMOption {
	return func(o *LLMOptions) {
		o.SystemPrompt = prompt
	}
}

// WithMaxTokens sets max tokens
func WithMaxTokens(maxTokens int) LLMOption {
	return func(o *LLMOptions) {
		o.MaxTokens = maxTokens
	}
}

// WithPriority sets the call priority
func WithPriority(priority int) LLMOption {
	return func(o *LLMOptions) {
		o.Priority = priority
	}
}

// truncateEntityIdentifier truncates entity identifiers that exceed the configured length limit
func truncateEntityIdentifier(identifier string, limit int, chunkKey, identifierRole string) string {
	if len(identifier) <= limit {
		return identifier
	}

	displayValue := identifier[:limit]
	preview := identifier
	if len(preview) > 20 {
		preview = preview[:20]
	}
	utils.Logger.Warnf("%s: %s len %d > %d chars (Name: '%s...')",
		chunkKey, identifierRole, len(identifier), limit, preview)
	return displayValue
}

// ChunkingByTokenSize chunks content by token size
func ChunkingByTokenSize(
	tokenizer utils.TokenizerInterface,
	content string,
	splitByCharacter *string,
	splitByCharacterOnly bool,
	chunkOverlapTokenSize int,
	chunkTokenSize int,
) ([]ChunkResult, error) {
	tokens := tokenizer.Encode(content)
	var results []ChunkResult

	if splitByCharacter != nil && *splitByCharacter != "" {
		rawChunks := strings.Split(content, *splitByCharacter)
		var newChunks []struct {
			tokens  int
			content string
		}

		if splitByCharacterOnly {
			for _, chunk := range rawChunks {
				chunkTokens := tokenizer.Encode(chunk)
				if len(chunkTokens) > chunkTokenSize {
					utils.Logger.Warnf("Chunk split_by_character exceeds token limit: len=%d limit=%d",
						len(chunkTokens), chunkTokenSize)
					return nil, &ChunkTokenLimitExceededError{
						ChunkTokens:     len(chunkTokens),
						ChunkTokenLimit: chunkTokenSize,
						ChunkPreview:    truncateString(chunk, 120),
					}
				}
				newChunks = append(newChunks, struct {
					tokens  int
					content string
				}{len(chunkTokens), chunk})
			}
		} else {
			for _, chunk := range rawChunks {
				chunkTokens := tokenizer.Encode(chunk)
				if len(chunkTokens) > chunkTokenSize {
					step := chunkTokenSize - chunkOverlapTokenSize
					for start := 0; start < len(chunkTokens); start += step {
						end := start + chunkTokenSize
						if end > len(chunkTokens) {
							end = len(chunkTokens)
						}
						chunkContent := tokenizer.Decode(chunkTokens[start:end])
						tokenCount := chunkTokenSize
						if len(chunkTokens)-start < chunkTokenSize {
							tokenCount = len(chunkTokens) - start
						}
						newChunks = append(newChunks, struct {
							tokens  int
							content string
						}{tokenCount, chunkContent})
					}
				} else {
					newChunks = append(newChunks, struct {
						tokens  int
						content string
					}{len(chunkTokens), chunk})
				}
			}
		}

		for index, chunk := range newChunks {
			results = append(results, ChunkResult{
				Tokens:          chunk.tokens,
				Content:         strings.TrimSpace(chunk.content),
				ChunkOrderIndex: index,
			})
		}
	} else {
		step := chunkTokenSize - chunkOverlapTokenSize
		for index, start := 0, 0; start < len(tokens); index, start = index+1, start+step {
			end := start + chunkTokenSize
			if end > len(tokens) {
				end = len(tokens)
			}
			chunkContent := tokenizer.Decode(tokens[start:end])
			tokenCount := chunkTokenSize
			if len(tokens)-start < chunkTokenSize {
				tokenCount = len(tokens) - start
			}
			results = append(results, ChunkResult{
				Tokens:          tokenCount,
				Content:         strings.TrimSpace(chunkContent),
				ChunkOrderIndex: index,
			})
		}
	}

	return results, nil
}

// ChunkTokenLimitExceededError is raised when a chunk exceeds token limit
type ChunkTokenLimitExceededError struct {
	ChunkTokens     int
	ChunkTokenLimit int
	ChunkPreview    string
}

func (e *ChunkTokenLimitExceededError) Error() string {
	return fmt.Sprintf("Chunk token limit exceeded: %d tokens (limit: %d) preview: %s",
		e.ChunkTokens, e.ChunkTokenLimit, e.ChunkPreview)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// HandleEntityRelationSummary handles entity/relation description summary using map-reduce approach
func HandleEntityRelationSummary(
	ctx context.Context,
	descriptionType string,
	entityOrRelationName string,
	descriptionList []string,
	separator string,
	config *GlobalConfig,
	llmResponseCache base.BaseKVStorage,
) (string, bool, error) {
	// Handle empty input
	if len(descriptionList) == 0 {
		return "", false, nil
	}

	// If only one description, return it directly
	if len(descriptionList) == 1 {
		return descriptionList[0], false, nil
	}

	currentList := make([]string, len(descriptionList))
	copy(currentList, descriptionList)
	llmWasUsed := false

	// Iterative map-reduce process
	for {
		// Calculate total tokens in current list
		totalTokens := 0
		for _, desc := range currentList {
			totalTokens += len(config.Tokenizer.Encode(desc))
		}

		// If total length is within limits, perform final summarization
		if totalTokens <= config.SummaryContextSize || len(currentList) <= 2 {
			if len(currentList) < config.ForceLLMSummaryOnMerge && totalTokens < config.SummaryMaxTokens {
				// No LLM needed, just join the descriptions
				finalDescription := strings.Join(currentList, separator)
				return finalDescription, llmWasUsed, nil
			}

			if totalTokens > config.SummaryContextSize && len(currentList) <= 2 {
				utils.Logger.Warnf("Summarizing %s: Oversize description found", entityOrRelationName)
			}

			// Final summarization of remaining descriptions
			finalSummary, err := summarizeDescriptions(ctx, descriptionType, entityOrRelationName,
				currentList, config, llmResponseCache)
			if err != nil {
				return "", false, err
			}
			return finalSummary, true, nil
		}

		// Need to split into chunks - Map phase
		var chunks [][]string
		var currentChunk []string
		currentTokens := 0

		for i, desc := range currentList {
			descTokens := len(config.Tokenizer.Encode(desc))

			// If adding current description would exceed limit, finalize current chunk
			if currentTokens+descTokens > config.SummaryContextSize && len(currentChunk) > 0 {
				if len(currentChunk) == 1 {
					// Force add one more description
					currentChunk = append(currentChunk, desc)
					chunks = append(chunks, currentChunk)
					utils.Logger.Warnf("Summarizing %s: Oversize description found", entityOrRelationName)
					currentChunk = nil
					currentTokens = 0
				} else {
					chunks = append(chunks, currentChunk)
					currentChunk = []string{desc}
					currentTokens = descTokens
				}
			} else {
				currentChunk = append(currentChunk, desc)
				currentTokens += descTokens
			}

			_ = i // suppress unused variable warning
		}

		// Add the last chunk if it exists
		if len(currentChunk) > 0 {
			chunks = append(chunks, currentChunk)
		}

		utils.Logger.Infof("   Summarizing %s: Map %d descriptions into %d groups",
			entityOrRelationName, len(currentList), len(chunks))

		// Reduce phase: summarize each group
		var newSummaries []string
		for _, chunk := range chunks {
			if len(chunk) == 1 {
				newSummaries = append(newSummaries, chunk[0])
			} else {
				summary, err := summarizeDescriptions(ctx, descriptionType, entityOrRelationName,
					chunk, config, llmResponseCache)
				if err != nil {
					return "", false, err
				}
				newSummaries = append(newSummaries, summary)
				llmWasUsed = true
			}
		}

		currentList = newSummaries
	}
}

// summarizeDescriptions helper function to summarize descriptions using LLM
func summarizeDescriptions(
	ctx context.Context,
	descriptionType string,
	descriptionName string,
	descriptionList []string,
	config *GlobalConfig,
	llmResponseCache base.BaseKVStorage,
) (string, error) {
	language := coreconstants.DefaultSummaryLanguage
	if config.AddonParams != nil {
		if lang, ok := config.AddonParams["language"].(string); ok {
			language = lang
		}
	}

	// Convert descriptions to descWrapper format
	jsonDescriptions := make([]descWrapper, len(descriptionList))
	for i, desc := range descriptionList {
		jsonDescriptions[i] = descWrapper{Description: desc}
	}

	// Truncate by token size
	truncated := truncateDescriptionsByTokenSize(jsonDescriptions, config.SummaryContextSize, config.Tokenizer)

	// Convert to JSONL format
	var lines []string
	for _, desc := range truncated {
		data, _ := json.Marshal(desc)
		lines = append(lines, string(data))
	}
	joinedDescriptions := strings.Join(lines, "\n")

	// Prepare prompt using string replacement for {placeholder} format
	usePrompt := prompt.SummarizeEntityDescriptionsPrompt
	usePrompt = strings.ReplaceAll(usePrompt, "{description_type}", descriptionType)
	usePrompt = strings.ReplaceAll(usePrompt, "{description_name}", descriptionName)
	usePrompt = strings.ReplaceAll(usePrompt, "{description_list}", joinedDescriptions)
	usePrompt = strings.ReplaceAll(usePrompt, "{summary_max_tokens}", fmt.Sprintf("%d", config.SummaryLengthRecommended))
	usePrompt = strings.ReplaceAll(usePrompt, "{language}", language)

	// Call LLM
	summary, err := config.LLMModelFunc(ctx, usePrompt, WithPriority(8))
	if err != nil {
		return "", err
	}

	// Check summary token length against embedding limit
	if config.EmbeddingTokenLimit > 0 && summary != "" {
		summaryTokenCount := len(config.Tokenizer.Encode(summary))
		if summaryTokenCount > config.EmbeddingTokenLimit {
			utils.Logger.Warnf("Summary tokens(%d) exceeds embedding_token_limit(%d) for %s: %s",
				summaryTokenCount, config.EmbeddingTokenLimit, descriptionType, descriptionName)
		}
	}

	return summary, nil
}

// descWrapper is used to wrap description strings for JSON serialization
type descWrapper struct {
	Description string `json:"Description"`
}

func truncateDescriptionsByTokenSize(
	descriptions []descWrapper,
	maxTokenSize int,
	tokenizer utils.TokenizerInterface,
) []descWrapper {
	totalTokens := 0
	for i, desc := range descriptions {
		data, _ := json.Marshal(desc)
		tokens := len(tokenizer.Encode(string(data)))
		totalTokens += tokens
		if totalTokens > maxTokenSize {
			return descriptions[:i]
		}
	}
	return descriptions
}

// HandleSingleEntityExtraction extracts a single entity from record attributes
func HandleSingleEntityExtraction(
	recordAttributes []string,
	chunkKey string,
	timestamp int64,
	filePath string,
) *EntityData {
	if len(recordAttributes) != 4 || !strings.Contains(recordAttributes[0], "entity") {
		if len(recordAttributes) > 1 && strings.Contains(recordAttributes[0], "entity") {
			typeInfo := "N/A"
			if len(recordAttributes) > 2 {
				typeInfo = recordAttributes[2]
			}
			utils.Logger.Warnf("%s: LLM output format error; found %d/4 fields on ENTITY `%s` @ `%s`",
				chunkKey, len(recordAttributes), recordAttributes[1], typeInfo)
		}
		return nil
	}

	entityName := utils.SanitizeAndNormalizeExtractedText(recordAttributes[1], true)
	if entityName == "" {
		utils.Logger.Infof("Empty entity name found after sanitization. Original: '%s'", recordAttributes[1])
		return nil
	}

	entityType := utils.SanitizeAndNormalizeExtractedText(recordAttributes[2], true)
	invalidChars := []string{"'", "(", ")", "<", ">", "|", "/", "\\"}
	for _, char := range invalidChars {
		if strings.Contains(entityType, char) {
			utils.Logger.Warnf("Entity extraction error: invalid entity type in: %v", recordAttributes)
			return nil
		}
	}

	entityType = strings.ToLower(strings.ReplaceAll(entityType, " ", ""))

	entityDescription := utils.SanitizeAndNormalizeExtractedText(recordAttributes[3], false)
	if strings.TrimSpace(entityDescription) == "" {
		utils.Logger.Warnf("Entity extraction error: empty description for entity '%s' of type '%s'",
			entityName, entityType)
		return nil
	}

	return &EntityData{
		EntityName:  entityName,
		EntityType:  entityType,
		Description: entityDescription,
		SourceID:    chunkKey,
		FilePath:    filePath,
		Timestamp:   timestamp,
	}
}

// HandleSingleRelationshipExtraction extracts a single relationship from record attributes
func HandleSingleRelationshipExtraction(
	recordAttributes []string,
	chunkKey string,
	timestamp int64,
	filePath string,
) *RelationshipData {
	if len(recordAttributes) != 5 || !strings.Contains(recordAttributes[0], "relation") {
		if len(recordAttributes) > 1 && strings.Contains(recordAttributes[0], "relation") {
			targetInfo := "N/A"
			if len(recordAttributes) > 2 {
				targetInfo = recordAttributes[2]
			}
			utils.Logger.Warnf("%s: LLM output format error; found %d/5 fields on RELATION `%s`~`%s`",
				chunkKey, len(recordAttributes), recordAttributes[1], targetInfo)
		}
		return nil
	}

	source := utils.SanitizeAndNormalizeExtractedText(recordAttributes[1], true)
	target := utils.SanitizeAndNormalizeExtractedText(recordAttributes[2], true)

	if source == "" {
		utils.Logger.Infof("Empty source entity found after sanitization. Original: '%s'", recordAttributes[1])
		return nil
	}

	if target == "" {
		utils.Logger.Infof("Empty target entity found after sanitization. Original: '%s'", recordAttributes[2])
		return nil
	}

	if source == target {
		utils.Logger.Debugf("Relationship source and target are the same in: %v", recordAttributes)
		return nil
	}

	edgeKeywords := utils.SanitizeAndNormalizeExtractedText(recordAttributes[3], true)
	edgeKeywords = strings.ReplaceAll(edgeKeywords, "，", ",")

	edgeDescription := utils.SanitizeAndNormalizeExtractedText(recordAttributes[4], false)

	weight := 1.0
	if len(recordAttributes) > 5 {
		lastAttr := strings.Trim(recordAttributes[len(recordAttributes)-1], `"'`)
		if utils.IsFloatRegex(lastAttr) {
			if w, err := parseFloat(lastAttr); err == nil {
				weight = w
			}
		}
	}

	return &RelationshipData{
		SrcID:       source,
		TgtID:       target,
		Weight:      weight,
		Description: edgeDescription,
		Keywords:    edgeKeywords,
		SourceID:    chunkKey,
		FilePath:    filePath,
		Timestamp:   timestamp,
	}
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// ProcessExtractionResult processes a single extraction result
func ProcessExtractionResult(
	result string,
	chunkKey string,
	timestamp int64,
	filePath string,
	tupleDelimiter string,
	completionDelimiter string,
) (*ExtractionResult, error) {
	maybeNodes := make(map[string][]EntityData)
	maybeEdges := make(map[string][]RelationshipData)

	if !strings.Contains(result, completionDelimiter) {
		utils.Logger.Warnf("%s: Complete delimiter can not be found in extraction result", chunkKey)
	}

	// Split LLM output result to records by "\n"
	records := utils.SplitStringByMultiMarkers(result, []string{
		"\n", completionDelimiter, strings.ToLower(completionDelimiter),
	})

	// Fix LLM output format errors
	var fixedRecords []string
	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		entityMarker := fmt.Sprintf("%sentity%s", tupleDelimiter, tupleDelimiter)
		entityRecords := utils.SplitStringByMultiMarkers(record, []string{entityMarker})

		for _, entityRecord := range entityRecords {
			if !strings.HasPrefix(entityRecord, "entity") && !strings.HasPrefix(entityRecord, "relation") {
				entityRecord = fmt.Sprintf("entity<|%s", entityRecord)
			}

			relationMarkers := []string{
				fmt.Sprintf("%srelationship%s", tupleDelimiter, tupleDelimiter),
				fmt.Sprintf("%srelation%s", tupleDelimiter, tupleDelimiter),
			}
			entityRelationRecords := utils.SplitStringByMultiMarkers(entityRecord, relationMarkers)

			for _, entityRelationRecord := range entityRelationRecords {
				if !strings.HasPrefix(entityRelationRecord, "entity") && !strings.HasPrefix(entityRelationRecord, "relation") {
					entityRelationRecord = fmt.Sprintf("relation%s%s", tupleDelimiter, entityRelationRecord)
				}
				fixedRecords = append(fixedRecords, entityRelationRecord)
			}
		}
	}

	if len(fixedRecords) != len(records) {
		utils.Logger.Warnf("%s: LLM output format error; find LLM use %s as record separators instead new-line",
			chunkKey, tupleDelimiter)
	}

	for _, record := range fixedRecords {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		// Fix tuple delimiter corruption
		delimiterCore := tupleDelimiter[2 : len(tupleDelimiter)-2] // Extract "#" from "<|#|>"
		record = fixTupleDelimiterCorruption(record, delimiterCore, tupleDelimiter)
		if delimiterCore != strings.ToLower(delimiterCore) {
			delimiterCore = strings.ToLower(delimiterCore)
			record = fixTupleDelimiterCorruption(record, delimiterCore, tupleDelimiter)
		}

		recordAttributes := utils.SplitStringByMultiMarkers(record, []string{tupleDelimiter})

		// Try to parse as entity
		entityData := HandleSingleEntityExtraction(recordAttributes, chunkKey, timestamp, filePath)
		if entityData != nil {
			truncatedName := truncateEntityIdentifier(
				entityData.EntityName,
				coreconstants.DefaultEntityNameMaxLength,
				chunkKey,
				"Entity name",
			)
			entityData.EntityName = truncatedName
			maybeNodes[truncatedName] = append(maybeNodes[truncatedName], *entityData)
			continue
		}

		// Try to parse as relationship
		relationshipData := HandleSingleRelationshipExtraction(recordAttributes, chunkKey, timestamp, filePath)
		if relationshipData != nil {
			truncatedSource := truncateEntityIdentifier(
				relationshipData.SrcID,
				coreconstants.DefaultEntityNameMaxLength,
				chunkKey,
				"Relation entity",
			)
			truncatedTarget := truncateEntityIdentifier(
				relationshipData.TgtID,
				coreconstants.DefaultEntityNameMaxLength,
				chunkKey,
				"Relation entity",
			)
			relationshipData.SrcID = truncatedSource
			relationshipData.TgtID = truncatedTarget
			edgeKey := fmt.Sprintf("%s||%s", truncatedSource, truncatedTarget)
			maybeEdges[edgeKey] = append(maybeEdges[edgeKey], *relationshipData)
		}
	}

	return &ExtractionResult{
		Entities:      maybeNodes,
		Relationships: maybeEdges,
	}, nil
}

// fixTupleDelimiterCorruption fixes various forms of tuple delimiter corruption
func fixTupleDelimiterCorruption(record, delimiterCore, tupleDelimiter string) string {
	// Only fix corrupted delimiters, not valid ones
	// Escape special regex characters in delimiterCore
	escapedCore := regexp.QuoteMeta(delimiterCore)

	// Pattern to match corrupted delimiters (with spaces around the core)
	patterns := []string{
		fmt.Sprintf(`<\s+%s\s+>`, escapedCore), // < # > (spaces around core)
		fmt.Sprintf(`<\s+%s>`, escapedCore),    // < #> (space before)
		fmt.Sprintf(`<%s\s+>`, escapedCore),    // <# > (space after)
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		record = re.ReplaceAllString(record, tupleDelimiter)
	}

	return record
}

// MakeRelationChunkKey creates a consistent key for relation chunk storage
func MakeRelationChunkKey(src, tgt string) string {
	// Sort to ensure consistent ordering
	if src > tgt {
		src, tgt = tgt, src
	}
	return fmt.Sprintf("%s||%s", src, tgt)
}

// ApplySourceIDsLimit applies source IDs limit based on the method
func ApplySourceIDsLimit(sourceIDs []string, limit int, method, identifier string) []string {
	if len(sourceIDs) <= limit {
		return sourceIDs
	}

	var result []string
	switch method {
	case coreconstants.SourceIDsLimitMethodFIFO:
		// FIFO: keep tail (newest), discard head
		result = sourceIDs[len(sourceIDs)-limit:]
	default:
		// KEEP: keep head (earliest), discard tail
		result = sourceIDs[:limit]
	}

	utils.Logger.Infof("Limited %s: source_ids %d -> %d (%s)",
		identifier, len(sourceIDs), limit, method)

	return result
}

// MergeSourceIDs merges and deduplicates source IDs
func MergeSourceIDs(existing, new []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, id := range existing {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	for _, id := range new {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}

	return result
}

// QueryContext holds context for query operations
type QueryContext struct {
	GlobalConfig          *GlobalConfig
	KnowledgeGraph        base.BaseGraphStorage
	EntitiesVDB           base.BaseVectorStorage
	RelationshipsVDB      base.BaseVectorStorage
	ChunksVDB             base.BaseVectorStorage
	TextChunksStorage     base.BaseKVStorage
	LLMResponseCache      base.BaseKVStorage
	EntityChunksStorage   base.BaseKVStorage
	RelationChunksStorage base.BaseKVStorage
	EmbeddingFunc         *base.EmbeddingFunc
}

// KGQuery performs a knowledge graph query
func KGQuery(
	ctx context.Context,
	query string,
	queryParam *base.QueryParam,
	queryContext *QueryContext,
) (*base.QueryResult, error) {
	if queryParam == nil {
		queryParam = base.NewQueryParam()
	}

	// Perform knowledge graph-based retrieval
	contextResult, err := buildQueryContext(ctx, query, queryParam, queryContext)
	if err != nil {
		return nil, fmt.Errorf("failed to build query context: %w", err)
	}

	// Generate response using LLM
	response, err := generateRAGResponse(ctx, query, contextResult, queryParam, queryContext)
	if err != nil {
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	return &base.QueryResult{
		Answer:        response,
		Context:       contextResult,
		Status:        "success",
		ContextChunks: contextResult.TextChunks,
	}, nil
}

// NaiveQuery performs a naive (non-KG) query
func NaiveQuery(
	ctx context.Context,
	query string,
	queryParam *base.QueryParam,
	queryContext *QueryContext,
) (*base.QueryResult, error) {
	if queryParam == nil {
		queryParam = base.NewQueryParam()
	}

	// Perform vector similarity search on chunks
	chunks, err := retrieveChunksByVector(ctx, query, queryParam, queryContext)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve chunks: %w", err)
	}

	// Build context from chunks
	contextResult := &base.QueryContextResult{
		TextChunks: chunks,
	}

	// Generate response using LLM
	response, err := generateNaiveRAGResponse(ctx, query, contextResult, queryParam, queryContext)
	if err != nil {
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	return &base.QueryResult{
		Answer:        response,
		Context:       contextResult,
		Status:        "success",
		ContextChunks: chunks,
	}, nil
}

// buildQueryContext builds the context for a KG query
func buildQueryContext(
	ctx context.Context,
	query string,
	queryParam *base.QueryParam,
	queryContext *QueryContext,
) (*base.QueryContextResult, error) {
	result := &base.QueryContextResult{}

	// Extract keywords from query
	keywords, err := extractKeywords(ctx, query, queryContext)
	if err != nil {
		utils.Logger.Warnf("Failed to extract keywords: %v", err)
	}

	// Search entities by keywords
	if len(keywords) > 0 {
		entities, err := searchEntitiesByKeywords(ctx, keywords, queryParam, queryContext)
		if err != nil {
			utils.Logger.Warnf("Failed to search entities: %v", err)
		}
		result.Entities = entities
	}

	// Search relationships
	relationships, err := searchRelationships(ctx, keywords, queryParam, queryContext)
	if err != nil {
		utils.Logger.Warnf("Failed to search relationships: %v", err)
	}
	result.Relationships = relationships

	// Retrieve relevant text chunks
	chunks, err := retrieveRelatedChunks(ctx, query, result, queryParam, queryContext)
	if err != nil {
		utils.Logger.Warnf("Failed to retrieve chunks: %v", err)
	}
	result.TextChunks = chunks

	return result, nil
}

// extractKeywords extracts keywords from a query
func extractKeywords(
	ctx context.Context,
	query string,
	queryContext *QueryContext,
) ([]string, error) {
	promptText := strings.ReplaceAll(prompt.KeywordExtractionPrompt, "{query}", query)

	response, err := queryContext.GlobalConfig.LLMModelFunc(ctx, promptText, WithPriority(5))
	if err != nil {
		return nil, err
	}

	// Parse keywords from response
	response = utils.RemoveThinkTags(response)
	keywords := parseKeywordsFromResponse(response)

	return keywords, nil
}

func parseKeywordsFromResponse(response string) []string {
	// Simple keyword parsing - split by common delimiters
	delimiters := []string{",", ";", "\n", "|"}
	keywords := utils.SplitStringByMultiMarkers(response, delimiters)

	var result []string
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw != "" && len(kw) > 1 {
			result = append(result, kw)
		}
	}

	return result
}

// searchEntitiesByKeywords searches entities by keywords
func searchEntitiesByKeywords(
	ctx context.Context,
	keywords []string,
	queryParam *base.QueryParam,
	queryContext *QueryContext,
) ([]map[string]interface{}, error) {
	var entities []map[string]interface{}

	// Search in vector DB
	if queryContext.EntitiesVDB != nil {
		queryText := strings.Join(keywords, " ")
		// Get embedding for the query
		var queryEmbedding []float64
		if queryContext.EmbeddingFunc != nil {
			embeddings, err := queryContext.EmbeddingFunc.Embed(ctx, []string{queryText})
			if err != nil {
				return nil, fmt.Errorf("failed to get embedding: %w", err)
			}
			if len(embeddings) > 0 {
				queryEmbedding = embeddings[0]
			}
		}
		results, err := queryContext.EntitiesVDB.Query(ctx, queryText, queryParam.TopK, queryEmbedding)
		if err != nil {
			return nil, err
		}
		entities = append(entities, results...)
	}

	return entities, nil
}

// searchRelationships searches relationships
func searchRelationships(
	ctx context.Context,
	keywords []string,
	queryParam *base.QueryParam,
	queryContext *QueryContext,
) ([]map[string]interface{}, error) {
	var relationships []map[string]interface{}

	// Search in vector DB
	if queryContext.RelationshipsVDB != nil {
		queryText := strings.Join(keywords, " ")
		// Get embedding for the query
		var queryEmbedding []float64
		if queryContext.EmbeddingFunc != nil {
			embeddings, err := queryContext.EmbeddingFunc.Embed(ctx, []string{queryText})
			if err != nil {
				return nil, fmt.Errorf("failed to get embedding: %w", err)
			}
			if len(embeddings) > 0 {
				queryEmbedding = embeddings[0]
			}
		}
		results, err := queryContext.RelationshipsVDB.Query(ctx, queryText, queryParam.TopK, queryEmbedding)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, results...)
	}

	return relationships, nil
}

// retrieveRelatedChunks retrieves related text chunks
func retrieveRelatedChunks(
	ctx context.Context,
	query string,
	contextResult *base.QueryContextResult,
	queryParam *base.QueryParam,
	queryContext *QueryContext,
) ([]map[string]interface{}, error) {
	var chunks []map[string]interface{}

	// Collect chunk IDs from entities and relationships
	chunkIDs := collectChunkIDs(contextResult)

	// Retrieve chunks from storage
	if queryContext.TextChunksStorage != nil && len(chunkIDs) > 0 {
		for _, chunkID := range chunkIDs {
			chunkData, err := queryContext.TextChunksStorage.GetByID(ctx, chunkID)
			if err != nil {
				continue
			}
			if chunkData != nil {
				chunks = append(chunks, chunkData)
			}
		}
	}

	// Apply chunk limits and processing
	chunks = processChunks(chunks, queryParam, queryContext.GlobalConfig)

	return chunks, nil
}

func collectChunkIDs(contextResult *base.QueryContextResult) []string {
	seen := make(map[string]bool)
	var chunkIDs []string

	// From entities
	for _, entity := range contextResult.Entities {
		if sourceID, ok := entity["source_id"].(string); ok {
			ids := strings.Split(sourceID, coreconstants.GraphFieldSep)
			for _, id := range ids {
				id = strings.TrimSpace(id)
				if id != "" && !seen[id] {
					seen[id] = true
					chunkIDs = append(chunkIDs, id)
				}
			}
		}
	}

	// From relationships
	for _, rel := range contextResult.Relationships {
		if sourceID, ok := rel["source_id"].(string); ok {
			ids := strings.Split(sourceID, coreconstants.GraphFieldSep)
			for _, id := range ids {
				id = strings.TrimSpace(id)
				if id != "" && !seen[id] {
					seen[id] = true
					chunkIDs = append(chunkIDs, id)
				}
			}
		}
	}

	return chunkIDs
}

func processChunks(chunks []map[string]interface{}, queryParam *base.QueryParam, config *GlobalConfig) []map[string]interface{} {
	// Apply chunk_top_k limit
	if queryParam.ChunkTopK > 0 && len(chunks) > queryParam.ChunkTopK {
		chunks = chunks[:queryParam.ChunkTopK]
	}

	// Token-based truncation if tokenizer available
	if config.Tokenizer != nil && len(chunks) > 0 {
		maxTokens := queryParam.MaxTotalTokens
		if maxTokens <= 0 {
			maxTokens = coreconstants.DefaultMaxTotalTokens
		}

		totalTokens := 0
		var truncatedChunks []map[string]interface{}
		for _, chunk := range chunks {
			content := ""
			if c, ok := chunk["content"].(string); ok {
				content = c
			}
			tokens := len(config.Tokenizer.Encode(content))
			if totalTokens+tokens > maxTokens {
				break
			}
			totalTokens += tokens
			truncatedChunks = append(truncatedChunks, chunk)
		}
		chunks = truncatedChunks
	}

	return chunks
}

// retrieveChunksByVector retrieves chunks by vector similarity
func retrieveChunksByVector(
	ctx context.Context,
	query string,
	queryParam *base.QueryParam,
	queryContext *QueryContext,
) ([]map[string]interface{}, error) {
	var chunks []map[string]interface{}

	if queryContext.ChunksVDB != nil {
		// Get embedding for the query
		var queryEmbedding []float64
		if queryContext.EmbeddingFunc != nil {
			embeddings, err := queryContext.EmbeddingFunc.Embed(ctx, []string{query})
			if err != nil {
				return nil, fmt.Errorf("failed to get embedding: %w", err)
			}
			if len(embeddings) > 0 {
				queryEmbedding = embeddings[0]
			}
		}
		results, err := queryContext.ChunksVDB.Query(ctx, query, queryParam.ChunkTopK, queryEmbedding)
		if err != nil {
			return nil, err
		}
		chunks = results
	}

	return processChunks(chunks, queryParam, queryContext.GlobalConfig), nil
}

// generateRAGResponse generates a RAG response
func generateRAGResponse(
	ctx context.Context,
	query string,
	contextResult *base.QueryContextResult,
	queryParam *base.QueryParam,
	queryContext *QueryContext,
) (string, error) {
	// Build context string
	contextStr := buildContextString(contextResult)

	// Determine response type
	responseType := queryParam.ResponseType
	if responseType == "" {
		responseType = "Multiple Paragraphs"
	}

	// Build prompt
	promptText := prompt.RAGResponsePrompt
	promptText = strings.ReplaceAll(promptText, "{context_data}", contextStr)
	promptText = strings.ReplaceAll(promptText, "{response_type}", responseType)

	// Call LLM
	response, err := queryContext.GlobalConfig.LLMModelFunc(ctx, promptText, WithPriority(3))
	if err != nil {
		return "", err
	}

	return utils.RemoveThinkTags(response), nil
}

// generateNaiveRAGResponse generates a naive RAG response
func generateNaiveRAGResponse(
	ctx context.Context,
	query string,
	contextResult *base.QueryContextResult,
	queryParam *base.QueryParam,
	queryContext *QueryContext,
) (string, error) {
	// Build context string from chunks
	var contextParts []string
	for _, chunk := range contextResult.TextChunks {
		if content, ok := chunk["content"].(string); ok {
			contextParts = append(contextParts, content)
		}
	}
	contextStr := strings.Join(contextParts, "\n\n")

	// Determine response type
	responseType := queryParam.ResponseType
	if responseType == "" {
		responseType = "Multiple Paragraphs"
	}

	// Build prompt
	promptText := prompt.NaiveRAGResponsePrompt
	promptText = strings.ReplaceAll(promptText, "{content_data}", contextStr)
	promptText = strings.ReplaceAll(promptText, "{response_type}", responseType)

	// Call LLM
	response, err := queryContext.GlobalConfig.LLMModelFunc(ctx, promptText, WithPriority(3))
	if err != nil {
		return "", err
	}

	return utils.RemoveThinkTags(response), nil
}

func buildContextString(contextResult *base.QueryContextResult) string {
	var parts []string

	// Add entities
	if len(contextResult.Entities) > 0 {
		parts = append(parts, "## Entities:")
		for _, entity := range contextResult.Entities {
			name := getString(entity, "entity_name")
			desc := getString(entity, "description")
			if name != "" {
				parts = append(parts, fmt.Sprintf("- %s: %s", name, desc))
			}
		}
	}

	// Add relationships
	if len(contextResult.Relationships) > 0 {
		parts = append(parts, "\n## Relationships:")
		for _, rel := range contextResult.Relationships {
			src := getString(rel, "src_id")
			tgt := getString(rel, "tgt_id")
			desc := getString(rel, "description")
			if src != "" && tgt != "" {
				parts = append(parts, fmt.Sprintf("- %s -> %s: %s", src, tgt, desc))
			}
		}
	}

	// Add text chunks
	if len(contextResult.TextChunks) > 0 {
		parts = append(parts, "\n## Source Documents:")
		for i, chunk := range contextResult.TextChunks {
			content := getString(chunk, "content")
			if content != "" {
				parts = append(parts, fmt.Sprintf("### Document %d\n%s", i+1, content))
			}
		}
	}

	return strings.Join(parts, "\n")
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// PipelineStatus tracks the status of a processing pipeline
type PipelineStatus struct {
	LatestMessage   string
	HistoryMessages []string
	mu              sync.Mutex
}

// NewPipelineStatus creates a new pipeline status tracker
func NewPipelineStatus() *PipelineStatus {
	return &PipelineStatus{
		HistoryMessages: make([]string, 0),
	}
}

// Update updates the pipeline status
func (p *PipelineStatus) Update(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.LatestMessage = message
	p.HistoryMessages = append(p.HistoryMessages, message)
}

// GetStatus returns the current pipeline status
func (p *PipelineStatus) GetStatus() (string, []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.LatestMessage, append([]string{}, p.HistoryMessages...)
}

// PickByWeightedPolling implements linear gradient weighted polling for chunk selection
func PickByWeightedPolling(
	entitiesOrRelations []map[string]interface{},
	maxRelatedChunks int,
	minRelatedChunks int,
) []string {
	if len(entitiesOrRelations) == 0 {
		return []string{}
	}

	n := len(entitiesOrRelations)
	if n == 1 {
		sortedChunks := getStringSlice(entitiesOrRelations[0], "sorted_chunks")
		if len(sortedChunks) > maxRelatedChunks {
			return sortedChunks[:maxRelatedChunks]
		}
		return sortedChunks
	}

	// Calculate expected chunk count for each position
	expectedCounts := make([]int, n)
	for i := 0; i < n; i++ {
		ratio := float64(i) / float64(n-1)
		expected := float64(maxRelatedChunks) - ratio*float64(maxRelatedChunks-minRelatedChunks)
		expectedCounts[i] = int(expected + 0.5) // Round
	}

	// First round allocation
	var selectedChunks []string
	usedCounts := make([]int, n)
	totalRemaining := 0

	for i, entityRel := range entitiesOrRelations {
		sortedChunks := getStringSlice(entityRel, "sorted_chunks")
		expected := expectedCounts[i]
		actual := expected
		if len(sortedChunks) < actual {
			actual = len(sortedChunks)
		}
		selectedChunks = append(selectedChunks, sortedChunks[:actual]...)
		usedCounts[i] = actual

		remaining := expected - actual
		if remaining > 0 {
			totalRemaining += remaining
		}
	}

	// Second round allocation
	for j := 0; j < totalRemaining; j++ {
		allocated := false
		for i, entityRel := range entitiesOrRelations {
			sortedChunks := getStringSlice(entityRel, "sorted_chunks")
			if usedCounts[i] < len(sortedChunks) {
				selectedChunks = append(selectedChunks, sortedChunks[usedCounts[i]])
				usedCounts[i]++
				allocated = true
				break
			}
		}
		if !allocated {
			break
		}
	}

	return selectedChunks
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key].([]string); ok {
		return v
	}
	if v, ok := m[key].([]interface{}); ok {
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return []string{}
}

// SortChunksByTimestamp sorts chunks by their timestamp in descending order
func SortChunksByTimestamp(chunks []map[string]interface{}) {
	sort.Slice(chunks, func(i, j int) bool {
		ti := getInt64(chunks[i], "timestamp")
		tj := getInt64(chunks[j], "timestamp")
		return ti > tj // Descending order (newest first)
	})
}

func getInt64(m map[string]interface{}, key string) int64 {
	switch v := m[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

// GetCurrentTimestamp returns the current Unix timestamp
func GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}
