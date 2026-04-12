package adkgo

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"google.golang.org/genai"

	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"

	fagent "aicompass/foundation/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// Factory — picks the right model.LLM for a provider string.
// ──────────────────────────────────────────────────────────────────────────────

// NewModel creates an adk-go model.LLM for the given provider.
func NewModel(ctx context.Context, provider, modelName, apiKey string) (adkmodel.LLM, error) {
	switch strings.ToLower(provider) {
	case "gemini", "google":
		return gemini.NewModel(ctx, modelName, &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		})
	case "openai":
		return newOpenAIModel(modelName, apiKey, ""), nil
	case "anthropic":
		// Anthropic exposes an OpenAI-compatible endpoint.
		return newOpenAIModel(modelName, apiKey, "https://api.anthropic.com/v1/"), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// OpenAI adapter implementing model.LLM
// ──────────────────────────────────────────────────────────────────────────────

type openAIModel struct {
	client    openai.Client
	modelName string
}

func newOpenAIModel(modelName, apiKey, baseURL string) *openAIModel {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &openAIModel{
		client:    openai.NewClient(opts...),
		modelName: modelName,
	}
}

// isOpenAIReasoningModel returns true for models that don't support custom temperature.
// GPT-5 and o1-series models only support temperature=1.
func isOpenAIReasoningModel(modelName string) bool {
	return strings.HasPrefix(modelName, "gpt-5") || strings.HasPrefix(modelName, "o1")
}

func (m *openAIModel) Name() string { return m.modelName }

// GenerateContent converts between genai content format and OpenAI's chat
// completion API, supporting both streaming and non-streaming modes.
func (m *openAIModel) GenerateContent(ctx context.Context, req *adkmodel.LLMRequest, stream bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		modelName := req.Model
		if modelName == "" {
			modelName = m.modelName
		}

		messages := contentsToOpenAI(req.Contents)
		if req.Config != nil && req.Config.SystemInstruction != nil {
			sysText := extractText(req.Config.SystemInstruction)
			if sysText != "" {
				messages = append([]openai.ChatCompletionMessageParamUnion{
					openai.SystemMessage(sysText),
				}, messages...)
			}
		}

		params := openai.ChatCompletionNewParams{
			Model:    modelName,
			Messages: messages,
		}

		// Convert tool declarations to OpenAI tools.
		if req.Config != nil {
			// GPT-5 and o1-series models only support default temperature (1).
			// Setting custom temperature causes errors.
			if req.Config.Temperature != nil && !isOpenAIReasoningModel(modelName) {
				params.Temperature = openai.Float(float64(*req.Config.Temperature))
			}
			if req.Config.TopP != nil {
				params.TopP = openai.Float(float64(*req.Config.TopP))
			}
			if req.Config.MaxOutputTokens > 0 {
				params.MaxTokens = openai.Int(int64(req.Config.MaxOutputTokens))
			}
			for _, t := range req.Config.Tools {
				for _, fd := range t.FunctionDeclarations {
					funcDef := openai.FunctionDefinitionParam{
						Name:        fd.Name,
						Description: openai.String(fd.Description),
						Parameters:  declarationParamsToOpenAI(fd),
					}
					params.Tools = append(params.Tools, openai.ChatCompletionFunctionTool(funcDef))
				}
			}
		}

		if stream {
			m.generateStream(ctx, params, yield)
		} else {
			m.generate(ctx, params, yield)
		}
	}
}

// generate performs a non-streaming request.
func (m *openAIModel) generate(ctx context.Context, params openai.ChatCompletionNewParams, yield func(*adkmodel.LLMResponse, error) bool) {
	resp, err := m.client.Chat.Completions.New(ctx, params)
	if err != nil {
		yield(nil, fmt.Errorf("openai completion error: %w", err))
		return
	}
	if len(resp.Choices) == 0 {
		yield(nil, fmt.Errorf("openai returned empty choices"))
		return
	}

	llmResp := openAIChoiceToLLMResponse(resp.Choices[0], resp.Usage)
	llmResp.TurnComplete = true
	yield(llmResp, nil)
}

// generateStream performs a streaming request.
func (m *openAIModel) generateStream(ctx context.Context, params openai.ChatCompletionNewParams, yield func(*adkmodel.LLMResponse, error) bool) {
	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{
		IncludeUsage: openai.Bool(true),
	}
	streamer := m.client.Chat.Completions.NewStreaming(ctx, params)

	var (
		accText      strings.Builder
		accToolCalls = map[int]*genai.FunctionCall{}
		accToolIDs   = map[int]string{}
		finalUsage   *genai.GenerateContentResponseUsageMetadata
	)

	for streamer.Next() {
		chunk := streamer.Current()

		// Stream usage metadata.
		if chunk.Usage.TotalTokens > 0 {
			finalUsage = &genai.GenerateContentResponseUsageMetadata{
				PromptTokenCount:     int32(chunk.Usage.PromptTokens),
				CandidatesTokenCount: int32(chunk.Usage.CompletionTokens),
				TotalTokenCount:      int32(chunk.Usage.TotalTokens),
			}
		}

		for _, choice := range chunk.Choices {
			delta := choice.Delta

			// Text content delta.
			if delta.Content != "" {
				accText.WriteString(delta.Content)
				if !yield(&adkmodel.LLMResponse{
					Content: &genai.Content{
						Role:  "model",
						Parts: []*genai.Part{{Text: delta.Content}},
					},
					Partial: true,
				}, nil) {
					return
				}
			}

			// Tool call deltas.
			for _, tc := range delta.ToolCalls {
				idx := int(tc.Index)
				if tc.ID != "" {
					accToolIDs[idx] = tc.ID
				}
				existing, ok := accToolCalls[idx]
				if !ok {
					existing = &genai.FunctionCall{Name: tc.Function.Name}
					accToolCalls[idx] = existing
				}
				if tc.Function.Name != "" {
					existing.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					// Accumulate JSON arguments.
					if existing.Args == nil {
						existing.Args = map[string]any{"__raw": tc.Function.Arguments}
					} else {
						existing.Args["__raw"] = existing.Args["__raw"].(string) + tc.Function.Arguments
					}
				}
			}
		}
	}

	if err := streamer.Err(); err != nil {
		yield(nil, fmt.Errorf("openai stream error: %w", err))
		return
	}

	// Build final response with accumulated content.
	var parts []*genai.Part

	if accText.Len() > 0 {
		parts = append(parts, &genai.Part{Text: accText.String()})
	}

	// Finalize tool calls.
	for i := 0; i < len(accToolCalls); i++ {
		fc := accToolCalls[i]
		if raw, ok := fc.Args["__raw"]; ok {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(raw.(string)), &parsed); err == nil {
				fc.Args = parsed
			}
		}
		fc.ID = accToolIDs[i]
		parts = append(parts, &genai.Part{FunctionCall: fc})
	}

	yield(&adkmodel.LLMResponse{
		Content: &genai.Content{
			Role:  "model",
			Parts: parts,
		},
		UsageMetadata: finalUsage,
		TurnComplete:  true,
	}, nil)
}

// ──────────────────────────────────────────────────────────────────────────────
// Conversion helpers: genai ↔ OpenAI
// ──────────────────────────────────────────────────────────────────────────────

// contentsToOpenAI converts a slice of genai.Content to OpenAI chat messages.
func contentsToOpenAI(contents []*genai.Content) []openai.ChatCompletionMessageParamUnion {
	var msgs []openai.ChatCompletionMessageParamUnion
	for _, c := range contents {
		if c == nil {
			continue
		}
		msgs = append(msgs, contentToOpenAI(c)...)
	}
	return msgs
}

// contentToOpenAI converts a single genai.Content to one or more OpenAI messages.
func contentToOpenAI(c *genai.Content) []openai.ChatCompletionMessageParamUnion {
	var msgs []openai.ChatCompletionMessageParamUnion

	switch c.Role {
	case "user":
		text := extractText(c)
		if text != "" {
			msgs = append(msgs, openai.UserMessage(text))
		}
		// Also handle function responses from user role (tool results).
		for _, p := range c.Parts {
			if p.FunctionResponse != nil {
				resultJSON, _ := json.Marshal(p.FunctionResponse.Response)
				msgs = append(msgs, openai.ToolMessage(string(resultJSON), p.FunctionResponse.ID))
			}
		}

	case "model":
		var textParts []string
		var toolCalls []openai.ChatCompletionMessageToolCallUnionParam

		for _, p := range c.Parts {
			if p.Text != "" {
				textParts = append(textParts, p.Text)
			}
			if p.FunctionCall != nil {
				argsJSON, _ := json.Marshal(p.FunctionCall.Args)
				toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: p.FunctionCall.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      p.FunctionCall.Name,
							Arguments: string(argsJSON),
						},
					},
				})
			}
		}

		text := strings.Join(textParts, "")
		if len(toolCalls) > 0 {
			var asst openai.ChatCompletionAssistantMessageParam
			if text != "" {
				asst.Content.OfString = openai.String(text)
			}
			asst.Role = "assistant"
			asst.ToolCalls = toolCalls
			msgs = append(msgs, openai.ChatCompletionMessageParamUnion{OfAssistant: &asst})
		} else if text != "" {
			msgs = append(msgs, openai.AssistantMessage(text))
		}

	default:
		// Treat unknown roles as user.
		text := extractText(c)
		if text != "" {
			msgs = append(msgs, openai.UserMessage(text))
		}
	}

	return msgs
}

// extractText concatenates all text parts from a genai.Content.
func extractText(c *genai.Content) string {
	if c == nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range c.Parts {
		if p.Text != "" {
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

// openAIChoiceToLLMResponse converts an OpenAI choice to an adk-go LLMResponse.
func openAIChoiceToLLMResponse(choice openai.ChatCompletionChoice, usage openai.CompletionUsage) *adkmodel.LLMResponse {
	var parts []*genai.Part

	if choice.Message.Content != "" {
		parts = append(parts, &genai.Part{Text: choice.Message.Content})
	}

	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		parts = append(parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: args,
			},
		})
	}

	return &adkmodel.LLMResponse{
		Content: &genai.Content{
			Role:  "model",
			Parts: parts,
		},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(usage.PromptTokens),
			CandidatesTokenCount: int32(usage.CompletionTokens),
			TotalTokenCount:      int32(usage.TotalTokens),
		},
	}
}

// declarationParamsToOpenAI converts a genai FunctionDeclaration's parameters
// to an OpenAI-compatible JSON schema interface.
func declarationParamsToOpenAI(fd *genai.FunctionDeclaration) openai.FunctionParameters {
	if fd.ParametersJsonSchema != nil {
		// ParametersJsonSchema may be a *jsonschema.Schema (from adk-go's
		// functiontool) rather than a plain map[string]any. Marshal through
		// JSON so any type that implements json.Marshaler is handled.
		data, err := json.Marshal(fd.ParametersJsonSchema)
		if err == nil {
			var m map[string]any
			if json.Unmarshal(data, &m) == nil {
				ensureObjectProperties(m)
				return openai.FunctionParameters(m)
			}
		}
	}
	// Fallback: build from genai.Schema if available.
	if fd.Parameters != nil {
		m := genaiSchemaToMap(fd.Parameters)
		ensureObjectProperties(m)
		return m
	}
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

// ensureObjectProperties walks a JSON-schema map, lowercases Gemini-style type
// values (e.g. "OBJECT" → "object"), and ensures every "object" node has a
// "properties" key. OpenAI strictly requires both.
func ensureObjectProperties(m map[string]any) {
	if m == nil {
		return
	}
	if t, _ := m["type"].(string); t != "" {
		lower := strings.ToLower(t)
		m["type"] = lower
		if lower == "object" {
			if _, ok := m["properties"]; !ok {
				m["properties"] = map[string]any{}
			}
		}
	}
	// Recurse into properties.
	if props, ok := m["properties"].(map[string]any); ok {
		for _, v := range props {
			if sub, ok := v.(map[string]any); ok {
				ensureObjectProperties(sub)
			}
		}
	}
	// Recurse into items (array element schema).
	if items, ok := m["items"].(map[string]any); ok {
		ensureObjectProperties(items)
	}
}

// genaiSchemaToMap converts a genai.Schema to a JSON-schema–style map.
func genaiSchemaToMap(s *genai.Schema) map[string]any {
	if s == nil {
		return nil
	}
	m := map[string]any{}
	if s.Type != "" {
		m["type"] = strings.ToLower(string(s.Type))
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if s.Items != nil {
		m["items"] = genaiSchemaToMap(s.Items)
	}
	if len(s.Properties) > 0 {
		props := map[string]any{}
		for k, v := range s.Properties {
			props[k] = genaiSchemaToMap(v)
		}
		m["properties"] = props
	}
	if len(s.Required) > 0 {
		m["required"] = s.Required
	}
	return m
}

// ──────────────────────────────────────────────────────────────────────────────
// Foundation → genai translation helpers
//
// These functions translate foundation types to genai types and are
// co-located with other model-specific code to contain the genai import
// surface in this single file.
// ──────────────────────────────────────────────────────────────────────────────

// buildGenerateContentConfig creates a genai config from model settings, with
// optional per-agent and per-request overrides applied in priority order.
//
// Precedence (highest wins):
//  1. runCfg non-zero values  — per-request overrides (e.g. from the frontend send button)
//  2. agentCfg non-nil values — per-agent saved preferences (Gap 1)
//  3. modelCfg pointer values — user global model settings (from aisettings)
//  4. ADK-Go defaults         — zero values
func buildGenerateContentConfig(
	cfg *fagent.ModelConfig,
	agentCfg *fagent.AgentModelConfig,
	runCfg *fagent.RunConfig,
) *genai.GenerateContentConfig {
	gc := &genai.GenerateContentConfig{}

	// Layer 3: user global model settings.
	if cfg.Temperature != nil {
		t := float32(*cfg.Temperature)
		gc.Temperature = &t
	}
	if cfg.MaxTokens != nil {
		gc.MaxOutputTokens = int32(*cfg.MaxTokens)
	}

	// Layer 2: per-agent saved preferences (override user global).
	if agentCfg != nil {
		if agentCfg.Temperature != nil {
			t := float32(*agentCfg.Temperature)
			gc.Temperature = &t
		}
		if agentCfg.TopP != nil {
			p := float32(*agentCfg.TopP)
			gc.TopP = &p
		}
		if agentCfg.TopK != nil {
			k := *agentCfg.TopK
			gc.TopK = &k
		}
		if agentCfg.MaxOutputTokens != nil {
			gc.MaxOutputTokens = *agentCfg.MaxOutputTokens
		}
		if len(agentCfg.StopSequences) > 0 {
			gc.StopSequences = agentCfg.StopSequences
		}
	}

	// Layer 1: per-request RunConfig (highest priority, overrides everything).
	if runCfg != nil {
		if runCfg.Temperature > 0 {
			t := float32(runCfg.Temperature)
			gc.Temperature = &t
		}
		if runCfg.MaxTokens > 0 {
			gc.MaxOutputTokens = int32(runCfg.MaxTokens)
		}
		if len(runCfg.StopSequences) > 0 {
			gc.StopSequences = runCfg.StopSequences
		}
	}

	return gc
}

// buildUserContent converts an agent.Message to genai.Content for the adk-go runner.
// Supports text, image, audio, and file multimodal content.
func buildUserContent(msg *fagent.Message) *genai.Content {
	if msg == nil {
		return genai.NewContentFromText("", "user")
	}

	var parts []*genai.Part
	for _, c := range msg.Content {
		switch c.Type {
		case fagent.ContentTypeText:
			parts = append(parts, &genai.Part{Text: c.Text})

		case fagent.ContentTypeImageURL:
			if c.ImageURL != nil {
				mimeType := mimeFromDataURL(c.ImageURL.URL)
				parts = append(parts, &genai.Part{
					InlineData: &genai.Blob{
						MIMEType: mimeType,
						Data:     []byte(c.ImageURL.URL),
					},
				})
			}

		case fagent.ContentTypeInputAudio:
			if c.AudioInput != nil {
				mimeType := "audio/" + c.AudioInput.Format
				parts = append(parts, &genai.Part{
					InlineData: &genai.Blob{
						MIMEType: mimeType,
						Data:     []byte(c.AudioInput.Data),
					},
				})
			}

		case fagent.ContentTypeFile:
			if c.FileInput != nil {
				mimeType := c.FileInput.MimeType
				if mimeType == "" {
					mimeType = "application/octet-stream"
				}
				parts = append(parts, &genai.Part{
					InlineData: &genai.Blob{
						MIMEType: mimeType,
						Data:     []byte(c.FileInput.Data),
					},
				})
			}

		default:
			if c.Text != "" {
				parts = append(parts, &genai.Part{Text: c.Text})
			}
		}
	}

	if len(parts) == 0 {
		parts = []*genai.Part{{Text: ""}}
	}

	return &genai.Content{
		Role:  "user",
		Parts: parts,
	}
}

// mimeFromDataURL extracts the MIME type from a data URL.
// Returns "image/png" as default if parsing fails.
func mimeFromDataURL(dataURL string) string {
	if !strings.HasPrefix(dataURL, "data:") {
		return "image/png"
	}
	rest := dataURL[5:]
	if idx := strings.Index(rest, ";"); idx > 0 {
		return rest[:idx]
	}
	return "image/png"
}
