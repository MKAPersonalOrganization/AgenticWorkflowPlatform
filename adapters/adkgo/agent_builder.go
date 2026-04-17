package adkgo

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	adkmodel "google.golang.org/adk/model"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/genai"

	fagent "aicompass/foundation/agent"
)

// foundationTypeToGenAI maps our JSON-Schema-style lowercase type tokens to the
// OpenAPI-style uppercase tokens genai.Type uses (see vendor/google.golang.org/genai/types.go).
// An empty/unknown value becomes TypeUnspecified so it is surfaced as a schema
// validation error by genai rather than silently accepted as a literal.
func foundationTypeToGenAI(t fagent.SchemaType) genai.Type {
	switch t {
	case fagent.TypeString:
		return genai.TypeString
	case fagent.TypeNumber:
		return genai.TypeNumber
	case fagent.TypeInteger:
		return genai.TypeInteger
	case fagent.TypeBoolean:
		return genai.TypeBoolean
	case fagent.TypeArray:
		return genai.TypeArray
	case fagent.TypeObject:
		return genai.TypeObject
	default:
		return genai.TypeUnspecified
	}
}

// foundationSchemaToGenAI converts a foundation agent.Schema to a genai.Schema.
// Used to populate llmagent.Config.InputSchema and OutputSchema.
//
// Note on case: our foundation.SchemaType uses JSON-Schema lowercase tokens
// (`"object"`, `"string"`, …) while genai.Type uses OpenAPI uppercase tokens
// (`"OBJECT"`, `"STRING"`, …). A plain string cast silently produced invalid
// genai schemas that the model would reject or ignore — causing agents with an
// OutputSchema set to not follow it. Always route through foundationTypeToGenAI.
func foundationSchemaToGenAI(s *fagent.Schema) *genai.Schema {
	if s == nil {
		return nil
	}
	gs := &genai.Schema{
		Type:        foundationTypeToGenAI(s.Type),
		Description: s.Description,
	}
	if s.Nullable {
		t := true
		gs.Nullable = &t
	}
	if len(s.Properties) > 0 {
		gs.Properties = make(map[string]*genai.Schema, len(s.Properties))
		for k, v := range s.Properties {
			gs.Properties[k] = foundationSchemaToGenAI(v)
		}
	}
	if len(s.Required) > 0 {
		gs.Required = s.Required
	}
	if s.Items != nil {
		gs.Items = foundationSchemaToGenAI(s.Items)
	}
	if len(s.Enum) > 0 {
		gs.Enum = s.Enum
	}
	return gs
}

// ──────────────────────────────────────────────────────────────────────────────
// LLM Agent Builder
//
// buildLLMAgent centralises the construction of an ADK-Go llmagent from the
// resolved dependencies. Both the regular single-agent path (runner.go) and
// the workflow leaf-agent path (workflow.go) use this helper, eliminating the
// duplication that previously existed between Run() and buildLeafAgent().
//
// Transfer policies, output keys, and schemas are read from the agent's own
// config (stored on AgentDefinition). Composition nodes only carry structural
// information (type, children, position) — not agent-level configuration.
//
// ──────────────────────────────────────────────────────────────────────────────

// llmAgentParams carries all resolved dependencies needed to build an LLM agent.
type llmAgentParams struct {
	// Core identity
	name       string
	foundAgent fagent.Agent
	agentCfg   *fagent.Config // may be nil for agents without Configurable

	// Execution
	llm    adkmodel.LLM
	genCfg *genai.GenerateContentConfig
	tools  []adktool.Tool
	prompt string

	// Callbacks (from middleware bridge and requirements)
	beforeAgentCallbacks []agent.BeforeAgentCallback
	afterAgentCallbacks  []agent.AfterAgentCallback
	beforeModelCallbacks []llmagent.BeforeModelCallback
	afterModelCallbacks  []llmagent.AfterModelCallback
	beforeToolCallbacks  []llmagent.BeforeToolCallback
	afterToolCallbacks   []llmagent.AfterToolCallback
	onToolErrorCallbacks []llmagent.OnToolErrorCallback

	// SubAgents for coordinator pattern (LLM agent with children for peer transfers).
	subAgents []agent.Agent
}

// buildLLMAgent creates an ADK-Go llmagent from the resolved parameters.
// It applies agent config defaults, then overrides with workflow node values
// where provided.
func buildLLMAgent(p llmAgentParams) (agent.Agent, error) {
	cfg := llmagent.Config{
		Name:                  p.name,
		Description:           p.foundAgent.Description(),
		Model:                 p.llm,
		Instruction:           p.prompt,
		Tools:                 p.tools,
		GenerateContentConfig: p.genCfg,
		BeforeAgentCallbacks:  p.beforeAgentCallbacks,
		AfterAgentCallbacks:   p.afterAgentCallbacks,
		BeforeModelCallbacks:  p.beforeModelCallbacks,
		AfterModelCallbacks:   p.afterModelCallbacks,
		BeforeToolCallbacks:   p.beforeToolCallbacks,
		AfterToolCallbacks:    p.afterToolCallbacks,
		OnToolErrorCallbacks:  p.onToolErrorCallbacks,
		SubAgents:             p.subAgents,
	}

	// Apply agent config defaults.
	if p.agentCfg != nil {
		cfg.DisallowTransferToParent = p.agentCfg.TransferPolicy.DisallowTransferToParent
		cfg.DisallowTransferToPeers = p.agentCfg.TransferPolicy.DisallowTransferToPeers
		cfg.OutputKey = p.agentCfg.OutputKey
		cfg.GlobalInstruction = p.agentCfg.GlobalInstruction
		cfg.InputSchema = foundationSchemaToGenAI(p.agentCfg.InputSchema)
		cfg.OutputSchema = foundationSchemaToGenAI(p.agentCfg.OutputSchema)

		if p.agentCfg.IncludeContents == fagent.IncludeContentsNone {
			cfg.IncludeContents = llmagent.IncludeContentsNone
		}
	}

	return llmagent.New(cfg)
}
