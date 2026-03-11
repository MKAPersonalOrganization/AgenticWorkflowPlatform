package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"school-with-ai/foundation/agent"
)

// ═══════════════════════════════════════════════════════════════════════════════
// tracker_create_task
// ═══════════════════════════════════════════════════════════════════════════════

type createTaskArgs struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	ParentID     string   `json:"parent_id,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

var createTaskSchema = &agent.Schema{
	Type:        agent.TypeObject,
	Description: "Create a new task (epic, task, or bug) with an optional parent and dependency links. Use this to decompose complex projects into trackable work items.",
	Properties: map[string]*agent.Schema{
		"title": {
			Type:        agent.TypeString,
			Description: "A concise title for the task.",
		},
		"description": {
			Type:        agent.TypeString,
			Description: "A detailed description of what needs to be done.",
		},
		"type": {
			Type:        agent.TypeString,
			Description: "The category of the task.",
			Enum:        []string{"epic", "task", "bug"},
		},
		"parent_id": {
			Type:        agent.TypeString,
			Description: "The ID of the parent task (for creating sub-tasks under an epic).",
		},
		"dependencies": {
			Type:        agent.TypeArray,
			Description: "IDs of tasks that must be completed before this one can start.",
			Items:       &agent.Schema{Type: agent.TypeString},
		},
	},
	Required: []string{"title", "description", "type"},
}

// CreateTaskTool implements agent.ExecutableTool for tracker_create_task.
type CreateTaskTool struct{ stores *StoreManager }

func (t *CreateTaskTool) Name() string          { return "tracker_create_task" }
func (t *CreateTaskTool) Description() string   { return createTaskSchema.Description }
func (t *CreateTaskTool) Schema() *agent.Schema { return createTaskSchema }

func (t *CreateTaskTool) Execute(_ context.Context, args agent.Arguments) (agent.Result, error) {
	var p createTaskArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
	}

	store := t.stores.ForUser(args.UserID)
	task, err := store.CreateTask(p.Title, p.Description, TaskType(p.Type), p.ParentID, p.Dependencies)
	if err != nil {
		return agent.ErrorResult(err), nil
	}

	result := map[string]any{
		"status":  "ok",
		"message": fmt.Sprintf("Created %s %s: %s", task.Type, task.ID, task.Title),
		"task":    task,
	}
	b, _ := json.Marshal(result)
	return agent.Result{Content: []any{string(b)}}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// tracker_update_task
// ═══════════════════════════════════════════════════════════════════════════════

type updateTaskArgs struct {
	ID           string   `json:"id"`
	Title        *string  `json:"title,omitempty"`
	Description  *string  `json:"description,omitempty"`
	Status       *string  `json:"status,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

var updateTaskSchema = &agent.Schema{
	Type:        agent.TypeObject,
	Description: "Update an existing task's title, description, status, or dependencies. Only provide the fields you want to change.",
	Properties: map[string]*agent.Schema{
		"id": {
			Type:        agent.TypeString,
			Description: "The ID of the task to update.",
		},
		"title": {
			Type:        agent.TypeString,
			Description: "New title for the task.",
		},
		"description": {
			Type:        agent.TypeString,
			Description: "New description for the task.",
		},
		"status": {
			Type:        agent.TypeString,
			Description: "New status for the task.",
			Enum:        []string{"open", "in_progress", "blocked", "closed"},
		},
		"dependencies": {
			Type:        agent.TypeArray,
			Description: "Replace the task's dependency list with these IDs.",
			Items:       &agent.Schema{Type: agent.TypeString},
		},
	},
	Required: []string{"id"},
}

// UpdateTaskTool implements agent.ExecutableTool for tracker_update_task.
type UpdateTaskTool struct{ stores *StoreManager }

func (t *UpdateTaskTool) Name() string          { return "tracker_update_task" }
func (t *UpdateTaskTool) Description() string   { return updateTaskSchema.Description }
func (t *UpdateTaskTool) Schema() *agent.Schema { return updateTaskSchema }

func (t *UpdateTaskTool) Execute(_ context.Context, args agent.Arguments) (agent.Result, error) {
	var p updateTaskArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
	}
	if p.ID == "" {
		return agent.ErrorResult(fmt.Errorf("id is required")), nil
	}

	store := t.stores.ForUser(args.UserID)
	var statusPtr *TaskStatus
	if p.Status != nil {
		s := TaskStatus(*p.Status)
		statusPtr = &s
	}
	var depsPtr *[]string
	if p.Dependencies != nil {
		depsPtr = &p.Dependencies
	}

	task, err := store.UpdateTask(p.ID, p.Title, p.Description, statusPtr, depsPtr)
	if err != nil {
		return agent.ErrorResult(err), nil
	}

	result := map[string]any{
		"status":  "ok",
		"message": fmt.Sprintf("Updated task %s. Status: %s", task.ID, task.Status),
		"task":    task,
	}
	b, _ := json.Marshal(result)
	return agent.Result{Content: []any{string(b)}}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// tracker_get_task
// ═══════════════════════════════════════════════════════════════════════════════

type getTaskArgs struct {
	ID string `json:"id"`
}

var getTaskSchema = &agent.Schema{
	Type:        agent.TypeObject,
	Description: "Retrieve detailed information about a specific task by its ID.",
	Properties: map[string]*agent.Schema{
		"id": {
			Type:        agent.TypeString,
			Description: "The ID of the task to retrieve.",
		},
	},
	Required: []string{"id"},
}

// GetTaskTool implements agent.ExecutableTool for tracker_get_task.
type GetTaskTool struct{ stores *StoreManager }

func (t *GetTaskTool) Name() string          { return "tracker_get_task" }
func (t *GetTaskTool) Description() string   { return getTaskSchema.Description }
func (t *GetTaskTool) Schema() *agent.Schema { return getTaskSchema }

func (t *GetTaskTool) Execute(_ context.Context, args agent.Arguments) (agent.Result, error) {
	var p getTaskArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
	}
	if p.ID == "" {
		return agent.ErrorResult(fmt.Errorf("id is required")), nil
	}

	store := t.stores.ForUser(args.UserID)
	task := store.GetTask(p.ID)
	if task == nil {
		return agent.TextResult(fmt.Sprintf("Task %s not found.", p.ID)), nil
	}

	b, _ := json.Marshal(task)
	return agent.Result{Content: []any{string(b)}}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// tracker_list_tasks
// ═══════════════════════════════════════════════════════════════════════════════

type listTasksArgs struct {
	Status   string `json:"status,omitempty"`
	Type     string `json:"type,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
}

var listTasksSchema = &agent.Schema{
	Type:        agent.TypeObject,
	Description: "List and filter tasks. All parameters are optional — omit them to list everything.",
	Properties: map[string]*agent.Schema{
		"status": {
			Type:        agent.TypeString,
			Description: "Filter tasks by status.",
			Enum:        []string{"open", "in_progress", "blocked", "closed"},
		},
		"type": {
			Type:        agent.TypeString,
			Description: "Filter tasks by type.",
			Enum:        []string{"epic", "task", "bug"},
		},
		"parent_id": {
			Type:        agent.TypeString,
			Description: "Filter tasks by parent task ID (list sub-tasks of an epic).",
		},
	},
}

// ListTasksTool implements agent.ExecutableTool for tracker_list_tasks.
type ListTasksTool struct{ stores *StoreManager }

func (t *ListTasksTool) Name() string          { return "tracker_list_tasks" }
func (t *ListTasksTool) Description() string   { return listTasksSchema.Description }
func (t *ListTasksTool) Schema() *agent.Schema { return listTasksSchema }

func (t *ListTasksTool) Execute(_ context.Context, args agent.Arguments) (agent.Result, error) {
	var p listTasksArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
	}

	store := t.stores.ForUser(args.UserID)
	tasks := store.ListTasks()

	// Apply filters.
	if p.Status != "" {
		tasks = filterTasks(tasks, func(tk *Task) bool { return string(tk.Status) == p.Status })
	}
	if p.Type != "" {
		tasks = filterTasks(tasks, func(tk *Task) bool { return string(tk.Type) == p.Type })
	}
	if p.ParentID != "" {
		tasks = filterTasks(tasks, func(tk *Task) bool { return tk.ParentID == p.ParentID })
	}

	if len(tasks) == 0 {
		return agent.TextResult("No tasks found matching the criteria."), nil
	}

	result := map[string]any{
		"status":  "ok",
		"message": fmt.Sprintf("Found %d task(s).", len(tasks)),
		"tasks":   tasks,
		"total":   len(tasks),
	}
	b, _ := json.Marshal(result)
	return agent.Result{Content: []any{string(b)}}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// tracker_add_dependency
// ═══════════════════════════════════════════════════════════════════════════════

type addDependencyArgs struct {
	TaskID       string `json:"task_id"`
	DependencyID string `json:"dependency_id"`
}

var addDependencySchema = &agent.Schema{
	Type:        agent.TypeObject,
	Description: "Link a dependency: 'task_id depends on dependency_id'. The system automatically detects and rejects cycles.",
	Properties: map[string]*agent.Schema{
		"task_id": {
			Type:        agent.TypeString,
			Description: "The ID of the task that has the dependency.",
		},
		"dependency_id": {
			Type:        agent.TypeString,
			Description: "The ID of the task that must be completed first.",
		},
	},
	Required: []string{"task_id", "dependency_id"},
}

// AddDependencyTool implements agent.ExecutableTool for tracker_add_dependency.
type AddDependencyTool struct{ stores *StoreManager }

func (t *AddDependencyTool) Name() string          { return "tracker_add_dependency" }
func (t *AddDependencyTool) Description() string   { return addDependencySchema.Description }
func (t *AddDependencyTool) Schema() *agent.Schema { return addDependencySchema }

func (t *AddDependencyTool) Execute(_ context.Context, args agent.Arguments) (agent.Result, error) {
	var p addDependencyArgs
	if err := args.Decode(&p); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
	}
	if p.TaskID == "" || p.DependencyID == "" {
		return agent.ErrorResult(fmt.Errorf("both task_id and dependency_id are required")), nil
	}

	store := t.stores.ForUser(args.UserID)
	if err := store.AddDependency(p.TaskID, p.DependencyID); err != nil {
		return agent.ErrorResult(err), nil
	}

	result := map[string]any{
		"status":  "ok",
		"message": fmt.Sprintf("Linked %s → %s (dependency added).", p.TaskID, p.DependencyID),
	}
	b, _ := json.Marshal(result)
	return agent.Result{Content: []any{string(b)}}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// tracker_visualize
// ═══════════════════════════════════════════════════════════════════════════════

// VisualizeTool implements agent.ExecutableTool for tracker_visualize.
type VisualizeTool struct{ stores *StoreManager }

func (t *VisualizeTool) Name() string { return "tracker_visualize" }
func (t *VisualizeTool) Description() string {
	return "Render a visual task graph showing the hierarchy, dependencies, and status of all tasks using emojis."
}
func (t *VisualizeTool) Schema() *agent.Schema { return nil }

func (t *VisualizeTool) Execute(_ context.Context, args agent.Arguments) (agent.Result, error) {
	store := t.stores.ForUser(args.UserID)
	tasks := store.ListTasks()
	if len(tasks) == 0 {
		return agent.TextResult("No tasks to visualize."), nil
	}

	statusEmojis := map[TaskStatus]string{
		StatusOpen:       "⭕",
		StatusInProgress: "🚧",
		StatusBlocked:    "🚫",
		StatusClosed:     "✅",
	}
	typeLabels := map[TaskType]string{
		TaskTypeEpic: "[EPIC]",
		TaskTypeTask: "[TASK]",
		TaskTypeBug:  "[BUG]",
	}

	// Build parent→children map.
	childrenMap := make(map[string][]*Task)
	var roots []*Task
	for _, task := range tasks {
		if task.ParentID != "" {
			childrenMap[task.ParentID] = append(childrenMap[task.ParentID], task)
		} else {
			roots = append(roots, task)
		}
	}

	var sb strings.Builder
	sb.WriteString("Task Tracker Graph:\n")

	var renderTask func(task *Task, depth int, visited map[string]bool)
	renderTask = func(task *Task, depth int, visited map[string]bool) {
		if visited[task.ID] {
			sb.WriteString(strings.Repeat("  ", depth))
			sb.WriteString(fmt.Sprintf("[CYCLE DETECTED: %s]\n", task.ID))
			return
		}
		visited[task.ID] = true

		indent := strings.Repeat("  ", depth)
		emoji := statusEmojis[task.Status]
		label := typeLabels[task.Type]
		sb.WriteString(fmt.Sprintf("%s%s %s %s %s\n", indent, emoji, task.ID, label, task.Title))

		if len(task.Dependencies) > 0 {
			sb.WriteString(fmt.Sprintf("%s  └─ Depends on: %s\n", indent, strings.Join(task.Dependencies, ", ")))
		}

		children := childrenMap[task.ID]
		for _, child := range children {
			// Copy visited set so sibling trees don't interfere.
			childVisited := make(map[string]bool, len(visited))
			for k, v := range visited {
				childVisited[k] = v
			}
			renderTask(child, depth+1, childVisited)
		}
	}

	for _, root := range roots {
		renderTask(root, 0, make(map[string]bool))
	}

	// Build structured result for UI rendering.
	result := map[string]any{
		"status": "ok",
		"graph":  sb.String(),
		"tasks":  tasks,
		"total":  len(tasks),
		"counts": countStatuses(tasks),
	}
	b, _ := json.Marshal(result)
	return agent.Result{Content: []any{string(b)}}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func filterTasks(tasks []*Task, pred func(*Task) bool) []*Task {
	var out []*Task
	for _, t := range tasks {
		if pred(t) {
			out = append(out, t)
		}
	}
	return out
}

func countStatuses(tasks []*Task) map[string]int {
	counts := map[string]int{
		"open":        0,
		"in_progress": 0,
		"blocked":     0,
		"closed":      0,
	}
	for _, t := range tasks {
		counts[string(t.Status)]++
	}
	return counts
}

// Compile-time interface checks.
var (
	_ agent.ExecutableTool = (*CreateTaskTool)(nil)
	_ agent.ExecutableTool = (*UpdateTaskTool)(nil)
	_ agent.ExecutableTool = (*GetTaskTool)(nil)
	_ agent.ExecutableTool = (*ListTasksTool)(nil)
	_ agent.ExecutableTool = (*AddDependencyTool)(nil)
	_ agent.ExecutableTool = (*VisualizeTool)(nil)
)
