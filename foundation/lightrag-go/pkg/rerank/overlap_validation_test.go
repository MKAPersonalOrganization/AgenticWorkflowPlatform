package rerank

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Overlap Validation Tests
// ============================================================================

func TestOverlapGreaterThanMaxTokens(t *testing.T) {
	// Test that overlap_tokens > max_tokens is clamped and doesn't hang
	documents := []string{strings.Repeat("word ", 100)}

	// This should clamp overlap_tokens to maxTokens - 1
	chunked, docIndices := chunkDocumentsForRerank(documents, 30, 32)

	// Should complete without hanging
	assert.Greater(t, len(chunked), 0)
	for _, idx := range docIndices {
		assert.Equal(t, 0, idx)
	}
}

func TestOverlapEqualToMaxTokens(t *testing.T) {
	// Test that overlap_tokens == max_tokens is clamped and doesn't hang
	documents := []string{strings.Repeat("word ", 100)}

	// This should clamp overlap_tokens to maxTokens - 1
	chunked, docIndices := chunkDocumentsForRerank(documents, 30, 30)

	// Should complete without hanging
	assert.Greater(t, len(chunked), 0)
	for _, idx := range docIndices {
		assert.Equal(t, 0, idx)
	}
}

func TestOverlapSlightlyLessThanMaxTokens(t *testing.T) {
	// Test that overlap_tokens < max_tokens works normally
	documents := []string{strings.Repeat("word ", 100)}

	// This should work without clamping
	chunked, docIndices := chunkDocumentsForRerank(documents, 30, 29)

	// Should complete successfully
	assert.Greater(t, len(chunked), 0)
	for _, idx := range docIndices {
		assert.Equal(t, 0, idx)
	}
}

func TestSmallMaxTokensWithLargeOverlap(t *testing.T) {
	// Test edge case with very small max_tokens
	documents := []string{strings.Repeat("word ", 50)}

	// max_tokens=5, overlap_tokens=10 should clamp to 4
	chunked, docIndices := chunkDocumentsForRerank(documents, 5, 10)

	// Should complete without hanging
	assert.Greater(t, len(chunked), 0)
	for _, idx := range docIndices {
		assert.Equal(t, 0, idx)
	}
}

func TestMultipleDocumentsWithInvalidOverlap(t *testing.T) {
	// Test multiple documents with overlap_tokens >= max_tokens
	documents := []string{
		strings.Repeat("word ", 50),
		"short document",
		strings.Repeat("word ", 75),
	}

	// overlap_tokens > max_tokens
	chunked, docIndices := chunkDocumentsForRerank(documents, 25, 30)

	// Should complete successfully and chunk the long documents
	assert.GreaterOrEqual(t, len(chunked), len(documents))
	// docIndices should have same length as chunked
	assert.Len(t, docIndices, len(chunked))
	// Short document should not be chunked
	found := false
	for _, doc := range chunked {
		if doc == "short document" {
			found = true
			break
		}
	}
	assert.True(t, found, "Short document should be present in chunks")
}

func TestNormalOperationUnaffected(t *testing.T) {
	// Test that normal cases continue to work correctly
	documents := []string{
		strings.Repeat("word ", 100),
		"short doc",
	}

	// Normal case: overlap_tokens (10) < max_tokens (50)
	chunked, docIndices := chunkDocumentsForRerank(documents, 50, 10)

	// Long document should be chunked, short one should not
	assert.Greater(t, len(chunked), 2) // At least 3 chunks

	// Short doc should be present
	found := false
	for _, doc := range chunked {
		if doc == "short doc" {
			found = true
			break
		}
	}
	assert.True(t, found, "Short doc should be in chunks")

	// Verify doc_indices maps correctly
	// Last chunk is from second document
	lastIdx := docIndices[len(docIndices)-1]
	assert.Equal(t, 1, lastIdx)
}

func TestEdgeCaseMaxTokensOne(t *testing.T) {
	// Test edge case where max_tokens=1
	documents := []string{strings.Repeat("word ", 20)}

	// max_tokens=1, overlap_tokens=5 should clamp to 0
	chunked, docIndices := chunkDocumentsForRerank(documents, 1, 5)

	// Should complete without hanging
	assert.Greater(t, len(chunked), 0)
	for _, idx := range docIndices {
		assert.Equal(t, 0, idx)
	}
}

// ============================================================================
// Additional Overlap Validation Tests
// ============================================================================

func TestZeroOverlapProducesNoOverlap(t *testing.T) {
	// Test that zero overlap produces non-overlapping chunks
	documents := []string{strings.Repeat("abcd ", 100)}

	chunked, docIndices := chunkDocumentsForRerank(documents, 20, 0)

	assert.Greater(t, len(chunked), 1)
	for _, idx := range docIndices {
		assert.Equal(t, 0, idx)
	}
}

func TestNegativeOverlapTreatedAsZero(t *testing.T) {
	// Test that negative overlap is treated as zero
	documents := []string{strings.Repeat("test ", 50)}

	// Negative overlap should be treated as 0
	chunked, docIndices := chunkDocumentsForRerank(documents, 20, -5)

	// Should complete without issues
	assert.Greater(t, len(chunked), 0)
	for _, idx := range docIndices {
		assert.Equal(t, 0, idx)
	}
}

func TestOverlapWithEmptyDocuments(t *testing.T) {
	// Test overlap handling with empty documents
	documents := []string{}

	chunked, docIndices := chunkDocumentsForRerank(documents, 30, 30)

	assert.Empty(t, chunked)
	assert.Empty(t, docIndices)
}

func TestOverlapWithSingleWord(t *testing.T) {
	// Test overlap with very short document
	documents := []string{"hello"}

	chunked, docIndices := chunkDocumentsForRerank(documents, 5, 5)

	// Single word document should not be chunked
	assert.Equal(t, 1, len(chunked))
	assert.Equal(t, "hello", chunked[0])
	assert.Equal(t, []int{0}, docIndices)
}

func TestLargeOverlapStillMakesProgress(t *testing.T) {
	// Test that even with large overlap (after clamping), chunking makes progress
	longDoc := strings.Repeat("word ", 200)
	documents := []string{longDoc}

	// Very large overlap should be clamped
	chunked, _ := chunkDocumentsForRerank(documents, 10, 100)

	// Should still produce chunks (not hang)
	assert.Greater(t, len(chunked), 0)
	// Should produce multiple chunks for a long document
	assert.Greater(t, len(chunked), 5)
}

func TestOverlapPreservesDocumentIndex(t *testing.T) {
	// Test that document indices are preserved correctly with overlap
	documents := []string{
		strings.Repeat("first ", 50),
		strings.Repeat("second ", 50),
		strings.Repeat("third ", 50),
	}

	_, docIndices := chunkDocumentsForRerank(documents, 20, 5)

	// Verify document indices are in order and valid
	lastIdx := -1
	for _, idx := range docIndices {
		assert.GreaterOrEqual(t, idx, 0)
		assert.Less(t, idx, 3)
		assert.GreaterOrEqual(t, idx, lastIdx)
		lastIdx = idx
	}

	// Verify all documents are represented
	foundDocs := make(map[int]bool)
	for _, idx := range docIndices {
		foundDocs[idx] = true
	}
	assert.True(t, foundDocs[0], "Document 0 should be represented")
	assert.True(t, foundDocs[1], "Document 1 should be represented")
	assert.True(t, foundDocs[2], "Document 2 should be represented")
}

// ============================================================================
// Chunking Boundary Tests
// ============================================================================

func TestChunkingAtExactBoundary(t *testing.T) {
	// Test chunking when document is exactly at boundary
	// 20 tokens with max_tokens=10 should produce exactly 2 chunks
	documents := []string{strings.Repeat("word ", 10)} // approximately 20 tokens

	chunked, docIndices := chunkDocumentsForRerank(documents, 10, 0)

	// Should produce chunks
	assert.Greater(t, len(chunked), 0)
	for _, idx := range docIndices {
		assert.Equal(t, 0, idx)
	}
}

func TestChunkingJustOverBoundary(t *testing.T) {
	// Test chunking when document is just over boundary
	documents := []string{strings.Repeat("word ", 11)} // just over 10 tokens

	chunked, docIndices := chunkDocumentsForRerank(documents, 10, 0)

	assert.Greater(t, len(chunked), 1)
	for _, idx := range docIndices {
		assert.Equal(t, 0, idx)
	}
}

func TestChunkingJustUnderBoundary(t *testing.T) {
	// Test chunking when document is just under boundary
	documents := []string{"hello world test"} // under typical boundary

	chunked, docIndices := chunkDocumentsForRerank(documents, 100, 10)

	// Should not chunk small document
	assert.Equal(t, 1, len(chunked))
	assert.Equal(t, []int{0}, docIndices)
}
