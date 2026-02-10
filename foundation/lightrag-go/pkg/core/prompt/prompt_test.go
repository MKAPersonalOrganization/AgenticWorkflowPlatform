package prompt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultDelimiters(t *testing.T) {
	assert.Equal(t, "<|#|>", DefaultTupleDelimiter)
	assert.Equal(t, "<|COMPLETE|>", DefaultCompletionDelimiter)
}

func TestPromptsMap(t *testing.T) {
	// Test that prompts map is initialized
	assert.NotNil(t, Prompts)

	// Test default delimiters are in map
	assert.Equal(t, DefaultTupleDelimiter, Prompts["DEFAULT_TUPLE_DELIMITER"])
	assert.Equal(t, DefaultCompletionDelimiter, Prompts["DEFAULT_COMPLETION_DELIMITER"])
}

func TestGetPrompt(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"entity extraction system prompt", "entity_extraction_system_prompt", true},
		{"entity extraction user prompt", "entity_extraction_user_prompt", true},
		{"entity continue extraction", "entity_continue_extraction_user_prompt", true},
		{"summarize entity descriptions", "summarize_entity_descriptions", true},
		{"rag response", "rag_response", true},
		{"naive rag response", "naive_rag_response", true},
		{"keyword extraction", "keyword_extraction", true},
		{"fail response", "fail_response", true},
		{"nonexistent", "nonexistent_prompt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, ok := GetPrompt(tt.key)
			assert.Equal(t, tt.expected, ok)
			if tt.expected {
				assert.NotEmpty(t, prompt)
			}
		})
	}
}

func TestEntityExtractionSystemPrompt(t *testing.T) {
	// Test that the prompt contains key instructions
	assert.Contains(t, EntityExtractionSystemPrompt, "Knowledge Graph Specialist")
	assert.Contains(t, EntityExtractionSystemPrompt, "entity_name")
	assert.Contains(t, EntityExtractionSystemPrompt, "entity_type")
	assert.Contains(t, EntityExtractionSystemPrompt, "entity_description")
	assert.Contains(t, EntityExtractionSystemPrompt, "relationship_keywords")
	assert.Contains(t, EntityExtractionSystemPrompt, "{tuple_delimiter}")
	assert.Contains(t, EntityExtractionSystemPrompt, "{completion_delimiter}")
	assert.Contains(t, EntityExtractionSystemPrompt, "{language}")
}

func TestEntityExtractionUserPrompt(t *testing.T) {
	assert.Contains(t, EntityExtractionUserPrompt, "Extract entities and relationships")
	assert.Contains(t, EntityExtractionUserPrompt, "{entity_types}")
	assert.Contains(t, EntityExtractionUserPrompt, "{input_text}")
	assert.Contains(t, EntityExtractionUserPrompt, "{completion_delimiter}")
}

func TestSummarizeEntityDescriptionsPrompt(t *testing.T) {
	assert.Contains(t, SummarizeEntityDescriptionsPrompt, "Knowledge Graph Specialist")
	assert.Contains(t, SummarizeEntityDescriptionsPrompt, "synthesize")
	assert.Contains(t, SummarizeEntityDescriptionsPrompt, "{language}")
	assert.Contains(t, SummarizeEntityDescriptionsPrompt, "{description_list}")
}

func TestRAGResponsePrompt(t *testing.T) {
	assert.Contains(t, RAGResponsePrompt, "helpful assistant")
	assert.Contains(t, RAGResponsePrompt, "{response_type}")
	assert.Contains(t, RAGResponsePrompt, "{context_data}")
}

func TestNaiveRAGResponsePrompt(t *testing.T) {
	assert.Contains(t, NaiveRAGResponsePrompt, "helpful assistant")
	assert.Contains(t, NaiveRAGResponsePrompt, "{content_data}")
	assert.Contains(t, NaiveRAGResponsePrompt, "{response_type}")
}

func TestKeywordExtractionPrompt(t *testing.T) {
	assert.Contains(t, KeywordExtractionPrompt, "high-level")
	assert.Contains(t, KeywordExtractionPrompt, "low-level")
	assert.Contains(t, KeywordExtractionPrompt, "high_level_keywords")
	assert.Contains(t, KeywordExtractionPrompt, "low_level_keywords")
	assert.Contains(t, KeywordExtractionPrompt, "{query}")
}

func TestFailPrompt(t *testing.T) {
	assert.Contains(t, FailPrompt, "extraction failed")
}
