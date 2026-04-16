// Package taskmgmt provides HTTP handlers for the chat module's task management API.
//
// Handlers are organised by domain concern across separate files:
//
//   - handlers.go          — TaskManagementHandler struct + constructor (this file)
//   - message_handler.go   — SendMessage (inline SSE streaming)
//   - thread_handler.go    — DeleteThread, UpdateThreadTitle
//   - agent_handler.go     — ListAgents, GetAgent
//   - artifact_handler.go  — ListArtifacts, DownloadArtifact, UploadAttachment
//   - toolset_handler.go   — ListAgentToolSets
//   - plugin_handler.go    — ListPlugins
//   - document_handler.go  — ListDocuments, UploadDocument
//   - helpers.go           — WriteSSEEvent
package taskmgmt

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/logger"
	"aicompass/modules/chat/chatcontracts"
	"aicompass/modules/chat/projection/agui"
	"aicompass/modules/chat/threadmgmt"
	"aicompass/platform/artifact"
	"aicompass/platform/toolregistry"
	"aicompass/shared"
	"time"
)

// SSEConfig holds tunable parameters for Server-Sent Event connections.
// All values have safe production defaults via DefaultSSEConfig().
type SSEConfig struct {
	// WriteTimeout is the deadline applied to every individual SSE write.
	// If a write cannot complete within this window the connection is torn
	// down and the client can retry.
	WriteTimeout time.Duration
}

// DefaultSSEConfig returns production-safe defaults.
func DefaultSSEConfig() SSEConfig {
	return SSEConfig{
		WriteTimeout: 15 * time.Second,
	}
}

// TaskManagementHandler encapsulates dependencies for HTTP handlers for task management.
type TaskManagementHandler struct {
	log                      *logger.Logger
	orchestrator             Orchestrator
	agentProvider            agent.AgentProvider
	authService              shared.AuthService
	threadQueryService       chatcontracts.ThreadQueryService
	aguiProjection           agui.AguiProjection
	threadMgmt               *threadmgmt.Manager
	artifactStore            artifact.Service
	toolRegistry             *toolregistry.Registry
	requirementConfigService chatcontracts.ToolRequirementConfigService
	documentProvider         chatcontracts.DocumentProvider
	tenantID                 string
	allowedOrigin            string
	sse                      SSEConfig
}

// NewTaskManagementHandler creates a new HTTP handler for task management.
func NewTaskManagementHandler(
	log *logger.Logger,
	orchestrator Orchestrator,
	agentProvider agent.AgentProvider,
	authService shared.AuthService,
	threadQueryService chatcontracts.ThreadQueryService,
	aguiProjection agui.AguiProjection,
	threadMgmt *threadmgmt.Manager,
	artifactStore artifact.Service,
	toolRegistry *toolregistry.Registry,
	requirementConfigService chatcontracts.ToolRequirementConfigService,
	documentProvider chatcontracts.DocumentProvider,
	tenantID string,
	allowedOrigin string,
	sse SSEConfig,
) *TaskManagementHandler {
	return &TaskManagementHandler{
		log:                      log,
		orchestrator:             orchestrator,
		agentProvider:            agentProvider,
		authService:              authService,
		threadQueryService:       threadQueryService,
		aguiProjection:           aguiProjection,
		threadMgmt:               threadMgmt,
		artifactStore:            artifactStore,
		toolRegistry:             toolRegistry,
		requirementConfigService: requirementConfigService,
		documentProvider:         documentProvider,
		tenantID:                 tenantID,
		allowedOrigin:            allowedOrigin,
		sse:                      sse,
	}
}
