package adkgo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	adkrunner "google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

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
	// 1. Load agent (type irrelevant - it's a black box)
	foundAgent, err := r.agentProvider.GetForUser(ctx, req.AgentID, req.UserID)
	if err != nil {
		return nil, err
	}

	agentCfg := extractAgentConfig(foundAgent)

	// 2. Get model config
	modelCfg, err := r.configResolver.GetModelConfigForUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	// 3. Resolve provider/model/apiKey; apply per-agent override
	provider := modelCfg.Provider
	modelName := modelCfg.Model
	apiKey := modelCfg.APIKey

	if agentCfg.ModelOverride != nil {
		provider = agentCfg.ModelOverride.Provider
		modelName = agentCfg.ModelOverride.ModelName
		if r.apiKeyResolver != nil {
			altKey, _ := r.apiKeyResolver.GetAPIKeyForProvider(ctx, req.UserID, provider)
			if altKey != "" {
				apiKey = altKey
			}
		}
	}

	// 4. Build LLM
	llm, err := NewModel(ctx, provider, modelName, apiKey)
	if err != nil {
		return nil, err
	}

	// 5. Build agent as LLM with complete system prompt
	// The system prompt IS the complete agent specification:
	// - For simple agents: "You are X, do Y with inputs Z"
	// - For workflows: "Coordinate agents A, B, C to accomplish X"
	// When invoked, the LLM will follow these instructions.
	genCfg := buildGenerateContentConfig(modelCfg, agentCfg.ModelConfig, nil)
	adkAgent, err := buildLLMAgent(llmAgentParams{
		name:       "eval",
		foundAgent: foundAgent,
		agentCfg:   agentCfg,
		llm:        llm,
		genCfg:     genCfg,
		tools:      nil,
		prompt:     agentCfg.SystemPrompt,
	})
	if err != nil {
		return nil, err
	}

	// 6. Create in-memory session (ephemeral, no persistence)
	sessionSvc := session.InMemoryService()
	evalSessionID := "eval-" + uuid.New().String()

	// 7. Build ADK runner
	adkRunnerCfg := adkrunner.Config{
		AppName:           "eval",
		Agent:             adkAgent,
		SessionService:    sessionSvc,
		AutoCreateSession: true,
	}
	adkRunnerInstance, err := adkrunner.New(adkRunnerCfg)
	if err != nil {
		return nil, err
	}

	// 8. Run with 2-minute timeout
	evalCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	userMsg := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: req.Input}},
	}

	result := &evaluation.EvalInvokeResult{
		JudgeAPIKey: apiKey,
		JudgeModel:  "gemini-2.5-flash",
	}

	// 9. Execute and capture output
	for evt, err := range adkRunnerInstance.Run(evalCtx, req.UserID, evalSessionID, userMsg, agent.RunConfig{}) {
		if err != nil {
			break
		}
		if evt == nil || evt.Content == nil || evt.Partial {
			continue
		}

		// Collect tool calls
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

		// Collect final response
		if evt.IsFinalResponse() {
			for _, part := range evt.Content.Parts {
				if part.Text != "" {
					result.FinalResponse += part.Text
				}
			}
		}
	}

	return result, nil
}
