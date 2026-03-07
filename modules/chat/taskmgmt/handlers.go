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
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/chat/chatcontracts"
	"school-with-ai/modules/chat/memory"
	"school-with-ai/modules/chat/projection/agui"
	"school-with-ai/modules/chat/taskmanagement"
	"school-with-ai/modules/chat/threadmgmt"
	"school-with-ai/modules/permissions/tool"
	"school-with-ai/platform/artifact"
	"school-with-ai/platform/toolregistry"
	"school-with-ai/shared"
)

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
	permissionsService *tool.Service
	documentProvider   chatcontracts.DocumentProvider
	tenantID           string
	allowedOrigin      string
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
	permissionsService *tool.Service,
	documentProvider chatcontracts.DocumentProvider,
	tenantID string,
	allowedOrigin string,
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
	}
}
