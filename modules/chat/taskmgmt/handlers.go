// Package taskmgmt provides HTTP handlers for the chat module's task management API.
//
// Handlers are organised by domain concern across separate files:
//
//   - handlers.go          — TaskManagementHandler struct + constructor (this file)
//   - message_handler.go   — SendMessage
//   - stream_handler.go    — StreamJobEvents, GetJobStatus, CancelJob
//   - thread_handler.go    — DeleteThread, UpdateThreadTitle
//   - agent_handler.go     — ListAgents, GetAgent
//   - artifact_handler.go  — ListArtifacts, DownloadArtifact, UploadAttachment, GetMemory
//   - toolset_handler.go   — ListAgentToolSets, ToggleAgentToolSet, ToggleIndividualTool
//   - plugin_handler.go    — ListPlugins, TogglePlugin, TogglePluginTool
//   - document_handler.go  — ListDocuments, UploadDocument
//   - task_types.go        — Task / TaskState value types
//   - helpers.go           — WriteSSEEvent
package taskmgmt

import (
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/chat/chatcontracts"
	"school-with-ai/modules/chat/memory"
	"school-with-ai/modules/chat/projection/agui"
	"school-with-ai/modules/chat/taskmanagement"
	"school-with-ai/modules/chat/threadmgmt"
	"school-with-ai/platform/artifact"
	"school-with-ai/platform/toolregistry"
	"school-with-ai/shared"
)

// SSEConfig holds tunable parameters for Server-Sent Event connections.
// All values have safe production defaults via DefaultSSEConfig().
type SSEConfig struct {
	// WriteTimeout is the deadline applied to every individual SSE write
	// (heartbeat or event frame). If a write cannot complete within this
	// window the connection is torn down and the client reconnects with
	// ?lastEventId= for lossless replay.
	WriteTimeout time.Duration

	// HeartbeatInterval controls how often an SSE comment line is sent to
	// probe the connection. Should be ≤ WriteTimeout so idle connections
	// are tested within one deadline window.
	HeartbeatInterval time.Duration

	// MaxConnectionTTL is the maximum lifetime of a single SSE connection.
	// After this duration the server closes the connection with a retry: 0
	// hint so the client reconnects immediately (to any healthy pod).
	// This is a safety net for graceful rolling deploys — with per-write
	// deadlines, dead clients are caught much sooner.
	MaxConnectionTTL time.Duration
}

// DefaultSSEConfig returns production-safe defaults.
func DefaultSSEConfig() SSEConfig {
	return SSEConfig{
		WriteTimeout:      15 * time.Second,
		HeartbeatInterval: 15 * time.Second,
		MaxConnectionTTL:  30 * time.Minute,
	}
}

// TaskManagementHandler encapsulates dependencies for HTTP handlers for task management.
type TaskManagementHandler struct {
	log                *logger.Logger
	taskManager        *taskmanagement.AsynqTaskManager
	agentProvider      agent.AgentProvider
	authService        shared.AuthService
	threadQueryService chatcontracts.ThreadQueryService
	aguiProjection     agui.AguiProjection
	broadcaster        agui.AguiBroadcaster
	threadMgmt         *threadmgmt.Manager
	artifactStore      artifact.Service
	memoryStore        memory.Service
	toolRegistry       *toolregistry.Registry
	permissionsService chatcontracts.ToolPermissionService
	documentProvider   chatcontracts.DocumentProvider
	tenantID           string
	allowedOrigin      string
	sse                SSEConfig
}

// NewTaskManagementHandler creates a new HTTP handler for task management.
func NewTaskManagementHandler(
	log *logger.Logger,
	taskManager *taskmanagement.AsynqTaskManager,
	agentProvider agent.AgentProvider,
	authService shared.AuthService,
	threadQueryService chatcontracts.ThreadQueryService,
	aguiProjection agui.AguiProjection,
	broadcaster agui.AguiBroadcaster,
	threadMgmt *threadmgmt.Manager,
	artifactStore artifact.Service,
	memoryStore memory.Service,
	toolRegistry *toolregistry.Registry,
	permissionsService chatcontracts.ToolPermissionService,
	documentProvider chatcontracts.DocumentProvider,
	tenantID string,
	allowedOrigin string,
	sse SSEConfig,
) *TaskManagementHandler {
	return &TaskManagementHandler{
		log:                log,
		taskManager:        taskManager,
		agentProvider:      agentProvider,
		authService:        authService,
		threadQueryService: threadQueryService,
		aguiProjection:     aguiProjection,
		broadcaster:        broadcaster,
		threadMgmt:         threadMgmt,
		artifactStore:      artifactStore,
		memoryStore:        memoryStore,
		toolRegistry:       toolRegistry,
		permissionsService: permissionsService,
		documentProvider:   documentProvider,
		tenantID:           tenantID,
		allowedOrigin:      allowedOrigin,
		sse:                sse,
	}
}
