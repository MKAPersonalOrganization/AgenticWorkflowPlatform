package operate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lightrag/lightrag-go/pkg/core/base"
)

// MockTokenizer implements utils.TokenizerInterface for testing
type MockTokenizer struct {
	CharsPerToken int
}

func NewMockTokenizer() *MockTokenizer {
	return &MockTokenizer{CharsPerToken: 4}
}

func (t *MockTokenizer) Encode(content string) []int {
	numTokens := len(content) / t.CharsPerToken
	if numTokens == 0 && len(content) > 0 {
		numTokens = 1
	}
	tokens := make([]int, numTokens)
	for i := range tokens {
		tokens[i] = i
	}
	return tokens
}

func (t *MockTokenizer) Decode(tokens []int) string {
	// Simple simulation: return characters based on token count
	chars := len(tokens) * t.CharsPerToken
	return string(make([]byte, chars))
}

func TestTruncateEntityIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		limit      int
		expected   string
	}{
		{
			name:       "short identifier",
			identifier: "Person",
			limit:      256,
			expected:   "Person",
		},
		{
			name:       "exact limit",
			identifier: "abc",
			limit:      3,
			expected:   "abc",
		},
		{
			name:       "exceeds limit",
			identifier: "This is a very long entity name that exceeds the limit",
			limit:      20,
			expected:   "This is a very long ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateEntityIdentifier(tt.identifier, tt.limit, "test-chunk", "Entity")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChunkingByTokenSize(t *testing.T) {
	tokenizer := NewMockTokenizer()

	t.Run("simple content chunking", func(t *testing.T) {
		content := "This is a test content that needs to be chunked into smaller pieces."
		results, err := ChunkingByTokenSize(tokenizer, content, nil, false, 2, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, results)
	})

	t.Run("split by character", func(t *testing.T) {
		content := "Section 1\n\nSection 2\n\nSection 3"
		sep := "\n\n"
		results, err := ChunkingByTokenSize(tokenizer, content, &sep, false, 2, 100)
		require.NoError(t, err)
		assert.Equal(t, 3, len(results))
	})

	t.Run("empty content", func(t *testing.T) {
		results, err := ChunkingByTokenSize(tokenizer, "", nil, false, 2, 10)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestChunkTokenLimitExceededError(t *testing.T) {
	err := &ChunkTokenLimitExceededError{
		ChunkTokens:     1500,
		ChunkTokenLimit: 1000,
		ChunkPreview:    "test content...",
	}

	assert.Contains(t, err.Error(), "1500")
	assert.Contains(t, err.Error(), "1000")
}

func TestHandleSingleEntityExtraction(t *testing.T) {
	t.Run("valid entity", func(t *testing.T) {
		attributes := []string{"entity", "John Doe", "Person", "A software engineer from San Francisco"}
		result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")

		require.NotNil(t, result)
		assert.Equal(t, "John Doe", result.EntityName)
		assert.Equal(t, "person", result.EntityType)
		assert.NotEmpty(t, result.Description)
		assert.Equal(t, "chunk-1", result.SourceID)
	})

	t.Run("invalid entity - wrong field count", func(t *testing.T) {
		attributes := []string{"entity", "John Doe", "Person"}
		result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")
		assert.Nil(t, result)
	})

	t.Run("invalid entity - empty name", func(t *testing.T) {
		attributes := []string{"entity", "", "Person", "Description"}
		result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")
		assert.Nil(t, result)
	})

	t.Run("invalid entity - no entity marker", func(t *testing.T) {
		attributes := []string{"other", "John Doe", "Person", "Description"}
		result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")
		assert.Nil(t, result)
	})
}

func TestHandleSingleRelationshipExtraction(t *testing.T) {
	t.Run("valid relationship", func(t *testing.T) {
		attributes := []string{"relation", "John", "Apple", "works_at", "John works at Apple Inc."}
		result := HandleSingleRelationshipExtraction(attributes, "chunk-1", 1234567890, "test.txt")

		require.NotNil(t, result)
		assert.Equal(t, "John", result.SrcID)
		assert.Equal(t, "Apple", result.TgtID)
		assert.NotEmpty(t, result.Description)
		assert.Equal(t, 1.0, result.Weight)
	})

	t.Run("relationship with relationship marker", func(t *testing.T) {
		attributes := []string{"relationship", "John", "Apple", "works_at", "John works at Apple"}
		result := HandleSingleRelationshipExtraction(attributes, "chunk-1", 1234567890, "test.txt")

		require.NotNil(t, result)
		assert.Equal(t, "John", result.SrcID)
	})

	t.Run("invalid relationship - wrong field count", func(t *testing.T) {
		attributes := []string{"relation", "John", "Apple", "works_at"}
		result := HandleSingleRelationshipExtraction(attributes, "chunk-1", 1234567890, "test.txt")
		assert.Nil(t, result)
	})

	t.Run("self-relationship rejected", func(t *testing.T) {
		attributes := []string{"relation", "John", "John", "self", "Self reference"}
		result := HandleSingleRelationshipExtraction(attributes, "chunk-1", 1234567890, "test.txt")
		assert.Nil(t, result)
	})
}

func TestProcessExtractionResult(t *testing.T) {
	t.Run("valid extraction", func(t *testing.T) {
		input := `entity<|#|>John Doe<|#|>Person<|#|>A software engineer
entity<|#|>Apple Inc<|#|>Organization<|#|>A technology company
relation<|#|>John Doe<|#|>Apple Inc<|#|>works_at<|#|>John works at Apple
<|COMPLETE|>`

		result, err := ProcessExtractionResult(input, "chunk-1", 1234567890, "test.txt", "<|#|>", "<|COMPLETE|>")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.Entities)
		assert.NotEmpty(t, result.Relationships)
	})

	t.Run("missing completion delimiter", func(t *testing.T) {
		input := `entity<|#|>Test<|#|>Person<|#|>Description`
		result, err := ProcessExtractionResult(input, "chunk-1", 1234567890, "test.txt", "<|#|>", "<|COMPLETE|>")

		require.NoError(t, err) // Should not error, just warn
		require.NotNil(t, result)
	})

	t.Run("empty input", func(t *testing.T) {
		result, err := ProcessExtractionResult("", "chunk-1", 1234567890, "test.txt", "<|#|>", "<|COMPLETE|>")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.Entities)
		assert.Empty(t, result.Relationships)
	})
}

func TestMakeRelationChunkKey(t *testing.T) {
	tests := []struct {
		src      string
		tgt      string
		expected string
	}{
		{"a", "b", "a||b"},
		{"b", "a", "a||b"}, // Should be sorted
		{"John", "Apple", "Apple||John"},
	}

	for _, tt := range tests {
		t.Run(tt.src+"_"+tt.tgt, func(t *testing.T) {
			result := MakeRelationChunkKey(tt.src, tt.tgt)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplySourceIDsLimit(t *testing.T) {
	t.Run("no limit needed", func(t *testing.T) {
		ids := []string{"a", "b", "c"}
		result := ApplySourceIDsLimit(ids, 5, "KEEP", "test")
		assert.Equal(t, ids, result)
	})

	t.Run("KEEP method", func(t *testing.T) {
		ids := []string{"a", "b", "c", "d", "e"}
		result := ApplySourceIDsLimit(ids, 3, "KEEP", "test")
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})

	t.Run("FIFO method", func(t *testing.T) {
		ids := []string{"a", "b", "c", "d", "e"}
		result := ApplySourceIDsLimit(ids, 3, "FIFO", "test")
		assert.Equal(t, []string{"c", "d", "e"}, result)
	})
}

func TestMergeSourceIDs(t *testing.T) {
	t.Run("merge with deduplication", func(t *testing.T) {
		existing := []string{"a", "b", "c"}
		new := []string{"b", "c", "d"}
		result := MergeSourceIDs(existing, new)

		assert.Equal(t, []string{"a", "b", "c", "d"}, result)
	})

	t.Run("empty existing", func(t *testing.T) {
		result := MergeSourceIDs([]string{}, []string{"a", "b"})
		assert.Equal(t, []string{"a", "b"}, result)
	})

	t.Run("empty new", func(t *testing.T) {
		result := MergeSourceIDs([]string{"a", "b"}, []string{})
		assert.Equal(t, []string{"a", "b"}, result)
	})
}

func TestPickByWeightedPolling(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := PickByWeightedPolling(nil, 5, 1)
		assert.Empty(t, result)
	})

	t.Run("single entity", func(t *testing.T) {
		entities := []map[string]interface{}{
			{"sorted_chunks": []string{"c1", "c2", "c3", "c4", "c5"}},
		}
		result := PickByWeightedPolling(entities, 3, 1)
		assert.Equal(t, []string{"c1", "c2", "c3"}, result)
	})

	t.Run("multiple entities", func(t *testing.T) {
		entities := []map[string]interface{}{
			{"sorted_chunks": []string{"a1", "a2", "a3"}},
			{"sorted_chunks": []string{"b1", "b2", "b3"}},
			{"sorted_chunks": []string{"c1", "c2", "c3"}},
		}
		result := PickByWeightedPolling(entities, 3, 1)

		// First entity should have more chunks due to higher weight
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "a1")
	})
}

func TestPipelineStatus(t *testing.T) {
	status := NewPipelineStatus()

	status.Update("Step 1 completed")
	latest, history := status.GetStatus()
	assert.Equal(t, "Step 1 completed", latest)
	assert.Len(t, history, 1)

	status.Update("Step 2 completed")
	latest, history = status.GetStatus()
	assert.Equal(t, "Step 2 completed", latest)
	assert.Len(t, history, 2)
}

func TestSortChunksByTimestamp(t *testing.T) {
	chunks := []map[string]interface{}{
		{"id": "c1", "timestamp": int64(1000)},
		{"id": "c2", "timestamp": int64(3000)},
		{"id": "c3", "timestamp": int64(2000)},
	}

	SortChunksByTimestamp(chunks)

	// Should be sorted in descending order
	assert.Equal(t, int64(3000), chunks[0]["timestamp"])
	assert.Equal(t, int64(2000), chunks[1]["timestamp"])
	assert.Equal(t, int64(1000), chunks[2]["timestamp"])
}

func TestGetCurrentTimestamp(t *testing.T) {
	ts := GetCurrentTimestamp()
	assert.Greater(t, ts, int64(0))
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		assert.Equal(t, tt.expected, result)
	}
}

func TestFixTupleDelimiterCorruption(t *testing.T) {
	tests := []struct {
		name      string
		record    string
		core      string
		delimiter string
		expected  string
	}{
		{
			name:      "no corruption",
			record:    "entity<|#|>name<|#|>type<|#|>desc",
			core:      "#",
			delimiter: "<|#|>",
			expected:  "entity<|#|>name<|#|>type<|#|>desc",
		},
		{
			name:      "corrupted with spaces",
			record:    "entity< # >name< # >type",
			core:      "#",
			delimiter: "<|#|>",
			expected:  "entity<|#|>name<|#|>type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixTupleDelimiterCorruption(tt.record, tt.core, tt.delimiter)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEntityData(t *testing.T) {
	entity := EntityData{
		EntityName:  "Test Entity",
		EntityType:  "person",
		Description: "A test entity",
		SourceID:    "chunk-1",
		FilePath:    "test.txt",
		Timestamp:   1234567890,
	}

	assert.Equal(t, "Test Entity", entity.EntityName)
	assert.Equal(t, "person", entity.EntityType)
}

func TestRelationshipData(t *testing.T) {
	rel := RelationshipData{
		SrcID:       "Entity A",
		TgtID:       "Entity B",
		Weight:      1.5,
		Description: "A relationship",
		Keywords:    "related, connected",
		SourceID:    "chunk-1",
		FilePath:    "test.txt",
		Timestamp:   1234567890,
	}

	assert.Equal(t, "Entity A", rel.SrcID)
	assert.Equal(t, "Entity B", rel.TgtID)
	assert.Equal(t, 1.5, rel.Weight)
}

func TestGlobalConfig(t *testing.T) {
	config := GlobalConfig{
		SummaryContextSize:       12000,
		SummaryMaxTokens:         1200,
		ForceLLMSummaryOnMerge:   8,
		SummaryLengthRecommended: 600,
		LLMModelMaxAsync:         4,
		MaxSourceIDsPerEntity:    300,
		MaxSourceIDsPerRelation:  300,
		SourceIDsLimitMethod:     "FIFO",
		MaxFilePaths:             100,
	}

	assert.Equal(t, 12000, config.SummaryContextSize)
	assert.Equal(t, 4, config.LLMModelMaxAsync)
}

func TestCollectChunkIDs(t *testing.T) {
	contextResult := &base.QueryContextResult{
		Entities: []map[string]interface{}{
			{"source_id": "chunk1<SEP>chunk2"},
			{"source_id": "chunk2<SEP>chunk3"},
		},
		Relationships: []map[string]interface{}{
			{"source_id": "chunk3<SEP>chunk4"},
		},
	}

	ids := collectChunkIDs(contextResult)

	// Should have unique IDs
	assert.Contains(t, ids, "chunk1")
	assert.Contains(t, ids, "chunk2")
	assert.Contains(t, ids, "chunk3")
	assert.Contains(t, ids, "chunk4")
}

func TestParseKeywordsFromResponse(t *testing.T) {
	tests := []struct {
		response string
		expected []string
	}{
		{
			response: "keyword1, keyword2, keyword3",
			expected: []string{"keyword1", "keyword2", "keyword3"},
		},
		{
			response: "keyword1\nkeyword2\nkeyword3",
			expected: []string{"keyword1", "keyword2", "keyword3"},
		},
		{
			response: "keyword1; keyword2; keyword3",
			expected: []string{"keyword1", "keyword2", "keyword3"},
		},
	}

	for _, tt := range tests {
		result := parseKeywordsFromResponse(tt.response)
		assert.Equal(t, tt.expected, result)
	}
}

func TestBuildContextString(t *testing.T) {
	contextResult := &base.QueryContextResult{
		Entities: []map[string]interface{}{
			{"entity_name": "John", "description": "A person"},
		},
		Relationships: []map[string]interface{}{
			{"src_id": "John", "tgt_id": "Apple", "description": "Works at"},
		},
		TextChunks: []map[string]interface{}{
			{"content": "Some document content"},
		},
	}

	result := buildContextString(contextResult)

	assert.Contains(t, result, "Entities")
	assert.Contains(t, result, "John")
	assert.Contains(t, result, "Relationships")
	assert.Contains(t, result, "Apple")
	assert.Contains(t, result, "Source Documents")
}

func TestGetString(t *testing.T) {
	m := map[string]interface{}{
		"name":   "test",
		"number": 123,
	}

	assert.Equal(t, "test", getString(m, "name"))
	assert.Equal(t, "", getString(m, "number")) // Not a string
	assert.Equal(t, "", getString(m, "missing"))
}

func TestGetStringSlice(t *testing.T) {
	t.Run("string slice", func(t *testing.T) {
		m := map[string]interface{}{
			"items": []string{"a", "b", "c"},
		}
		result := getStringSlice(m, "items")
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})

	t.Run("interface slice", func(t *testing.T) {
		m := map[string]interface{}{
			"items": []interface{}{"a", "b", "c"},
		}
		result := getStringSlice(m, "items")
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})

	t.Run("missing key", func(t *testing.T) {
		m := map[string]interface{}{}
		result := getStringSlice(m, "items")
		assert.Empty(t, result)
	})
}

func TestGetInt64(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]interface{}
		key      string
		expected int64
	}{
		{
			name:     "int64 value",
			m:        map[string]interface{}{"ts": int64(1234)},
			key:      "ts",
			expected: 1234,
		},
		{
			name:     "int value",
			m:        map[string]interface{}{"ts": 1234},
			key:      "ts",
			expected: 1234,
		},
		{
			name:     "float64 value",
			m:        map[string]interface{}{"ts": float64(1234.5)},
			key:      "ts",
			expected: 1234,
		},
		{
			name:     "missing key",
			m:        map[string]interface{}{},
			key:      "ts",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getInt64(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}
