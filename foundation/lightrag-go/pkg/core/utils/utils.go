// Package utils provides utility functions for LightRAG.
// This is a direct translation of Python lightrag/utils.py to Go.
package utils

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
)

// Logger is the default logger for lightrag
var Logger = logrus.New()

// StatisticData tracks LLM and embedding call statistics
var StatisticData = struct {
	LLMCall   atomic.Int64
	LLMCache  atomic.Int64
	EmbedCall atomic.Int64
}{}

// VerboseDebug controls verbose debug output
var VerboseDebug bool

func init() {
	// Initialize logger with default settings
	Logger.SetLevel(logrus.InfoLevel)
	Logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Check environment for verbose debug
	VerboseDebug = strings.ToLower(os.Getenv("VERBOSE")) == "true"
}

// SetVerboseDebug enables or disables verbose debug output
func SetVerboseDebug(enabled bool) {
	VerboseDebug = enabled
}

// VerboseLog outputs detailed debug information based on VerboseDebug setting
func VerboseLog(msg string, args ...interface{}) {
	if VerboseDebug {
		Logger.Debugf(msg, args...)
	} else {
		// Truncate message for non-verbose mode
		formatted := fmt.Sprintf(msg, args...)
		if len(formatted) > 150 {
			formatted = formatted[:150] + "..."
		}
		// Remove consecutive newlines
		formatted = regexp.MustCompile(`\n+`).ReplaceAllString(formatted, "\n")
		Logger.Debug(formatted)
	}
}

// SetupLogger configures logging with optional file output
func SetupLogger(loggerName string, level logrus.Level, enableFileLogging bool, logFilePath string) *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(level)

	// Simple formatter for console
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// TODO: Add file logging support if needed
	// This would require rotating file handler implementation

	return logger
}

// ----- Surrogate Pattern for JSON Sanitization -----

// surrogatePattern matches surrogate characters that cannot be encoded in JSON
var surrogatePattern = regexp.MustCompile(`[\x{D800}-\x{DFFF}\x{FFFE}\x{FFFF}]`)

// SanitizeStringForJSON removes characters that cannot be encoded in UTF-8 for JSON serialization
func SanitizeStringForJSON(text string) string {
	if text == "" {
		return text
	}

	// Fast path: Check if sanitization is needed
	if !surrogatePattern.MatchString(text) {
		return text
	}

	// Slow path: Remove problematic characters
	return surrogatePattern.ReplaceAllString(text, "")
}

// ----- Hash Functions -----

// ComputeArgsHash computes a hash for the given arguments with safe Unicode handling
func ComputeArgsHash(args ...interface{}) string {
	var builder strings.Builder
	for _, arg := range args {
		builder.WriteString(fmt.Sprintf("%v", arg))
	}

	hash := md5.Sum([]byte(builder.String()))
	return hex.EncodeToString(hash[:])
}

// ComputeMDHashID computes a unique ID for a given content string
func ComputeMDHashID(content string, prefix string) string {
	return prefix + ComputeArgsHash(content)
}

// GenerateCacheKey generates a flattened cache key in the format {mode}:{cache_type}:{hash}
func GenerateCacheKey(mode, cacheType, hashValue string) string {
	return fmt.Sprintf("%s:%s:%s", mode, cacheType, hashValue)
}

// ParseCacheKey parses a flattened cache key back into its components
func ParseCacheKey(cacheKey string) (mode, cacheType, hash string, ok bool) {
	parts := strings.SplitN(cacheKey, ":", 3)
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2], true
	}
	return "", "", "", false
}

// ----- Environment Variables -----

// GetEnvValue gets value from environment variable with type conversion
func GetEnvValue(envKey string, defaultValue interface{}, specialNone bool) interface{} {
	value := os.Getenv(envKey)
	if value == "" {
		return defaultValue
	}

	// Handle special case for "None" string
	if specialNone && value == "None" {
		return nil
	}

	// Type conversion based on default value type
	switch defaultValue.(type) {
	case bool:
		return strings.ToLower(value) == "true" || value == "1" || value == "yes"
	case int:
		if v, err := strconv.Atoi(value); err == nil {
			return v
		}
		return defaultValue
	case int64:
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			return v
		}
		return defaultValue
	case float64:
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			return v
		}
		return defaultValue
	case string:
		return value
	case []string:
		var result []string
		if err := json.Unmarshal([]byte(value), &result); err == nil {
			return result
		}
		return defaultValue
	default:
		return value
	}
}

// GetEnvString gets a string value from environment variable
func GetEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetEnvInt gets an integer value from environment variable
func GetEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if v, err := strconv.Atoi(value); err == nil {
			return v
		}
	}
	return defaultValue
}

// GetEnvBool gets a boolean value from environment variable
func GetEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		lower := strings.ToLower(value)
		return lower == "true" || lower == "1" || lower == "yes"
	}
	return defaultValue
}

// GetEnvFloat gets a float value from environment variable
func GetEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			return v
		}
	}
	return defaultValue
}

// ----- JSON Utilities -----

// LoadJSON loads JSON from a file
func LoadJSON(fileName string) (interface{}, error) {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// WriteJSON writes JSON data to file with sanitization
func WriteJSON(jsonObj interface{}, fileName string) (bool, error) {
	data, err := json.MarshalIndent(jsonObj, "", "  ")
	if err != nil {
		// Try with sanitization
		sanitized := sanitizeForJSON(jsonObj)
		data, err = json.MarshalIndent(sanitized, "", "  ")
		if err != nil {
			return false, err
		}
		err = os.WriteFile(fileName, data, 0o644)
		return true, err // Sanitization was applied
	}

	err = os.WriteFile(fileName, data, 0o644)
	return false, err
}

// sanitizeForJSON recursively sanitizes an object for JSON encoding
func sanitizeForJSON(obj interface{}) interface{} {
	switch v := obj.(type) {
	case string:
		return SanitizeStringForJSON(v)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			cleanKey := SanitizeStringForJSON(key)
			result[cleanKey] = sanitizeForJSON(value)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = sanitizeForJSON(item)
		}
		return result
	default:
		return obj
	}
}

// ----- Tokenizer Interface -----

// TokenizerInterface defines the interface for a tokenizer
type TokenizerInterface interface {
	Encode(content string) []int
	Decode(tokens []int) string
}

// Tokenizer is a wrapper around a tokenizer to provide a consistent interface
type Tokenizer struct {
	ModelName string
	Tokenizer TokenizerInterface
}

// NewTokenizer creates a new Tokenizer instance
func NewTokenizer(modelName string, tokenizer TokenizerInterface) *Tokenizer {
	return &Tokenizer{
		ModelName: modelName,
		Tokenizer: tokenizer,
	}
}

// Encode encodes a string into a list of tokens
func (t *Tokenizer) Encode(content string) []int {
	return t.Tokenizer.Encode(content)
}

// Decode decodes a list of tokens into a string
func (t *Tokenizer) Decode(tokens []int) string {
	return t.Tokenizer.Decode(tokens)
}

// SimpleTokenizer provides a basic tokenizer implementation
type SimpleTokenizer struct {
	// Average characters per token (rough approximation)
	CharsPerToken int
}

// NewSimpleTokenizer creates a new simple tokenizer
func NewSimpleTokenizer() *SimpleTokenizer {
	return &SimpleTokenizer{
		CharsPerToken: 4, // Average for English text
	}
}

// Encode implements TokenizerInterface
func (t *SimpleTokenizer) Encode(content string) []int {
	// Simple approximation: divide by chars per token
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

// Decode implements TokenizerInterface
func (t *SimpleTokenizer) Decode(tokens []int) string {
	// This is a placeholder - real implementation would need actual token mapping
	return fmt.Sprintf("[%d tokens]", len(tokens))
}

// ----- String Utilities -----

// PackUserAssToOpenAIMessages packs alternating user/assistant messages
func PackUserAssToOpenAIMessages(args ...string) []map[string]string {
	roles := []string{"user", "assistant"}
	messages := make([]map[string]string, len(args))
	for i, content := range args {
		messages[i] = map[string]string{
			"role":    roles[i%2],
			"content": content,
		}
	}
	return messages
}

// SplitStringByMultiMarkers splits a string by multiple markers
func SplitStringByMultiMarkers(content string, markers []string) []string {
	if len(markers) == 0 {
		return []string{content}
	}
	if content == "" {
		return []string{}
	}

	// Build regex pattern
	patterns := make([]string, len(markers))
	for i, marker := range markers {
		patterns[i] = regexp.QuoteMeta(marker)
	}
	pattern := regexp.MustCompile(strings.Join(patterns, "|"))

	// Split and filter
	parts := pattern.Split(content, -1)
	results := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			results = append(results, trimmed)
		}
	}
	return results
}

// IsFloatRegex checks if a string is a valid float number
func IsFloatRegex(value string) bool {
	matched, _ := regexp.MatchString(`^[-+]?[0-9]*\.?[0-9]+$`, value)
	return matched
}

// TruncateListByTokenSize truncates a list of data by token size
func TruncateListByTokenSize[T any](
	listData []T,
	keyFunc func(T) string,
	maxTokenSize int,
	tokenizer *Tokenizer,
) []T {
	if maxTokenSize <= 0 {
		return []T{}
	}

	tokens := 0
	for i, data := range listData {
		tokens += len(tokenizer.Encode(keyFunc(data)))
		if tokens > maxTokenSize {
			return listData[:i]
		}
	}
	return listData
}

// ----- Math Utilities -----

// CosineSimilarity calculates cosine similarity between two vectors
func CosineSimilarity(v1, v2 []float64) float64 {
	if len(v1) != len(v2) {
		return 0.0
	}

	var dotProduct, norm1, norm2 float64
	for i := range v1 {
		dotProduct += v1[i] * v2[i]
		norm1 += v1[i] * v1[i]
		norm2 += v2[i] * v2[i]
	}

	norm1 = math.Sqrt(norm1)
	norm2 = math.Sqrt(norm2)

	if norm1 == 0 || norm2 == 0 {
		return 0.0
	}

	return dotProduct / (norm1 * norm2)
}

// ----- Text Sanitization -----

// SanitizeTextForEncoding sanitizes text to ensure safe UTF-8 encoding
func SanitizeTextForEncoding(text string) (string, error) {
	if text == "" {
		return text, nil
	}

	// Strip whitespace
	text = strings.TrimSpace(text)
	if text == "" {
		return text, nil
	}

	// Validate UTF-8
	if !utf8.ValidString(text) {
		// Replace invalid sequences
		text = strings.ToValidUTF8(text, "")
	}

	// Remove surrogate characters and problematic Unicode
	var builder strings.Builder
	for _, r := range text {
		// Check for surrogate characters (U+D800 to U+DFFF)
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		// Check for non-characters
		if r == 0xFFFE || r == 0xFFFF {
			continue
		}
		// Check for control characters (except common whitespace)
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			continue
		}
		if r == 0x7F {
			continue
		}
		builder.WriteRune(r)
	}

	sanitized := builder.String()

	// Unescape HTML entities
	sanitized = html.UnescapeString(sanitized)

	return strings.TrimSpace(sanitized), nil
}

// NormalizeExtractedInfo normalizes entity/relation names and descriptions
func NormalizeExtractedInfo(name string, removeInnerQuotes bool) string {
	if name == "" {
		return ""
	}

	// Clean HTML tags
	name = regexp.MustCompile(`</p\s*>|<p\s*>|<p/>`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`</br\s*>|<br\s*>|<br/>`).ReplaceAllString(name, "")

	// Chinese full-width letters to half-width
	fullWidthUpper := []rune("ＡＢＣＤＥＦＧＨＩＪＫＬＭＮＯＰＱＲＳＴＵＶＷＸＹＺ")
	halfWidthUpper := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	fullWidthLower := []rune("ａｂｃｄｅｆｇｈｉｊｋｌｍｎｏｐｑｒｓｔｕｖｗｘｙｚ")
	halfWidthLower := []rune("abcdefghijklmnopqrstuvwxyz")
	fullWidthDigits := []rune("０１２３４５６７８９")
	halfWidthDigits := []rune("0123456789")

	for i, r := range fullWidthUpper {
		name = strings.ReplaceAll(name, string(r), string(halfWidthUpper[i]))
	}
	for i, r := range fullWidthLower {
		name = strings.ReplaceAll(name, string(r), string(halfWidthLower[i]))
	}
	for i, r := range fullWidthDigits {
		name = strings.ReplaceAll(name, string(r), string(halfWidthDigits[i]))
	}

	// Chinese full-width symbols to half-width
	replacements := map[string]string{
		"－": "-", "＋": "+", "／": "/", "＊": "*",
		"（": "(", "）": ")", "—": "-", "　": " ",
	}
	for old, new := range replacements {
		name = strings.ReplaceAll(name, old, new)
	}

	// Remove spaces between Chinese characters
	chineseSpacePattern := regexp.MustCompile(`([\x{4e00}-\x{9fa5}])\s+([\x{4e00}-\x{9fa5}])`)
	for chineseSpacePattern.MatchString(name) {
		name = chineseSpacePattern.ReplaceAllString(name, "$1$2")
	}

	// Remove spaces between Chinese and English/numbers
	name = regexp.MustCompile(`([\x{4e00}-\x{9fa5}])\s+([a-zA-Z0-9\(\)\[\]@#$%!&\*\-=+_])`).ReplaceAllString(name, "$1$2")
	name = regexp.MustCompile(`([a-zA-Z0-9\(\)\[\]@#$%!&\*\-=+_])\s+([\x{4e00}-\x{9fa5}])`).ReplaceAllString(name, "$1$2")

	// Remove outer quotes
	if len(name) >= 2 {
		// Handle double quotes
		if strings.HasPrefix(name, `"`) && strings.HasSuffix(name, `"`) {
			inner := name[1 : len(name)-1]
			if !strings.Contains(inner, `"`) {
				name = inner
			}
		}
		// Handle single quotes
		if strings.HasPrefix(name, "'") && strings.HasSuffix(name, "'") {
			inner := name[1 : len(name)-1]
			if !strings.Contains(inner, "'") {
				name = inner
			}
		}
		// Handle Chinese double quotes (left: \u201c, right: \u201d)
		chineseLeftQuote := "\u201c"
		chineseRightQuote := "\u201d"
		if strings.HasPrefix(name, chineseLeftQuote) && strings.HasSuffix(name, chineseRightQuote) {
			inner := name[3 : len(name)-3] // Chinese quotes are 3 bytes
			if !strings.Contains(inner, chineseLeftQuote) && !strings.Contains(inner, chineseRightQuote) {
				name = inner
			}
		}
		// Handle Chinese book title marks (\u300a and \u300b)
		chineseLeftBook := "\u300a"
		chineseRightBook := "\u300b"
		if strings.HasPrefix(name, chineseLeftBook) && strings.HasSuffix(name, chineseRightBook) {
			inner := name[3 : len(name)-3]
			if !strings.Contains(inner, chineseLeftBook) && !strings.Contains(inner, chineseRightBook) {
				name = inner
			}
		}
	}

	if removeInnerQuotes {
		// Remove Chinese quotes
		name = strings.ReplaceAll(name, "\u201c", "")
		name = strings.ReplaceAll(name, "\u201d", "")
		name = strings.ReplaceAll(name, "\u2018", "")
		name = strings.ReplaceAll(name, "\u2019", "")

		// Remove English quotes around Chinese characters
		// Note: Go regex doesn't support lookbehind, so we use a different approach
		// Remove quotes that appear before Chinese characters
		quoteBeforeChinese := regexp.MustCompile(`['"]([\x{4e00}-\x{9fa5}])`)
		name = quoteBeforeChinese.ReplaceAllString(name, "$1")
		// Remove quotes that appear after Chinese characters
		quoteAfterChinese := regexp.MustCompile(`([\x{4e00}-\x{9fa5}])['"]`)
		name = quoteAfterChinese.ReplaceAllString(name, "$1")

		// Convert non-breaking space to regular space
		name = strings.ReplaceAll(name, "\u00a0", " ")
	}

	name = strings.TrimSpace(name)

	// Filter out pure numeric content with length < 3
	if len(name) < 3 && regexp.MustCompile(`^[0-9]+$`).MatchString(name) {
		return ""
	}

	// Filter out mixed numeric and dot content with length < 6
	if len(name) < 6 && shouldFilterByDots(name) {
		return ""
	}

	return name
}

// shouldFilterByDots checks if string consists only of dots and digits
func shouldFilterByDots(text string) bool {
	hasDot := false
	for _, r := range text {
		if r == '.' {
			hasDot = true
		} else if !unicode.IsDigit(r) {
			return false
		}
	}
	return hasDot
}

// SanitizeAndNormalizeExtractedText sanitizes and normalizes extracted text
func SanitizeAndNormalizeExtractedText(inputText string, removeInnerQuotes bool) string {
	safe, err := SanitizeTextForEncoding(inputText)
	if err != nil {
		return ""
	}
	if safe != "" {
		return NormalizeExtractedInfo(safe, removeInnerQuotes)
	}
	return ""
}

// GetContentSummary gets a summary of document content
func GetContentSummary(content string, maxLength int) string {
	content = strings.TrimSpace(content)
	if len(content) <= maxLength {
		return content
	}
	return content[:maxLength] + "..."
}

// RemoveThinkTags removes <think>...</think> tags from the text
func RemoveThinkTags(text string) string {
	pattern := regexp.MustCompile(`^(<think>.*?</think>|.*</think>)`)
	return strings.TrimSpace(pattern.ReplaceAllString(text, ""))
}

// ----- Concurrent Operations -----

// QueueFullError is raised when the queue is full and wait times out
type QueueFullError struct {
	Message string
}

func (e *QueueFullError) Error() string {
	return e.Message
}

// WorkerTimeoutError is a worker-level timeout exception
type WorkerTimeoutError struct {
	TimeoutValue float64
	TimeoutType  string
}

func (e *WorkerTimeoutError) Error() string {
	return fmt.Sprintf("Worker %s timeout after %.1fs", e.TimeoutType, e.TimeoutValue)
}

// HealthCheckTimeoutError is a health check-level timeout exception
type HealthCheckTimeoutError struct {
	TimeoutValue      float64
	ExecutionDuration float64
}

func (e *HealthCheckTimeoutError) Error() string {
	return fmt.Sprintf("Task forcefully terminated due to execution timeout (>%.1fs, actual: %.1fs)",
		e.TimeoutValue, e.ExecutionDuration)
}

// ----- Token Tracker -----

// TokenTracker tracks token usage for LLM calls
type TokenTracker struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	CallCount        int64
	mu               sync.Mutex
}

// NewTokenTracker creates a new TokenTracker
func NewTokenTracker() *TokenTracker {
	return &TokenTracker{}
}

// Reset resets all tracking counters
func (t *TokenTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.PromptTokens = 0
	t.CompletionTokens = 0
	t.TotalTokens = 0
	t.CallCount = 0
}

// AddUsage adds token usage from one LLM call
func (t *TokenTracker) AddUsage(tokenCounts map[string]int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.PromptTokens += tokenCounts["prompt_tokens"]
	t.CompletionTokens += tokenCounts["completion_tokens"]

	if total, ok := tokenCounts["total_tokens"]; ok {
		t.TotalTokens += total
	} else {
		t.TotalTokens += tokenCounts["prompt_tokens"] + tokenCounts["completion_tokens"]
	}

	t.CallCount++
}

// GetUsage returns current usage statistics
func (t *TokenTracker) GetUsage() map[string]int64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	return map[string]int64{
		"prompt_tokens":     t.PromptTokens,
		"completion_tokens": t.CompletionTokens,
		"total_tokens":      t.TotalTokens,
		"call_count":        t.CallCount,
	}
}

func (t *TokenTracker) String() string {
	usage := t.GetUsage()
	return fmt.Sprintf("LLM call count: %d, Prompt tokens: %d, Completion tokens: %d, Total tokens: %d",
		usage["call_count"], usage["prompt_tokens"], usage["completion_tokens"], usage["total_tokens"])
}

// ----- Async Operation Helpers -----

// SafeVDBOperationWithRetry safely executes VDB operations with retry mechanism
func SafeVDBOperationWithRetry(
	ctx context.Context,
	operation func() error,
	operationName string,
	entityName string,
	maxRetries int,
	retryDelay time.Duration,
) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := operation(); err != nil {
				if attempt >= maxRetries-1 {
					return fmt.Errorf("VDB %s failed for %s after %d attempts: %w",
						operationName, entityName, maxRetries, err)
				}
				Logger.Warnf("VDB %s attempt %d failed for %s: %v, retrying...",
					operationName, attempt+1, entityName, err)
				if retryDelay > 0 {
					time.Sleep(retryDelay)
				}
			} else {
				return nil
			}
		}
	}
	return nil
}

// ----- Weighted Polling Selection -----

// EntityOrRelation represents an entity or relation with sorted chunks
type EntityOrRelation struct {
	SortedChunks []string
}

// PickByWeightedPolling implements linear gradient weighted polling for chunk selection
func PickByWeightedPolling(
	entitiesOrRelations []EntityOrRelation,
	maxRelatedChunks int,
	minRelatedChunks int,
) []string {
	if len(entitiesOrRelations) == 0 {
		return []string{}
	}

	n := len(entitiesOrRelations)
	if n == 1 {
		chunks := entitiesOrRelations[0].SortedChunks
		if len(chunks) > maxRelatedChunks {
			return chunks[:maxRelatedChunks]
		}
		return chunks
	}

	// Calculate expected chunk count for each position (linear decrease)
	expectedCounts := make([]int, n)
	for i := 0; i < n; i++ {
		ratio := float64(i) / float64(n-1)
		expected := float64(maxRelatedChunks) - ratio*float64(maxRelatedChunks-minRelatedChunks)
		expectedCounts[i] = int(math.Round(expected))
	}

	// First round allocation
	selectedChunks := []string{}
	usedCounts := make([]int, n)
	totalRemaining := 0

	for i, entityRel := range entitiesOrRelations {
		entityChunks := entityRel.SortedChunks
		expected := expectedCounts[i]

		actual := expected
		if len(entityChunks) < actual {
			actual = len(entityChunks)
		}
		selectedChunks = append(selectedChunks, entityChunks[:actual]...)
		usedCounts[i] = actual

		remaining := expected - actual
		if remaining > 0 {
			totalRemaining += remaining
		}
	}

	// Second round allocation: distribute remaining quotas
	for j := 0; j < totalRemaining; j++ {
		allocated := false
		for i, entityRel := range entitiesOrRelations {
			entityChunks := entityRel.SortedChunks
			if usedCounts[i] < len(entityChunks) {
				selectedChunks = append(selectedChunks, entityChunks[usedCounts[i]])
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

// ----- Cache Data -----

// CacheData represents data to be cached
type CacheData struct {
	ArgsHash   string
	Content    string
	Prompt     string
	Mode       string
	CacheType  string
	ChunkID    string
	QueryParam map[string]interface{}
}

// NewCacheData creates a new CacheData instance with defaults
func NewCacheData(argsHash, content, prompt string) *CacheData {
	return &CacheData{
		ArgsHash:  argsHash,
		Content:   content,
		Prompt:    prompt,
		Mode:      "default",
		CacheType: "query",
	}
}

// ----- Utility Functions -----

// ExistsFunc checks if a method exists on an object
func ExistsFunc(obj interface{}, funcName string) bool {
	// In Go, this would typically be done via interfaces or reflection
	// For now, return false as a placeholder
	return false
}

// CheckStorageEnvVars checks if all required environment variables exist
func CheckStorageEnvVars(storageName string, requirements map[string][]string) error {
	requiredVars, ok := requirements[storageName]
	if !ok {
		return nil
	}

	var missingVars []string
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			missingVars = append(missingVars, v)
		}
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("storage implementation '%s' requires the following environment variables: %s",
			storageName, strings.Join(missingVars, ", "))
	}

	return nil
}
