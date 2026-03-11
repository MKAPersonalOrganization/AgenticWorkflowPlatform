package tracker

import (
	"context"
	"encoding/json"
	"testing"

	"school-with-ai/foundation/agent"
)

func TestStore_CreateTask(t *testing.T) {
	s := NewStore()
	task, err := s.CreateTask("Build API", "Implement REST endpoints", TaskTypeTask, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if task.Title != "Build API" {
		t.Errorf("title = %q, want %q", task.Title, "Build API")
	}
	if task.Status != StatusOpen {
		t.Errorf("status = %q, want %q", task.Status, StatusOpen)
	}
}

func TestStore_CreateTask_AllTypes(t *testing.T) {
	s := NewStore()
	for _, tt := range []TaskType{TaskTypeEpic, TaskTypeTask, TaskTypeBug} {
		task, err := s.CreateTask("Title", "Desc", tt, "", nil)
		if err != nil {
			t.Fatalf("type %q: unexpected error: %v", tt, err)
		}
		if task.Type != tt {
			t.Errorf("type = %q, want %q", task.Type, tt)
		}
	}
}

func TestStore_CreateTask_InvalidType(t *testing.T) {
	s := NewStore()
	_, err := s.CreateTask("Title", "Desc", "invalid", "", nil)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestStore_CreateTask_EmptyTitle(t *testing.T) {
	s := NewStore()
	_, err := s.CreateTask("", "Desc", TaskTypeTask, "", nil)
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestStore_CreateTask_WithParent(t *testing.T) {
	s := NewStore()
	epic, _ := s.CreateTask("Epic", "Parent epic", TaskTypeEpic, "", nil)
	child, err := s.CreateTask("Child", "Sub-task", TaskTypeTask, epic.ID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child.ParentID != epic.ID {
		t.Errorf("parent_id = %q, want %q", child.ParentID, epic.ID)
	}
}

func TestStore_CreateTask_InvalidParent(t *testing.T) {
	s := NewStore()
	_, err := s.CreateTask("Child", "Sub-task", TaskTypeTask, "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent parent")
	}
}

func TestStore_CreateTask_WithDependencies(t *testing.T) {
	s := NewStore()
	dep1, _ := s.CreateTask("Dep 1", "First dep", TaskTypeTask, "", nil)
	dep2, _ := s.CreateTask("Dep 2", "Second dep", TaskTypeTask, "", nil)
	task, err := s.CreateTask("Main", "Main task", TaskTypeTask, "", []string{dep1.ID, dep2.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(task.Dependencies) != 2 {
		t.Errorf("got %d dependencies, want 2", len(task.Dependencies))
	}
}

func TestStore_CreateTask_InvalidDependency(t *testing.T) {
	s := NewStore()
	_, err := s.CreateTask("Main", "Main task", TaskTypeTask, "", []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent dependency")
	}
}

func TestStore_CreateTask_MaxLimit(t *testing.T) {
	s := NewStore()
	for i := 0; i < MaxTasksPerThread; i++ {
		_, err := s.CreateTask("Task", "Desc", TaskTypeTask, "", nil)
		if err != nil {
			t.Fatalf("task %d: unexpected error: %v", i, err)
		}
	}
	_, err := s.CreateTask("Overflow", "Desc", TaskTypeTask, "", nil)
	if err == nil {
		t.Fatal("expected error for exceeding max task limit")
	}
}

func TestStore_GetTask(t *testing.T) {
	s := NewStore()
	created, _ := s.CreateTask("Title", "Desc", TaskTypeTask, "", nil)
	got := s.GetTask(created.ID)
	if got == nil {
		t.Fatal("expected non-nil task")
	}
	if got.Title != "Title" {
		t.Errorf("title = %q, want %q", got.Title, "Title")
	}
}

func TestStore_GetTask_NotFound(t *testing.T) {
	s := NewStore()
	got := s.GetTask("nonexistent")
	if got != nil {
		t.Fatal("expected nil for nonexistent task")
	}
}

func TestStore_UpdateTask_Title(t *testing.T) {
	s := NewStore()
	task, _ := s.CreateTask("Old", "Desc", TaskTypeTask, "", nil)
	newTitle := "New Title"
	updated, err := s.UpdateTask(task.ID, &newTitle, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Title != "New Title" {
		t.Errorf("title = %q, want %q", updated.Title, "New Title")
	}
}

func TestStore_UpdateTask_Status(t *testing.T) {
	s := NewStore()
	task, _ := s.CreateTask("Title", "Desc", TaskTypeTask, "", nil)
	status := StatusInProgress
	updated, err := s.UpdateTask(task.ID, nil, nil, &status, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != StatusInProgress {
		t.Errorf("status = %q, want %q", updated.Status, StatusInProgress)
	}
}

func TestStore_UpdateTask_InvalidStatus(t *testing.T) {
	s := NewStore()
	task, _ := s.CreateTask("Title", "Desc", TaskTypeTask, "", nil)
	status := TaskStatus("invalid")
	_, err := s.UpdateTask(task.ID, nil, nil, &status, nil)
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestStore_UpdateTask_NotFound(t *testing.T) {
	s := NewStore()
	title := "New"
	_, err := s.UpdateTask("nonexistent", &title, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestStore_ListTasks(t *testing.T) {
	s := NewStore()
	s.CreateTask("First", "Desc", TaskTypeTask, "", nil)
	s.CreateTask("Second", "Desc", TaskTypeEpic, "", nil)
	tasks := s.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("got %d tasks, want 2", len(tasks))
	}
	if tasks[0].Title != "First" || tasks[1].Title != "Second" {
		t.Errorf("unexpected order: %v, %v", tasks[0].Title, tasks[1].Title)
	}
}

func TestStore_ListTasks_Empty(t *testing.T) {
	s := NewStore()
	tasks := s.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("got %d tasks, want 0", len(tasks))
	}
}

func TestStore_AddDependency(t *testing.T) {
	s := NewStore()
	a, _ := s.CreateTask("A", "Desc", TaskTypeTask, "", nil)
	b, _ := s.CreateTask("B", "Desc", TaskTypeTask, "", nil)
	if err := s.AddDependency(b.ID, a.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := s.GetTask(b.ID)
	if len(got.Dependencies) != 1 || got.Dependencies[0] != a.ID {
		t.Errorf("dependencies = %v, want [%s]", got.Dependencies, a.ID)
	}
}

func TestStore_AddDependency_SelfReference(t *testing.T) {
	s := NewStore()
	a, _ := s.CreateTask("A", "Desc", TaskTypeTask, "", nil)
	err := s.AddDependency(a.ID, a.ID)
	if err == nil {
		t.Fatal("expected error for self-referential dependency")
	}
}

func TestStore_AddDependency_Idempotent(t *testing.T) {
	s := NewStore()
	a, _ := s.CreateTask("A", "Desc", TaskTypeTask, "", nil)
	b, _ := s.CreateTask("B", "Desc", TaskTypeTask, "", nil)
	s.AddDependency(b.ID, a.ID)
	err := s.AddDependency(b.ID, a.ID)
	if err != nil {
		t.Fatalf("duplicate add should be idempotent, got: %v", err)
	}
	got := s.GetTask(b.ID)
	if len(got.Dependencies) != 1 {
		t.Errorf("dependencies = %d, want 1", len(got.Dependencies))
	}
}

func TestStore_AddDependency_CycleDetection(t *testing.T) {
	s := NewStore()
	a, _ := s.CreateTask("A", "Desc", TaskTypeTask, "", nil)
	b, _ := s.CreateTask("B", "Desc", TaskTypeTask, "", nil)
	c, _ := s.CreateTask("C", "Desc", TaskTypeTask, "", nil)
	s.AddDependency(b.ID, a.ID)
	s.AddDependency(c.ID, b.ID)
	err := s.AddDependency(a.ID, c.ID)
	if err == nil {
		t.Fatal("expected error for cycle")
	}
}

func TestStore_AddDependency_NonexistentTask(t *testing.T) {
	s := NewStore()
	a, _ := s.CreateTask("A", "Desc", TaskTypeTask, "", nil)
	err := s.AddDependency("nonexistent", a.ID)
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
	err = s.AddDependency(a.ID, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent dependency")
	}
}

func TestStore_AddDependency_MaxLimit(t *testing.T) {
	s := NewStore()
	main, _ := s.CreateTask("Main", "Desc", TaskTypeTask, "", nil)
	for i := 0; i < MaxDependenciesPerTask; i++ {
		dep, _ := s.CreateTask("Dep", "Desc", TaskTypeTask, "", nil)
		if err := s.AddDependency(main.ID, dep.ID); err != nil {
			t.Fatalf("dep %d: unexpected error: %v", i, err)
		}
	}
	extra, _ := s.CreateTask("Extra", "Desc", TaskTypeTask, "", nil)
	err := s.AddDependency(main.ID, extra.ID)
	if err == nil {
		t.Fatal("expected error for exceeding max dependencies")
	}
}

func makeArgs(data any) agent.Arguments {
	b, _ := json.Marshal(data)
	return agent.Arguments{Raw: b, UserID: "test-user"}
}

// storeManagerWith wraps a pre-populated Store in a StoreManager keyed to "test-user".
func storeManagerWith(s *Store) *StoreManager {
	mgr := NewStoreManager()
	mgr.mu.Lock()
	mgr.stores["test-user"] = s
	mgr.mu.Unlock()
	return mgr
}

func TestCreateTaskTool(t *testing.T) {
	store := NewStore()
	tool := &CreateTaskTool{stores: storeManagerWith(store)}
	if tool.Name() != "tracker_create_task" {
		t.Errorf("name = %q", tool.Name())
	}
	result, err := tool.Execute(context.Background(), makeArgs(map[string]any{
		"title": "Build feature", "description": "Implement the new feature", "type": "task",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("result error: %v", result.Error)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
	var parsed map[string]any
	json.Unmarshal([]byte(result.Content[0].(string)), &parsed)
	if parsed["status"] != "ok" {
		t.Errorf("status = %v, want ok", parsed["status"])
	}
}

func TestUpdateTaskTool(t *testing.T) {
	store := NewStore()
	task, _ := store.CreateTask("Original", "Desc", TaskTypeTask, "", nil)
	tool := &UpdateTaskTool{stores: storeManagerWith(store)}
	result, err := tool.Execute(context.Background(), makeArgs(map[string]any{
		"id": task.ID, "status": "in_progress",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("result error: %v", result.Error)
	}
	updated := store.GetTask(task.ID)
	if updated.Status != StatusInProgress {
		t.Errorf("status = %q, want %q", updated.Status, StatusInProgress)
	}
}

func TestGetTaskTool(t *testing.T) {
	store := NewStore()
	task, _ := store.CreateTask("Test", "Desc", TaskTypeTask, "", nil)
	tool := &GetTaskTool{stores: storeManagerWith(store)}
	result, _ := tool.Execute(context.Background(), makeArgs(map[string]any{"id": task.ID}))
	if result.Error != nil {
		t.Fatalf("result error: %v", result.Error)
	}
	var parsed Task
	json.Unmarshal([]byte(result.Content[0].(string)), &parsed)
	if parsed.Title != "Test" {
		t.Errorf("title = %q, want %q", parsed.Title, "Test")
	}
}

func TestGetTaskTool_NotFound(t *testing.T) {
	store := NewStore()
	tool := &GetTaskTool{stores: storeManagerWith(store)}
	result, _ := tool.Execute(context.Background(), makeArgs(map[string]any{"id": "xxx"}))
	content := result.Content[0].(string)
	if content != "Task xxx not found." {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestListTasksTool(t *testing.T) {
	store := NewStore()
	store.CreateTask("Task 1", "Desc", TaskTypeTask, "", nil)
	store.CreateTask("Task 2", "Desc", TaskTypeEpic, "", nil)
	tool := &ListTasksTool{stores: storeManagerWith(store)}
	result, _ := tool.Execute(context.Background(), makeArgs(map[string]any{}))
	if result.Error != nil {
		t.Fatalf("result error: %v", result.Error)
	}
	var parsed map[string]any
	json.Unmarshal([]byte(result.Content[0].(string)), &parsed)
	if int(parsed["total"].(float64)) != 2 {
		t.Errorf("total = %v, want 2", parsed["total"])
	}
}

func TestListTasksTool_WithFilters(t *testing.T) {
	store := NewStore()
	store.CreateTask("Epic 1", "Desc", TaskTypeEpic, "", nil)
	store.CreateTask("Task 1", "Desc", TaskTypeTask, "", nil)
	tool := &ListTasksTool{stores: storeManagerWith(store)}
	result, _ := tool.Execute(context.Background(), makeArgs(map[string]any{"type": "epic"}))
	var parsed map[string]any
	json.Unmarshal([]byte(result.Content[0].(string)), &parsed)
	if int(parsed["total"].(float64)) != 1 {
		t.Errorf("total = %v, want 1", parsed["total"])
	}
}

func TestListTasksTool_Empty(t *testing.T) {
	store := NewStore()
	tool := &ListTasksTool{stores: storeManagerWith(store)}
	result, _ := tool.Execute(context.Background(), makeArgs(map[string]any{}))
	content := result.Content[0].(string)
	if content != "No tasks found matching the criteria." {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestAddDependencyTool(t *testing.T) {
	store := NewStore()
	a, _ := store.CreateTask("A", "Desc", TaskTypeTask, "", nil)
	b, _ := store.CreateTask("B", "Desc", TaskTypeTask, "", nil)
	tool := &AddDependencyTool{stores: storeManagerWith(store)}
	result, _ := tool.Execute(context.Background(), makeArgs(map[string]any{
		"task_id": b.ID, "dependency_id": a.ID,
	}))
	if result.Error != nil {
		t.Fatalf("result error: %v", result.Error)
	}
	got := store.GetTask(b.ID)
	if len(got.Dependencies) != 1 {
		t.Errorf("dependencies = %d, want 1", len(got.Dependencies))
	}
}

func TestAddDependencyTool_SelfReference(t *testing.T) {
	store := NewStore()
	a, _ := store.CreateTask("A", "Desc", TaskTypeTask, "", nil)
	tool := &AddDependencyTool{stores: storeManagerWith(store)}
	result, _ := tool.Execute(context.Background(), makeArgs(map[string]any{
		"task_id": a.ID, "dependency_id": a.ID,
	}))
	if result.Error == nil {
		t.Fatal("expected error for self-referential dependency")
	}
}

func TestVisualizeTool(t *testing.T) {
	store := NewStore()
	epic, _ := store.CreateTask("Project", "Root epic", TaskTypeEpic, "", nil)
	task1, _ := store.CreateTask("Design", "Design the system", TaskTypeTask, epic.ID, nil)
	store.CreateTask("Implement", "Build it", TaskTypeTask, epic.ID, []string{task1.ID})
	tool := &VisualizeTool{stores: storeManagerWith(store)}
	result, _ := tool.Execute(context.Background(), agent.Arguments{UserID: "test-user"})
	if result.Error != nil {
		t.Fatalf("result error: %v", result.Error)
	}
	var parsed map[string]any
	json.Unmarshal([]byte(result.Content[0].(string)), &parsed)
	graph := parsed["graph"].(string)
	if graph == "" {
		t.Fatal("expected non-empty graph")
	}
	if int(parsed["total"].(float64)) != 3 {
		t.Errorf("total = %v, want 3", parsed["total"])
	}
}

func TestVisualizeTool_Empty(t *testing.T) {
	store := NewStore()
	tool := &VisualizeTool{stores: storeManagerWith(store)}
	result, _ := tool.Execute(context.Background(), agent.Arguments{UserID: "test-user"})
	content := result.Content[0].(string)
	if content != "No tasks to visualize." {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestProvider_Interface(t *testing.T) {
	p := &Provider{}
	if p.ID() != "tracker" {
		t.Errorf("ID = %q", p.ID())
	}
	if p.Icon() == "" {
		t.Error("expected non-empty icon")
	}
	sets := p.DefaultSets()
	if len(sets) != 1 || sets[0].ID != "tracker-set" {
		t.Errorf("unexpected sets: %v", sets)
	}
	if len(sets[0].ToolIDs) != 6 {
		t.Errorf("expected 6 tool IDs, got %d", len(sets[0].ToolIDs))
	}
	tools := p.DefaultTools()
	if len(tools) != 6 {
		t.Errorf("expected 6 tools, got %d", len(tools))
	}
	if p.SettingsSchema() != nil {
		t.Error("expected nil settings schema")
	}
	handled := p.UIHandledTools()
	if len(handled) != 6 {
		t.Errorf("expected 6 UI-handled tools, got %d", len(handled))
	}
}

func TestProvider_CreateExecutable(t *testing.T) {
	ResetGlobalStoreManager()
	defer ResetGlobalStoreManager()
	p := &Provider{}
	ctx := context.Background()
	toolIDs := []string{
		"tracker_create_task", "tracker_update_task", "tracker_get_task",
		"tracker_list_tasks", "tracker_add_dependency", "tracker_visualize",
	}
	for _, id := range toolIDs {
		tool, err := p.CreateExecutable(ctx, id, nil)
		if err != nil {
			t.Errorf("tool %q: unexpected error: %v", id, err)
		}
		if tool == nil {
			t.Errorf("tool %q: expected non-nil tool", id)
		}
	}
	tool, err := p.CreateExecutable(ctx, "unknown", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tool != nil {
		t.Error("expected nil for unknown tool")
	}
}
