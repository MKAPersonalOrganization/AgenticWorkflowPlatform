package askuser

import (
	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"
	"encoding/json"
	"testing"
)

func TestAskUserTool_Name(t *testing.T) {
	tool := &AskUserTool{}
	if got := tool.Name(); got != "ask_user" {
		t.Errorf("Name() = %q, want %q", got, "ask_user")
	}
}

func TestAskUserTool_Description(t *testing.T) {
	tool := &AskUserTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestAskUserTool_Schema(t *testing.T) {
	tool := &AskUserTool{}
	schema := tool.Schema()
	if schema == nil {
		t.Fatal("Schema() returned nil")
	}
	if schema.Type != agent.TypeObject {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, agent.TypeObject)
	}
	questions, ok := schema.Properties["questions"]
	if !ok {
		t.Fatal("Schema() missing 'questions' property")
	}
	if questions.Type != agent.TypeArray {
		t.Errorf("questions.Type = %q, want %q", questions.Type, agent.TypeArray)
	}
	if questions.Items == nil {
		t.Fatal("questions.Items is nil")
	}
	// Verify question items have required fields
	itemProps := questions.Items.Properties
	for _, field := range []string{"question", "header", "type"} {
		if _, ok := itemProps[field]; !ok {
			t.Errorf("question item missing property %q", field)
		}
	}
}

func TestAskUserTool_Execute_NoQuestions(t *testing.T) {
	tool := &AskUserTool{}
	args := map[string]any{"questions": []any{}}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(t.Context(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for empty questions, got nil")
	}
}

func TestAskUserTool_Execute_TooManyQuestions(t *testing.T) {
	tool := &AskUserTool{}
	questions := make([]map[string]any, 5)
	for i := range questions {
		questions[i] = map[string]any{
			"question": "q?",
			"header":   "h",
			"type":     "text",
		}
	}
	args := map[string]any{"questions": questions}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(t.Context(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for >4 questions, got nil")
	}
}

func TestAskUserTool_Execute_MissingQuestionText(t *testing.T) {
	tool := &AskUserTool{}
	args := map[string]any{
		"questions": []map[string]any{
			{"question": "", "header": "Test", "type": "text"},
		},
	}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(t.Context(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for empty question text, got nil")
	}
}

func TestAskUserTool_Execute_MissingHeader(t *testing.T) {
	tool := &AskUserTool{}
	args := map[string]any{
		"questions": []map[string]any{
			{"question": "What?", "header": "", "type": "text"},
		},
	}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(t.Context(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for empty header, got nil")
	}
}

func TestAskUserTool_Execute_InvalidType(t *testing.T) {
	tool := &AskUserTool{}
	args := map[string]any{
		"questions": []map[string]any{
			{"question": "What?", "header": "H", "type": "invalid"},
		},
	}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(t.Context(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
}

func TestAskUserTool_Execute_ChoiceTooFewOptions(t *testing.T) {
	tool := &AskUserTool{}
	args := map[string]any{
		"questions": []map[string]any{
			{
				"question": "Which?",
				"header":   "Choice",
				"type":     "choice",
				"options": []map[string]string{
					{"label": "A", "description": "First"},
				},
			},
		},
	}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(t.Context(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for <2 options, got nil")
	}
}

func TestAskUserTool_Execute_ChoiceTooManyOptions(t *testing.T) {
	tool := &AskUserTool{}
	opts := make([]map[string]string, 5)
	for i := range opts {
		opts[i] = map[string]string{"label": "opt", "description": "d"}
	}
	args := map[string]any{
		"questions": []map[string]any{
			{
				"question": "Which?",
				"header":   "Choice",
				"type":     "choice",
				"options":  opts,
			},
		},
	}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(t.Context(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for >4 options, got nil")
	}
}

func TestAskUserTool_Execute_ChoiceEmptyLabel(t *testing.T) {
	tool := &AskUserTool{}
	args := map[string]any{
		"questions": []map[string]any{
			{
				"question": "Which?",
				"header":   "Choice",
				"type":     "choice",
				"options": []map[string]string{
					{"label": "A", "description": "First"},
					{"label": "", "description": "Second"},
				},
			},
		},
	}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(t.Context(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for empty option label, got nil")
	}
}

func TestAskUserTool_Execute_InvalidJSON(t *testing.T) {
	tool := &AskUserTool{}
	result, err := tool.Execute(t.Context(), agent.Arguments{Raw: []byte(`{invalid}`)})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// =============================================================================
// Provider tests
// =============================================================================

func TestProvider_Interface(t *testing.T) {
	var p toolregistry.PluginProvider = &Provider{}
	_ = p // Compile-time check
}

func TestProvider_ID(t *testing.T) {
	p := &Provider{}
	if got := p.ID(); got != "askuser" {
		t.Errorf("ID() = %q, want %q", got, "askuser")
	}
}

func TestProvider_Icon(t *testing.T) {
	p := &Provider{}
	icon := p.Icon()
	if icon == "" {
		t.Error("Icon() returned empty string")
	}
}

func TestProvider_DefaultSets(t *testing.T) {
	p := &Provider{}
	sets := p.DefaultSets()
	if len(sets) != 1 {
		t.Fatalf("DefaultSets() len = %d, want 1", len(sets))
	}
	if sets[0].ID != "askuser-set" {
		t.Errorf("DefaultSets()[0].ID = %q, want %q", sets[0].ID, "askuser-set")
	}
}

func TestProvider_DefaultTools(t *testing.T) {
	p := &Provider{}
	tools := p.DefaultTools()
	if len(tools) != 1 {
		t.Fatalf("DefaultTools() len = %d, want 1", len(tools))
	}
	if tools[0].ID != "ask_user" {
		t.Errorf("DefaultTools()[0].ID = %q, want %q", tools[0].ID, "ask_user")
	}
}

func TestProvider_CreateExecutable(t *testing.T) {
	p := &Provider{}
	tool, err := p.CreateExecutable(t.Context(), "ask_user", nil)
	if err != nil {
		t.Fatalf("CreateExecutable() error = %v", err)
	}
	if tool == nil {
		t.Fatal("CreateExecutable('ask_user') returned nil")
	}
	if tool.Name() != "ask_user" {
		t.Errorf("CreateExecutable().Name() = %q, want %q", tool.Name(), "ask_user")
	}
}

func TestProvider_CreateExecutable_Unknown(t *testing.T) {
	p := &Provider{}
	tool, err := p.CreateExecutable(t.Context(), "unknown_tool", nil)
	if err != nil {
		t.Fatalf("CreateExecutable() error = %v", err)
	}
	if tool != nil {
		t.Error("CreateExecutable('unknown_tool') should return nil")
	}
}

func TestProvider_UIHandledTools(t *testing.T) {
	p := &Provider{}
	handled := p.UIHandledTools()
	if len(handled) != 1 || handled[0] != "ask_user" {
		t.Errorf("UIHandledTools() = %v, want [\"ask_user\"]", handled)
	}
}

func TestProvider_GetUITemplate(t *testing.T) {
	p := &Provider{}
	tmpl := p.GetUITemplate()
	if tmpl == nil {
		t.Fatal("GetUITemplate() returned nil")
	}
}

// =============================================================================
// Type tests
// =============================================================================

func TestQuestionTypes(t *testing.T) {
	if QuestionTypeChoice != "choice" {
		t.Errorf("QuestionTypeChoice = %q, want %q", QuestionTypeChoice, "choice")
	}
	if QuestionTypeText != "text" {
		t.Errorf("QuestionTypeText = %q, want %q", QuestionTypeText, "text")
	}
	if QuestionTypeYesNo != "yesno" {
		t.Errorf("QuestionTypeYesNo = %q, want %q", QuestionTypeYesNo, "yesno")
	}
}
