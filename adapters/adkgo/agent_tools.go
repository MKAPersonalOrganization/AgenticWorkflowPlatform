package adkgo

import (
	"context"
	"fmt"

	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/agenttool"

	fagent "aicompass/foundation/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// Agent-as-Tool wrapping
//
// resolveAgentTools turns each fagent.AgentToolRef on the parent's Config
// into an ADK-Go adktool.Tool by loading the referenced agent definition,
// building a minimal llmagent for it, and wrapping that agent via
// google.golang.org/adk/tool/agenttool.
//
// Why the child is built as a plain leaf:
//   - The child runs inside agenttool's isolated runner (its own session,
//     memory, and artifact services — see adk-go tool/agenttool.Run).
//   - The child's own AgentTools are intentionally NOT resolved here —
//     flattening the graph keeps semantics clear ("agent B is a callable")
//     and prevents accidental cycles between peer agent-tool references.
//   - Composed (workflow) agents are skipped because a workflow agent's
//     root runs via the workflow builder, which expects thread-backed
//     session state rather than agenttool's in-memory session.
//
// resolveAgentTools returns a (possibly empty) slice and a soft error
// policy: malformed or missing refs are skipped with a log, never fatal.
// This matches our wider adapter pattern where tool resolution never
// aborts an agent run for a single misconfigured attachment.
// ──────────────────────────────────────────────────────────────────────────────

func (r *RunnerAdapter) resolveAgentTools(
	ctx context.Context,
	userID string,
	modelCfg *fagent.ModelConfig,
	threadID string,
	parentCfg *fagent.Config,
	req *fagent.RunRequest,
) []adktool.Tool {
	if parentCfg == nil || len(parentCfg.AgentTools) == 0 {
		return nil
	}

	out := make([]adktool.Tool, 0, len(parentCfg.AgentTools))
	for _, ref := range parentCfg.AgentTools {
		if ref.AgentID == "" {
			continue
		}
		tool, err := r.buildAgentToolFromRef(ctx, userID, modelCfg, threadID, req, ref)
		if err != nil {
			if r.log != nil {
				r.log.Warn(ctx, "skipping agent-tool ref",
					"parent", parentCfg.Name,
					"agentID", ref.AgentID,
					"error", err,
				)
			}
			continue
		}
		out = append(out, tool)
	}
	return out
}

// buildAgentToolFromRef resolves a single AgentToolRef into a wrapped tool.
// Composed (workflow) agents are rejected — they must be run via the workflow
// builder, not the agenttool in-memory runner.
func (r *RunnerAdapter) buildAgentToolFromRef(
	ctx context.Context,
	userID string,
	modelCfg *fagent.ModelConfig,
	threadID string,
	req *fagent.RunRequest,
	ref fagent.AgentToolRef,
) (adktool.Tool, error) {
	foundAgent, err := r.agentProvider.GetForUser(ctx, ref.AgentID, userID)
	if err != nil {
		return nil, fmt.Errorf("resolving agent: %w", err)
	}

	if _, isWF := isWorkflowAgent(foundAgent); isWF {
		return nil, fmt.Errorf("workflow agents cannot be used as agent-tools; reference a leaf LLM agent instead")
	}

	childCfg := extractAgentConfig(foundAgent)

	// Apply child-level model override so the wrapped agent uses its own
	// provider/model even when called as a tool from the parent.
	childModelCfg := modelCfg
	if childCfg != nil && childCfg.ModelOverride != nil {
		overridden, ovErr := r.applyModelOverride(ctx, userID, modelCfg, childCfg.ModelOverride)
		if ovErr != nil {
			return nil, fmt.Errorf("applying model override: %w", ovErr)
		}
		childModelCfg = overridden
	}

	childLLM, err := NewModel(ctx, childModelCfg.Provider, childModelCfg.Model, childModelCfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("creating child LLM: %w", err)
	}

	// Build invocation-scoped inputs for the child. It sees the same user
	// message — agenttool will replace the user content with the LLM's
	// tool-call arguments at Run time, so this is just a placeholder.
	childInv := &fagent.Invocation{
		ID:        req.InvocationID,
		SessionID: req.SessionID,
		UserID:    userID,
		Message:   req.Message,
		Config:    req.Config,
		Context:   req.Context,
	}
	childRules := evaluateRequirements(ctx, childCfg, childInv)

	childTools := r.resolveTools(ctx, userID, ref.AgentID, childModelCfg, threadID, childCfg, req, childRules)

	childPrompt := r.buildSystemPrompt(ctx, foundAgent, userID, ref.AgentID, threadID, childCfg, childInv)

	var childAgentModelCfg *fagent.AgentModelConfig
	if childCfg != nil {
		childAgentModelCfg = childCfg.ModelConfig
	}
	childGenCfg := buildGenerateContentConfig(childModelCfg, childAgentModelCfg, req.Config)

	// Defensive copy of the child config with AgentTools cleared — the
	// child is flattened to a plain leaf so it cannot recursively expose
	// its own AgentTools (flattening matches the semantics we document to
	// users: an agent-tool is a simple callable).
	var childCfgCopy *fagent.Config
	if childCfg != nil {
		c := *childCfg
		c.AgentTools = nil
		c.SubAgents = nil
		childCfgCopy = &c
	}

	// Build without any before/after callbacks — the child's execution
	// runs inside agenttool's isolated runner where our parent-level
	// logging / confirmation pipeline does not apply. Tool errors inside
	// the child still bubble up as an error from agenttool.Run.
	childAgent, err := buildLLMAgent(llmAgentParams{
		name:       foundAgent.Name(),
		foundAgent: foundAgent,
		agentCfg:   childCfgCopy,
		llm:        childLLM,
		genCfg:     childGenCfg,
		tools:      childTools,
		prompt:     childPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("building child llmagent: %w", err)
	}

	return agenttool.New(childAgent, &agenttool.Config{
		SkipSummarization: ref.SkipSummarization,
	}), nil
}
