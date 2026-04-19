package agentdefinition

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// AgentModelConfig — per-agent LLM generation parameters
//
// Stored as JSONB in user_agents.model_config. All fields are optional
// (pointer types); nil means "use the user's global AI settings".
// Mirrors foundation/agent.AgentModelConfig but is defined here to keep
// the agentdefinition module free of foundation imports.
// ──────────────────────────────────────────────────────────────────────────────

// AgentModelConfig holds per-agent overrides for LLM generation parameters.
type AgentModelConfig struct {
	// Temperature controls randomness (0.0–2.0). nil = user global default.
	Temperature *float64 `json:"temperature,omitempty"`

	// TopP sets nucleus sampling probability (0.0–1.0). nil = user global default.
	TopP *float64 `json:"topP,omitempty"`

	// TopK limits the token vocabulary at each sampling step (stored as float32 matching genai). nil = provider default.
	TopK *float32 `json:"topK,omitempty"`

	// MaxOutputTokens caps the response length. nil = user global default.
	MaxOutputTokens *int32 `json:"maxOutputTokens,omitempty"`

	// StopSequences halts generation on any of these strings. Empty = no custom stops.
	StopSequences []string `json:"stopSequences,omitempty"`
}

// AgentMemoryPolicy mirrors foundation/agent.AgentMemoryPolicy and is the
// shape persisted on an AgentDefinition. The JSON tags use camelCase to
// match the frontend payload.
type AgentMemoryPolicy struct {
	// Enabled turns on cross-session memory for this agent. When true,
	// the orchestration engine's `load_memory` tool is injected so the
	// model can search past sessions on demand.
	Enabled bool `json:"enabled"`

	// Preload, when true, also injects `preload_memory` so relevant
	// memories are searched and added to the system instruction at the
	// start of every turn. Requires Enabled=true; ignored otherwise.
	Preload bool `json:"preload"`
}

// AgentCodeExecutionPolicy mirrors foundation/agent.AgentCodeExecutionPolicy.
// Intentionally provider-agnostic — the adapter dispatches the "enabled"
// flag to whatever code-interpreter mechanism the active model supports.
type AgentCodeExecutionPolicy struct {
	// Enabled lets the agent ask the LLM provider's sandbox to run code.
	// Gemini is wired today; OpenAI / Anthropic fall through silently
	// until their provider adapters gain support.
	Enabled bool `json:"enabled"`
}

// AgentStreamingPolicy mirrors foundation/agent.AgentStreamingPolicy.
// Controls whether the adapter asks ADK's runner to stream the LLM
// response via SSE (delta-by-delta) or to deliver it as a single frame.
type AgentStreamingPolicy struct {
	// Enabled toggles SSE streaming. Nil policy (the column is NULL for
	// pre-existing rows) means streaming is on — matches the adapter's
	// historical default before this field existed.
	Enabled bool `json:"enabled"`
}

// AgentArtifactPolicy mirrors foundation/agent.AgentArtifactPolicy.
type AgentArtifactPolicy struct {
	// Enabled turns on artifact support for this agent. When true,
	// `load_artifacts` is injected and the runner's artifact service is
	// wired so the agent can list, read, and create files that persist
	// across sessions.
	Enabled bool `json:"enabled"`

	// AutoSaveUserBlobs asks the runner to move inline blobs from user
	// messages into the artifact store automatically, replacing them with
	// text placeholders. Requires Enabled=true; ignored otherwise.
	AutoSaveUserBlobs bool `json:"autoSaveUserBlobs"`
}

// Validate checks that all non-nil fields are within acceptable ranges.
func (c *AgentModelConfig) Validate() error {
	if c == nil {
		return nil
	}
	if c.Temperature != nil && (*c.Temperature < 0.0 || *c.Temperature > 2.0) {
		return fmt.Errorf("temperature must be between 0.0 and 2.0, got %g", *c.Temperature)
	}
	if c.TopP != nil && (*c.TopP < 0.0 || *c.TopP > 1.0) {
		return fmt.Errorf("topP must be between 0.0 and 1.0, got %g", *c.TopP)
	}
	if c.TopK != nil && *c.TopK < 1 {
		return fmt.Errorf("topK must be at least 1, got %g", *c.TopK)
	}
	if c.MaxOutputTokens != nil && (*c.MaxOutputTokens < 1 || *c.MaxOutputTokens > 65536) {
		return fmt.Errorf("maxOutputTokens must be between 1 and 65536, got %d", *c.MaxOutputTokens)
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// AgentDefinition — the universal building block
//
// Every agent in the system — whether a simple LLM agent or a composed
// multi-agent workflow — is an AgentDefinition. This mirrors ADK-Go where
// every agent type (llmagent, sequentialagent, parallelagent, loopagent)
// implements the same Agent interface.
//
// An agent with a non-nil Graph is a "composed agent" that orchestrates
// sub-agents via sequential, parallel, or loop patterns. An agent without
// a graph is a simple LLM agent with an instruction (system prompt).
//
// Creating an agent programmatically (via ADK-Go constructors) and through
// the web interface (via this definition) are completely equivalent.
// ──────────────────────────────────────────────────────────────────────────────

type AgentDefinition struct {
	ID              string `json:"id" db:"id"`
	UserID          string `json:"user_id" db:"user_id"`
	Name            string `json:"name" db:"name"`
	Description     string `json:"description" db:"description"`
	SystemPrompt    string `json:"system_prompt" db:"system_prompt"`
	Icon            string `json:"icon" db:"icon"`
	SourceCatalogID string `json:"source_catalog_id,omitempty" db:"source_catalog_id"`

	// WorkflowID is deprecated. Kept for migration compatibility.
	// In the unified model all agents are first-class and visible.
	WorkflowID string `json:"-" db:"workflow_id"`

	// ── Composition ──
	// Graph holds the composition tree for composed agents.
	// When non-nil, this agent orchestrates sub-agents (sequential, parallel,
	// loop patterns). Leaf nodes reference other AgentDefinitions by ID.
	// Nil/empty means this is a simple LLM agent.
	Graph     *CompositionNode `json:"graph,omitempty" db:"-"`
	GraphJSON string           `json:"-" db:"graph"`

	// ── Ownership ──
	// ParentID is the ID of the workflow (composed AgentDefinition) that owns
	// this agent. Empty for top-level agents. When set, this agent is deleted
	// when its parent workflow is deleted.
	ParentID string `json:"parent_id,omitempty" db:"parent_id"`

	// ── Coordinator Sub-Agents ──
	// SubAgents holds inline sub-agent definitions. When non-empty, this agent
	// acts as a coordinator: an LlmAgent whose SubAgents are populated directly,
	// enabling peer-to-peer transfers without a workflow wrapper.
	// Mutually exclusive with Graph.
	SubAgents     []*SubAgentRef `json:"sub_agents,omitempty" db:"-"`
	SubAgentsJSON string         `json:"-" db:"sub_agents"`

	// ── Agent-as-Tool References ──
	// AgentTools exposes other agents as callable tools on this agent. Unlike
	// SubAgents (transfer-based), the parent keeps control: it calls the child,
	// receives a result, and continues. Maps to ADK-Go's tool/agenttool package.
	// Orthogonal to SubAgents (coordinator transfer) and Graph (workflow),
	// and may be combined with either.
	AgentTools     []*AgentToolRef `json:"agent_tools,omitempty" db:"-"`
	AgentToolsJSON string          `json:"-" db:"agent_tools"`

	// ── Multi-Agent Orchestration (used when this agent participates as a sub-agent) ──

	DisallowTransferToParent bool   `json:"disallow_transfer_to_parent" db:"disallow_transfer_to_parent"`
	DisallowTransferToPeers  bool   `json:"disallow_transfer_to_peers" db:"disallow_transfer_to_peers"`
	OutputKey                string `json:"output_key,omitempty" db:"output_key"`
	IncludeContents          string `json:"include_contents,omitempty" db:"include_contents"`
	GlobalInstruction        string `json:"global_instruction,omitempty" db:"global_instruction"`
	InputSchemaJSON          string `json:"input_schema,omitempty" db:"input_schema"`
	OutputSchemaJSON         string `json:"output_schema,omitempty" db:"output_schema"`

	// ── Model Configuration ──
	// ModelConfig holds per-agent LLM generation parameter overrides.
	// nil means the user's global AI settings apply.
	ModelConfig     *AgentModelConfig `json:"model_config,omitempty" db:"-"`
	ModelConfigJSON string            `json:"-" db:"model_config"`

	// ModelOverride selects a specific provider+model for this agent.
	// nil means use the user's global AI provider/model.
	ModelOverride     *AgentModelOverride `json:"model_override,omitempty" db:"-"`
	ModelOverrideJSON string              `json:"-" db:"model_override"`

	// ── Memory ──
	// MemoryPolicy controls cross-session memory for this agent. nil means
	// memory is off for this agent; non-nil carries the user's choice.
	MemoryPolicy     *AgentMemoryPolicy `json:"memory_policy,omitempty" db:"-"`
	MemoryPolicyJSON string             `json:"-" db:"memory_policy"`

	// ── Artifacts ──
	// ArtifactPolicy controls artifact support for this agent. nil means
	// artifacts are off; non-nil toggles the load_artifacts tool and the
	// auto-save-inline-blobs behaviour.
	ArtifactPolicy     *AgentArtifactPolicy `json:"artifact_policy,omitempty" db:"-"`
	ArtifactPolicyJSON string               `json:"-" db:"artifact_policy"`

	// ── Code Execution ──
	// CodeExecutionPolicy controls whether the agent may use the LLM
	// provider's built-in code executor. nil means off; non-nil enables
	// the adapter's provider-specific dispatch.
	CodeExecutionPolicy     *AgentCodeExecutionPolicy `json:"code_execution_policy,omitempty" db:"-"`
	CodeExecutionPolicyJSON string                    `json:"-" db:"code_execution_policy"`

	// ── Streaming ──
	// StreamingPolicy controls whether the adapter requests SSE streaming
	// from ADK's runner. nil means stream (historical default); non-nil
	// carries the explicit toggle value from the designer.
	StreamingPolicy     *AgentStreamingPolicy `json:"streaming_policy,omitempty" db:"-"`
	StreamingPolicyJSON string                `json:"-" db:"streaming_policy"`

	// ── Sharing ──
	// ShareToken is a unique token that enables sharing this agent with other users.
	// Empty means the agent is private. Generate via the share endpoint.
	ShareToken string `json:"share_token,omitempty" db:"share_token"`
	// Visibility controls who can see this agent: "private" (default) or "shared".
	Visibility string `json:"visibility,omitempty" db:"visibility"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// IsComposed returns true if this agent has a composition graph
// (i.e., it orchestrates sub-agents rather than being a simple LLM agent).
func (a *AgentDefinition) IsComposed() bool {
	return a.Graph != nil
}

// MarshalGraph serializes the Graph to GraphJSON for storage.
func (a *AgentDefinition) MarshalGraph() error {
	if a.Graph == nil {
		a.GraphJSON = ""
		return nil
	}
	b, err := json.Marshal(a.Graph)
	if err != nil {
		return fmt.Errorf("marshalling composition graph: %w", err)
	}
	a.GraphJSON = string(b)
	return nil
}

// UnmarshalGraph deserializes GraphJSON into Graph.
func (a *AgentDefinition) UnmarshalGraph() error {
	if a.GraphJSON == "" || a.GraphJSON == "null" {
		a.Graph = nil
		return nil
	}
	var g CompositionNode
	if err := json.Unmarshal([]byte(a.GraphJSON), &g); err != nil {
		return fmt.Errorf("unmarshalling composition graph: %w", err)
	}
	a.Graph = &g
	return nil
}

// MarshalModelConfig serializes ModelConfig to ModelConfigJSON for storage.
func (a *AgentDefinition) MarshalModelConfig() error {
	if a.ModelConfig == nil {
		a.ModelConfigJSON = ""
		return nil
	}
	b, err := json.Marshal(a.ModelConfig)
	if err != nil {
		return fmt.Errorf("marshalling model config: %w", err)
	}
	a.ModelConfigJSON = string(b)
	return nil
}

// UnmarshalModelConfig deserializes ModelConfigJSON into ModelConfig.
func (a *AgentDefinition) UnmarshalModelConfig() error {
	if a.ModelConfigJSON == "" || a.ModelConfigJSON == "null" {
		a.ModelConfig = nil
		return nil
	}
	var mc AgentModelConfig
	if err := json.Unmarshal([]byte(a.ModelConfigJSON), &mc); err != nil {
		return fmt.Errorf("unmarshalling model config: %w", err)
	}
	a.ModelConfig = &mc
	return nil
}

// MarshalMemoryPolicy serializes MemoryPolicy to MemoryPolicyJSON for storage.
func (a *AgentDefinition) MarshalMemoryPolicy() error {
	if a.MemoryPolicy == nil {
		a.MemoryPolicyJSON = ""
		return nil
	}
	b, err := json.Marshal(a.MemoryPolicy)
	if err != nil {
		return fmt.Errorf("marshalling memory policy: %w", err)
	}
	a.MemoryPolicyJSON = string(b)
	return nil
}

// UnmarshalMemoryPolicy deserializes MemoryPolicyJSON into MemoryPolicy.
// Empty JSON means nil — memory is off for this agent.
func (a *AgentDefinition) UnmarshalMemoryPolicy() error {
	if a.MemoryPolicyJSON == "" || a.MemoryPolicyJSON == "null" {
		a.MemoryPolicy = nil
		return nil
	}
	var mp AgentMemoryPolicy
	if err := json.Unmarshal([]byte(a.MemoryPolicyJSON), &mp); err != nil {
		return fmt.Errorf("unmarshalling memory policy: %w", err)
	}
	a.MemoryPolicy = &mp
	return nil
}

// MarshalArtifactPolicy serializes ArtifactPolicy to ArtifactPolicyJSON.
func (a *AgentDefinition) MarshalArtifactPolicy() error {
	if a.ArtifactPolicy == nil {
		a.ArtifactPolicyJSON = ""
		return nil
	}
	b, err := json.Marshal(a.ArtifactPolicy)
	if err != nil {
		return fmt.Errorf("marshalling artifact policy: %w", err)
	}
	a.ArtifactPolicyJSON = string(b)
	return nil
}

// UnmarshalArtifactPolicy deserializes ArtifactPolicyJSON into ArtifactPolicy.
// Empty JSON means nil — artifacts are off for this agent.
func (a *AgentDefinition) UnmarshalArtifactPolicy() error {
	if a.ArtifactPolicyJSON == "" || a.ArtifactPolicyJSON == "null" {
		a.ArtifactPolicy = nil
		return nil
	}
	var ap AgentArtifactPolicy
	if err := json.Unmarshal([]byte(a.ArtifactPolicyJSON), &ap); err != nil {
		return fmt.Errorf("unmarshalling artifact policy: %w", err)
	}
	a.ArtifactPolicy = &ap
	return nil
}

// MarshalCodeExecutionPolicy serializes CodeExecutionPolicy for storage.
func (a *AgentDefinition) MarshalCodeExecutionPolicy() error {
	if a.CodeExecutionPolicy == nil {
		a.CodeExecutionPolicyJSON = ""
		return nil
	}
	b, err := json.Marshal(a.CodeExecutionPolicy)
	if err != nil {
		return fmt.Errorf("marshalling code execution policy: %w", err)
	}
	a.CodeExecutionPolicyJSON = string(b)
	return nil
}

// UnmarshalCodeExecutionPolicy deserializes CodeExecutionPolicyJSON.
// Empty JSON means nil — code execution is off for this agent.
func (a *AgentDefinition) UnmarshalCodeExecutionPolicy() error {
	if a.CodeExecutionPolicyJSON == "" || a.CodeExecutionPolicyJSON == "null" {
		a.CodeExecutionPolicy = nil
		return nil
	}
	var cp AgentCodeExecutionPolicy
	if err := json.Unmarshal([]byte(a.CodeExecutionPolicyJSON), &cp); err != nil {
		return fmt.Errorf("unmarshalling code execution policy: %w", err)
	}
	a.CodeExecutionPolicy = &cp
	return nil
}

// MarshalStreamingPolicy serializes StreamingPolicy for storage.
func (a *AgentDefinition) MarshalStreamingPolicy() error {
	if a.StreamingPolicy == nil {
		a.StreamingPolicyJSON = ""
		return nil
	}
	b, err := json.Marshal(a.StreamingPolicy)
	if err != nil {
		return fmt.Errorf("marshalling streaming policy: %w", err)
	}
	a.StreamingPolicyJSON = string(b)
	return nil
}

// UnmarshalStreamingPolicy deserializes StreamingPolicyJSON.
// Empty JSON means nil — streaming is on by default (matches the adapter's
// historical behaviour before this field existed).
func (a *AgentDefinition) UnmarshalStreamingPolicy() error {
	if a.StreamingPolicyJSON == "" || a.StreamingPolicyJSON == "null" {
		a.StreamingPolicy = nil
		return nil
	}
	var sp AgentStreamingPolicy
	if err := json.Unmarshal([]byte(a.StreamingPolicyJSON), &sp); err != nil {
		return fmt.Errorf("unmarshalling streaming policy: %w", err)
	}
	a.StreamingPolicy = &sp
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// CoordinatorSubAgent — inline sub-agent definition for coordinator pattern
//
// When an AgentDefinition has non-nil SubAgents, it is a coordinator: an
// LlmAgent whose SubAgents are populated directly from these definitions.
// This enables peer-to-peer transfers without a workflow wrapper.
// ──────────────────────────────────────────────────────────────────────────────

// SubAgentRef references an existing agent to be used as a sub-agent.
type SubAgentRef struct {
	// AgentID is the UUID of the referenced AgentDefinition.
	AgentID string `json:"agent_id"`
	// OutputKey is an optional key to store the sub-agent's result in session state.
	OutputKey string `json:"output_key,omitempty"`
}

// MarshalSubAgents serializes SubAgents to SubAgentsJSON for storage.
func (a *AgentDefinition) MarshalSubAgents() error {
	if len(a.SubAgents) == 0 {
		a.SubAgentsJSON = ""
		return nil
	}
	b, err := json.Marshal(a.SubAgents)
	if err != nil {
		return fmt.Errorf("marshalling sub_agents: %w", err)
	}
	a.SubAgentsJSON = string(b)
	return nil
}

// UnmarshalSubAgents deserializes SubAgentsJSON into SubAgents.
func (a *AgentDefinition) UnmarshalSubAgents() error {
	if a.SubAgentsJSON == "" || a.SubAgentsJSON == "null" {
		a.SubAgents = nil
		return nil
	}
	var subs []*SubAgentRef
	if err := json.Unmarshal([]byte(a.SubAgentsJSON), &subs); err != nil {
		return fmt.Errorf("unmarshalling sub_agents: %w", err)
	}
	a.SubAgents = subs
	return nil
}

// IsCoordinator returns true if this agent has inline sub-agent definitions.
func (a *AgentDefinition) IsCoordinator() bool {
	return len(a.SubAgents) > 0
}

// ──────────────────────────────────────────────────────────────────────────────
// AgentToolRef — expose another agent as a callable tool
//
// Stored as JSONB in user_agents.agent_tools. Each ref names another agent
// by ID; the target is resolved at runtime and wrapped via ADK-Go's
// tool/agenttool package. The parent agent calls the child and receives
// the child's output as the tool result — control does not transfer.
// ──────────────────────────────────────────────────────────────────────────────

// AgentToolRef references an existing agent to be exposed as a callable tool.
type AgentToolRef struct {
	// AgentID is the UUID of the referenced AgentDefinition.
	AgentID string `json:"agent_id"`

	// SkipSummarization skips the parent's LLM summarisation of the child's
	// output — the parent hands the raw output back verbatim.
	SkipSummarization bool `json:"skip_summarization,omitempty"`

	// InputSchema is the JSON schema for the callable signature when this agent
	// is used as a tool. If empty, defaults to "request" string parameter.
	InputSchema string `json:"input_schema,omitempty"`
}

// MarshalAgentTools serializes AgentTools to AgentToolsJSON for storage.
func (a *AgentDefinition) MarshalAgentTools() error {
	if len(a.AgentTools) == 0 {
		a.AgentToolsJSON = ""
		return nil
	}
	b, err := json.Marshal(a.AgentTools)
	if err != nil {
		return fmt.Errorf("marshalling agent_tools: %w", err)
	}
	a.AgentToolsJSON = string(b)
	return nil
}

// UnmarshalAgentTools deserializes AgentToolsJSON into AgentTools.
func (a *AgentDefinition) UnmarshalAgentTools() error {
	if a.AgentToolsJSON == "" || a.AgentToolsJSON == "null" {
		a.AgentTools = nil
		return nil
	}
	var refs []*AgentToolRef
	if err := json.Unmarshal([]byte(a.AgentToolsJSON), &refs); err != nil {
		return fmt.Errorf("unmarshalling agent_tools: %w", err)
	}
	a.AgentTools = refs
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// AgentModelOverride — per-agent provider + model selection
//
// Stored as JSONB in user_agents.model_override. When set, overrides the user's
// global AI provider and model for this agent. The API key is resolved at
// runtime from the user's saved provider credentials (aisettings module).
// Mirrors foundation/agent.AgentModelOverride.
// ──────────────────────────────────────────────────────────────────────────────

// AgentModelOverride selects a specific LLM provider and model for one agent.
type AgentModelOverride struct {
	// Provider is the LLM provider (e.g. "gemini", "openai", "anthropic").
	Provider string `json:"provider"`

	// ModelName is the model identifier within that provider.
	ModelName string `json:"modelName"`
}

// Validate checks that Provider and ModelName are both non-empty.
func (o *AgentModelOverride) Validate() error {
	if o == nil {
		return nil
	}
	if o.Provider == "" {
		return fmt.Errorf("model override provider is required")
	}
	if o.ModelName == "" {
		return fmt.Errorf("model override modelName is required")
	}
	return nil
}

// MarshalModelOverride serializes ModelOverride to ModelOverrideJSON for storage.
func (a *AgentDefinition) MarshalModelOverride() error {
	if a.ModelOverride == nil {
		a.ModelOverrideJSON = ""
		return nil
	}
	b, err := json.Marshal(a.ModelOverride)
	if err != nil {
		return fmt.Errorf("marshalling model override: %w", err)
	}
	a.ModelOverrideJSON = string(b)
	return nil
}

// UnmarshalModelOverride deserializes ModelOverrideJSON into ModelOverride.
func (a *AgentDefinition) UnmarshalModelOverride() error {
	if a.ModelOverrideJSON == "" || a.ModelOverrideJSON == "null" {
		a.ModelOverride = nil
		return nil
	}
	var mo AgentModelOverride
	if err := json.Unmarshal([]byte(a.ModelOverrideJSON), &mo); err != nil {
		return fmt.Errorf("unmarshalling model override: %w", err)
	}
	a.ModelOverride = &mo
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// CompositionNode — recursive building block for agent composition
//
// Maps 1:1 to ADK-Go's agent types:
//   - "agent"      → llmagent.New (leaf — references another AgentDefinition)
//   - "sequential" → sequentialagent.New (runs children in order)
//   - "parallel"   → parallelagent.New (runs children concurrently)
//   - "loop"       → loopagent.New (repeats children until exit condition)
//   - "remote"     → custom agent.New wrapping an A2A connector (delegates via A2A protocol)
//
// Example tree:
//
//   sequential
//   ├─ agent(researcher)
//   ├─ parallel
//   │  ├─ agent(writer)
//   │  └─ agent(fact-checker)
//   └─ loop
//      ├─ agent(reviewer)
//      └─ agent(editor)
// ──────────────────────────────────────────────────────────────────────────────

// NodeType identifies what kind of node this is in the composition tree.
type NodeType string

const (
	NodeAgent       NodeType = "agent"
	NodeSequential  NodeType = "sequential"
	NodeParallel    NodeType = "parallel"
	NodeLoop        NodeType = "loop"
	NodeRemote      NodeType = "remote"
	NodeConditional NodeType = "conditional"
)

// ValidNodeTypes is the set of supported node types.
var ValidNodeTypes = map[NodeType]bool{
	NodeAgent:       true,
	NodeSequential:  true,
	NodeParallel:    true,
	NodeLoop:        true,
	NodeRemote:      true,
	NodeConditional: true,
}

// IsComposite returns true if the node type can contain children.
func (nt NodeType) IsComposite() bool {
	return nt == NodeSequential || nt == NodeParallel || nt == NodeLoop || nt == NodeConditional
}

// CompositionNode is the recursive building block for agent composition.
// Composite nodes hold children, leaf nodes reference agent definitions.
type CompositionNode struct {
	ID    string   `json:"id"`
	Type  NodeType `json:"type"`
	Label string   `json:"label,omitempty"`

	// ── Leaf-node fields (Type = "agent") ──
	AgentID string `json:"agent_id,omitempty"`

	// ── Remote-node fields (Type = "remote") ──
	// A2AConnectorID references a registered A2A connector for remote delegation.
	A2AConnectorID string `json:"a2a_connector_id,omitempty"`

	// ── Conditional routing fields ──
	// Condition specifies when this node should execute as a branch within
	// a "conditional" parent. The value is matched against the evaluator's
	// output (read from session state). Use "*" for the fallback/default branch.
	Condition string `json:"condition,omitempty"`

	// ── Composite-node fields ──
	Children      []CompositionNode `json:"children,omitempty"`
	MaxIterations int               `json:"max_iterations,omitempty"`
	// AllowEarlyExit controls whether the exit_loop tool is injected into the
	// last child of this loop node. When true the LLM may break out early;
	// when false (default) the loop always runs MaxIterations times.
	AllowEarlyExit bool `json:"allow_early_exit,omitempty"`

	// ── Visual layout (AlpineFlow canvas — ignored by ADK-Go runtime) ──
	// X and Y store the node's position on the flow canvas (pixels).
	// W and H store optional explicit width/height (0 = auto-size).
	// Collapsed hides children of composite nodes on the canvas.
	X         float64 `json:"x,omitempty"`
	Y         float64 `json:"y,omitempty"`
	W         float64 `json:"w,omitempty"`
	H         float64 `json:"h,omitempty"`
	Collapsed bool    `json:"collapsed,omitempty"`
}

// CollectAgentIDs returns all agent IDs referenced in the tree.
func (n *CompositionNode) CollectAgentIDs() []string {
	var ids []string
	n.Walk(func(node *CompositionNode) {
		if node.Type == NodeAgent && node.AgentID != "" {
			ids = append(ids, node.AgentID)
		}
	})
	return ids
}

// CountNodes returns the total number of nodes in the tree.
func (n *CompositionNode) CountNodes() int {
	count := 0
	n.Walk(func(_ *CompositionNode) { count++ })
	return count
}

// Depth returns the maximum depth of the tree (1 for a single leaf).
func (n *CompositionNode) Depth() int {
	if len(n.Children) == 0 {
		return 1
	}
	maxChildDepth := 0
	for i := range n.Children {
		d := n.Children[i].Depth()
		if d > maxChildDepth {
			maxChildDepth = d
		}
	}
	return 1 + maxChildDepth
}

// Walk visits every node in the tree depth-first.
func (n *CompositionNode) Walk(fn func(*CompositionNode)) {
	fn(n)
	for i := range n.Children {
		n.Children[i].Walk(fn)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Repository
// ──────────────────────────────────────────────────────────────────────────────

// AgentDefinitionRepository defines the storage contract for user-created agents.
type AgentDefinitionRepository interface {
	Create(ctx context.Context, agent *AgentDefinition) error
	Update(ctx context.Context, agent *AgentDefinition) error
	Delete(ctx context.Context, id, userID string) error
	GetByID(ctx context.Context, id, userID string) (*AgentDefinition, error)
	ListByUser(ctx context.Context, userID string) ([]AgentDefinition, error)
	Search(ctx context.Context, userID, query string) ([]AgentDefinition, error)
	ListAddedCatalogIDs(ctx context.Context, userID string) ([]string, error)
	ExistsByCatalogID(ctx context.Context, userID, catalogID string) (bool, error)

	// GetByShareToken retrieves a shared agent by its share token (no user scoping).
	GetByShareToken(ctx context.Context, token string) (*AgentDefinition, error)

	// UpdateShareToken sets or clears the share token for an agent.
	UpdateShareToken(ctx context.Context, id, userID, token, visibility string) error

	// SetParent sets the parent_id on a sub-agent to establish workflow ownership.
	SetParent(ctx context.Context, agentID, parentID, userID string) error

	// DeleteByParent deletes all agents owned by a workflow (parent_id = parentID).
	DeleteByParent(ctx context.Context, parentID, userID string) error
}
