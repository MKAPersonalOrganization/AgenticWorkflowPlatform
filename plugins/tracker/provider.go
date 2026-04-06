package tracker

import (
	"context"
	"sync"
	"time"

	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"

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
// All tools share a StoreManager that hands out per-user stores, ensuring
// concurrent users have isolated task tracker state.
func (p *Provider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	stores := globalStoreManager()
	switch toolID {
	case "tracker_create_task":
		return &CreateTaskTool{stores: stores}, nil
	case "tracker_update_task":
		return &UpdateTaskTool{stores: stores}, nil
	case "tracker_get_task":
		return &GetTaskTool{stores: stores}, nil
	case "tracker_list_tasks":
		return &ListTasksTool{stores: stores}, nil
	case "tracker_add_dependency":
		return &AddDependencyTool{stores: stores}, nil
	case "tracker_visualize":
		return &VisualizeTool{stores: stores}, nil
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
// StoreManager — per-user store isolation
// ──────────────────────────────────────────────────────────────────────────────

// StoreManager provides per-user Store instances so that concurrent users
// have isolated task tracker state.
type StoreManager struct {
	mu     sync.Mutex
	stores map[string]*Store // userID → store
}

// NewStoreManager creates an empty manager.
func NewStoreManager() *StoreManager {
	return &StoreManager{stores: make(map[string]*Store)}
}

// ForUser returns the Store for the given user, creating one if needed.
func (m *StoreManager) ForUser(userID string) *Store {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.stores[userID]
	if !ok {
		s = NewStore()
		m.stores[userID] = s
	}
	return s
}

// Reset clears all stores. Exposed for tests.
func (m *StoreManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stores = make(map[string]*Store)
}

// global instance — lazily created
var (
	globalMgrMu   sync.Mutex
	globalMgrInst *StoreManager
)

func globalStoreManager() *StoreManager {
	globalMgrMu.Lock()
	defer globalMgrMu.Unlock()
	if globalMgrInst == nil {
		globalMgrInst = NewStoreManager()
	}
	return globalMgrInst
}

// ResetGlobalStoreManager clears the global StoreManager. Exposed for tests.
func ResetGlobalStoreManager() {
	globalMgrMu.Lock()
	defer globalMgrMu.Unlock()
	globalMgrInst = nil
}
