package todos

import (
	"aicompass/foundation/agent"
	"context"
	"encoding/json"
	"testing"
)

func TestWriteTodosTool_Name(t *testing.T) {
	t.Parallel()

	tool := &WriteTodosTool{}
	if got := tool.Name(); got != "write_todos" {
		t.Errorf("Name() = %q, want %q", got, "write_todos")
	}
}

func TestWriteTodosTool_Schema(t *testing.T) {
	t.Parallel()

	tool := &WriteTodosTool{}
	schema := tool.Schema()
	if schema == nil {
		t.Fatal("Schema() returned nil")
	}
	if schema.Type != agent.TypeObject {
		t.Errorf("Schema().Type = %q, want %q", schema.Type, agent.TypeObject)
	}
	if _, ok := schema.Properties["todos"]; !ok {
		t.Error("Schema() missing 'todos' property")
	}
}

func TestWriteTodosTool_Execute_Success(t *testing.T) {
	t.Parallel()

	tool := &WriteTodosTool{}
	args := WriteTodosArgs{
		Todos: []TodoItem{
			{Description: "Research the problem", Status: StatusCompleted},
			{Description: "Implement the solution", Status: StatusInProgress},
			{Description: "Write tests", Status: StatusPending},
		},
	}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error != nil {
		t.Fatalf("Execute() result.Error = %v", result.Error)
	}
	if len(result.Content) == 0 {
		t.Fatal("Execute() returned empty content")
	}

	// Parse the result and verify structure
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].(string)), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed["status"] != "ok" {
		t.Errorf("result status = %v, want 'ok'", parsed["status"])
	}
	if parsed["total"].(float64) != 3 {
		t.Errorf("result total = %v, want 3", parsed["total"])
	}
	if parsed["completed"].(float64) != 1 {
		t.Errorf("result completed = %v, want 1", parsed["completed"])
	}
}

func TestWriteTodosTool_Execute_EmptyList(t *testing.T) {
	t.Parallel()

	tool := &WriteTodosTool{}
	args := WriteTodosArgs{Todos: []TodoItem{}}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error != nil {
		t.Fatalf("Execute() result.Error = %v", result.Error)
	}
}

func TestWriteTodosTool_Execute_MultipleInProgress(t *testing.T) {
	t.Parallel()

	tool := &WriteTodosTool{}
	args := WriteTodosArgs{
		Todos: []TodoItem{
			{Description: "Task A", Status: StatusInProgress},
			{Description: "Task B", Status: StatusInProgress},
		},
	}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	// Should return an error in the result (validation failure)
	if result.Error == nil {
		t.Error("expected validation error for multiple in_progress tasks")
	}
}

func TestWriteTodosTool_Execute_TooManyTodos(t *testing.T) {
	t.Parallel()

	tool := &WriteTodosTool{}
	todos := make([]TodoItem, 30)
	for i := range todos {
		todos[i] = TodoItem{Description: "Task", Status: StatusPending}
	}
	args := WriteTodosArgs{Todos: todos}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Error("expected validation error for too many todos")
	}
}

func TestWriteTodosTool_Execute_EmptyDescription(t *testing.T) {
	t.Parallel()

	tool := &WriteTodosTool{}
	args := WriteTodosArgs{
		Todos: []TodoItem{
			{Description: "", Status: StatusPending},
		},
	}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Error("expected validation error for empty description")
	}
}

func TestWriteTodosTool_Execute_InvalidStatus(t *testing.T) {
	t.Parallel()

	tool := &WriteTodosTool{}
	args := WriteTodosArgs{
		Todos: []TodoItem{
			{Description: "Valid task", Status: "invalid_status"},
		},
	}
	raw, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: raw})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Error == nil {
		t.Error("expected validation error for invalid status")
	}
}

func TestProvider_Interface(t *testing.T) {
	t.Parallel()

	p := &Provider{}
	if got := p.ID(); got != "todos" {
		t.Errorf("ID() = %q, want %q", got, "todos")
	}
	if icon := p.Icon(); icon == "" {
		t.Error("Icon() returned empty string")
	}
	sets := p.DefaultSets()
	if len(sets) != 1 {
		t.Fatalf("DefaultSets() returned %d sets, want 1", len(sets))
	}
	if sets[0].ID != "todos-set" {
		t.Errorf("DefaultSets()[0].ID = %q, want %q", sets[0].ID, "todos-set")
	}
	tools := p.DefaultTools()
	if len(tools) != 1 {
		t.Fatalf("DefaultTools() returned %d tools, want 1", len(tools))
	}
	if tools[0].ID != "write_todos" {
		t.Errorf("DefaultTools()[0].ID = %q, want %q", tools[0].ID, "write_todos")
	}
	handled := p.UIHandledTools()
	if len(handled) != 1 || handled[0] != "write_todos" {
		t.Errorf("UIHandledTools() = %v, want [write_todos]", handled)
	}
}

func TestProvider_CreateExecutable(t *testing.T) {
	t.Parallel()

	p := &Provider{}
	ctx := context.Background()

	// Known tool
	tool, err := p.CreateExecutable(ctx, "write_todos", nil)
	if err != nil {
		t.Fatalf("CreateExecutable(write_todos) error = %v", err)
	}
	if tool == nil {
		t.Fatal("CreateExecutable(write_todos) returned nil")
	}

	// Unknown tool
	tool, err = p.CreateExecutable(ctx, "unknown_tool", nil)
	if err != nil {
		t.Fatalf("CreateExecutable(unknown_tool) error = %v", err)
	}
	if tool != nil {
		t.Error("CreateExecutable(unknown_tool) should return nil")
	}
}
