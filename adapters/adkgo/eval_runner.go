package adkgo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	adkmodel "google.golang.org/adk/model"
	adkrunner "google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	fagent "aicompass/foundation/agent"
	"aicompass/modules/evaluation"
)

// EvalRunner implements evaluation.AgentInvoker using the ADK-Go runtime.
// It treats the agent as a complete black-box function: gives it an input
// and captures the output. The agent's internal structure (simple LLM vs workflow)
// doesn't matter - the system prompt fully specifies its behavior.
type EvalRunner struct {
	agentProvider  AgentProvider
	configResolver ModelConfigResolver
	apiKeyResolver APIKeyResolver
}

// NewEvalRunner creates a new EvalRunner for evaluation invocations.
func NewEvalRunner(
	agentProvider AgentProvider,
	configResolver ModelConfigResolver,
	apiKeyResolver APIKeyResolver,
) *EvalRunner {
	return &EvalRunner{
		agentProvider:  agentProvider,
		configResolver: configResolver,
		apiKeyResolver: apiKeyResolver,
	}
}

// InvokeForEval runs the agent as a black-box function: input → process → output.
// The agent is a function. Its system prompt completely specifies its behavior.
// Whether it's a simple LLM or a workflow, the system prompt contains all logic.
// We invoke it with that prompt and capture what it produces.
func (r *EvalRunner) InvokeForEval(ctx context.Context, req evaluation.EvalInvokeRequest) (*evaluation.EvalInvokeResult, error) {
	foundAgent, err := r.agentProvider.GetForUser(ctx, req.AgentID, req.UserID)
	if err != nil {
		return nil, err
	}
	agentCfg := extractAgentConfig(foundAgent)

	modelCfg, err := r.configResolver.GetModelConfigForUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	provider, modelName, apiKey := r.resolveEvalModelConfig(ctx, req.UserID, modelCfg, agentCfg)

	llm, err := NewModel(ctx, provider, modelName, apiKey)
	if err != nil {
		return nil, err
	}
	adkAgent, err := buildEvalAgent(foundAgent, agentCfg, llm, modelCfg)
	if err != nil {
		return nil, err
	}
	adkRunnerInstance, err := buildEvalRunner(adkAgent)
	if err != nil {
		return nil, err
	}

	evalCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	result := &evaluation.EvalInvokeResult{
		JudgeAPIKey: apiKey,
		JudgeModel:  "gemini-2.5-flash",
	}
	evalSessionID := "eval-" + uuid.New().String()
	userMsg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: req.Input}}}
	for evt, runErr := range adkRunnerInstance.Run(evalCtx, req.UserID, evalSessionID, userMsg, agent.RunConfig{}) {
		if runErr != nil {
			break
		}
		collectEvalEvent(result, evt)
	}
	return result, nil
}

// resolveEvalModelConfig picks (provider, model, apiKey) for the
// evaluation run, applying the per-agent ModelOverride when set. The
// override's provider may require a different API key — fall back to
// the user's default key when no override-specific key is registered.
func (r *EvalRunner) resolveEvalModelConfig(
	ctx context.Context, userID string, modelCfg *fagent.ModelConfig, agentCfg *fagent.Config,
) (string, string, string) {
	provider := modelCfg.Provider
	modelName := modelCfg.Model
	apiKey := modelCfg.APIKey
	if agentCfg.ModelOverride == nil {
		return provider, modelName, apiKey
	}
	provider = agentCfg.ModelOverride.Provider
	modelName = agentCfg.ModelOverride.ModelName
	if r.apiKeyResolver == nil {
		return provider, modelName, apiKey
	}
	altKey, _ := r.apiKeyResolver.GetAPIKeyForProvider(ctx, userID, provider)
	if altKey != "" {
		apiKey = altKey
	}
	return provider, modelName, apiKey
}

// buildEvalAgent builds an ADK LLM agent for the evaluation harness. The
// system prompt IS the complete agent specification: for simple agents
// it's "You are X, do Y"; for workflows, "Coordinate agents A, B, C to
// accomplish X". No tools are wired — evaluation treats the agent as a
// pure function.
func buildEvalAgent(foundAgent fagent.Agent, agentCfg *fagent.Config, llm adkmodel.LLM, modelCfg *fagent.ModelConfig) (agent.Agent, error) {
	genCfg := buildGenerateContentConfig(modelCfg, agentCfg.ModelConfig, nil)
	return buildLLMAgent(llmAgentParams{
		name:       "eval",
		foundAgent: foundAgent,
		agentCfg:   agentCfg,
		llm:        llm,
		genCfg:     genCfg,
		tools:      nil,
		prompt:     agentCfg.SystemPrompt,
	})
}

// buildEvalRunner wires the ADK runner with an in-memory session
// service. Sessions are ephemeral — evaluation runs never persist
// conversation state.
func buildEvalRunner(adkAgent agent.Agent) (*adkrunner.Runner, error) {
	return adkrunner.New(adkrunner.Config{
		AppName:           "eval",
		Agent:             adkAgent,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
}

// collectEvalEvent folds one ADK session event into the evaluation
// result: tool-call parts append to ToolCalls; the final-response
// event's text parts append to FinalResponse. Partial streaming
// events and content-less events are skipped.
func collectEvalEvent(result *evaluation.EvalInvokeResult, evt *session.Event) {
	if evt == nil || evt.Content == nil || evt.Partial {
		return
	}
	for _, part := range evt.Content.Parts {
		if part.FunctionCall != nil {
			args := part.FunctionCall.Args
			if args == nil {
				args = map[string]any{}
			}
			result.ToolCalls = append(result.ToolCalls, evaluation.EvalToolCall{
				Name: part.FunctionCall.Name,
				Args: args,
			})
		}
	}
	if !evt.IsFinalResponse() {
		return
	}
	for _, part := range evt.Content.Parts {
		if part.Text != "" {
			result.FinalResponse += part.Text
		}
	}
}
