package middleware

import (
	"context"
	"encoding/json"
	"school-with-ai/foundation/agentsdk/core"
	"strings"
	"testing"
)

func TestTodo_JSONMarshaling(t *testing.T) {
	t.Parallel()
	todo := Todo{
		Content: "Complete the implementation",
		Status:  TodoStatusInProgress,
	}

	data, err := json.Marshal(todo)
	if err != nil {
		t.Fatalf("Failed to marshal todo: %v", err)
	}

	var unmarshaled Todo
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal todo: %v", err)
	}

	if unmarshaled.Content != todo.Content {
		t.Errorf("Expected content %s, got %s", todo.Content, unmarshaled.Content)
	}

	if unmarshaled.Status != todo.Status {
		t.Errorf("Expected status %s, got %s", todo.Status, unmarshaled.Status)
	}
}

func TestWriteTodosTool_Name(t *testing.T) {
	t.Parallel()
	tool := NewWriteTodosTool()
	if tool.Name() != "write_todos" {
		t.Errorf("Expected name 'write_todos', got %s", tool.Name())
	}
}

func TestWriteTodosTool_Description(t *testing.T) {
	t.Parallel()
	tool := NewWriteTodosTool()
	desc := tool.Description()

	keyPhrases := []string{
		"structured task list",
		"When to Use This Tool",
		"Task States and Management",
		"pending",
		"in_progress",
		"completed",
	}

	for _, phrase := range keyPhrases {
		if !strings.Contains(desc, phrase) {
			t.Errorf("Description missing key phrase: %s", phrase)
		}
	}
}

func TestWriteTodosTool_CustomDescription(t *testing.T) {
	t.Parallel()
	customDesc := "Custom todo tool description"
	tool := NewWriteTodosToolWithDescription(customDesc)

	if tool.Description() != customDesc {
		t.Errorf("Expected custom description, got %s", tool.Description())
	}
}

func TestWriteTodosTool_Parameters(t *testing.T) {
	t.Parallel()
	tool := NewWriteTodosTool()
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Error("Expected type to be 'object'")
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	todosParam, ok := props["todos"].(map[string]any)
	if !ok {
		t.Fatal("Expected todos parameter to exist")
	}

	if todosParam["type"] != "array" {
		t.Error("Expected todos type to be 'array'")
	}

	items, ok := todosParam["items"].(map[string]any)
	if !ok {
		t.Fatal("Expected items to be defined")
	}

	itemProps, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected item properties to be defined")
	}

	if _, ok := itemProps["content"]; !ok {
		t.Error("Expected content property in todo items")
	}

	statusProp, ok := itemProps["status"].(map[string]any)
	if !ok {
		t.Fatal("Expected status property to be defined")
	}

	enum, ok := statusProp["enum"].([]string)
	if !ok {
		t.Fatal("Expected status to have enum values")
	}

	expectedStatuses := []string{"pending", "in_progress", "completed"}
	if len(enum) != len(expectedStatuses) {
		t.Errorf("Expected %d status values, got %d", len(expectedStatuses), len(enum))
	}
}

func TestWriteTodosTool_Invoke(t *testing.T) {
	t.Parallel()
	tool := NewWriteTodosTool()

	todos := []Todo{
		{Content: "Task 1", Status: TodoStatusPending},
		{Content: "Task 2", Status: TodoStatusInProgress},
		{Content: "Task 3", Status: TodoStatusCompleted},
	}

	args := map[string]any{
		"todos": todos,
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("Failed to marshal arguments: %v", err)
	}

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Tool invocation failed: %v", err)
	}

	if !strings.Contains(result.Result, "Updated todo list") {
		t.Errorf("Expected confirmation message in result, got: %s", result.Result)
	}

	if !strings.Contains(result.Result, "Task 1") {
		t.Error("Result should contain Task 1")
	}
}

func TestWriteTodosTool_Invoke_InvalidJSON(t *testing.T) {
	t.Parallel()
	tool := NewWriteTodosTool()

	_, err := tool.Invoke(context.Background(), "invalid json")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestNewTodoListMiddleware(t *testing.T) {
	t.Parallel()
	middleware := NewTodoListMiddleware()

	if middleware == nil {
		t.Fatal("Expected middleware to be created")
	}

	if middleware.systemPrompt != WriteTodosSystemPrompt {
		t.Error("Expected default system prompt")
	}

	if middleware.toolDescription != WriteTodosToolDescription {
		t.Error("Expected default tool description")
	}

	if middleware.tool == nil {
		t.Error("Expected tool to be initialized")
	}
}

func TestNewTodoListMiddlewareWithPrompts(t *testing.T) {
	t.Parallel()
	customSystemPrompt := "Custom system prompt"
	customToolDesc := "Custom tool description"

	middleware := NewTodoListMiddlewareWithPrompts(customSystemPrompt, customToolDesc)

	if middleware.systemPrompt != customSystemPrompt {
		t.Error("Expected custom system prompt")
	}

	if middleware.toolDescription != customToolDesc {
		t.Error("Expected custom tool description")
	}
}

func TestTodoListMiddleware_Process_AddsToolAndPrompt(t *testing.T) {
	t.Parallel()
	middleware := NewTodoListMiddleware()

	reqCtx := core.NewRequestContext(
		"",
		nil,
		"Original prompt",
		[]core.Tool{},
		&core.AgentConfig{},
	)

	nextCalled := false
	nextHandler := func(ctx context.Context, rc *core.RequestContext) error {
		nextCalled = true

		return nil
	}

	err := middleware.Process(context.Background(), reqCtx, nextHandler)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if !nextCalled {
		t.Error("Expected next handler to be called")
	}

	tools := reqCtx.GetEffectiveTools()
	foundTool := false
	for _, tool := range tools {
		if tool.Name() == "write_todos" {
			foundTool = true

			break
		}
	}
	if !foundTool {
		t.Error("Expected write_todos tool to be added")
	}

	effectivePrompt := reqCtx.GetEffectiveSystemPrompt()
	if !strings.Contains(effectivePrompt, "write_todos") {
		t.Error("Expected system prompt to mention write_todos tool")
	}
}

func TestTodoListMiddleware_Process_PreservesExistingTools(t *testing.T) {
	t.Parallel()
	middleware := NewTodoListMiddleware()

	existingTool := &MockTool{name: "existing_tool"}

	reqCtx := core.NewRequestContext(
		"",
		nil,
		"Original prompt",
		[]core.Tool{existingTool},
		&core.AgentConfig{},
	)

	nextHandler := func(ctx context.Context, rc *core.RequestContext) error {
		return nil
	}

	err := middleware.Process(context.Background(), reqCtx, nextHandler)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	tools := reqCtx.GetEffectiveTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	foundExisting := false
	foundWriteTodos := false
	for _, tool := range tools {
		if tool.Name() == "existing_tool" {
			foundExisting = true
		}
		if tool.Name() == "write_todos" {
			foundWriteTodos = true
		}
	}

	if !foundExisting {
		t.Error("Expected existing tool to be preserved")
	}
	if !foundWriteTodos {
		t.Error("Expected write_todos tool to be added")
	}
}

func TestGetTodos_EmptyState(t *testing.T) {
	t.Parallel()
	reqCtx := core.NewRequestContext("", nil, "", nil, &core.AgentConfig{})

	todos := GetTodos(reqCtx)
	if todos != nil {
		t.Errorf("Expected nil todos for empty state, got %v", todos)
	}
}

func TestSetTodos_AndGetTodos(t *testing.T) {
	t.Parallel()
	reqCtx := core.NewRequestContext("", nil, "", nil, &core.AgentConfig{})

	expectedTodos := []Todo{
		{Content: "Task 1", Status: TodoStatusPending},
		{Content: "Task 2", Status: TodoStatusInProgress},
	}

	SetTodos(reqCtx, expectedTodos)

	retrievedTodos := GetTodos(reqCtx)
	if retrievedTodos == nil {
		t.Fatal("Expected todos to be set")
	}

	if len(retrievedTodos) != len(expectedTodos) {
		t.Errorf("Expected %d todos, got %d", len(expectedTodos), len(retrievedTodos))
	}

	for i, todo := range retrievedTodos {
		if todo.Content != expectedTodos[i].Content {
			t.Errorf("Expected content %s at index %d, got %s", expectedTodos[i].Content, i, todo.Content)
		}
		if todo.Status != expectedTodos[i].Status {
			t.Errorf("Expected status %s at index %d, got %s", expectedTodos[i].Status, i, todo.Status)
		}
	}
}

func TestTodoListMiddleware_Integration(t *testing.T) {
	t.Parallel()
	middleware := NewTodoListMiddleware()

	reqCtx := core.NewRequestContext(
		"",
		nil,
		"You are a helpful assistant",
		[]core.Tool{},
		&core.AgentConfig{},
	)

	nextHandler := func(ctx context.Context, rc *core.RequestContext) error {
		todos := []Todo{
			{Content: "Read user requirements", Status: TodoStatusCompleted},
			{Content: "Design solution", Status: TodoStatusInProgress},
			{Content: "Implement code", Status: TodoStatusPending},
			{Content: "Write tests", Status: TodoStatusPending},
		}

		SetTodos(rc, todos)

		return nil
	}

	err := middleware.Process(context.Background(), reqCtx, nextHandler)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	todos := GetTodos(reqCtx)
	if todos == nil {
		t.Fatal("Expected todos to be set")
	}

	if len(todos) != 4 {
		t.Errorf("Expected 4 todos, got %d", len(todos))
	}

	if todos[0].Status != TodoStatusCompleted {
		t.Error("Expected first todo to be completed")
	}
	if todos[1].Status != TodoStatusInProgress {
		t.Error("Expected second todo to be in_progress")
	}
	if todos[2].Status != TodoStatusPending {
		t.Error("Expected third todo to be pending")
	}
}

func TestTodoStatus_AllValidValues(t *testing.T) {
	t.Parallel()
	validStatuses := []TodoStatus{
		TodoStatusPending,
		TodoStatusInProgress,
		TodoStatusCompleted,
	}

	expectedValues := []string{"pending", "in_progress", "completed"}

	for i, status := range validStatuses {
		if string(status) != expectedValues[i] {
			t.Errorf("Expected status %s, got %s", expectedValues[i], string(status))
		}
	}
}

// MockTool for testing.
type MockTool struct {
	name string
}

func (m *MockTool) Name() string {
	return m.name
}

func (m *MockTool) Description() string {
	return "Mock tool for testing"
}

func (m *MockTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (m *MockTool) Invoke(ctx context.Context, arguments string) (core.ToolResult, error) {
	return core.NewToolResult("mock result"), nil
}
