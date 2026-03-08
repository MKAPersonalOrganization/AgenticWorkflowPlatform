package skills

import (
	"context"
	"encoding/json"
	"testing"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/skillstore"
	"school-with-ai/platform/toolregistry"
)

// ─── Mock Skill Resolver ────────────────────────────────────────────────────

type mockSkillResolver struct {
	skills []skillstore.UserSkill
}

func (m *mockSkillResolver) ListSkillsForAgent(_ context.Context, _, _ string) ([]skillstore.UserSkill, error) {
	return m.skills, nil
}

func (m *mockSkillResolver) GetSkillByNameForAgent(_ context.Context, _, _, name string) (*skillstore.UserSkill, error) {
	for i := range m.skills {
		if m.skills[i].Name == name {
			return &m.skills[i], nil
		}
	}
	return nil, nil
}

// ─── Provider Tests ─────────────────────────────────────────────────────────

func TestProvider_ImplementsPluginProvider(t *testing.T) {
	var _ toolregistry.PluginProvider = (*Provider)(nil)
}

func TestProvider_ID(t *testing.T) {
	p := &Provider{}
	if p.ID() != "skills" {
		t.Errorf("expected ID 'skills', got %q", p.ID())
	}
}

func TestProvider_DefaultSets(t *testing.T) {
	p := &Provider{}
	sets := p.DefaultSets()
	if len(sets) != 1 {
		t.Fatalf("expected 1 tool set, got %d", len(sets))
	}
	if sets[0].ID != "skills-set" {
		t.Errorf("expected set ID 'skills-set', got %q", sets[0].ID)
	}
}

func TestProvider_DefaultTools(t *testing.T) {
	p := &Provider{}
	tools := p.DefaultTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].ID != "list_skills" {
		t.Errorf("expected first tool ID 'list_skills', got %q", tools[0].ID)
	}
	if tools[1].ID != "activate_skill" {
		t.Errorf("expected second tool ID 'activate_skill', got %q", tools[1].ID)
	}
}

func TestProvider_CreateExecutable(t *testing.T) {
	resolver := &mockSkillResolver{}
	p := &Provider{SkillResolver: resolver}
	tool, err := p.CreateExecutable(context.Background(), "activate_skill", toolregistry.FactoryDeps{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name() != "activate_skill" {
		t.Errorf("expected tool name 'activate_skill', got %q", tool.Name())
	}
}

func TestProvider_CreateExecutable_UnknownTool(t *testing.T) {
	p := &Provider{}
	tool, err := p.CreateExecutable(context.Background(), "unknown_tool", toolregistry.FactoryDeps{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool != nil {
		t.Error("expected nil tool for unknown tool ID")
	}
}

// ─── ActivateSkillTool Tests ────────────────────────────────────────────────

func TestActivateSkillTool_Schema(t *testing.T) {
	tool := &ActivateSkillTool{}
	schema := tool.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if schema.Type != agent.TypeObject {
		t.Errorf("expected schema type 'object', got %q", schema.Type)
	}
	nameProp, ok := schema.Properties["name"]
	if !ok {
		t.Fatal("expected 'name' property in schema")
	}
	if nameProp.Type != agent.TypeString {
		t.Errorf("expected name type 'string', got %q", nameProp.Type)
	}
}

func TestActivateSkillTool_Execute_Success(t *testing.T) {
	resolver := &mockSkillResolver{
		skills: []skillstore.UserSkill{
			{
				ID:           "skill-1",
				Name:         "Statistics Expert",
				Description:  "Advanced statistical analysis",
				Instructions: "You are a statistics specialist.",
				Category:     "Math & Science",
			},
		},
	}
	tool := &ActivateSkillTool{resolver: resolver}
	argsJSON, _ := json.Marshal(map[string]string{"name": "Statistics Expert"})
	args := agent.Arguments{
		Raw:    argsJSON,
		UserID: "user-1",
		Metadata: map[string]any{
			"agent_id": "agent-1",
		},
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result content")
	}

	var parsed activateSkillResult
	if err := json.Unmarshal([]byte(result.Content[0].(string)), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed.Status != "activated" {
		t.Errorf("expected status 'activated', got %q", parsed.Status)
	}
	if parsed.SkillName != "Statistics Expert" {
		t.Errorf("expected skill name 'Statistics Expert', got %q", parsed.SkillName)
	}
}

func TestActivateSkillTool_Execute_CaseInsensitive(t *testing.T) {
	resolver := &mockSkillResolver{
		skills: []skillstore.UserSkill{
			{
				ID:           "skill-1",
				Name:         "Statistics Expert",
				Description:  "Advanced statistical analysis",
				Instructions: "You are a statistics specialist.",
				Category:     "Math & Science",
			},
		},
	}
	tool := &ActivateSkillTool{resolver: resolver}
	argsJSON, _ := json.Marshal(map[string]string{"name": "statistics expert"})
	args := agent.Arguments{
		Raw:    argsJSON,
		UserID: "user-1",
		Metadata: map[string]any{
			"agent_id": "agent-1",
		},
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
}

func TestActivateSkillTool_Execute_NotFound(t *testing.T) {
	resolver := &mockSkillResolver{
		skills: []skillstore.UserSkill{
			{
				ID:   "skill-1",
				Name: "Statistics Expert",
			},
		},
	}
	tool := &ActivateSkillTool{resolver: resolver}
	argsJSON, _ := json.Marshal(map[string]string{"name": "Nonexistent Skill"})
	args := agent.Arguments{
		Raw:    argsJSON,
		UserID: "user-1",
		Metadata: map[string]any{
			"agent_id": "agent-1",
		},
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for nonexistent skill")
	}
}

func TestActivateSkillTool_Execute_EmptyName(t *testing.T) {
	tool := &ActivateSkillTool{resolver: &mockSkillResolver{}}
	argsJSON, _ := json.Marshal(map[string]string{"name": ""})
	args := agent.Arguments{
		Raw:    argsJSON,
		UserID: "user-1",
		Metadata: map[string]any{
			"agent_id": "agent-1",
		},
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for empty skill name")
	}
}

func TestActivateSkillTool_Execute_NoResolver(t *testing.T) {
	tool := &ActivateSkillTool{resolver: nil}
	argsJSON, _ := json.Marshal(map[string]string{"name": "Statistics Expert"})
	args := agent.Arguments{
		Raw:    argsJSON,
		UserID: "user-1",
		Metadata: map[string]any{
			"agent_id": "agent-1",
		},
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when resolver is nil")
	}
}

func TestActivateSkillTool_Execute_NoAgentID(t *testing.T) {
	tool := &ActivateSkillTool{resolver: &mockSkillResolver{}}
	argsJSON, _ := json.Marshal(map[string]string{"name": "Statistics Expert"})
	args := agent.Arguments{
		Raw:    argsJSON,
		UserID: "user-1",
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when agent ID is missing")
	}
}

func TestActivateSkillTool_Execute_NoSkillsAvailable(t *testing.T) {
	resolver := &mockSkillResolver{skills: nil}
	tool := &ActivateSkillTool{resolver: resolver}
	argsJSON, _ := json.Marshal(map[string]string{"name": "Anything"})
	args := agent.Arguments{
		Raw:    argsJSON,
		UserID: "user-1",
		Metadata: map[string]any{
			"agent_id": "agent-1",
		},
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when no skills are available")
	}
}

func TestActivateSkillResult_ContainsXMLInstructions(t *testing.T) {
	skill := &skillstore.UserSkill{
		Name:         "Test Skill",
		Description:  "A test skill",
		Category:     "Test",
		Instructions: "Follow these test instructions.",
	}
	result := buildActivationResult(skill)

	if result.Status != "activated" {
		t.Errorf("expected status 'activated', got %q", result.Status)
	}
	if result.SkillName != "Test Skill" {
		t.Errorf("expected skill name 'Test Skill', got %q", result.SkillName)
	}

	// Verify XML wrapping (Gemini CLI pattern)
	if !contains(result.Instructions, "<activated_skill") {
		t.Error("expected XML-wrapped instructions with <activated_skill> tag")
	}
	if !contains(result.Instructions, "Follow these test instructions.") {
		t.Error("expected skill instructions in output")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ───────────────────────────────────────────────────────────────────────────
// list_skills tool tests
// ───────────────────────────────────────────────────────────────────────────

func TestListSkillsTool_Schema(t *testing.T) {
	tool := &ListSkillsTool{}
	if tool.Name() != "list_skills" {
		t.Errorf("expected name 'list_skills', got %q", tool.Name())
	}
	schema := tool.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if schema.Type != agent.TypeObject {
		t.Errorf("expected schema type 'object', got %q", schema.Type)
	}
}

func TestListSkillsTool_Execute_Success(t *testing.T) {
	resolver := &mockSkillResolver{
		skills: []skillstore.UserSkill{
			{Name: "Statistics Expert", Description: "Stats help", Category: "Data Science", Icon: "📊", IsCustom: false},
			{Name: "Code Review", Description: "Review code", Category: "Engineering", Icon: "🔍", IsCustom: true},
		},
	}
	tool := &ListSkillsTool{resolver: resolver}
	args := agent.Arguments{
		Raw:      json.RawMessage(`{}`),
		UserID:   "user-1",
		Metadata: map[string]any{"agent_id": "agent-1"},
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}

	var parsed listSkillsResult
	if err := json.Unmarshal([]byte(result.Content[0].(string)), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", parsed.Status)
	}
	if parsed.Count != 2 {
		t.Errorf("expected 2 skills, got %d", parsed.Count)
	}
	if parsed.Skills[0].Name != "Statistics Expert" {
		t.Errorf("expected first skill 'Statistics Expert', got %q", parsed.Skills[0].Name)
	}
	if parsed.Skills[1].IsCustom != true {
		t.Error("expected second skill to be custom")
	}
}

func TestListSkillsTool_Execute_Empty(t *testing.T) {
	resolver := &mockSkillResolver{skills: []skillstore.UserSkill{}}
	tool := &ListSkillsTool{resolver: resolver}
	args := agent.Arguments{
		Raw:      json.RawMessage(`{}`),
		UserID:   "user-1",
		Metadata: map[string]any{"agent_id": "agent-1"},
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed listSkillsResult
	if err := json.Unmarshal([]byte(result.Content[0].(string)), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed.Count != 0 {
		t.Errorf("expected 0 skills, got %d", parsed.Count)
	}
}

func TestListSkillsTool_Execute_NoResolver(t *testing.T) {
	tool := &ListSkillsTool{}
	args := agent.Arguments{
		Raw:      json.RawMessage(`{}`),
		UserID:   "user-1",
		Metadata: map[string]any{"agent_id": "agent-1"},
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error result when resolver is nil")
	}
}

func TestListSkillsTool_Execute_NoAgentID(t *testing.T) {
	resolver := &mockSkillResolver{}
	tool := &ListSkillsTool{resolver: resolver}
	args := agent.Arguments{
		Raw:    json.RawMessage(`{}`),
		UserID: "user-1",
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error result when agent ID is missing")
	}
}

func TestProvider_CreateExecutable_ListSkills(t *testing.T) {
	resolver := &mockSkillResolver{}
	p := &Provider{SkillResolver: resolver}
	tool, err := p.CreateExecutable(context.Background(), "list_skills", toolregistry.FactoryDeps{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name() != "list_skills" {
		t.Errorf("expected tool name 'list_skills', got %q", tool.Name())
	}
}
