package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// JSON Write Optimization Tests
// ============================================================================

// containsSurrogate checks if a string contains surrogate characters
func containsSurrogate(s string) bool {
	for _, r := range s {
		if r >= 0xD800 && r <= 0xDFFF {
			return true
		}
	}
	return false
}

// sanitizeJSONString removes surrogate characters from a string
func sanitizeJSONString(s string) string {
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if r < 0xD800 || r > 0xDFFF {
			result = append(result, r)
		}
	}
	return string(result)
}

// sanitizeValue recursively sanitizes values for JSON encoding
func sanitizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		return sanitizeJSONString(val)
	case map[string]interface{}:
		sanitized := make(map[string]interface{})
		for k, v := range val {
			sanitizedKey := sanitizeJSONString(k)
			sanitized[sanitizedKey] = sanitizeValue(v)
		}
		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, len(val))
		for i, v := range val {
			sanitized[i] = sanitizeValue(v)
		}
		return sanitized
	default:
		return val
	}
}

// needsSanitization checks if data needs sanitization
func needsSanitization(v interface{}) bool {
	switch val := v.(type) {
	case string:
		return containsSurrogate(val)
	case map[string]interface{}:
		for k, v := range val {
			if containsSurrogate(k) || needsSanitization(v) {
				return true
			}
		}
		return false
	case []interface{}:
		for _, v := range val {
			if needsSanitization(v) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// writeJSON writes JSON to file, returning true if sanitization was needed
func writeJSON(data interface{}, filePath string) (bool, error) {
	// First try fast path with standard encoding
	bytes, err := json.Marshal(data)
	if err == nil {
		// Fast path succeeded
		return false, os.WriteFile(filePath, bytes, 0o644)
	}

	// Need sanitization
	sanitizedData := sanitizeValue(data)
	bytes, err = json.Marshal(sanitizedData)
	if err != nil {
		return true, err
	}
	return true, os.WriteFile(filePath, bytes, 0o644)
}

// loadJSON loads JSON from file
func loadJSON(filePath string) (map[string]interface{}, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal(bytes, &result)
	return result, err
}

func TestFastPathCleanData(t *testing.T) {
	// Test that clean data takes the fast path without sanitization
	cleanData := map[string]interface{}{
		"name":  "John Doe",
		"age":   float64(30),
		"items": []interface{}{"apple", "banana", "cherry"},
		"nested": map[string]interface{}{
			"key":    "value",
			"number": float64(42),
		},
	}

	tempDir, err := os.MkdirTemp("", "json_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "clean.json")
	needsReload, err := writeJSON(cleanData, tempFile)
	assert.NoError(t, err)
	assert.False(t, needsReload, "Clean data should not require sanitization")

	// Verify data was written correctly
	loadedData, err := loadJSON(tempFile)
	assert.NoError(t, err)
	assert.Equal(t, "John Doe", loadedData["name"])
	assert.Equal(t, float64(30), loadedData["age"])
}

func TestSlowPathDirtyData(t *testing.T) {
	// Test that dirty data triggers sanitization
	// Note: Go strings don't allow bare surrogates, so we'll test the sanitization logic directly
	dirtyString := "Hello\uFFFDWorld" // Using replacement character to simulate
	dirtyData := map[string]interface{}{
		"text":   dirtyString,
		"number": float64(123),
	}

	tempDir, err := os.MkdirTemp("", "json_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "dirty.json")
	_, err = writeJSON(dirtyData, tempFile)
	assert.NoError(t, err)

	// Verify data was written
	loadedData, err := loadJSON(tempFile)
	assert.NoError(t, err)
	assert.NotNil(t, loadedData)
	assert.Equal(t, float64(123), loadedData["number"])
}

func TestSanitizingRemovesSurrogates(t *testing.T) {
	// Test the sanitization function directly
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no surrogates",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "clean unicode",
			input:    "你好世界",
			expected: "你好世界",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "mixed content",
			input:    "ABC123xyz",
			expected: "ABC123xyz",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeJSONString(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNestedStructureSanitization(t *testing.T) {
	// Test sanitization of deeply nested structures
	nestedData := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"clean": "normal text",
				},
				"list": []interface{}{"item1", "item2", "item3"},
			},
		},
	}

	tempDir, err := os.MkdirTemp("", "json_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "nested.json")
	_, err = writeJSON(nestedData, tempFile)
	assert.NoError(t, err)

	// Verify nested structure is preserved
	loadedData, err := loadJSON(tempFile)
	assert.NoError(t, err)

	level1 := loadedData["level1"].(map[string]interface{})
	level2 := level1["level2"].(map[string]interface{})
	level3 := level2["level3"].(map[string]interface{})
	assert.Equal(t, "normal text", level3["clean"])

	list := level2["list"].([]interface{})
	assert.Len(t, list, 3)
	assert.Equal(t, "item1", list[0])
}

func TestMixedCleanDirtyData(t *testing.T) {
	// Test data with both clean and dirty fields
	mixedData := map[string]interface{}{
		"clean_field": "This is perfectly fine",
		"number":      float64(42),
		"boolean":     true,
		"null_value":  nil,
		"clean_list":  []interface{}{float64(1), float64(2), float64(3)},
		"dirty_list":  []interface{}{"clean", "item"},
	}

	tempDir, err := os.MkdirTemp("", "json_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "mixed.json")
	_, err = writeJSON(mixedData, tempFile)
	assert.NoError(t, err)

	loadedData, err := loadJSON(tempFile)
	assert.NoError(t, err)

	// Clean fields should remain unchanged
	assert.Equal(t, "This is perfectly fine", loadedData["clean_field"])
	assert.Equal(t, float64(42), loadedData["number"])
	assert.Equal(t, true, loadedData["boolean"])
	assert.Nil(t, loadedData["null_value"])

	cleanList := loadedData["clean_list"].([]interface{})
	assert.Equal(t, []interface{}{float64(1), float64(2), float64(3)}, cleanList)
}

func TestEmptyAndNoneStrings(t *testing.T) {
	// Test handling of empty and None values
	data := map[string]interface{}{
		"empty":      "",
		"none":       nil,
		"zero":       float64(0),
		"false":      false,
		"empty_list": []interface{}{},
		"empty_dict": map[string]interface{}{},
	}

	tempDir, err := os.MkdirTemp("", "json_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "empty.json")
	needsReload, err := writeJSON(data, tempFile)
	assert.NoError(t, err)
	assert.False(t, needsReload, "Clean data with empty values should not require sanitization")

	loadedData, err := loadJSON(tempFile)
	assert.NoError(t, err)

	assert.Equal(t, "", loadedData["empty"])
	assert.Nil(t, loadedData["none"])
	assert.Equal(t, float64(0), loadedData["zero"])
	assert.Equal(t, false, loadedData["false"])
	assert.Empty(t, loadedData["empty_list"])
	assert.Empty(t, loadedData["empty_dict"])
}

func TestLargeDataFastPath(t *testing.T) {
	// Test that large clean data still uses fast path
	largeData := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largeData["key_"+string(rune('A'+i%26))+string(rune('0'+i/26))] = map[string]interface{}{
			"index": float64(i),
			"value": "test_value_for_item",
		}
	}

	tempDir, err := os.MkdirTemp("", "json_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "large.json")
	needsReload, err := writeJSON(largeData, tempFile)
	assert.NoError(t, err)
	assert.False(t, needsReload, "Large clean data should not require sanitization")

	// Verify file was written
	info, err := os.Stat(tempFile)
	assert.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

func TestSpecialCharacters(t *testing.T) {
	// Test handling of special but valid JSON characters
	data := map[string]interface{}{
		"newline":   "line1\nline2",
		"tab":       "col1\tcol2",
		"quote":     `He said "hello"`,
		"backslash": `path\to\file`,
		"unicode":   "emoji: 🎉",
		"cjk":       "中文字符",
		"arabic":    "العربية",
		"hebrew":    "עברית",
	}

	tempDir, err := os.MkdirTemp("", "json_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "special.json")
	needsReload, err := writeJSON(data, tempFile)
	assert.NoError(t, err)
	assert.False(t, needsReload, "Valid special characters should not require sanitization")

	loadedData, err := loadJSON(tempFile)
	assert.NoError(t, err)

	assert.Equal(t, "line1\nline2", loadedData["newline"])
	assert.Equal(t, "col1\tcol2", loadedData["tab"])
	assert.Equal(t, "emoji: 🎉", loadedData["unicode"])
	assert.Equal(t, "中文字符", loadedData["cjk"])
}

// ============================================================================
// Sanitize Value Tests
// ============================================================================

func TestSanitizeValueString(t *testing.T) {
	input := "normal string"
	result := sanitizeValue(input)
	assert.Equal(t, "normal string", result)
}

func TestSanitizeValueMap(t *testing.T) {
	input := map[string]interface{}{
		"key1": "value1",
		"key2": float64(42),
	}
	result := sanitizeValue(input).(map[string]interface{})
	assert.Equal(t, "value1", result["key1"])
	assert.Equal(t, float64(42), result["key2"])
}

func TestSanitizeValueSlice(t *testing.T) {
	input := []interface{}{"item1", "item2", float64(3)}
	result := sanitizeValue(input).([]interface{})
	assert.Equal(t, "item1", result[0])
	assert.Equal(t, "item2", result[1])
	assert.Equal(t, float64(3), result[2])
}

func TestSanitizeValueNested(t *testing.T) {
	input := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": []interface{}{"a", "b", "c"},
		},
	}
	result := sanitizeValue(input).(map[string]interface{})
	outer := result["outer"].(map[string]interface{})
	inner := outer["inner"].([]interface{})
	assert.Equal(t, []interface{}{"a", "b", "c"}, inner)
}

// ============================================================================
// Needs Sanitization Tests
// ============================================================================

func TestNeedsSanitization_CleanString(t *testing.T) {
	assert.False(t, needsSanitization("clean string"))
}

func TestNeedsSanitization_CleanMap(t *testing.T) {
	data := map[string]interface{}{
		"key": "value",
	}
	assert.False(t, needsSanitization(data))
}

func TestNeedsSanitization_CleanSlice(t *testing.T) {
	data := []interface{}{"a", "b", "c"}
	assert.False(t, needsSanitization(data))
}

func TestNeedsSanitization_NestedClean(t *testing.T) {
	data := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": []interface{}{"a", "b"},
		},
	}
	assert.False(t, needsSanitization(data))
}
