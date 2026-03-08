package websearch

import (
	"context"
	"encoding/json"
	"testing"

	"school-with-ai/foundation/agent"
	"school-with-ai/platform/toolregistry"
)

// =========================================================================
// Tool identity tests
// =========================================================================

func TestWebSearchTool_Name(t *testing.T) {
	tool := NewWebSearchTool("test-key", "")
	if got := tool.Name(); got != "web_search" {
		t.Errorf("Name() = %q, want %q", got, "web_search")
	}
}

func TestWebSearchTool_Schema(t *testing.T) {
	tool := NewWebSearchTool("test-key", "")
	schema := tool.Schema()
	if schema == nil {
		t.Fatal("Schema() returned nil")
	}
	if schema.Type != agent.TypeObject {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, agent.TypeObject)
	}
	if _, ok := schema.Properties["query"]; !ok {
		t.Error("Schema() missing 'query' property")
	}
	if len(schema.Required) != 1 || schema.Required[0] != "query" {
		t.Errorf("Schema().Required = %v, want [query]", schema.Required)
	}
}

func TestWebSearchTool_DefaultModel(t *testing.T) {
	tool := NewWebSearchTool("test-key", "")
	if tool.model != defaultSearchModel {
		t.Errorf("default model = %q, want %q", tool.model, defaultSearchModel)
	}
}

func TestWebSearchTool_CustomModel(t *testing.T) {
	tool := NewWebSearchTool("test-key", "gemini-2.0-pro")
	if tool.model != "gemini-2.0-pro" {
		t.Errorf("model = %q, want %q", tool.model, "gemini-2.0-pro")
	}
}

// =========================================================================
// Validation tests (no API call needed)
// =========================================================================

func TestWebSearchTool_Execute_EmptyQuery(t *testing.T) {
	tool := NewWebSearchTool("test-key", "")
	args := WebSearchArgs{Query: ""}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Error("expected validation error for empty query")
	}
}

func TestWebSearchTool_Execute_WhitespaceQuery(t *testing.T) {
	tool := NewWebSearchTool("test-key", "")
	args := WebSearchArgs{Query: "   \t  "}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Error("expected validation error for whitespace-only query")
	}
}

func TestWebSearchTool_Execute_QueryTooLong(t *testing.T) {
	tool := NewWebSearchTool("test-key", "")
	longQuery := make([]byte, maxQueryLength+1)
	for i := range longQuery {
		longQuery[i] = 'a'
	}
	args := WebSearchArgs{Query: string(longQuery)}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Error("expected validation error for query exceeding max length")
	}
}

func TestWebSearchTool_Execute_InvalidJSON(t *testing.T) {
	tool := NewWebSearchTool("test-key", "")
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: json.RawMessage(`{invalid}`)})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for invalid JSON arguments")
	}
}

func TestWebSearchTool_Execute_MissingAPIKey(t *testing.T) {
	tool := NewWebSearchTool("", "")
	args := WebSearchArgs{Query: "what is Go programming language"}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: raw, UserID: "test-user-nokey"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for missing API key")
	}
}

// =========================================================================
// Rate limiter tests
// =========================================================================

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := newRateLimiter()
	for i := 0; i < rateLimitMax; i++ {
		if !rl.allow("user-1") {
			t.Fatalf("rate limiter denied request %d (should allow up to %d)", i+1, rateLimitMax)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := newRateLimiter()
	for i := 0; i < rateLimitMax; i++ {
		rl.allow("user-2")
	}
	if rl.allow("user-2") {
		t.Error("rate limiter should deny request over limit")
	}
}

func TestRateLimiter_SeparateUsers(t *testing.T) {
	rl := newRateLimiter()
	for i := 0; i < rateLimitMax; i++ {
		rl.allow("user-3")
	}
	if !rl.allow("user-4") {
		t.Error("rate limiter should not block different users")
	}
}

// =========================================================================
// Response parsing tests (unit tests for helper functions)
// =========================================================================

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		url    string
		domain string
	}{
		{"https://www.example.com/path?q=1", "www.example.com"},
		{"http://docs.go.dev/doc/tutorial", "docs.go.dev"},
		{"https://en.wikipedia.org/wiki/Go_(programming_language)", "en.wikipedia.org"},
		{"ftp://files.example.org/data", "files.example.org"},
		{"example.com", "example.com"},
	}
	for _, tc := range tests {
		got := extractDomain(tc.url)
		if got != tc.domain {
			t.Errorf("extractDomain(%q) = %q, want %q", tc.url, got, tc.domain)
		}
	}
}

func TestInsertCitationMarkers_NoCitations(t *testing.T) {
	text := "Hello world"
	got := insertCitationMarkers(text, nil)
	if got != text {
		t.Errorf("insertCitationMarkers with no citations should return original text, got %q", got)
	}
}

func TestInsertCitationMarkers_SingleCitation(t *testing.T) {
	text := "Go is a programming language."
	citations := []Citation{
		{Text: "Go is a programming language.", SourceIndices: []int{1}, StartIndex: 0, EndIndex: 29},
	}
	got := insertCitationMarkers(text, citations)
	expected := "Go is a programming language.[1]"
	if got != expected {
		t.Errorf("insertCitationMarkers = %q, want %q", got, expected)
	}
}

func TestInsertCitationMarkers_MultipleSources(t *testing.T) {
	text := "Go is fast."
	citations := []Citation{
		{Text: "Go is fast.", SourceIndices: []int{1, 3}, StartIndex: 0, EndIndex: 11},
	}
	got := insertCitationMarkers(text, citations)
	expected := "Go is fast.[1][3]"
	if got != expected {
		t.Errorf("insertCitationMarkers = %q, want %q", got, expected)
	}
}

// =========================================================================
// Provider interface tests
// =========================================================================

func TestProvider_Interface(t *testing.T) {
	p := &Provider{}
	if got := p.ID(); got != "websearch" {
		t.Errorf("ID() = %q, want %q", got, "websearch")
	}
	if icon := p.Icon(); icon == "" {
		t.Error("Icon() returned empty string")
	}
	sets := p.DefaultSets()
	if len(sets) != 1 {
		t.Fatalf("DefaultSets() returned %d sets, want 1", len(sets))
	}
	if sets[0].ID != "websearch-set" {
		t.Errorf("DefaultSets()[0].ID = %q, want %q", sets[0].ID, "websearch-set")
	}
	if sets[0].Category != "Research" {
		t.Errorf("DefaultSets()[0].Category = %q, want %q", sets[0].Category, "Research")
	}
	tools := p.DefaultTools()
	if len(tools) != 1 {
		t.Fatalf("DefaultTools() returned %d tools, want 1", len(tools))
	}
	if tools[0].ID != "web_search" {
		t.Errorf("DefaultTools()[0].ID = %q, want %q", tools[0].ID, "web_search")
	}
	handled := p.UIHandledTools()
	if len(handled) != 1 || handled[0] != "web_search" {
		t.Errorf("UIHandledTools() = %v, want [web_search]", handled)
	}

	// SettingsSchema
	schema := p.SettingsSchema()
	if len(schema) != 2 {
		t.Fatalf("SettingsSchema() returned %d fields, want 2", len(schema))
	}
	if schema[0].Key != "api_key" || schema[0].Type != toolregistry.FieldTypePassword || !schema[0].Required {
		t.Errorf("SettingsSchema()[0] = %+v, want api_key/password/required", schema[0])
	}
	if schema[1].Key != "model" || schema[1].Type != toolregistry.FieldTypeSelect {
		t.Errorf("SettingsSchema()[1] = %+v, want model/select", schema[1])
	}
}

func TestProvider_CreateExecutable(t *testing.T) {
	p := &Provider{}
	ctx := context.Background()

	// With plugin settings configured
	tool, err := p.CreateExecutable(ctx, "web_search", toolregistry.FactoryDeps{
		PluginSettings: map[string]map[string]string{
			"websearch": {"api_key": "test-key", "model": "gemini-2.0-flash"},
		},
	})
	if err != nil {
		t.Fatalf("CreateExecutable(web_search) error = %v", err)
	}
	if tool == nil {
		t.Fatal("CreateExecutable(web_search) returned nil")
	}
	wst, ok := tool.(*WebSearchTool)
	if !ok {
		t.Fatal("CreateExecutable(web_search) returned wrong type")
	}
	if wst.apiKey != "test-key" {
		t.Errorf("apiKey = %q, want %q", wst.apiKey, "test-key")
	}
	if wst.model != "gemini-2.0-flash" {
		t.Errorf("model = %q, want %q", wst.model, "gemini-2.0-flash")
	}

	// Without plugin settings (no settings configured)
	tool2, err := p.CreateExecutable(ctx, "web_search", toolregistry.FactoryDeps{})
	if err != nil {
		t.Fatalf("CreateExecutable(web_search) no-settings error = %v", err)
	}
	if tool2 == nil {
		t.Fatal("CreateExecutable(web_search) no-settings returned nil")
	}
	wst2 := tool2.(*WebSearchTool)
	if wst2.apiKey != "" {
		t.Errorf("apiKey = %q, want empty", wst2.apiKey)
	}
	if wst2.model != defaultSearchModel {
		t.Errorf("model = %q, want %q", wst2.model, defaultSearchModel)
	}

	// Unknown tool
	tool, err = p.CreateExecutable(ctx, "unknown_tool", toolregistry.FactoryDeps{})
	if err != nil {
		t.Fatalf("CreateExecutable(unknown_tool) error = %v", err)
	}
	if tool != nil {
		t.Error("CreateExecutable(unknown_tool) should return nil")
	}
}
