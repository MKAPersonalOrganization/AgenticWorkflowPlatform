package agent

import (
	"context"
	"strings"
	"testing"
)

// mockRequirement returns predefined rules
type mockRequirement struct {
	rules []Rule
	err   error
}

func (m *mockRequirement) Run(ctx context.Context, inv *Invocation) ([]Rule, error) {
	return m.rules, m.err
}

// mockTool implements the Tool interface simply
type mockTool struct {
	name string
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return "" }
func (m *mockTool) Schema() *Schema     { return nil }

// chatContextWithTools creates a Context with tools on the typed ChatOp.Tools field.
func chatContextWithTools(ctx context.Context, tools []Tool, messages []Message) *Context {
	c := NewChatContext(ctx, nil, messages, nil)
	c.Chat.Tools = tools
	return c
}

// --------------------------------------------------------------------------
// Basic middleware behavior
// --------------------------------------------------------------------------

func TestRequirementMiddleware(t *testing.T) {
	ctx := context.Background()

	t.Run("Filters disallowed tools", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{
					{TargetTool: "badTool", Blocked: true},
				},
			},
		}

		middleware := RequirementMiddleware(reqs...)

		tools := []Tool{
			&mockTool{name: "goodTool"},
			&mockTool{name: "badTool"},
		}
		chatCtx := chatContextWithTools(ctx, tools, nil)

		var next HandlerFunc = func(c *Context) error {
			if len(c.Chat.Tools) != 1 || c.Chat.Tools[0].Name() != "goodTool" {
				t.Errorf("expected only goodTool, got %v", c.Chat.Tools)
			}
			return nil
		}

		err := middleware(next).Handle(chatCtx)
		if err != nil {
			t.Fatal("unexpected error")
		}
	})

	t.Run("Injects forced tool instructions", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{
					{TargetTool: "think", Forced: true},
				},
			},
		}

		middleware := RequirementMiddleware(reqs...)

		tools := []Tool{
			&mockTool{name: "think"},
			&mockTool{name: "search"},
		}
		chatCtx := chatContextWithTools(ctx, tools, []Message{})

		var next HandlerFunc = func(c *Context) error {
			if len(c.Chat.Messages) == 0 {
				t.Fatal("expected instruction to be injected")
			}
			text := c.Chat.Messages[0].Content[0].Text
			if !strings.Contains(text, "MUST use one of the following tools immediately: 'think'") {
				t.Errorf("expected forced instruction, got %s", text)
			}
			return nil
		}

		err := middleware(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Injects prevent stop instructions", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{
					{TargetTool: "search", PreventStop: true},
				},
			},
		}

		middleware := RequirementMiddleware(reqs...)

		tools := []Tool{&mockTool{name: "search"}}
		chatCtx := chatContextWithTools(ctx, tools, []Message{})

		var next HandlerFunc = func(c *Context) error {
			if len(c.Chat.Messages) == 0 {
				t.Fatal("expected instruction to be injected")
			}
			text := c.Chat.Messages[0].Content[0].Text
			if !strings.Contains(text, "cannot provide a final answer yet") {
				t.Errorf("expected prevent stop instruction, got %s", text)
			}
			return nil
		}

		err := middleware(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})
}

// --------------------------------------------------------------------------
// Tools on ChatOp.Tools (typed field)
// --------------------------------------------------------------------------

func TestRequirementMiddleware_TypedToolsField(t *testing.T) {
	ctx := context.Background()

	t.Run("forced tool sets ToolChoice and restricts tools", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{
					{TargetTool: "think", Forced: true},
				},
			},
		}

		middleware := RequirementMiddleware(reqs...)

		tools := []Tool{
			&mockTool{name: "think"},
			&mockTool{name: "search"},
		}
		chatCtx := chatContextWithTools(ctx, tools, []Message{})

		var next HandlerFunc = func(c *Context) error {
			// Verify system message
			if len(c.Chat.Messages) == 0 {
				t.Fatal("expected system instruction injected for forced tool")
			}
			text := c.Chat.Messages[0].Content[0].Text
			if !strings.Contains(text, "'think'") {
				t.Errorf("expected forced think instruction, got: %s", text)
			}

			// Verify ToolChoice
			if c.Chat.Options == nil || c.Chat.Options.ToolChoice != "think" {
				t.Error("expected ToolChoice='think' on ChatOptions")
			}

			// Verify tools restricted to forced only
			if len(c.Chat.Tools) != 1 || c.Chat.Tools[0].Name() != "think" {
				t.Errorf("expected only 'think' tool, got %d tools", len(c.Chat.Tools))
			}
			return nil
		}

		err := middleware(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("filtered tools written back to c.Chat.Tools", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{
					{TargetTool: "badTool", Blocked: true},
				},
			},
		}

		middleware := RequirementMiddleware(reqs...)

		tools := []Tool{
			&mockTool{name: "goodTool"},
			&mockTool{name: "badTool"},
		}
		chatCtx := chatContextWithTools(ctx, tools, []Message{})

		var next HandlerFunc = func(c *Context) error {
			if len(c.Chat.Tools) != 1 || c.Chat.Tools[0].Name() != "goodTool" {
				t.Errorf("expected only goodTool, got %d tools", len(c.Chat.Tools))
			}
			return nil
		}

		err := middleware(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})
}

// --------------------------------------------------------------------------
// ToolChoice enforcement
// --------------------------------------------------------------------------

func TestRequirementMiddleware_ToolChoice(t *testing.T) {
	ctx := context.Background()

	t.Run("single forced tool sets ToolChoice to tool name", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{
					{TargetTool: "think", Forced: true},
				},
			},
		}

		tools := []Tool{&mockTool{name: "think"}, &mockTool{name: "search"}}
		chatCtx := chatContextWithTools(ctx, tools, []Message{})

		var next HandlerFunc = func(c *Context) error {
			if c.Chat.Options.ToolChoice != "think" {
				t.Errorf("expected ToolChoice='think', got '%s'", c.Chat.Options.ToolChoice)
			}
			return nil
		}

		err := RequirementMiddleware(reqs...)(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("multiple forced tools sets ToolChoice to required", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{
					{TargetTool: "think", Forced: true},
					{TargetTool: "search", Forced: true},
				},
			},
		}

		tools := []Tool{&mockTool{name: "think"}, &mockTool{name: "search"}}
		chatCtx := chatContextWithTools(ctx, tools, []Message{})

		var next HandlerFunc = func(c *Context) error {
			if c.Chat.Options.ToolChoice != "required" {
				t.Errorf("expected ToolChoice='required', got '%s'", c.Chat.Options.ToolChoice)
			}
			return nil
		}

		err := RequirementMiddleware(reqs...)(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("prevent stop sets ToolChoice to required", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{
					{TargetTool: "search", PreventStop: true},
				},
			},
		}

		tools := []Tool{&mockTool{name: "search"}}
		chatCtx := chatContextWithTools(ctx, tools, []Message{})

		var next HandlerFunc = func(c *Context) error {
			if c.Chat.Options.ToolChoice != "required" {
				t.Errorf("expected ToolChoice='required', got '%s'", c.Chat.Options.ToolChoice)
			}
			return nil
		}

		err := RequirementMiddleware(reqs...)(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})
}

// --------------------------------------------------------------------------
// Forced tools restrict available tools list
// --------------------------------------------------------------------------

func TestRequirementMiddleware_ForcedToolRestriction(t *testing.T) {
	ctx := context.Background()

	t.Run("forced tool restricts available tools to only forced tool", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{
					{TargetTool: "think", Forced: true},
				},
			},
		}

		tools := []Tool{
			&mockTool{name: "think"},
			&mockTool{name: "search"},
			&mockTool{name: "calculator"},
		}
		chatCtx := chatContextWithTools(ctx, tools, []Message{})

		var next HandlerFunc = func(c *Context) error {
			if len(c.Chat.Tools) != 1 {
				t.Fatalf("expected 1 tool (only forced), got %d", len(c.Chat.Tools))
			}
			if c.Chat.Tools[0].Name() != "think" {
				t.Errorf("expected 'think', got '%s'", c.Chat.Tools[0].Name())
			}
			return nil
		}

		err := RequirementMiddleware(reqs...)(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})
}

// --------------------------------------------------------------------------
// Conflict resolution: forbidden overrides forced
// --------------------------------------------------------------------------

func TestRequirementMiddleware_ForbiddenOverridesForced(t *testing.T) {
	ctx := context.Background()

	t.Run("forbidden by one requirement overrides forced by another", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{{TargetTool: "think", Forced: true}},
			},
			&mockRequirement{
				rules: []Rule{{TargetTool: "think", Blocked: true}},
			},
		}

		tools := []Tool{&mockTool{name: "think"}, &mockTool{name: "search"}}
		chatCtx := chatContextWithTools(ctx, tools, []Message{})

		var next HandlerFunc = func(c *Context) error {
			for _, t2 := range c.Chat.Tools {
				if t2.Name() == "think" {
					t.Error("expected think to be removed (forbidden overrides forced)")
				}
			}
			return nil
		}

		err := RequirementMiddleware(reqs...)(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})
}

// --------------------------------------------------------------------------
// PreventStop processed even when tool not in tools list
// --------------------------------------------------------------------------

func TestRequirementMiddleware_PreventStopWithoutToolInList(t *testing.T) {
	ctx := context.Background()

	t.Run("PreventStop is enforced even if target tool is not in tools list", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{
					{TargetTool: "missing_tool", PreventStop: true},
				},
			},
		}

		tools := []Tool{&mockTool{name: "search"}}
		chatCtx := chatContextWithTools(ctx, tools, []Message{})

		var next HandlerFunc = func(c *Context) error {
			if c.Chat.Options.ToolChoice != "required" {
				t.Errorf("expected ToolChoice='required' for PreventStop, got '%s'", c.Chat.Options.ToolChoice)
			}
			if len(c.Chat.Messages) == 0 {
				t.Fatal("expected prevent stop instruction")
			}
			text := c.Chat.Messages[0].Content[0].Text
			if !strings.Contains(text, "cannot provide a final answer yet") {
				t.Errorf("expected prevent stop instruction, got: %s", text)
			}
			return nil
		}

		err := RequirementMiddleware(reqs...)(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})
}

// --------------------------------------------------------------------------
// Forced tool by name when tool not in tools list
// --------------------------------------------------------------------------

func TestRequirementMiddleware_ForcedToolNotInList(t *testing.T) {
	ctx := context.Background()

	t.Run("forced tool still sets ToolChoice even if not in tools list", func(t *testing.T) {
		reqs := []Requirement{
			&mockRequirement{
				rules: []Rule{
					{TargetTool: "think", Forced: true},
				},
			},
		}

		chatCtx := chatContextWithTools(ctx, nil, []Message{})

		var next HandlerFunc = func(c *Context) error {
			if c.Chat.Options.ToolChoice != "think" {
				t.Errorf("expected ToolChoice='think', got '%s'", c.Chat.Options.ToolChoice)
			}
			if len(c.Chat.Messages) == 0 {
				t.Fatal("expected system instruction")
			}
			text := c.Chat.Messages[0].Content[0].Text
			if !strings.Contains(text, "'think'") {
				t.Errorf("expected forced instruction mentioning think, got: %s", text)
			}
			return nil
		}

		err := RequirementMiddleware(reqs...)(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})
}

// --------------------------------------------------------------------------
// End-to-end: ConditionalRequirement + RequirementMiddleware
// --------------------------------------------------------------------------

func TestRequirementMiddleware_EndToEnd_MathAgent(t *testing.T) {
	ctx := context.Background()

	t.Run("math agent think requirement at step 1", func(t *testing.T) {
		thinkReq := NewConditionalRequirement("think", ForceAtStep(1))
		middleware := RequirementMiddleware(thinkReq)

		tools := []Tool{
			&mockTool{name: "think"},
			&mockTool{name: "search"},
			&mockTool{name: "calculator"},
		}

		chatCtx := chatContextWithTools(ctx, tools, []Message{
			*NewSystemMessage("You are a math instructor"),
			*NewUserMessage("msg1", "What is 2+2?"),
		})

		var next HandlerFunc = func(c *Context) error {
			if c.Chat.Options.ToolChoice != "think" {
				t.Errorf("expected ToolChoice='think' at step 1, got '%s'", c.Chat.Options.ToolChoice)
			}
			if len(c.Chat.Tools) != 1 || c.Chat.Tools[0].Name() != "think" {
				names := make([]string, len(c.Chat.Tools))
				for i, at := range c.Chat.Tools {
					names[i] = at.Name()
				}
				t.Errorf("expected only 'think' tool available, got: %v", names)
			}
			foundInstruction := false
			for _, msg := range c.Chat.Messages {
				if msg.Role == RoleSystem {
					for _, content := range msg.Content {
						if strings.Contains(content.Text, "'think'") {
							foundInstruction = true
						}
					}
				}
			}
			if !foundInstruction {
				t.Error("expected forced instruction mentioning 'think'")
			}
			return nil
		}

		err := middleware(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("math agent think requirement at step 2 (not forced)", func(t *testing.T) {
		thinkReq := NewConditionalRequirement("think", ForceAtStep(1))
		middleware := RequirementMiddleware(thinkReq)

		tools := []Tool{
			&mockTool{name: "think"},
			&mockTool{name: "search"},
		}

		chatCtx := chatContextWithTools(ctx, tools, []Message{
			*NewSystemMessage("You are a math instructor"),
			*NewUserMessage("msg1", "What is 2+2?"),
			{
				Role: RoleAssistant,
				Content: []Content{{
					Type:     ContentTypeToolCall,
					ToolCall: &ToolCall{Name: "think", ID: "tc1"},
				}},
			},
			{
				Role: RoleTool,
				Content: []Content{{
					Type:       ContentTypeToolResult,
					ToolResult: &ToolResult{ToolCallID: "tc1", Output: "OK"},
				}},
			},
		})

		var next HandlerFunc = func(c *Context) error {
			if c.Chat.Options != nil && c.Chat.Options.ToolChoice != "" {
				t.Errorf("expected no ToolChoice at step 2, got '%s'", c.Chat.Options.ToolChoice)
			}
			return nil
		}

		err := middleware(next).Handle(chatCtx)
		if err != nil {
			t.Fatal(err)
		}
	})
}

// --------------------------------------------------------------------------
// End-to-end: ReAct pattern (Think → Act → Think → Act → ...)
// --------------------------------------------------------------------------

func TestRequirementMiddleware_EndToEnd_ReActPattern(t *testing.T) {
	ctx := context.Background()

	t.Run("full ReAct cycle with ConditionalRequirement", func(t *testing.T) {
		thinkReq := NewConditionalRequirement("think",
			ForceAtStep(1),
			ForceAfterAnyTool(),
			PreventConsecutive(),
		)
		middleware := RequirementMiddleware(thinkReq)

		tools := []Tool{
			&mockTool{name: "think"},
			&mockTool{name: "search"},
		}

		// --- Step 1: think should be forced ---
		step1Ctx := chatContextWithTools(ctx, tools, []Message{
			*NewUserMessage("msg1", "Find me information about Go"),
		})

		var step1ToolChoice string
		middleware(HandlerFunc(func(c *Context) error {
			step1ToolChoice = c.Chat.Options.ToolChoice
			return nil
		})).Handle(step1Ctx)

		if step1ToolChoice != "think" {
			t.Errorf("step 1: expected ToolChoice='think', got '%s'", step1ToolChoice)
		}

		// --- Step 2: after think → think NOT allowed, no forced tool ---
		step2Ctx := chatContextWithTools(ctx, tools, []Message{
			*NewUserMessage("msg1", "Find me information about Go"),
			{Role: RoleAssistant, Content: []Content{{Type: ContentTypeToolCall, ToolCall: &ToolCall{Name: "think"}}}},
			{Role: RoleTool, Content: []Content{{Type: ContentTypeToolResult, ToolResult: &ToolResult{Output: "OK"}}}},
		})

		var step2ToolChoice string
		var step2ToolNames []string
		middleware(HandlerFunc(func(c *Context) error {
			if c.Chat.Options != nil {
				step2ToolChoice = c.Chat.Options.ToolChoice
			}
			for _, t2 := range c.Chat.Tools {
				step2ToolNames = append(step2ToolNames, t2.Name())
			}
			return nil
		})).Handle(step2Ctx)

		if step2ToolChoice != "" {
			t.Errorf("step 2: expected no ToolChoice (think is blocked), got '%s'", step2ToolChoice)
		}
		for _, name := range step2ToolNames {
			if name == "think" {
				t.Error("step 2: think should NOT be in available tools (consecutive)")
			}
		}

		// --- Step 3: after search → think should be forced again ---
		step3Ctx := chatContextWithTools(ctx, tools, []Message{
			*NewUserMessage("msg1", "Find me information about Go"),
			{Role: RoleAssistant, Content: []Content{{Type: ContentTypeToolCall, ToolCall: &ToolCall{Name: "think"}}}},
			{Role: RoleTool, Content: []Content{{Type: ContentTypeToolResult, ToolResult: &ToolResult{Output: "OK"}}}},
			{Role: RoleAssistant, Content: []Content{{Type: ContentTypeToolCall, ToolCall: &ToolCall{Name: "search"}}}},
			{Role: RoleTool, Content: []Content{{Type: ContentTypeToolResult, ToolResult: &ToolResult{Output: "results"}}}},
		})

		var step3ToolChoice string
		middleware(HandlerFunc(func(c *Context) error {
			if c.Chat.Options != nil {
				step3ToolChoice = c.Chat.Options.ToolChoice
			}
			return nil
		})).Handle(step3Ctx)

		if step3ToolChoice != "think" {
			t.Errorf("step 3: expected ToolChoice='think' (forced after search), got '%s'", step3ToolChoice)
		}
	})
}

// --------------------------------------------------------------------------
// Non-chat operations are passed through
// --------------------------------------------------------------------------

func TestRequirementMiddleware_PassesNonChatOperations(t *testing.T) {
	middleware := RequirementMiddleware(&mockRequirement{
		rules: []Rule{{TargetTool: "think", Forced: true}},
	})

	called := false
	next := HandlerFunc(func(c *Context) error {
		called = true
		return nil
	})

	agentCtx := NewAgentContext(context.Background(), nil, nil, false)
	err := middleware(next).Handle(agentCtx)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected non-chat context to pass through to next handler")
	}
}
