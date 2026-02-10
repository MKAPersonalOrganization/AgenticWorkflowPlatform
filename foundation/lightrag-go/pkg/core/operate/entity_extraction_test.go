package operate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Entity Extraction Tests
// ============================================================================

func TestExtractEntitiesBasicFormat(t *testing.T) {
	// Test basic entity extraction with standard format
	input := `entity<|#|>John Doe<|#|>Person<|#|>A software engineer from San Francisco
entity<|#|>Apple Inc<|#|>Organization<|#|>A technology company
<|COMPLETE|>`

	result, err := ProcessExtractionResult(input, "chunk-1", 1234567890, "test.txt", "<|#|>", "<|COMPLETE|>")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Entities, 2)
}

func TestExtractEntitiesWithRelationships(t *testing.T) {
	// Test extraction with both entities and relationships
	input := `entity<|#|>John Doe<|#|>Person<|#|>A software engineer
entity<|#|>Apple Inc<|#|>Organization<|#|>A technology company
relation<|#|>John Doe<|#|>Apple Inc<|#|>works_at<|#|>John works at Apple
<|COMPLETE|>`

	result, err := ProcessExtractionResult(input, "chunk-1", 1234567890, "test.txt", "<|#|>", "<|COMPLETE|>")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Entities)
	assert.NotEmpty(t, result.Relationships)
}

func TestExtractEntitiesMissingCompletionDelimiter(t *testing.T) {
	// Test extraction when completion delimiter is missing
	input := `entity<|#|>Test<|#|>Person<|#|>Description`

	result, err := ProcessExtractionResult(input, "chunk-1", 1234567890, "test.txt", "<|#|>", "<|COMPLETE|>")

	// Should not error, just warn
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestExtractEntitiesEmptyInput(t *testing.T) {
	// Test with empty input
	result, err := ProcessExtractionResult("", "chunk-1", 1234567890, "test.txt", "<|#|>", "<|COMPLETE|>")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Entities)
	assert.Empty(t, result.Relationships)
}

func TestHandleSingleEntityExtraction_ValidEntity(t *testing.T) {
	attributes := []string{"entity", "John Doe", "Person", "A software engineer from San Francisco"}
	result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")

	require.NotNil(t, result)
	assert.Equal(t, "John Doe", result.EntityName)
	assert.Equal(t, "person", result.EntityType)
	assert.NotEmpty(t, result.Description)
	assert.Equal(t, "chunk-1", result.SourceID)
}

func TestHandleSingleEntityExtraction_WrongFieldCount(t *testing.T) {
	// Too few fields
	attributes := []string{"entity", "John Doe", "Person"}
	result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")
	assert.Nil(t, result)
}

func TestHandleSingleEntityExtraction_EmptyName(t *testing.T) {
	attributes := []string{"entity", "", "Person", "Description"}
	result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")
	assert.Nil(t, result)
}

func TestHandleSingleEntityExtraction_NoEntityMarker(t *testing.T) {
	attributes := []string{"other", "John Doe", "Person", "Description"}
	result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")
	assert.Nil(t, result)
}

func TestHandleSingleEntityExtraction_WhitespaceOnlyName(t *testing.T) {
	attributes := []string{"entity", "   ", "Person", "Description"}
	result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")
	assert.Nil(t, result)
}

func TestHandleSingleEntityExtraction_SpecialCharactersInName(t *testing.T) {
	attributes := []string{"entity", "O'Brien-Smith", "Person", "A person with special chars in name"}
	result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")

	require.NotNil(t, result)
	assert.Equal(t, "O'Brien-Smith", result.EntityName)
}

func TestHandleSingleEntityExtraction_UnicodeInName(t *testing.T) {
	attributes := []string{"entity", "北京市", "Location", "Capital of China"}
	result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")

	require.NotNil(t, result)
	assert.Equal(t, "北京市", result.EntityName)
}

func TestHandleSingleEntityExtraction_TypeNormalization(t *testing.T) {
	// Entity type should be normalized to lowercase
	attributes := []string{"entity", "Test", "ORGANIZATION", "Description"}
	result := HandleSingleEntityExtraction(attributes, "chunk-1", 1234567890, "test.txt")

	require.NotNil(t, result)
	assert.Equal(t, "organization", result.EntityType)
}

// ============================================================================
// Relationship Extraction Tests
// ============================================================================

func TestHandleSingleRelationshipExtraction_ValidRelationship(t *testing.T) {
	attributes := []string{"relation", "John", "Apple", "works_at", "John works at Apple Inc."}
	result := HandleSingleRelationshipExtraction(attributes, "chunk-1", 1234567890, "test.txt")

	require.NotNil(t, result)
	assert.Equal(t, "John", result.SrcID)
	assert.Equal(t, "Apple", result.TgtID)
	assert.NotEmpty(t, result.Description)
	assert.Equal(t, 1.0, result.Weight)
}

func TestHandleSingleRelationshipExtraction_RelationshipMarker(t *testing.T) {
	// Test with "relationship" instead of "relation"
	attributes := []string{"relationship", "John", "Apple", "works_at", "John works at Apple"}
	result := HandleSingleRelationshipExtraction(attributes, "chunk-1", 1234567890, "test.txt")

	require.NotNil(t, result)
	assert.Equal(t, "John", result.SrcID)
}

func TestHandleSingleRelationshipExtraction_WrongFieldCount(t *testing.T) {
	attributes := []string{"relation", "John", "Apple", "works_at"}
	result := HandleSingleRelationshipExtraction(attributes, "chunk-1", 1234567890, "test.txt")
	assert.Nil(t, result)
}

func TestHandleSingleRelationshipExtraction_SelfRelationshipRejected(t *testing.T) {
	// Self-relationships should be rejected
	attributes := []string{"relation", "John", "John", "self", "Self reference"}
	result := HandleSingleRelationshipExtraction(attributes, "chunk-1", 1234567890, "test.txt")
	assert.Nil(t, result)
}

func TestHandleSingleRelationshipExtraction_EmptySource(t *testing.T) {
	attributes := []string{"relation", "", "Apple", "works_at", "Description"}
	result := HandleSingleRelationshipExtraction(attributes, "chunk-1", 1234567890, "test.txt")
	assert.Nil(t, result)
}

func TestHandleSingleRelationshipExtraction_EmptyTarget(t *testing.T) {
	attributes := []string{"relation", "John", "", "works_at", "Description"}
	result := HandleSingleRelationshipExtraction(attributes, "chunk-1", 1234567890, "test.txt")
	assert.Nil(t, result)
}

func TestHandleSingleRelationshipExtraction_CaseInsensitiveSelfCheck(t *testing.T) {
	// Self-relationship check is case-sensitive in current implementation
	// "John" != "JOHN" after sanitization, so this creates a valid relationship
	attributes := []string{"relation", "John", "JOHN", "self", "Self reference"}
	result := HandleSingleRelationshipExtraction(attributes, "chunk-1", 1234567890, "test.txt")
	// Case-sensitive comparison means these are treated as different entities
	assert.NotNil(t, result)
	assert.Equal(t, "John", result.SrcID)
	assert.Equal(t, "JOHN", result.TgtID)
}

// ============================================================================
// Relation Chunk Key Tests
// ============================================================================

func TestMakeRelationChunkKey_Ordering(t *testing.T) {
	// Key should be sorted alphabetically
	tests := []struct {
		src      string
		tgt      string
		expected string
	}{
		{"a", "b", "a||b"},
		{"b", "a", "a||b"}, // Should be sorted
		{"John", "Apple", "Apple||John"},
		{"Zebra", "Apple", "Apple||Zebra"},
	}

	for _, tt := range tests {
		t.Run(tt.src+"_"+tt.tgt, func(t *testing.T) {
			result := MakeRelationChunkKey(tt.src, tt.tgt)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMakeRelationChunkKey_Consistency(t *testing.T) {
	// Same pair should always produce same key regardless of order
	key1 := MakeRelationChunkKey("Entity1", "Entity2")
	key2 := MakeRelationChunkKey("Entity2", "Entity1")
	assert.Equal(t, key1, key2)
}

// ============================================================================
// Source IDs Limit Tests
// ============================================================================

func TestApplySourceIDsLimit_NoLimitNeeded(t *testing.T) {
	ids := []string{"a", "b", "c"}
	result := ApplySourceIDsLimit(ids, 5, "KEEP", "test")
	assert.Equal(t, ids, result)
}

func TestApplySourceIDsLimit_KEEPMethod(t *testing.T) {
	// KEEP method should keep first N items
	ids := []string{"a", "b", "c", "d", "e"}
	result := ApplySourceIDsLimit(ids, 3, "KEEP", "test")
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestApplySourceIDsLimit_FIFOMethod(t *testing.T) {
	// FIFO method should keep last N items
	ids := []string{"a", "b", "c", "d", "e"}
	result := ApplySourceIDsLimit(ids, 3, "FIFO", "test")
	assert.Equal(t, []string{"c", "d", "e"}, result)
}

func TestApplySourceIDsLimit_ZeroLimit(t *testing.T) {
	ids := []string{"a", "b", "c"}
	result := ApplySourceIDsLimit(ids, 0, "KEEP", "test")
	assert.Empty(t, result)
}

func TestApplySourceIDsLimit_EmptyInput(t *testing.T) {
	ids := []string{}
	result := ApplySourceIDsLimit(ids, 5, "KEEP", "test")
	assert.Empty(t, result)
}

// ============================================================================
// Merge Source IDs Tests
// ============================================================================

func TestMergeSourceIDs_WithDeduplication(t *testing.T) {
	existing := []string{"a", "b", "c"}
	new := []string{"b", "c", "d"}
	result := MergeSourceIDs(existing, new)

	assert.Equal(t, []string{"a", "b", "c", "d"}, result)
}

func TestMergeSourceIDs_EmptyExisting(t *testing.T) {
	result := MergeSourceIDs([]string{}, []string{"a", "b"})
	assert.Equal(t, []string{"a", "b"}, result)
}

func TestMergeSourceIDs_EmptyNew(t *testing.T) {
	result := MergeSourceIDs([]string{"a", "b"}, []string{})
	assert.Equal(t, []string{"a", "b"}, result)
}

func TestMergeSourceIDs_BothEmpty(t *testing.T) {
	result := MergeSourceIDs([]string{}, []string{})
	assert.Empty(t, result)
}

func TestMergeSourceIDs_NoDuplicates(t *testing.T) {
	existing := []string{"a", "b"}
	new := []string{"c", "d"}
	result := MergeSourceIDs(existing, new)

	assert.Equal(t, []string{"a", "b", "c", "d"}, result)
}

// ============================================================================
// Entity/Relationship Data Structure Tests
// ============================================================================

func TestEntityData_Fields(t *testing.T) {
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
	assert.Equal(t, "A test entity", entity.Description)
	assert.Equal(t, "chunk-1", entity.SourceID)
	assert.Equal(t, "test.txt", entity.FilePath)
	assert.Equal(t, int64(1234567890), entity.Timestamp)
}

func TestRelationshipData_Fields(t *testing.T) {
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
	assert.Equal(t, "A relationship", rel.Description)
	assert.Equal(t, "related, connected", rel.Keywords)
}

// ============================================================================
// Extraction Result Processing Tests
// ============================================================================

func TestProcessExtractionResult_MultipleEntities(t *testing.T) {
	input := `entity<|#|>Person1<|#|>Person<|#|>First person
entity<|#|>Person2<|#|>Person<|#|>Second person
entity<|#|>Company1<|#|>Organization<|#|>A company
<|COMPLETE|>`

	result, err := ProcessExtractionResult(input, "chunk-1", 1234567890, "test.txt", "<|#|>", "<|COMPLETE|>")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Entities, 3)
}

func TestProcessExtractionResult_MultipleRelationships(t *testing.T) {
	input := `entity<|#|>A<|#|>Person<|#|>Person A
entity<|#|>B<|#|>Person<|#|>Person B
entity<|#|>C<|#|>Person<|#|>Person C
relation<|#|>A<|#|>B<|#|>knows<|#|>A knows B
relation<|#|>B<|#|>C<|#|>knows<|#|>B knows C
relation<|#|>A<|#|>C<|#|>knows<|#|>A knows C
<|COMPLETE|>`

	result, err := ProcessExtractionResult(input, "chunk-1", 1234567890, "test.txt", "<|#|>", "<|COMPLETE|>")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Entities, 3)
	assert.Len(t, result.Relationships, 3)
}

func TestProcessExtractionResult_WithWhitespace(t *testing.T) {
	input := `
   entity<|#|>Test Entity<|#|>Person<|#|>Description
   
relation<|#|>A<|#|>B<|#|>knows<|#|>A knows B
   
<|COMPLETE|>`

	result, err := ProcessExtractionResult(input, "chunk-1", 1234567890, "test.txt", "<|#|>", "<|COMPLETE|>")

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestProcessExtractionResult_InvalidLines(t *testing.T) {
	// Input with some invalid lines that should be skipped
	input := `entity<|#|>Valid<|#|>Person<|#|>Valid entity
this is not a valid line
entity<|#|><|#|>Person<|#|>Empty name - should be skipped
entity<|#|>AnotherValid<|#|>Person<|#|>Another valid entity
<|COMPLETE|>`

	result, err := ProcessExtractionResult(input, "chunk-1", 1234567890, "test.txt", "<|#|>", "<|COMPLETE|>")

	require.NoError(t, err)
	require.NotNil(t, result)
	// Only valid entities should be extracted
	assert.Len(t, result.Entities, 2)
}
