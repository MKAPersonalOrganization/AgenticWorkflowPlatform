package adkgo

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	adkmodel "google.golang.org/adk/model"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/genai"

	fagent "school-with-ai/foundation/agent"
)

// foundationSchemaToGenAI converts a foundation agent.Schema to a genai.Schema.
// Used to populate llmagent.Config.InputSchema and OutputSchema.
func foundationSchemaToGenAI(s *fagent.Schema) *genai.Schema {
	if s == nil {
		return nil
	}
	gs := &genai.Schema{
		Type:        genai.Type(string(s.Type)),
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
// Precedence for overrideable fields (highest wins):
//  1. Workflow node override (WorkflowNodeInfo fields)
//  2. Agent config default (Config fields)
//  3. ADK-Go default (zero value)
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
	beforeAgentCallbacks  []agent.BeforeAgentCallback
	afterAgentCallbacks   []agent.AfterAgentCallback
	beforeModelCallbacks  []llmagent.BeforeModelCallback
	afterModelCallbacks   []llmagent.AfterModelCallback
	beforeToolCallbacks   []llmagent.BeforeToolCallback
	afterToolCallbacks    []llmagent.AfterToolCallback
	onToolErrorCallbacks  []llmagent.OnToolErrorCallback

	// Optional workflow node overrides (nil for regular agents)
	nodeOverride *fagent.WorkflowNodeInfo
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

	// Apply workflow node overrides (higher precedence than config defaults).
	if n := p.nodeOverride; n != nil {
		if n.DisallowTransferToParent != nil {
			cfg.DisallowTransferToParent = *n.DisallowTransferToParent
		}
		if n.DisallowTransferToPeers != nil {
			cfg.DisallowTransferToPeers = *n.DisallowTransferToPeers
		}
		if n.IncludeContents == "none" {
			cfg.IncludeContents = llmagent.IncludeContentsNone
		} else if n.IncludeContents == "default" {
			cfg.IncludeContents = llmagent.IncludeContentsDefault
		}
		if n.OutputKey != "" {
			cfg.OutputKey = n.OutputKey
		}
	}

	return llmagent.New(cfg)
}
