package tracker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// ──────────────────────────────────────────────────────────────────────────────
// Domain types
// ──────────────────────────────────────────────────────────────────────────────

// TaskType categorises the scope/nature of a tracked item.
type TaskType string

const (
	TaskTypeEpic TaskType = "epic"
	TaskTypeTask TaskType = "task"
	TaskTypeBug  TaskType = "bug"
)

// validTaskTypes is the allowlist.
var validTaskTypes = map[TaskType]bool{
	TaskTypeEpic: true,
	TaskTypeTask: true,
	TaskTypeBug:  true,
}

// TaskStatus represents a task's lifecycle stage.
type TaskStatus string

const (
	StatusOpen       TaskStatus = "open"
	StatusInProgress TaskStatus = "in_progress"
	StatusBlocked    TaskStatus = "blocked"
	StatusClosed     TaskStatus = "closed"
)

// validStatuses is the allowlist.
var validStatuses = map[TaskStatus]bool{
	StatusOpen:       true,
	StatusInProgress: true,
	StatusBlocked:    true,
	StatusClosed:     true,
}

// Task is the core tracker entity.
type Task struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Type         TaskType   `json:"type"`
	Status       TaskStatus `json:"status"`
	ParentID     string     `json:"parent_id,omitempty"`
	Dependencies []string   `json:"dependencies"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Store — thread-scoped, in-memory implementation
// ──────────────────────────────────────────────────────────────────────────────

// Limits (matching the ADR spec).
const (
	MaxTasksPerThread      = 100
	MaxDependenciesPerTask = 10
)

// Store provides CRUD and dependency management for tasks scoped to a single
// user+thread. The implementation is an in-memory map guarded by a mutex;
// a future iteration can swap this for a PostgreSQL-backed store without
// changing the tool code.
type Store struct {
	mu    sync.RWMutex
	tasks map[string]*Task // id → task
	order []string         // insertion order
}

// NewStore creates an empty store.
func NewStore() *Store {
	return &Store{
		tasks: make(map[string]*Task),
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// CRUD
// ──────────────────────────────────────────────────────────────────────────────

// CreateTask validates and persists a new task, returning the created task.
func (s *Store) CreateTask(title, description string, taskType TaskType, parentID string, deps []string) (*Task, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if !validTaskTypes[taskType] {
		return nil, fmt.Errorf("invalid task type %q (must be epic, task, or bug)", taskType)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.tasks) >= MaxTasksPerThread {
		return nil, fmt.Errorf("maximum task limit reached (%d tasks per thread)", MaxTasksPerThread)
	}

	// Validate parent exists if provided.
	if parentID != "" {
		if _, ok := s.tasks[parentID]; !ok {
			return nil, fmt.Errorf("parent task %q not found", parentID)
		}
	}

	// Validate dependencies exist and respect the limit.
	if len(deps) > MaxDependenciesPerTask {
		return nil, fmt.Errorf("too many dependencies: max %d allowed, got %d", MaxDependenciesPerTask, len(deps))
	}
	for _, depID := range deps {
		if _, ok := s.tasks[depID]; !ok {
			return nil, fmt.Errorf("dependency task %q not found", depID)
		}
	}

	task := &Task{
		ID:           generateID(),
		Title:        title,
		Description:  description,
		Type:         taskType,
		Status:       StatusOpen,
		ParentID:     parentID,
		Dependencies: uniqueStrings(deps),
	}
	s.tasks[task.ID] = task
	s.order = append(s.order, task.ID)
	return task, nil
}

// UpdateTask applies partial updates to an existing task.
func (s *Store) UpdateTask(id string, title, description *string, status *TaskStatus, deps *[]string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %q not found", id)
	}

	if title != nil {
		if *title == "" {
			return nil, fmt.Errorf("title must be a non-empty string")
		}
		task.Title = *title
	}
	if description != nil {
		task.Description = *description
	}
	if status != nil {
		if !validStatuses[*status] {
			return nil, fmt.Errorf("invalid status %q", *status)
		}
		task.Status = *status
	}
	if deps != nil {
		if len(*deps) > MaxDependenciesPerTask {
			return nil, fmt.Errorf("too many dependencies: max %d allowed, got %d", MaxDependenciesPerTask, len(*deps))
		}
		for _, depID := range *deps {
			if depID == id {
				return nil, fmt.Errorf("task cannot depend on itself")
			}
			if _, exists := s.tasks[depID]; !exists {
				return nil, fmt.Errorf("dependency task %q not found", depID)
			}
		}
		// Cycle detection: ensure adding these deps doesn't create a cycle.
		if err := s.detectCycleLocked(id, uniqueStrings(*deps)); err != nil {
			return nil, err
		}
		task.Dependencies = uniqueStrings(*deps)
	}

	return task, nil
}

// GetTask returns a task by ID (nil if not found).
func (s *Store) GetTask(id string) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tasks[id]
	if !ok {
		return nil
	}
	// Return a copy to prevent mutation outside the lock.
	cp := *t
	cp.Dependencies = append([]string(nil), t.Dependencies...)
	return &cp
}

// ListTasks returns all tasks in insertion order.
func (s *Store) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Task, 0, len(s.order))
	for _, id := range s.order {
		if t, ok := s.tasks[id]; ok {
			cp := *t
			cp.Dependencies = append([]string(nil), t.Dependencies...)
			result = append(result, &cp)
		}
	}
	return result
}

// AddDependency links dependencyID as a prerequisite of taskID.
func (s *Store) AddDependency(taskID, dependencyID string) error {
	if taskID == dependencyID {
		return fmt.Errorf("task %q cannot depend on itself", taskID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	if _, ok := s.tasks[dependencyID]; !ok {
		return fmt.Errorf("dependency task %q not found", dependencyID)
	}

	// Already present?
	for _, d := range task.Dependencies {
		if d == dependencyID {
			return nil // idempotent
		}
	}

	if len(task.Dependencies) >= MaxDependenciesPerTask {
		return fmt.Errorf("task %q already has the maximum number of dependencies (%d)", taskID, MaxDependenciesPerTask)
	}

	// Build candidate list and check for cycles.
	candidate := append(append([]string(nil), task.Dependencies...), dependencyID)
	if err := s.detectCycleLocked(taskID, candidate); err != nil {
		return err
	}

	task.Dependencies = candidate
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Cycle detection
// ──────────────────────────────────────────────────────────────────────────────

// detectCycleLocked performs DFS to ensure the proposed dependency set for
// taskID does not create a cycle. Must be called with s.mu held.
func (s *Store) detectCycleLocked(taskID string, proposedDeps []string) error {
	visited := make(map[string]bool)
	var dfs func(id string) bool
	dfs = func(id string) bool {
		if id == taskID {
			return true // cycle!
		}
		if visited[id] {
			return false
		}
		visited[id] = true
		t, ok := s.tasks[id]
		if !ok {
			return false
		}
		for _, dep := range t.Dependencies {
			if dfs(dep) {
				return true
			}
		}
		return false
	}

	for _, depID := range proposedDeps {
		if dfs(depID) {
			return fmt.Errorf("adding dependency would create a cycle: %s -> ... -> %s", taskID, taskID)
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// generateID returns a random 6-character hex string (matching Gemini CLI's id format).
func generateID() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// uniqueStrings deduplicates while preserving order.
func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
