package tracker

import (
	"context"
	"sync"
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/platform/toolregistry"

	"github.com/a-h/templ"
)

// Provider implements the toolregistry.PluginProvider interface for the
// Task Tracker plugin — a full in-session project management toolkit for agents.
//
// Inspired by Gemini CLI's tracker_* tool family, it provides six tools:
//   - tracker_create_task: create tasks (epic/task/bug) with parent & dependencies
//   - tracker_update_task: update title, description, status, dependencies
//   - tracker_get_task:    retrieve task details
//   - tracker_list_tasks:  list/filter tasks by status, type, parent
//   - tracker_add_dependency: link task dependencies (with cycle detection)
//   - tracker_visualize:   render an ASCII/emoji task graph
type Provider struct{}

func (p *Provider) ID() string { return "tracker" }

func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(
		`<rect width="18" height="18" x="3" y="3" rx="2"/>` +
			`<path d="M7 7h10"/>` +
			`<path d="M7 12h10"/>` +
			`<path d="M7 17h10"/>`,
	)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "tracker-set",
			Name:        "Task Tracker",
			Description: "Full project management for agents — hierarchical tasks with dependencies, statuses, and visual graphs",
			Category:    "Planning",
			ToolIDs: []string{
				"tracker_create_task",
				"tracker_update_task",
				"tracker_get_task",
				"tracker_list_tasks",
				"tracker_add_dependency",
				"tracker_visualize",
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{
			ID:          "tracker_create_task",
			Name:        "Create Task",
			Description: "Create a new task (epic, task, or bug) with optional parent and dependencies",
			Category:    "Task Tracker",
			ToolSetID:   "tracker-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "tracker_update_task",
			Name:        "Update Task",
			Description: "Update an existing task's title, description, status, or dependencies",
			Category:    "Task Tracker",
			ToolSetID:   "tracker-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "tracker_get_task",
			Name:        "Get Task",
			Description: "Retrieve detailed information about a specific task by ID",
			Category:    "Task Tracker",
			ToolSetID:   "tracker-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "tracker_list_tasks",
			Name:        "List Tasks",
			Description: "List and filter tasks by status, type, or parent task",
			Category:    "Task Tracker",
			ToolSetID:   "tracker-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "tracker_add_dependency",
			Name:        "Add Dependency",
			Description: "Link a dependency between tasks — 'task A depends on task B' (with automatic cycle detection)",
			Category:    "Task Tracker",
			ToolSetID:   "tracker-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "tracker_visualize",
			Name:        "Visualize Tasks",
			Description: "Render a visual task graph showing hierarchy, dependencies, and status emojis",
			Category:    "Task Tracker",
			ToolSetID:   "tracker-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

// CreateExecutable creates the executable tool for the given tool ID.
// All tools share a single per-session Store that is stored in the agent session.
// For now we create a new store per factory call; the FactoryDeps can be
// extended later to carry a session-scoped Store for true persistence.
func (p *Provider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	// Each agent session should share the same store instance.
	// The session-scoped wiring is handled at the factory level; here we
	// simply instantiate. The factory/session manager is expected to re-use
	// the same Provider instance per session so the store survives.
	store := getOrCreateSessionStore()
	switch toolID {
	case "tracker_create_task":
		return &CreateTaskTool{store: store}, nil
	case "tracker_update_task":
		return &UpdateTaskTool{store: store}, nil
	case "tracker_get_task":
		return &GetTaskTool{store: store}, nil
	case "tracker_list_tasks":
		return &ListTasksTool{store: store}, nil
	case "tracker_add_dependency":
		return &AddDependencyTool{store: store}, nil
	case "tracker_visualize":
		return &VisualizeTool{store: store}, nil
	default:
		return nil, nil
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{
		"tracker_create_task",
		"tracker_update_task",
		"tracker_get_task",
		"tracker_list_tasks",
		"tracker_add_dependency",
		"tracker_visualize",
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Session-scoped store singleton
// ──────────────────────────────────────────────────────────────────────────────

// sessionStore holds a lazily-created Store that lives for the duration of the
// agent session. In a future iteration this can be replaced with a
// PostgreSQL-backed store keyed by user_id+thread_id.
var (
	sessionStoreMu   = &sessionStoreSync{}
	sessionStoreInst *Store
)

type sessionStoreSync struct{ sync.Mutex }

func getOrCreateSessionStore() *Store {
	sessionStoreMu.Lock()
	defer sessionStoreMu.Unlock()
	if sessionStoreInst == nil {
		sessionStoreInst = NewStore()
	}
	return sessionStoreInst
}

// ResetSessionStore is exposed for tests — clears the in-memory store.
func ResetSessionStore() {
	sessionStoreMu.Lock()
	defer sessionStoreMu.Unlock()
	sessionStoreInst = nil
}
