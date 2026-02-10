package coreconstants

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServerSettings(t *testing.T) {
	assert.Equal(t, 2, DefaultWorkers)
	assert.Equal(t, 1000, DefaultMaxGraphNodes)
}

func TestExtractionSettings(t *testing.T) {
	assert.Equal(t, "English", DefaultSummaryLanguage)
	assert.Equal(t, 1, DefaultMaxGleaning)
	assert.Equal(t, 256, DefaultEntityNameMaxLength)
}

func TestDefaultEntityTypes(t *testing.T) {
	assert.Len(t, DefaultEntityTypes, 11)
	assert.Contains(t, DefaultEntityTypes, "Person")
	assert.Contains(t, DefaultEntityTypes, "Organization")
	assert.Contains(t, DefaultEntityTypes, "Location")
	assert.Contains(t, DefaultEntityTypes, "Event")
	assert.Contains(t, DefaultEntityTypes, "Concept")
}

func TestSummaryConfiguration(t *testing.T) {
	assert.Equal(t, 8, DefaultForceLLMSummaryOnMerge)
	assert.Equal(t, 1200, DefaultSummaryMaxTokens)
	assert.Equal(t, 600, DefaultSummaryLengthRecommended)
	assert.Equal(t, 12000, DefaultSummaryContextSize)
	assert.Equal(t, 20480, DefaultMaxExtractInputTokens)
}

func TestGraphFieldSep(t *testing.T) {
	assert.Equal(t, "<SEP>", GraphFieldSep)
}

func TestQueryConfiguration(t *testing.T) {
	assert.Equal(t, 40, DefaultTopK)
	assert.Equal(t, 20, DefaultChunkTopK)
	assert.Equal(t, 6000, DefaultMaxEntityTokens)
	assert.Equal(t, 8000, DefaultMaxRelationTokens)
	assert.Equal(t, 30000, DefaultMaxTotalTokens)
	assert.Equal(t, 0.2, DefaultCosineThreshold)
	assert.Equal(t, 5, DefaultRelatedChunkNumber)
	assert.Equal(t, "VECTOR", DefaultKGChunkPickMethod)
}

func TestRerankConfiguration(t *testing.T) {
	assert.Equal(t, 0.0, DefaultMinRerankScore)
	assert.Equal(t, "null", DefaultRerankBinding)
}

func TestSourceIDsConfiguration(t *testing.T) {
	assert.Equal(t, 300, DefaultMaxSourceIDsPerEntity)
	assert.Equal(t, 300, DefaultMaxSourceIDsPerRelation)
	assert.Equal(t, "KEEP", SourceIDsLimitMethodKeep)
	assert.Equal(t, "FIFO", SourceIDsLimitMethodFIFO)
	assert.Equal(t, SourceIDsLimitMethodFIFO, DefaultSourceIDsLimitMethod)
}

func TestValidSourceIDsLimitMethods(t *testing.T) {
	assert.Len(t, ValidSourceIDsLimitMethods, 2)
	_, ok := ValidSourceIDsLimitMethods[SourceIDsLimitMethodKeep]
	assert.True(t, ok)
	_, ok = ValidSourceIDsLimitMethods[SourceIDsLimitMethodFIFO]
	assert.True(t, ok)
}

func TestIsValidSourceIDsLimitMethod(t *testing.T) {
	assert.True(t, IsValidSourceIDsLimitMethod("KEEP"))
	assert.True(t, IsValidSourceIDsLimitMethod("FIFO"))
	assert.False(t, IsValidSourceIDsLimitMethod("INVALID"))
	assert.False(t, IsValidSourceIDsLimitMethod(""))
}

func TestNormalizeSourceIDsLimitMethod(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"KEEP", "KEEP"},
		{"FIFO", "FIFO"},
		{"", DefaultSourceIDsLimitMethod},
		{"invalid", DefaultSourceIDsLimitMethod},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeSourceIDsLimitMethod(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilePathConfiguration(t *testing.T) {
	assert.Equal(t, 100, DefaultMaxFilePaths)
	assert.Equal(t, 32768, DefaultMaxFilePathLength)
	assert.Equal(t, "truncated", DefaultFilePathMorePlaceholder)
}

func TestAsyncConfiguration(t *testing.T) {
	assert.Equal(t, 4, DefaultMaxAsync)
	assert.Equal(t, 2, DefaultMaxParallelInsert)
}

func TestEmbeddingConfiguration(t *testing.T) {
	assert.Equal(t, 8, DefaultEmbeddingFuncMaxAsync)
	assert.Equal(t, 10, DefaultEmbeddingBatchNum)
}

func TestTimeoutConfiguration(t *testing.T) {
	assert.Equal(t, 300*time.Second, DefaultTimeout)
	assert.Equal(t, 180*time.Second, DefaultLLMTimeout)
	assert.Equal(t, 30*time.Second, DefaultEmbeddingTimeout)
}

func TestLoggingConfiguration(t *testing.T) {
	assert.Equal(t, 10485760, DefaultLogMaxBytes)
	assert.Equal(t, 5, DefaultLogBackupCount)
	assert.Equal(t, "lightrag.log", DefaultLogFilename)
}

func TestOllamaConfiguration(t *testing.T) {
	assert.Equal(t, "lightrag", DefaultOllamaModelName)
	assert.Equal(t, "latest", DefaultOllamaModelTag)
	assert.Equal(t, int64(7365960935), int64(DefaultOllamaModelSize))
	assert.Equal(t, "2024-01-15T00:00:00Z", DefaultOllamaCreatedAt)
	assert.Equal(t, "sha256:lightrag", DefaultOllamaDigest)
}

func TestChunkConfiguration(t *testing.T) {
	assert.Equal(t, 1200, DefaultChunkSize)
	assert.Equal(t, 100, DefaultChunkOverlapSize)
}
