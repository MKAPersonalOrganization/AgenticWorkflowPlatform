package operate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DummyTokenizer implements 1:1 character-to-token mapping for testing
type DummyTokenizer struct{}

func NewDummyTokenizer() *DummyTokenizer {
	return &DummyTokenizer{}
}

func (t *DummyTokenizer) Encode(content string) []int {
	tokens := make([]int, len(content))
	for i, ch := range content {
		tokens[i] = int(ch)
	}
	return tokens
}

func (t *DummyTokenizer) Decode(tokens []int) string {
	result := make([]byte, len(tokens))
	for i, token := range tokens {
		result[i] = byte(token)
	}
	return string(result)
}

// MultiTokenCharacterTokenizer where character-to-token ratio is non-uniform.
// - Uppercase letters: 2 tokens each
// - Punctuation (!, ?, .): 3 tokens each
// - Other characters: 1 token each
type MultiTokenCharacterTokenizer struct{}

func NewMultiTokenCharacterTokenizer() *MultiTokenCharacterTokenizer {
	return &MultiTokenCharacterTokenizer{}
}

func (t *MultiTokenCharacterTokenizer) Encode(content string) []int {
	var tokens []int
	for _, ch := range content {
		if ch >= 'A' && ch <= 'Z' {
			// Uppercase = 2 tokens
			tokens = append(tokens, int(ch), int(ch)+1000)
		} else if ch == '!' || ch == '?' || ch == '.' {
			// Punctuation = 3 tokens
			tokens = append(tokens, int(ch), int(ch)+2000, int(ch)+3000)
		} else {
			// Regular chars = 1 token
			tokens = append(tokens, int(ch))
		}
	}
	return tokens
}

func (t *MultiTokenCharacterTokenizer) Decode(tokens []int) string {
	var result []rune
	i := 0
	for i < len(tokens) {
		baseToken := tokens[i]
		// Check if this is part of a multi-token sequence
		if i+2 < len(tokens) && tokens[i+1] == baseToken+2000 && tokens[i+2] == baseToken+3000 {
			// 3-token punctuation
			result = append(result, rune(baseToken))
			i += 3
		} else if i+1 < len(tokens) && tokens[i+1] == baseToken+1000 {
			// 2-token uppercase
			result = append(result, rune(baseToken))
			i += 2
		} else {
			// Single token
			result = append(result, rune(baseToken))
			i++
		}
	}
	return string(result)
}

// ============================================================================
// Tests for split_by_character_only=true (raises error on oversized chunks)
// ============================================================================

func TestSplitByCharacterOnlyWithinLimit(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		"alpha\n\nbeta",
		&sep,
		true, // split_by_character_only
		0,
		10,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 2)
	assert.Equal(t, "alpha", chunks[0].Content)
	assert.Equal(t, "beta", chunks[1].Content)
}

func TestSplitByCharacterOnlyExceedingLimitRaises(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"
	oversized := "aaaaaaaaaaaa" // 12 chars

	_, err := ChunkingByTokenSize(
		tokenizer,
		oversized,
		&sep,
		true, // split_by_character_only
		0,
		5, // limit
	)

	require.Error(t, err)
	chunkErr, ok := err.(*ChunkTokenLimitExceededError)
	require.True(t, ok)
	assert.Equal(t, 12, chunkErr.ChunkTokens)
	assert.Equal(t, 5, chunkErr.ChunkTokenLimit)
}

func TestChunkErrorIncludesPreview(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"
	oversized := ""
	for i := 0; i < 100; i++ {
		oversized += "x"
	}

	_, err := ChunkingByTokenSize(
		tokenizer,
		oversized,
		&sep,
		true,
		0,
		10,
	)

	require.Error(t, err)
	chunkErr, ok := err.(*ChunkTokenLimitExceededError)
	require.True(t, ok)
	// Preview should be truncated
	assert.True(t, len(chunkErr.ChunkPreview) <= 120)
}

func TestSplitByCharacterOnlyAtExactLimit(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"
	exactSize := "aaaaaaaaaa" // 10 chars

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		exactSize,
		&sep,
		true,
		0,
		10,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Equal(t, exactSize, chunks[0].Content)
	assert.Equal(t, 10, chunks[0].Tokens)
}

func TestSplitByCharacterOnlyOneOverLimit(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"
	oneOver := "aaaaaaaaaaa" // 11 chars

	_, err := ChunkingByTokenSize(
		tokenizer,
		oneOver,
		&sep,
		true,
		0,
		10,
	)

	require.Error(t, err)
	chunkErr, ok := err.(*ChunkTokenLimitExceededError)
	require.True(t, ok)
	assert.Equal(t, 11, chunkErr.ChunkTokens)
	assert.Equal(t, 10, chunkErr.ChunkTokenLimit)
}

// ============================================================================
// Tests for split_by_character_only=false (recursive splitting)
// ============================================================================

func TestSplitRecursiveOversizedChunk(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"
	// 30 chars - should split into chunks of size 10
	oversized := ""
	for i := 0; i < 30; i++ {
		oversized += "a"
	}

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		oversized,
		&sep,
		false, // recursive splitting
		0,
		10,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 3)
	for _, chunk := range chunks {
		assert.Equal(t, 10, chunk.Tokens)
	}
}

func TestSplitWithChunkOverlap(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"
	// Each character is unique - enables exact position verification
	content := "0123456789abcdefghijklmno" // 25 chars

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		false,
		3,  // overlap
		10, // chunk size
	)

	require.NoError(t, err)
	// With overlap=3, step size = chunk_size - overlap = 10 - 3 = 7
	// Chunks start at positions: 0, 7, 14, 21
	assert.Len(t, chunks, 4)

	// Verify exact content and token counts
	assert.Equal(t, 10, chunks[0].Tokens)
	assert.Equal(t, "0123456789", chunks[0].Content)

	assert.Equal(t, 10, chunks[1].Tokens)
	assert.Equal(t, "789abcdefg", chunks[1].Content) // overlaps with "789"

	assert.Equal(t, 10, chunks[2].Tokens)
	assert.Equal(t, "efghijklmn", chunks[2].Content) // overlaps with "efg"

	assert.Equal(t, 4, chunks[3].Tokens)
	assert.Equal(t, "lmno", chunks[3].Content) // overlaps with "lmn"
}

func TestSplitExactBoundary(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"
	// Exactly 20 chars, should split into 2 chunks of 10
	content := ""
	for i := 0; i < 20; i++ {
		content += "a"
	}

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		false,
		0,
		10,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 2)
	assert.Equal(t, 10, chunks[0].Tokens)
	assert.Equal(t, 10, chunks[1].Tokens)
}

func TestSplitVeryLargeText(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"
	// 100 chars should create 10 chunks with chunk_size=10, overlap=0
	content := ""
	for i := 0; i < 100; i++ {
		content += "a"
	}

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		false,
		0,
		10,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 10)
	for _, chunk := range chunks {
		assert.Equal(t, 10, chunk.Tokens)
	}
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestEmptyContent(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		"",
		&sep,
		true,
		0,
		10,
	)

	require.NoError(t, err)
	// Empty content with split_by_character returns one chunk with empty content
	// This matches Python behavior: strings.Split("", sep) returns [""], which becomes one empty chunk
	assert.Len(t, chunks, 1)
	assert.Equal(t, 0, chunks[0].Tokens)
	assert.Equal(t, "", chunks[0].Content)
}

func TestSingleCharacter(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		"a",
		&sep,
		true,
		0,
		10,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "a", chunks[0].Content)
	assert.Equal(t, 1, chunks[0].Tokens)
}

func TestNoDelimiterInContent(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n" // Delimiter not in content
	content := ""
	for i := 0; i < 30; i++ {
		content += "a"
	}

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		false,
		0,
		10,
	)

	require.NoError(t, err)
	// Should still split based on token size
	assert.Len(t, chunks, 3)
	for _, chunk := range chunks {
		assert.Equal(t, 10, chunk.Tokens)
	}
}

func TestNoSplitCharacter(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	content := ""
	for i := 0; i < 30; i++ {
		content += "a"
	}

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		nil, // no split character
		false,
		0,
		10,
	)

	require.NoError(t, err)
	// Should split based purely on token size
	assert.Len(t, chunks, 3)
	for _, chunk := range chunks {
		assert.Equal(t, 10, chunk.Tokens)
	}
}

// ============================================================================
// Parameter Combinations
// ============================================================================

func TestDifferentDelimiterNewline(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		"alpha\nbeta\ngamma",
		&sep,
		true,
		0,
		10,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 3)
	assert.Equal(t, "alpha", chunks[0].Content)
	assert.Equal(t, "beta", chunks[1].Content)
	assert.Equal(t, "gamma", chunks[2].Content)
}

func TestDelimiterBasedSplittingVerification(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "||"

	// Content with clear delimiter boundaries
	content := "part1||part2||part3||part4"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		true,
		0,
		20,
	)

	require.NoError(t, err)
	// Should split exactly at || delimiters
	assert.Len(t, chunks, 4)
	assert.Equal(t, "part1", chunks[0].Content)
	assert.Equal(t, "part2", chunks[1].Content)
	assert.Equal(t, "part3", chunks[2].Content)
	assert.Equal(t, "part4", chunks[3].Content)
}

func TestMultiCharacterDelimiterSplitting(t *testing.T) {
	tokenizer := NewDummyTokenizer()

	// Test 1: Multi-character delimiter
	sep := "<SEP>"
	content := "data<SEP>more<SEP>final"
	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		true,
		0,
		50,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 3)
	assert.Equal(t, "data", chunks[0].Content)
	assert.Equal(t, "more", chunks[1].Content)
	assert.Equal(t, "final", chunks[2].Content)
}

func TestDelimiterPartialMatchNotSplit(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "||" // Only split on double pipe

	// Content contains "||" delimiter but also contains single "|"
	content := "data|single||data|with|pipes||final"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		true,
		0,
		50,
	)

	require.NoError(t, err)
	// Should split only at "||", not at single "|"
	assert.Len(t, chunks, 3)
	assert.Equal(t, "data|single", chunks[0].Content)
	assert.Equal(t, "data|with|pipes", chunks[1].Content)
	assert.Equal(t, "final", chunks[2].Content)
}

func TestDifferentDelimiterComma(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := ","

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		"one,two,three",
		&sep,
		true,
		0,
		10,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 3)
	assert.Equal(t, "one", chunks[0].Content)
	assert.Equal(t, "two", chunks[1].Content)
	assert.Equal(t, "three", chunks[2].Content)
}

func TestZeroOverlap(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	content := ""
	for i := 0; i < 20; i++ {
		content += "a"
	}

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		nil,
		false,
		0, // zero overlap
		10,
	)

	require.NoError(t, err)
	// Should create exactly 2 chunks with no overlap
	assert.Len(t, chunks, 2)
	assert.Equal(t, 10, chunks[0].Tokens)
	assert.Equal(t, 10, chunks[1].Tokens)
}

func TestLargeOverlap(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	// Use distinctive characters to verify exact positions
	content := "0123456789abcdefghijklmnopqrst" // 30 chars

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		nil,
		false,
		9,  // large overlap
		10, // chunk size
	)

	require.NoError(t, err)
	// With overlap=9, step size = 10 - 9 = 1
	// Chunks start at: 0, 1, 2, ..., 29
	assert.Len(t, chunks, 30)

	// Verify first few chunks have correct content with proper overlap
	assert.Equal(t, "0123456789", chunks[0].Content)
	assert.Equal(t, "123456789a", chunks[1].Content)
	assert.Equal(t, "23456789ab", chunks[2].Content)
	assert.Equal(t, "3456789abc", chunks[3].Content)

	// Verify last chunk
	assert.Equal(t, "t", chunks[29].Content)
}

// ============================================================================
// Chunk Order Index Tests
// ============================================================================

func TestChunkOrderIndexSimple(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	sep := "\n\n"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		"a\n\nb\n\nc",
		&sep,
		true,
		0,
		10,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 3)
	assert.Equal(t, 0, chunks[0].ChunkOrderIndex)
	assert.Equal(t, 1, chunks[1].ChunkOrderIndex)
	assert.Equal(t, 2, chunks[2].ChunkOrderIndex)
}

func TestChunkOrderIndexWithSplitting(t *testing.T) {
	tokenizer := NewDummyTokenizer()
	content := ""
	for i := 0; i < 30; i++ {
		content += "a"
	}

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		nil,
		false,
		0,
		10,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 3)
	assert.Equal(t, 0, chunks[0].ChunkOrderIndex)
	assert.Equal(t, 1, chunks[1].ChunkOrderIndex)
	assert.Equal(t, 2, chunks[2].ChunkOrderIndex)
}

// ============================================================================
// Token vs Character Counting Tests (Multi-Token Characters)
// ============================================================================

func TestTokenCountingNotCharacterCounting(t *testing.T) {
	tokenizer := NewMultiTokenCharacterTokenizer()
	sep := "\n\n"

	// "aXa" = 3 characters, 4 tokens (a=1, X=2, a=1)
	content := "aXa"

	_, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		true,
		0,
		3, // 3 token limit - should fail because we have 4 tokens
	)

	require.Error(t, err)
	chunkErr, ok := err.(*ChunkTokenLimitExceededError)
	require.True(t, ok)
	assert.Equal(t, 4, chunkErr.ChunkTokens) // Should be 4 tokens, not 3 characters
	assert.Equal(t, 3, chunkErr.ChunkTokenLimit)
}

func TestTokenLimitWithPunctuation(t *testing.T) {
	tokenizer := NewMultiTokenCharacterTokenizer()
	sep := "\n\n"

	// "Hi!" = 3 characters, 6 tokens (H=2, i=1, !=3)
	content := "Hi!"

	_, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		true,
		0,
		4,
	)

	require.Error(t, err)
	chunkErr, ok := err.(*ChunkTokenLimitExceededError)
	require.True(t, ok)
	assert.Equal(t, 6, chunkErr.ChunkTokens)
	assert.Equal(t, 4, chunkErr.ChunkTokenLimit)
}

func TestMultiTokenWithinLimit(t *testing.T) {
	tokenizer := NewMultiTokenCharacterTokenizer()
	sep := "\n\n"

	// "Hi" = 2 chars, 3 tokens (H=2, i=1)
	content := "Hi"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		true,
		0,
		5,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Equal(t, 3, chunks[0].Tokens)
	assert.Equal(t, "Hi", chunks[0].Content)
}

func TestRecursiveSplitWithMultiTokenChars(t *testing.T) {
	tokenizer := NewMultiTokenCharacterTokenizer()
	sep := "\n\n"

	// "AAAAA" = 5 characters, 10 tokens (each A = 2 tokens)
	content := "AAAAA"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		false,
		0,
		6,
	)

	require.NoError(t, err)
	// Should split into 2 chunks based on tokens, not characters
	assert.Len(t, chunks, 2)
	assert.Equal(t, 6, chunks[0].Tokens)
	assert.Equal(t, 4, chunks[1].Tokens)
}

func TestOverlapUsesTokenCount(t *testing.T) {
	tokenizer := NewMultiTokenCharacterTokenizer()

	// "aAaAa" = 5 characters, 7 tokens (a=1, A=2, a=1, A=2, a=1)
	content := "aAaAa"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		nil,
		false,
		2, // overlap
		4, // chunk size
	)

	require.NoError(t, err)
	// Chunks start at token positions: 0, 2, 4, 6
	assert.Len(t, chunks, 4)
	assert.Equal(t, 4, chunks[0].Tokens)
	assert.Equal(t, 4, chunks[1].Tokens)
	assert.Equal(t, 3, chunks[2].Tokens)
	assert.Equal(t, 1, chunks[3].Tokens)
}

func TestMixedMultiTokenContent(t *testing.T) {
	tokenizer := NewMultiTokenCharacterTokenizer()
	sep := "\n\n"

	// "hello\n\nWORLD!" = 12 chars
	// hello = 5 tokens, WORLD = 10 tokens (5 chars × 2), ! = 3 tokens
	// Total = 18 tokens
	content := "hello\n\nWORLD!"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		true,
		0,
		20,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 2)
	assert.Equal(t, "hello", chunks[0].Content)
	assert.Equal(t, 5, chunks[0].Tokens)
	assert.Equal(t, "WORLD!", chunks[1].Content)
	assert.Equal(t, 13, chunks[1].Tokens) // 10 + 3
}

func TestExactTokenBoundaryMultiToken(t *testing.T) {
	tokenizer := NewMultiTokenCharacterTokenizer()
	sep := "\n\n"

	// "AAA" = 3 chars, 6 tokens (each A = 2 tokens)
	content := "AAA"

	chunks, err := ChunkingByTokenSize(
		tokenizer,
		content,
		&sep,
		true,
		0,
		6,
	)

	require.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Equal(t, 6, chunks[0].Tokens)
	assert.Equal(t, "AAA", chunks[0].Content)
}

func TestDecodePreservesContent(t *testing.T) {
	tokenizer := NewMultiTokenCharacterTokenizer()

	testStrings := []string{
		"Hello",
		"WORLD",
		"Test!",
		"Mixed?Case.",
		"ABC123xyz",
	}

	for _, original := range testStrings {
		tokens := tokenizer.Encode(original)
		decoded := tokenizer.Decode(tokens)
		assert.Equal(t, original, decoded, "Failed to decode: %s", original)
	}
}
