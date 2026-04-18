package agent

// TransferPolicy controls how an LLM agent delegates to other agents.
// Maps directly to ADK-Go's transfer_to_agent tool behaviour.
type TransferPolicy struct {
	// DisallowTransferToParent prevents the agent from transferring back
	// to its parent agent. Default false (transfer allowed).
	DisallowTransferToParent bool

	// DisallowTransferToPeers prevents the agent from transferring to
	// sibling agents (peers under the same parent). Default false.
	DisallowTransferToPeers bool
}

// IncludeContents controls whether conversation history is forwarded
// to sub-agents when they run within a workflow.
type IncludeContents string

const (
	// IncludeContentsDefault forwards full conversation history (default).
	IncludeContentsDefault IncludeContents = "default"

	// IncludeContentsNone starts the sub-agent with a blank context.
	IncludeContentsNone IncludeContents = "none"
)

// AgentModelConfig holds per-agent overrides for the LLM generation parameters.
// All fields are optional pointers — nil means "fall back to the user's global
// model settings". This is intentionally framework-agnostic: the adapter layer
// converts these values to the provider-specific types (e.g. genai.GenerateContentConfig).
//
// Precedence when building the final generation config (highest wins):
//  1. Per-request RunConfig overrides (e.g. from the frontend send button)
//  2. AgentModelConfig (saved per-agent preferences, this struct)
//  3. User global ModelConfig (from aisettings)
//  4. Provider defaults
type AgentModelConfig struct {
	// Temperature controls randomness (0.0–2.0). Lower = more deterministic.
	Temperature *float64

	// TopP sets nucleus sampling probability (0.0–1.0).
	TopP *float64

	// TopK limits the token vocabulary at each step (genai uses float32 for this field).
	TopK *float32

	// MaxOutputTokens caps the response length.
	MaxOutputTokens *int32

	// StopSequences halts generation when any of these strings are encountered.
	StopSequences []string
}

// AgentArtifactPolicy controls whether an agent can list, load, and create
// artifacts, and whether user-uploaded blobs are auto-saved.
//
// Shape matches ADK's artifact tooling (see .READ/adk-go/artifact/service.go
// for the Service interface; .READ/adk-go/tool/loadartifactstool for the
// tool; and the RunConfig.SaveInputBlobsAsArtifacts field on
// .READ/adk-go/runner/runner.go:206):
//
//   - Enabled=false                    → neither load_artifacts tool nor
//     the runner's artifact service is
//     made visible to this agent.
//   - Enabled=true, AutoSaveUserBlobs=false
//                                       → load_artifacts is injected so the
//     agent can list and read files, but
//     uploaded inline blobs stay in the
//     user message unchanged.
//   - Enabled=true, AutoSaveUserBlobs=true
//                                       → as above, plus the runner
//     auto-persists inline blobs from
//     the user message into the artifact
//     service and replaces them with a
//     text reference, so the agent sees
//     "Uploaded file: {name}…" instead
//     of raw bytes. Referenceable in
//     prompts as {artifact.name}.
//
// Nil means the agent has no artifact capability. This is the design
// contract — there is no legacy fallback.
type AgentArtifactPolicy struct {
	// Enabled turns on artifact support for this agent. When true,
	// load_artifacts is injected and the runner's artifact service is
	// wired for the run.
	Enabled bool `json:"enabled"`

	// AutoSaveUserBlobs, when true, asks the runner to move any inline
	// blobs in the user message into the artifact store automatically,
	// replacing them with text placeholders the LLM can reference.
	// Requires Enabled=true; ignored otherwise.
	AutoSaveUserBlobs bool `json:"auto_save_user_blobs"`
}

// AgentMemoryPolicy controls whether an agent has access to the orchestration
// engine's cross-session memory, and how it uses it.
//
// Shape matches the two tools ADK ships for memory
// (.READ/adk-go/tool/preloadmemorytool, .READ/adk-go/tool/loadmemorytool):
//
//   - Enabled=false              → neither tool is injected; the agent has
//     no memory capability.
//   - Enabled=true, Preload=false → load_memory is injected. The model may
//     call it on demand with a custom query.
//   - Enabled=true, Preload=true  → both load_memory and preload_memory are
//     injected. preload_memory runs before every
//     turn and injects relevant memories into the
//     system instruction without the model doing
//     anything; load_memory stays available for
//     explicit lookups.
//
// The memory Service itself is wired once at the runner (a deployment
// concern); this policy is a per-agent choice about which tools appear on
// the agent. Nil means "use the legacy toolset-permission gate" so existing
// agents configured through the old plugin-style toggle keep working.
type AgentMemoryPolicy struct {
	// Enabled turns on cross-session memory for this agent. When true,
	// load_memory is injected.
	Enabled bool `json:"enabled"`

	// Preload, when true, also injects preload_memory so relevant memories
	// are searched and added to the system instruction at the start of
	// every turn. Requires Enabled=true; ignored otherwise.
	Preload bool `json:"preload"`
}

// Config defines the configuration for an agent.
//
// Config is the central description of an agent's identity, behaviour, and
// capabilities. It is framework-agnostic — adapters translate it to their
// native configuration format.
//
// Layering:
//   - Identity:     Name, Description
//   - Behaviour:    SystemPrompt, InstructionFunc, Requirements
//   - Capabilities: Tools, SubAgents, AgentTools
//   - Transfer:     TransferPolicy (parent/peer delegation control)
//   - I/O Schema:   InputSchema, OutputSchema
//   - Execution:    MaxIterations, OutputKey, IncludeContents
//   - Global:       GlobalInstruction (shared across agent tree)
//
// Two ways to combine agents — pick the one that matches the intent:
//   - SubAgents  (transfer):    control shifts to the child; the child
//     drives the conversation and produces the
//     final user-visible response. Use for
//     coordinator/specialist patterns where the
//     parent hands off ("I don't know — ask
//     Agent B to take over").
//   - AgentTools (function call): the parent stays in charge; it calls
//     the child like a function, receives its
//     output, and keeps reasoning. Use when the
//     parent needs a capability but must remain
//     responsible for the final answer
//     ("summarise this via Agent B, then
//     combine it with my other findings").
type Config struct {
	// Name is the unique identifier for the agent.
	// Must be unique within the agent tree. Cannot be "user" (reserved).
	Name string

	// Description is a human-readable description of the agent.
	// Used by LLMs to determine whether to delegate to this agent.
	Description string

	// SystemPrompt is the instructions for how the agent should behave.
	SystemPrompt string

	// InstructionFunc provides instructions that depend on runtime state.
	// Called before each agent run and appended to SystemPrompt.
	InstructionFunc func(inv *Invocation) (string, error)

	// GlobalInstruction is shared across the entire agent tree.
	// Only effective on the root agent — propagated to all descendants
	// by the ADK-Go runtime. Use for organisation-wide policies.
	GlobalInstruction string

	// Tools are the available capabilities for this agent.
	Tools []Tool

	// SubAgents are child agents that this agent can delegate tasks to
	// via ADK-Go's transfer_to_agent mechanism. When the parent transfers,
	// control shifts to the sub-agent for the rest of the turn and the
	// sub-agent's output becomes the user-visible response.
	//
	// For function-call-style delegation where the parent stays in charge
	// and receives the child's output as a tool result, use AgentTools
	// instead. The two are orthogonal and may be combined on one agent.
	SubAgents []Agent

	// Requirements are declarative constraints evaluated before each LLM turn.
	// Requirements that implement ToolProvider automatically contribute tools.
	// Requirements that implement PromptProvider automatically contribute to the system prompt.
	//
	// Example:
	//
	//    Requirements: []agent.Requirement{
	//        agent.NewConditionalRequirement("think", agent.ForceAtStep(1)),
	//    },
	Requirements []Requirement

	// MaxIterations limits the number of turns in a single run.
	// Default is 10. Set to 0 for unlimited.
	MaxIterations int

	// ── Multi-Agent Orchestration ──

	// TransferPolicy controls delegation to parent and peer agents.
	// Maps to ADK-Go's DisallowTransferToParent / DisallowTransferToPeers.
	TransferPolicy TransferPolicy

	// IncludeContents controls whether conversation history is forwarded
	// to this agent when it runs as a sub-agent in a workflow.
	// Default is "default" (include full history).
	IncludeContents IncludeContents

	// OutputKey optionally stores this agent's final text output in
	// session state under this key, making it available to subsequent
	// agents via {output_key} template syntax in their instructions.
	OutputKey string

	// ── Schema ──

	// InputSchema defines the expected input when this agent is invoked
	// as a tool (via AgentTool). Informs the caller about required args.
	InputSchema *Schema

	// OutputSchema forces the agent to produce structured JSON output
	// conforming to this schema.
	OutputSchema *Schema

	// ModelConfig holds per-agent LLM generation parameter overrides.
	// nil means use the user's global model settings.
	ModelConfig *AgentModelConfig

	// ModelOverride selects a specific provider and model for this agent,
	// replacing the user's global default. nil means use the global default.
	ModelOverride *AgentModelOverride

	// MemoryPolicy controls whether this agent can recall past sessions and
	// whether recall runs automatically. nil means memory is off.
	MemoryPolicy *AgentMemoryPolicy

	// ArtifactPolicy controls whether this agent can create, load, and list
	// artifacts, and whether user-uploaded blobs are auto-saved into the
	// artifact store. nil means artifacts are off.
	ArtifactPolicy *AgentArtifactPolicy

	// AgentTools exposes other user-owned agents as callable tools on this
	// agent. Each ref is resolved at runtime and wrapped via ADK-Go's
	// tool/agenttool package, so the child appears to the parent LLM as a
	// regular function: the parent calls it, reads the output, and keeps
	// reasoning.
	//
	// Orthogonal to SubAgents (control-transfer delegation) and to the
	// workflow Graph (fixed sequential/parallel/loop orchestration) — any
	// leaf agent can have AgentTools regardless of its role elsewhere.
	AgentTools []AgentToolRef
}

// AgentToolRef exposes another agent as a callable tool for this agent.
// Maps to ADK-Go's tool/agenttool package (see
// vendor/google.golang.org/adk/tool/agenttool).
//
// AgentTool vs SubAgent — pick the right shape up front:
//
//	                    AgentTool (this type)          SubAgent
//	Control flow        parent calls, child returns    parent transfers to child
//	Who answers user    parent (after using output)    child (directly)
//	Child isolation     fresh in-memory session        shares thread's session
//	Typical use         "run Agent B on this snippet"  "hand off to specialist B"
//	Summarization       parent summarises by default   no summarisation (child
//	                    (toggle via SkipSummarization) owns the response)
//
// Both lists may coexist on the same agent: a coordinator can transfer to
// peers and also call utility agents as tools. The adapter flattens the
// child: when wrapped as an AgentTool, the child's own AgentTools and
// SubAgents are NOT propagated — you get a plain callable, which keeps
// semantics simple and prevents accidental cycles.
//
// Only the AgentID is persisted; the target agent's definition is resolved
// at runtime so edits to the target propagate automatically.
type AgentToolRef struct {
	// AgentID is the ID of the target agent definition.
	AgentID string

	// SkipSummarization, when true, skips the parent LLM's summarisation
	// step after the child returns. Use it when the parent should hand
	// the child's output back verbatim (e.g. the child produced the
	// final structured response).
	SkipSummarization bool
}

// AgentModelOverride selects a specific LLM provider and model for one agent,
// overriding the user's global AI settings. Only the provider and model name
// are stored here; the API key is resolved at runtime from the user's saved
// provider credentials (modules/aisettings).
type AgentModelOverride struct {
	// Provider is the LLM provider (e.g. "gemini", "openai", "anthropic").
	Provider string

	// ModelName is the model identifier within that provider
	// (e.g. "gemini-2.0-flash", "gpt-4o", "claude-opus-4-5-20251001").
	ModelName string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(name string) Config {
	return Config{
		Name:          name,
		MaxIterations: 10,
	}
}

// CollectRequirementTools returns all tools provided by Requirements
// that implement ToolProvider.
func (cfg *Config) CollectRequirementTools() []ExecutableTool {
	var tools []ExecutableTool
	for _, req := range cfg.Requirements {
		if tp, ok := req.(ToolProvider); ok {
			tools = append(tools, tp.ProvidedTools()...)
		}
	}
	return tools
}

// CollectRequirementPrompts returns concatenated prompt additions from
// Requirements that implement PromptProvider.
func (cfg *Config) CollectRequirementPrompts() string {
	var additions string
	for _, req := range cfg.Requirements {
		if pp, ok := req.(PromptProvider); ok {
			additions += "\n\n" + pp.PromptAddition()
		}
	}
	return additions
}
