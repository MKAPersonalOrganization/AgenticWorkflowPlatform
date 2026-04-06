package websearch

import (
	"context"
	"encoding/json"
	"testing"

	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"
)

// =========================================================================
// Test doubles
// =========================================================================

// mockDeps implements toolregistry.FactoryDeps for testing.
type mockDeps struct {
	toolregistry.FactoryDeps
	settings map[string]map[string]string
}

func (m *mockDeps) PluginSettings(pluginID string) map[string]string {
	if m.settings == nil {
		return nil
	}
	return m.settings[pluginID]
}

// stubClient is a GroundedSearchClient test double.
type stubClient struct {
	configured bool
	result     *WebSearchResult
	err        error
	// capture last call for assertion
	lastQuery string
}

func (s *stubClient) Configured() bool { return s.configured }

func (s *stubClient) Search(_ context.Context, query string) (*WebSearchResult, error) {
	s.lastQuery = query
	return s.result, s.err
}

// stubFactory returns a ClientFactory that records the apiKey and model it
// receives and always vends the given stubClient.
type stubFactory struct {
	client   GroundedSearchClient
	apiKey   string
	model    string
	defaults string // the default model string expected when none is supplied
}

func (f *stubFactory) factory(apiKey, model string) GroundedSearchClient {
	f.apiKey = apiKey
	f.model = model
	return f.client
}

// =========================================================================
// Tool identity tests
// =========================================================================

func TestWebSearchTool_Name(t *testing.T) {
	tool := NewWebSearchTool(&stubClient{configured: true})
	if got := tool.Name(); got != "web_search" {
		t.Errorf("Name() = %q, want %q", got, "web_search")
	}
}

func TestWebSearchTool_Schema(t *testing.T) {
	tool := NewWebSearchTool(&stubClient{configured: true})
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

// =========================================================================
// Validation tests (no API call needed)
// =========================================================================

func TestWebSearchTool_Execute_EmptyQuery(t *testing.T) {
	tool := NewWebSearchTool(&stubClient{configured: true})
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
	tool := NewWebSearchTool(&stubClient{configured: true})
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
	tool := NewWebSearchTool(&stubClient{configured: true})
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
	tool := NewWebSearchTool(&stubClient{configured: true})
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: json.RawMessage(`{invalid}`)})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for invalid JSON arguments")
	}
}

func TestWebSearchTool_Execute_MissingAPIKey(t *testing.T) {
	tool := NewWebSearchTool(&stubClient{configured: false})
	args := WebSearchArgs{Query: "what is Go programming language"}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: raw, UserID: "test-user-nokey"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for unconfigured client (missing API key)")
	}
}

// =========================================================================
// Rate limiter tests
// =========================================================================

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := newRateLimiter()
	for i := range rateLimitMax {
		if !rl.allow("user-1") {
			t.Fatalf("rate limiter denied request %d (should allow up to %d)", i+1, rateLimitMax)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := newRateLimiter()
	for range rateLimitMax {
		rl.allow("user-2")
	}
	if rl.allow("user-2") {
		t.Error("rate limiter should deny request over limit")
	}
}

func TestRateLimiter_SeparateUsers(t *testing.T) {
	rl := newRateLimiter()
	for range rateLimitMax {
		rl.allow("user-3")
	}
	if !rl.allow("user-4") {
		t.Error("rate limiter should not block different users")
	}
}

// =========================================================================
// Citation helper tests
// =========================================================================

func TestInsertCitationMarkers_NoCitations(t *testing.T) {
	text := "Hello world"
	got := InsertCitationMarkers(text, nil)
	if got != text {
		t.Errorf("InsertCitationMarkers with no citations should return original text, got %q", got)
	}
}

func TestInsertCitationMarkers_SingleCitation(t *testing.T) {
	text := "Go is a programming language."
	citations := []Citation{
		{Text: "Go is a programming language.", SourceIndices: []int{1}, StartIndex: 0, EndIndex: 29},
	}
	got := InsertCitationMarkers(text, citations)
	expected := "Go is a programming language.[1]"
	if got != expected {
		t.Errorf("InsertCitationMarkers = %q, want %q", got, expected)
	}
}

func TestInsertCitationMarkers_MultipleSources(t *testing.T) {
	text := "Go is fast."
	citations := []Citation{
		{Text: "Go is fast.", SourceIndices: []int{1, 3}, StartIndex: 0, EndIndex: 11},
	}
	got := InsertCitationMarkers(text, citations)
	expected := "Go is fast.[1][3]"
	if got != expected {
		t.Errorf("InsertCitationMarkers = %q, want %q", got, expected)
	}
}

// =========================================================================
// Provider interface tests
// =========================================================================

func TestProvider_Interface(t *testing.T) {
	p := NewProvider(func(_, _ string) GroundedSearchClient { return &stubClient{} })
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
	sf := &stubFactory{client: &stubClient{configured: true}}
	p := NewProvider(sf.factory)
	ctx := context.Background()

	// With plugin settings configured
	tool, err := p.CreateExecutable(ctx, "web_search", &mockDeps{
		settings: map[string]map[string]string{
			"websearch": {"api_key": "test-key", "model": "gemini-2.0-flash"},
		},
	})
	if err != nil {
		t.Fatalf("CreateExecutable(web_search) error = %v", err)
	}
	if tool == nil {
		t.Fatal("CreateExecutable(web_search) returned nil")
	}
	if _, ok := tool.(*WebSearchTool); !ok {
		t.Fatal("CreateExecutable(web_search) returned wrong type")
	}
	if sf.apiKey != "test-key" {
		t.Errorf("factory received apiKey = %q, want %q", sf.apiKey, "test-key")
	}
	if sf.model != "gemini-2.0-flash" {
		t.Errorf("factory received model = %q, want %q", sf.model, "gemini-2.0-flash")
	}

	// Without plugin settings
	sf2 := &stubFactory{client: &stubClient{}}
	p2 := NewProvider(sf2.factory)
	tool2, err := p2.CreateExecutable(ctx, "web_search", &mockDeps{})
	if err != nil {
		t.Fatalf("CreateExecutable(web_search) no-settings error = %v", err)
	}
	if tool2 == nil {
		t.Fatal("CreateExecutable(web_search) no-settings returned nil")
	}
	if sf2.apiKey != "" {
		t.Errorf("factory received apiKey = %q, want empty", sf2.apiKey)
	}
	if sf2.model != "" {
		t.Errorf("factory received model = %q, want empty (factory decides default)", sf2.model)
	}

	// Unknown tool
	tool3, err := p.CreateExecutable(ctx, "unknown_tool", &mockDeps{})
	if err != nil {
		t.Fatalf("CreateExecutable(unknown_tool) error = %v", err)
	}
	if tool3 != nil {
		t.Error("CreateExecutable(unknown_tool) should return nil")
	}
}
