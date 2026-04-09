package todos

import (
	"aicompass/foundation/agent"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// TodoStatus represents the state of a single todo item.
type TodoStatus string

const (
	StatusPending    TodoStatus = "pending"
	StatusInProgress TodoStatus = "in_progress"
	StatusCompleted  TodoStatus = "completed"
	StatusCancelled  TodoStatus = "cancelled"
)

// validStatuses is the set of allowed status values.
var validStatuses = map[TodoStatus]bool{
	StatusPending:    true,
	StatusInProgress: true,
	StatusCompleted:  true,
	StatusCancelled:  true,
}

// TodoItem represents a single task in the agent's plan.
type TodoItem struct {
	Description string     `json:"description"`
	Status      TodoStatus `json:"status"`
}

// WriteTodosArgs is the input schema for the write_todos tool.
type WriteTodosArgs struct {
	Todos []TodoItem `json:"todos"`
}

var writeTodosSchema = &agent.Schema{
	Type:        agent.TypeObject,
	Description: "Manage a structured task list to track progress. Use this frequently during multi-step work to give the user visibility into your plan and progress. Mark tasks as in_progress before starting, and completed immediately after finishing. Only one task may be in_progress at a time.",
	Properties: map[string]*agent.Schema{
		"todos": {
			Type:        agent.TypeArray,
			Description: "The complete list of todos. This replaces any existing list — always provide ALL items.",
			Items: &agent.Schema{
				Type: agent.TypeObject,
				Properties: map[string]*agent.Schema{
					"description": {
						Type:        agent.TypeString,
						Description: "A concise, action-oriented description of the task.",
					},
					"status": {
						Type:        agent.TypeString,
						Description: "The current status of the task.",
						Enum:        []string{"pending", "in_progress", "completed", "cancelled"},
					},
				},
				Required: []string{"description", "status"},
			},
		},
	},
	Required: []string{"todos"},
}

// WriteTodosTool allows the agent to create and maintain a structured task list.
type WriteTodosTool struct{}

func (t *WriteTodosTool) Name() string          { return "write_todos" }
func (t *WriteTodosTool) Description() string   { return writeTodosSchema.Description }
func (t *WriteTodosTool) Schema() *agent.Schema { return writeTodosSchema }

func (t *WriteTodosTool) Execute(_ context.Context, args agent.Arguments) (agent.Result, error) {
	var parsed WriteTodosArgs
	if err := args.Decode(&parsed); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
	}

	// Validate todos
	if err := validateTodos(parsed.Todos); err != nil {
		return agent.ErrorResult(err), nil
	}

	// Build the result summary for the LLM
	todos := parsed.Todos
	if len(todos) == 0 {
		return agent.JSONResult(map[string]any{
			"status":  "ok",
			"message": "Todo list cleared.",
			"todos":   []any{},
		}), nil
	}

	var sb strings.Builder
	for i, todo := range todos {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, todo.Status, todo.Description))
	}

	counts := countByStatus(todos)

	// Return rich structured result for the UI to render
	result := map[string]any{
		"status":      "ok",
		"message":     fmt.Sprintf("Updated %d task(s).", len(todos)),
		"todos":       todos,
		"summary":     sb.String(),
		"completed":   counts[StatusCompleted],
		"pending":     counts[StatusPending],
		"in_progress": counts[StatusInProgress],
		"cancelled":   counts[StatusCancelled],
		"total":       len(todos),
	}

	resultJSON, _ := json.Marshal(result)
	return agent.Result{
		Content: []any{string(resultJSON)},
	}, nil
}

// validateTodos checks the todo list for structural correctness.
func validateTodos(todos []TodoItem) error {
	if len(todos) > 25 {
		return fmt.Errorf("too many todos: maximum 25 allowed, got %d", len(todos))
	}

	inProgressCount := 0
	for i, todo := range todos {
		if strings.TrimSpace(todo.Description) == "" {
			return fmt.Errorf("todo %d: description must be a non-empty string", i+1)
		}
		if !validStatuses[todo.Status] {
			return fmt.Errorf("todo %d: invalid status %q (must be pending, in_progress, completed, or cancelled)", i+1, todo.Status)
		}
		if todo.Status == StatusInProgress {
			inProgressCount++
		}
	}

	if inProgressCount > 1 {
		return fmt.Errorf("only one task may be in_progress at a time, found %d", inProgressCount)
	}

	return nil
}

// countByStatus counts todos by status.
func countByStatus(todos []TodoItem) map[TodoStatus]int {
	counts := make(map[TodoStatus]int)
	for _, todo := range todos {
		counts[todo.Status]++
	}
	return counts
}

var _ agent.ExecutableTool = (*WriteTodosTool)(nil)
