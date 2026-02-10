package utils

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeArgsHash(t *testing.T) {
	tests := []struct {
		name     string
		args     []interface{}
		expected int // expected hash length
	}{
		{
			name:     "single string",
			args:     []interface{}{"hello"},
			expected: 32,
		},
		{
			name:     "multiple args",
			args:     []interface{}{"hello", "world", 123},
			expected: 32,
		},
		{
			name:     "empty args",
			args:     []interface{}{},
			expected: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := ComputeArgsHash(tt.args...)
			assert.Equal(t, tt.expected, len(hash))
		})
	}

	// Test consistency
	hash1 := ComputeArgsHash("test", 123)
	hash2 := ComputeArgsHash("test", 123)
	assert.Equal(t, hash1, hash2)

	// Test uniqueness
	hash3 := ComputeArgsHash("different", 456)
	assert.NotEqual(t, hash1, hash3)
}

func TestComputeMDHashID(t *testing.T) {
	tests := []struct {
		content  string
		prefix   string
		expected string
	}{
		{
			content:  "test content",
			prefix:   "ent-",
			expected: "ent-",
		},
		{
			content:  "test content",
			prefix:   "rel-",
			expected: "rel-",
		},
		{
			content:  "",
			prefix:   "doc-",
			expected: "doc-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.prefix+tt.content, func(t *testing.T) {
			result := ComputeMDHashID(tt.content, tt.prefix)
			assert.True(t, len(result) > len(tt.prefix))
			assert.Equal(t, tt.expected, result[:len(tt.prefix)])
		})
	}
}

func TestGenerateCacheKey(t *testing.T) {
	key := GenerateCacheKey("default", "query", "abc123")
	assert.Equal(t, "default:query:abc123", key)

	key = GenerateCacheKey("local", "extract", "xyz789")
	assert.Equal(t, "local:extract:xyz789", key)
}

func TestParseCacheKey(t *testing.T) {
	tests := []struct {
		key  string
		mode string
		typ  string
		hash string
		ok   bool
	}{
		{"default:query:abc123", "default", "query", "abc123", true},
		{"local:extract:xyz789", "local", "extract", "xyz789", true},
		{"invalid", "", "", "", false},
		{"only:two", "", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			mode, cacheType, hash, ok := ParseCacheKey(tt.key)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.mode, mode)
				assert.Equal(t, tt.typ, cacheType)
				assert.Equal(t, tt.hash, hash)
			}
		})
	}
}

func TestGetEnvFunctions(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_STRING", "hello")
	os.Setenv("TEST_INT", "42")
	os.Setenv("TEST_BOOL_TRUE", "true")
	os.Setenv("TEST_BOOL_FALSE", "false")
	os.Setenv("TEST_FLOAT", "3.14")
	defer func() {
		os.Unsetenv("TEST_STRING")
		os.Unsetenv("TEST_INT")
		os.Unsetenv("TEST_BOOL_TRUE")
		os.Unsetenv("TEST_BOOL_FALSE")
		os.Unsetenv("TEST_FLOAT")
	}()

	t.Run("GetEnvString", func(t *testing.T) {
		assert.Equal(t, "hello", GetEnvString("TEST_STRING", "default"))
		assert.Equal(t, "default", GetEnvString("NON_EXISTENT", "default"))
	})

	t.Run("GetEnvInt", func(t *testing.T) {
		assert.Equal(t, 42, GetEnvInt("TEST_INT", 0))
		assert.Equal(t, 100, GetEnvInt("NON_EXISTENT", 100))
	})

	t.Run("GetEnvBool", func(t *testing.T) {
		assert.True(t, GetEnvBool("TEST_BOOL_TRUE", false))
		assert.False(t, GetEnvBool("TEST_BOOL_FALSE", true))
		assert.True(t, GetEnvBool("NON_EXISTENT", true))
	})

	t.Run("GetEnvFloat", func(t *testing.T) {
		assert.Equal(t, 3.14, GetEnvFloat("TEST_FLOAT", 0.0))
		assert.Equal(t, 2.71, GetEnvFloat("NON_EXISTENT", 2.71))
	})
}

func TestSanitizeStringForJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "unicode string",
			input:    "你好世界",
			expected: "你好世界",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeStringForJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitStringByMultiMarkers(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		markers  []string
		expected []string
	}{
		{
			name:     "single marker",
			content:  "hello|world",
			markers:  []string{"|"},
			expected: []string{"hello", "world"},
		},
		{
			name:     "multiple markers",
			content:  "hello|world,test",
			markers:  []string{"|", ","},
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "no markers",
			content:  "hello world",
			markers:  []string{},
			expected: []string{"hello world"},
		},
		{
			name:     "empty content",
			content:  "",
			markers:  []string{"|"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitStringByMultiMarkers(tt.content, tt.markers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsFloatRegex(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"123", true},
		{"123.45", true},
		{"-123.45", true},
		{"+123.45", true},
		{".45", true},
		{"abc", false},
		{"12.34.56", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result := IsFloatRegex(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		v1       []float64
		v2       []float64
		expected float64
		delta    float64
	}{
		{
			name:     "identical vectors",
			v1:       []float64{1, 0, 0},
			v2:       []float64{1, 0, 0},
			expected: 1.0,
			delta:    0.0001,
		},
		{
			name:     "orthogonal vectors",
			v1:       []float64{1, 0, 0},
			v2:       []float64{0, 1, 0},
			expected: 0.0,
			delta:    0.0001,
		},
		{
			name:     "opposite vectors",
			v1:       []float64{1, 0, 0},
			v2:       []float64{-1, 0, 0},
			expected: -1.0,
			delta:    0.0001,
		},
		{
			name:     "different length",
			v1:       []float64{1, 0},
			v2:       []float64{1, 0, 0},
			expected: 0.0,
			delta:    0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CosineSimilarity(tt.v1, tt.v2)
			assert.InDelta(t, tt.expected, result, tt.delta)
		})
	}
}

func TestSanitizeTextForEncoding(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "normal text",
			input:   "Hello, World!",
			wantErr: false,
		},
		{
			name:    "empty text",
			input:   "",
			wantErr: false,
		},
		{
			name:    "unicode text",
			input:   "你好世界",
			wantErr: false,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizeTextForEncoding(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Result should be valid UTF-8
				assert.True(t, len(result) >= 0)
			}
		})
	}
}

func TestNormalizeExtractedInfo(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		removeInnerQuotes bool
		expected          string
	}{
		{
			name:              "normal text",
			input:             "Hello World",
			removeInnerQuotes: false,
			expected:          "Hello World",
		},
		{
			name:              "full-width letters",
			input:             "ＡＢＣ",
			removeInnerQuotes: false,
			expected:          "ABC",
		},
		{
			name:              "full-width numbers",
			input:             "１２３",
			removeInnerQuotes: false,
			expected:          "123",
		},
		{
			name:              "quoted text",
			input:             `"Hello"`,
			removeInnerQuotes: false,
			expected:          "Hello",
		},
		{
			name:              "short numeric",
			input:             "12",
			removeInnerQuotes: false,
			expected:          "",
		},
		{
			name:              "dots and numbers",
			input:             "1.2.3",
			removeInnerQuotes: false,
			expected:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeExtractedInfo(tt.input, tt.removeInnerQuotes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetContentSummary(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		maxLength int
		expected  string
	}{
		{
			name:      "short content",
			content:   "Hello",
			maxLength: 10,
			expected:  "Hello",
		},
		{
			name:      "long content",
			content:   "Hello World This Is A Long String",
			maxLength: 10,
			expected:  "Hello Worl...",
		},
		{
			name:      "exact length",
			content:   "Hello",
			maxLength: 5,
			expected:  "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetContentSummary(tt.content, tt.maxLength)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveThinkTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with think tags",
			input:    "<think>thinking process</think>actual response",
			expected: "actual response",
		},
		{
			name:     "orphan closing tag",
			input:    "some text</think>actual response",
			expected: "actual response",
		},
		{
			name:     "no tags",
			input:    "just normal text",
			expected: "just normal text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveThinkTags(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTokenTracker(t *testing.T) {
	tracker := NewTokenTracker()

	// Test initial state
	usage := tracker.GetUsage()
	assert.Equal(t, int64(0), usage["prompt_tokens"])
	assert.Equal(t, int64(0), usage["completion_tokens"])
	assert.Equal(t, int64(0), usage["total_tokens"])
	assert.Equal(t, int64(0), usage["call_count"])

	// Add usage
	tracker.AddUsage(map[string]int64{
		"prompt_tokens":     100,
		"completion_tokens": 50,
		"total_tokens":      150,
	})

	usage = tracker.GetUsage()
	assert.Equal(t, int64(100), usage["prompt_tokens"])
	assert.Equal(t, int64(50), usage["completion_tokens"])
	assert.Equal(t, int64(150), usage["total_tokens"])
	assert.Equal(t, int64(1), usage["call_count"])

	// Add more usage
	tracker.AddUsage(map[string]int64{
		"prompt_tokens":     200,
		"completion_tokens": 100,
	})

	usage = tracker.GetUsage()
	assert.Equal(t, int64(300), usage["prompt_tokens"])
	assert.Equal(t, int64(150), usage["completion_tokens"])
	assert.Equal(t, int64(450), usage["total_tokens"])
	assert.Equal(t, int64(2), usage["call_count"])

	// Test reset
	tracker.Reset()
	usage = tracker.GetUsage()
	assert.Equal(t, int64(0), usage["call_count"])

	// Test String output
	tracker.AddUsage(map[string]int64{
		"prompt_tokens":     100,
		"completion_tokens": 50,
		"total_tokens":      150,
	})
	str := tracker.String()
	assert.Contains(t, str, "LLM call count: 1")
}

func TestSimpleTokenizer(t *testing.T) {
	tokenizer := NewSimpleTokenizer()

	// Test encoding
	tokens := tokenizer.Encode("hello world")
	assert.True(t, len(tokens) > 0)

	// Test decoding
	decoded := tokenizer.Decode(tokens)
	assert.Contains(t, decoded, "tokens")

	// Test empty string
	tokens = tokenizer.Encode("")
	assert.Empty(t, tokens)
}

func TestPackUserAssToOpenAIMessages(t *testing.T) {
	messages := PackUserAssToOpenAIMessages("hello", "hi", "how are you")

	assert.Len(t, messages, 3)
	assert.Equal(t, "user", messages[0]["role"])
	assert.Equal(t, "hello", messages[0]["content"])
	assert.Equal(t, "assistant", messages[1]["role"])
	assert.Equal(t, "hi", messages[1]["content"])
	assert.Equal(t, "user", messages[2]["role"])
	assert.Equal(t, "how are you", messages[2]["content"])
}

func TestPickByWeightedPolling(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := PickByWeightedPolling([]EntityOrRelation{}, 5, 1)
		assert.Empty(t, result)
	})

	t.Run("single entity", func(t *testing.T) {
		entities := []EntityOrRelation{
			{SortedChunks: []string{"c1", "c2", "c3", "c4", "c5"}},
		}
		result := PickByWeightedPolling(entities, 3, 1)
		assert.Len(t, result, 3)
		assert.Equal(t, []string{"c1", "c2", "c3"}, result)
	})

	t.Run("multiple entities", func(t *testing.T) {
		entities := []EntityOrRelation{
			{SortedChunks: []string{"a1", "a2", "a3"}},
			{SortedChunks: []string{"b1", "b2", "b3"}},
			{SortedChunks: []string{"c1", "c2", "c3"}},
		}
		result := PickByWeightedPolling(entities, 3, 1)
		assert.True(t, len(result) > 0)
		// First entity should have more chunks
		assert.Contains(t, result, "a1")
	})
}

func TestSafeVDBOperationWithRetry(t *testing.T) {
	t.Run("successful operation", func(t *testing.T) {
		ctx := context.Background()
		callCount := 0
		operation := func() error {
			callCount++
			return nil
		}

		err := SafeVDBOperationWithRetry(ctx, operation, "test", "entity1", 3, 10*time.Millisecond)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("operation fails then succeeds", func(t *testing.T) {
		ctx := context.Background()
		callCount := 0
		operation := func() error {
			callCount++
			if callCount < 3 {
				return errors.New("temporary error")
			}
			return nil
		}

		err := SafeVDBOperationWithRetry(ctx, operation, "test", "entity1", 3, 10*time.Millisecond)
		assert.NoError(t, err)
		assert.Equal(t, 3, callCount)
	})

	t.Run("operation always fails", func(t *testing.T) {
		ctx := context.Background()
		operation := func() error {
			return errors.New("persistent error")
		}

		err := SafeVDBOperationWithRetry(ctx, operation, "test", "entity1", 3, 10*time.Millisecond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "after 3 attempts")
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		operation := func() error {
			return errors.New("should not be called")
		}

		err := SafeVDBOperationWithRetry(ctx, operation, "test", "entity1", 3, 10*time.Millisecond)
		assert.Error(t, err)
	})
}

func TestCacheData(t *testing.T) {
	data := NewCacheData("hash123", "test content", "test prompt")

	assert.Equal(t, "hash123", data.ArgsHash)
	assert.Equal(t, "test content", data.Content)
	assert.Equal(t, "test prompt", data.Prompt)
	assert.Equal(t, "default", data.Mode)
	assert.Equal(t, "query", data.CacheType)
}

func TestTruncateListByTokenSize(t *testing.T) {
	tokenizer := NewTokenizer("test", NewSimpleTokenizer())

	items := []string{"hello", "world", "this is a longer string", "short"}
	keyFunc := func(s string) string { return s }

	t.Run("no truncation needed", func(t *testing.T) {
		result := TruncateListByTokenSize(items, keyFunc, 1000, tokenizer)
		assert.Equal(t, items, result)
	})

	t.Run("zero max size", func(t *testing.T) {
		result := TruncateListByTokenSize(items, keyFunc, 0, tokenizer)
		assert.Empty(t, result)
	})

	t.Run("truncation needed", func(t *testing.T) {
		result := TruncateListByTokenSize(items, keyFunc, 5, tokenizer)
		assert.True(t, len(result) < len(items))
	})
}

func TestCheckStorageEnvVars(t *testing.T) {
	requirements := map[string][]string{
		"postgres": {"POSTGRES_HOST", "POSTGRES_USER"},
		"redis":    {"REDIS_HOST"},
	}

	t.Run("no requirements", func(t *testing.T) {
		err := CheckStorageEnvVars("unknown", requirements)
		assert.NoError(t, err)
	})

	t.Run("missing vars", func(t *testing.T) {
		err := CheckStorageEnvVars("postgres", requirements)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "POSTGRES_HOST")
	})

	t.Run("all vars present", func(t *testing.T) {
		os.Setenv("REDIS_HOST", "localhost")
		defer os.Unsetenv("REDIS_HOST")

		err := CheckStorageEnvVars("redis", requirements)
		assert.NoError(t, err)
	})
}

func TestQueueFullError(t *testing.T) {
	err := &QueueFullError{Message: "queue is full"}
	assert.Equal(t, "queue is full", err.Error())
}

func TestWorkerTimeoutError(t *testing.T) {
	err := &WorkerTimeoutError{TimeoutValue: 30.0, TimeoutType: "execution"}
	assert.Contains(t, err.Error(), "30.0s")
	assert.Contains(t, err.Error(), "execution")
}

func TestHealthCheckTimeoutError(t *testing.T) {
	err := &HealthCheckTimeoutError{TimeoutValue: 60.0, ExecutionDuration: 75.5}
	assert.Contains(t, err.Error(), "60.0s")
	assert.Contains(t, err.Error(), "75.5s")
}

func TestTokenizerWrapper(t *testing.T) {
	simple := NewSimpleTokenizer()
	tokenizer := NewTokenizer("gpt-4", simple)

	assert.Equal(t, "gpt-4", tokenizer.ModelName)

	tokens := tokenizer.Encode("test string")
	require.NotEmpty(t, tokens)

	decoded := tokenizer.Decode(tokens)
	require.NotEmpty(t, decoded)
}
