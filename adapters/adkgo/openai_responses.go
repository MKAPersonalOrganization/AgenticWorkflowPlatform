package adkgo

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
	"google.golang.org/genai"

	adkmodel "google.golang.org/adk/model"
)

// openAIResponsesModel implements adkmodel.LLM via OpenAI's Responses API.
//
// This is the forward-looking path: Chat Completions only supports custom
// function tools, while Responses exposes every built-in OpenAI tool
// (code_interpreter, file_search, web_search, computer_use, image_generation,
// remote MCP). For M10 / M21 this unlocks the provider-agnostic code-execution
// toggle on OpenAI without touching the designer or foundation types.
//
// Scope: used exclusively for the "openai" provider. Anthropic's OpenAI-
// compatible endpoint does not implement /v1/responses, so that provider stays
// on the Chat Completions path (see openAIModel in model.go).
//
// Session model: stateless. We send the full genai.Content history as an
// Input item array on every request. No reliance on PreviousResponseID — the
// ADK session service is already the source of truth for conversation state,
// and keeping the API call stateless means replaying a session works the same
// regardless of which turn we're on.
type openAIResponsesModel struct {
	client    openai.Client
	modelName string
}

// newOpenAIResponsesModel builds an adapter against api.openai.com.
func newOpenAIResponsesModel(modelName, apiKey string) *openAIResponsesModel {
	return &openAIResponsesModel{
		client:    openai.NewClient(option.WithAPIKey(apiKey)),
		modelName: modelName,
	}
}

// Name implements adkmodel.LLM.
func (m *openAIResponsesModel) Name() string { return m.modelName }

// GenerateContent implements adkmodel.LLM. Translates the ADK request into
// a Responses API call and yields LLMResponse values shaped exactly like
// the Chat Completions adapter's output so the rest of the pipeline is
// unchanged.
func (m *openAIResponsesModel) GenerateContent(ctx context.Context, req *adkmodel.LLMRequest, stream bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		modelName := req.Model
		if modelName == "" {
			modelName = m.modelName
		}

		params := responses.ResponseNewParams{
			Model: shared.ResponsesModel(modelName),
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: contentsToResponsesInputItems(req.Contents),
			},
		}

		if req.Config != nil {
			// System instruction → top-level Instructions field (not an input item).
			if req.Config.SystemInstruction != nil {
				if sys := extractText(req.Config.SystemInstruction); sys != "" {
					params.Instructions = openai.String(sys)
				}
			}
			// GPT-5 and o-series reject custom temperature; skip on those.
			if req.Config.Temperature != nil && !isOpenAIReasoningModel(modelName) {
				params.Temperature = openai.Float(float64(*req.Config.Temperature))
			}
			if req.Config.TopP != nil {
				params.TopP = openai.Float(float64(*req.Config.TopP))
			}
			if req.Config.MaxOutputTokens > 0 {
				params.MaxOutputTokens = openai.Int(int64(req.Config.MaxOutputTokens))
			}
			params.Tools = toolsToResponsesParam(req.Config.Tools)

			// Structured outputs. ADK's basic_processor populates
			// ResponseSchema for every non-Gemini provider when the agent
			// has an OutputSchema; translate it to the Responses API's
			// Text.Format.JSONSchema (analogous to Chat Completions'
			// response_format). Reuses the same schema normalisation
			// helpers the Chat Completions adapter uses so strict-mode
			// rules (additionalProperties:false, all fields required,
			// nullable unions for optional fields) apply identically.
			if req.Config.ResponseSchema != nil {
				schemaMap := genaiSchemaToMap(req.Config.ResponseSchema)
				applyOpenAIStrictMode(schemaMap)
				params.Text = responses.ResponseTextConfigParam{
					Format: responses.ResponseFormatTextConfigUnionParam{
						OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
							Name:   "agent_output",
							Strict: openai.Bool(true),
							Schema: schemaMap,
						},
					},
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

// generate performs a non-streaming Responses-API request.
func (m *openAIResponsesModel) generate(ctx context.Context, params responses.ResponseNewParams, yield func(*adkmodel.LLMResponse, error) bool) {
	resp, err := m.client.Responses.New(ctx, params)
	if err != nil {
		yield(nil, fmt.Errorf("openai responses error: %w", err))
		return
	}
	if resp == nil {
		yield(nil, fmt.Errorf("openai responses returned nil response"))
		return
	}
	llmResp := responsesOutputToLLMResponse(resp.Output, resp.Usage)
	llmResp.TurnComplete = true
	yield(llmResp, nil)
}

// generateStream performs a streaming Responses-API request. The Responses
// stream is event-typed (many variants), not delta-typed like Chat
// Completions — we switch on `event.Type`, emit partial text frames as we
// see them, and fall back to ResponseCompletedEvent for the final
// authoritative output (it carries the full Response with Output items and
// Usage). Intermediate `…Done` events are used to finalise tool-call
// arguments and code blocks so the final LLMResponse has the right parts.
func (m *openAIResponsesModel) generateStream(ctx context.Context, params responses.ResponseNewParams, yield func(*adkmodel.LLMResponse, error) bool) {
	stream := m.client.Responses.NewStreaming(ctx, params)
	defer stream.Close()

	for stream.Next() {
		evt := stream.Current()
		switch evt.Type {
		case "response.output_text.delta":
			// Streaming text fragment.
			if evt.Delta == "" {
				continue
			}
			if !yield(&adkmodel.LLMResponse{
				Content: &genai.Content{
					Role:  "model",
					Parts: []*genai.Part{{Text: evt.Delta}},
				},
				Partial: true,
			}, nil) {
				return
			}

		case "response.completed":
			// Final frame: the API has assembled the whole Response.
			// evt.Response has Output + Usage; translate and emit once.
			resp := evt.Response
			final := responsesOutputToLLMResponse(resp.Output, resp.Usage)
			final.TurnComplete = true
			yield(final, nil)
			return

		case "response.failed", "error":
			msg := evt.Message
			if msg == "" {
				msg = "openai responses stream failed"
			}
			yield(nil, fmt.Errorf("openai stream error: %s", msg))
			return

		// Everything else (function_call_arguments.delta, code_interpreter_*,
		// lifecycle markers like response.created / in_progress / output_item.*)
		// is captured by response.completed at the end — no need to
		// accumulate by hand.
		default:
		}
	}

	if err := stream.Err(); err != nil {
		yield(nil, fmt.Errorf("openai responses stream error: %w", err))
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Conversions: genai ↔ Responses API
// ──────────────────────────────────────────────────────────────────────────────

// contentsToResponsesInputItems flattens our genai history into the Responses
// API's input-item array. Each genai.Content becomes one or more items:
//
//   - user message with text              → EasyInputMessage(user, text)
//   - user message with function response → FunctionCallOutput item
//   - model message with text             → EasyInputMessage(assistant, text)
//   - model message with function call    → FunctionCall item
//
// Internal ADK tools (adk_*) are filtered out — they exist only inside the
// orchestration engine and are not part of OpenAI's conversation shape.
func contentsToResponsesInputItems(contents []*genai.Content) responses.ResponseInputParam {
	var items responses.ResponseInputParam
	for _, c := range contents {
		if c == nil {
			continue
		}
		switch c.Role {
		case "user":
			items = append(items, userContentToResponsesInput(c)...)
		case "model":
			items = append(items, modelContentToResponsesInput(c)...)
		default:
			// Treat unknown roles as user, matching the Chat Completions adapter.
			if text := extractText(c); text != "" {
				items = append(items, responses.ResponseInputItemParamOfMessage(text, responses.EasyInputMessageRoleUser))
			}
		}
	}
	return items
}

// userContentToResponsesInput emits one user message for any text, plus
// one function_call_output entry per FunctionResponse. Internal adk_*
// responses never cross the provider boundary.
func userContentToResponsesInput(c *genai.Content) responses.ResponseInputParam {
	var items responses.ResponseInputParam
	if text := extractText(c); text != "" {
		items = append(items, responses.ResponseInputItemParamOfMessage(text, responses.EasyInputMessageRoleUser))
	}
	for _, p := range c.Parts {
		if p.FunctionResponse == nil || strings.HasPrefix(p.FunctionResponse.Name, "adk_") {
			continue
		}
		resultJSON, _ := json.Marshal(p.FunctionResponse.Response)
		items = append(items, responses.ResponseInputItemParamOfFunctionCallOutput(
			p.FunctionResponse.ID, string(resultJSON),
		))
	}
	return items
}

// modelContentToResponsesInput emits one assistant message for any text,
// plus one function_call entry per FunctionCall. Internal adk_* tool
// calls are filtered for the same reason as user responses.
func modelContentToResponsesInput(c *genai.Content) responses.ResponseInputParam {
	var items responses.ResponseInputParam
	if text := extractText(c); text != "" {
		items = append(items, responses.ResponseInputItemParamOfMessage(text, responses.EasyInputMessageRoleAssistant))
	}
	for _, p := range c.Parts {
		if p.FunctionCall == nil || strings.HasPrefix(p.FunctionCall.Name, "adk_") {
			continue
		}
		argsJSON, _ := json.Marshal(p.FunctionCall.Args)
		items = append(items, responses.ResponseInputItemParamOfFunctionCall(
			string(argsJSON), p.FunctionCall.ID, p.FunctionCall.Name,
		))
	}
	return items
}

// toolsToResponsesParam translates genai.Tool[] into ToolUnionParam[].
//
// Function declarations become function tools. Our code-execution marker
// (genai.Tool{CodeExecution: ...} appended by runner.applyCodeExecutionTool
// for Gemini) is recognised here and mapped to the OpenAI built-in
// code_interpreter tool with an auto-provisioned container. This lets a
// single per-agent CodeExecutionPolicy flag drive the right wire shape on
// each provider without leaking provider-specific types above the adapter.
func toolsToResponsesParam(tools []*genai.Tool) []responses.ToolUnionParam {
	var out []responses.ToolUnionParam
	for _, t := range tools {
		if t == nil {
			continue
		}
		// Provider-agnostic code-execution signal from applyCodeExecutionTool.
		// OpenAI's API rejects "auto" as a string container ID — it expects
		// either a `cntr_…` ID for a pre-created container or the explicit
		// auto-object form. We use the latter so OpenAI provisions a fresh
		// sandbox per invocation.
		if t.CodeExecution != nil {
			out = append(out, responses.ToolParamOfCodeInterpreter(
				responses.ToolCodeInterpreterContainerCodeInterpreterContainerAutoParam{},
			))
		}
		for _, fd := range t.FunctionDeclarations {
			if fd == nil || strings.HasPrefix(fd.Name, "adk_") {
				continue
			}
			ft := responses.FunctionToolParam{
				Name:        fd.Name,
				Description: openai.String(fd.Description),
				Parameters:  declarationParamsToOpenAI(fd),
			}
			out = append(out, responses.ToolUnionParam{OfFunction: &ft})
		}
	}
	return out
}

// responsesOutputToLLMResponse walks Response.Output and synthesises a
// genai.Content + LLMResponse shaped exactly like the Chat Completions
// adapter's output. Code interpreter calls are unpacked into
// genai.Part.ExecutableCode + genai.Part.CodeExecutionResult, which
// events.go:translateCodeExecution already knows how to turn into
// CodeExecutionRan domain events — so the same chat card that works for
// Gemini renders for OpenAI without any downstream changes.
// messageItemToParts pulls every non-empty output_text segment out of a
// Responses-API message item. Other content types (refusal, tool calls
// buried inside messages) are ignored — the Responses API surfaces them
// as their own top-level items.
func messageItemToParts(item responses.ResponseOutputItemUnion) []*genai.Part {
	var parts []*genai.Part
	for _, c := range item.Content {
		if c.Type == "output_text" && c.Text != "" {
			parts = append(parts, &genai.Part{Text: c.Text})
		}
	}
	return parts
}

// functionCallItemToPart converts a Responses function_call into a
// genai FunctionCall Part. Arguments is a union — OfString is the
// common wire form; we fall back to marshalling the opaque shape so
// unexpected variants aren't silently dropped. The call ID defaults to
// item.ID when CallID is absent (older API versions).
func functionCallItemToPart(item responses.ResponseOutputItemUnion) *genai.Part {
	raw := item.Arguments.OfString
	if raw == "" {
		if b, err := json.Marshal(item.Arguments); err == nil {
			raw = string(b)
		}
	}
	var args map[string]any
	_ = json.Unmarshal([]byte(raw), &args)
	id := item.CallID
	if id == "" {
		id = item.ID
	}
	return &genai.Part{
		FunctionCall: &genai.FunctionCall{
			ID:   id,
			Name: item.Name,
			Args: args,
		},
	}
}

// codeInterpreterItemToParts expands a code_interpreter_call into
// ExecutableCode + CodeExecutionResult pair keyed on the same item.ID,
// so events.go:translateCodeExecution groups them into one
// CodeExecutionRan domain event — the same chat card that renders for
// Gemini renders for OpenAI with zero downstream changes.
func codeInterpreterItemToParts(item responses.ResponseOutputItemUnion) []*genai.Part {
	var parts []*genai.Part
	if item.Code != "" {
		parts = append(parts, &genai.Part{
			ExecutableCode: &genai.ExecutableCode{
				ID:       item.ID,
				Language: genai.LanguagePython,
				Code:     item.Code,
			},
		})
	}
	outText := codeInterpreterOutputText(item.Outputs)
	if outText != "" || item.Status != "" {
		outcome := "OUTCOME_OK"
		if item.Status == "failed" || item.Status == "incomplete" {
			outcome = "OUTCOME_FAILED"
		}
		parts = append(parts, &genai.Part{
			CodeExecutionResult: &genai.CodeExecutionResult{
				ID:      item.ID,
				Outcome: genai.Outcome(outcome),
				Output:  outText,
			},
		})
	}
	return parts
}

// codeInterpreterOutputText concatenates textual logs and image URLs
// into a single blob — what the chat card renders beneath the code
// block. Images are referenced by URL so the card can link out.
func codeInterpreterOutputText(outputs []responses.ResponseCodeInterpreterToolCallOutputUnion) string {
	var b strings.Builder
	for _, o := range outputs {
		switch o.Type {
		case "logs":
			if o.Logs != "" {
				b.WriteString(o.Logs)
			}
		case "image":
			if o.URL != "" {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString("[image] ")
				b.WriteString(o.URL)
			}
		}
	}
	return b.String()
}

func responsesOutputToLLMResponse(out []responses.ResponseOutputItemUnion, usage responses.ResponseUsage) *adkmodel.LLMResponse {
	var parts []*genai.Part
	for _, item := range out {
		switch item.Type {
		case "message":
			parts = append(parts, messageItemToParts(item)...)
		case "function_call":
			parts = append(parts, functionCallItemToPart(item))
		case "code_interpreter_call":
			parts = append(parts, codeInterpreterItemToParts(item)...)
		default:
			// Ignore unsupported item types (reasoning, file_search_call,
			// web_search_call, image_generation_call, …). They'll land in
			// the pipeline without breaking anything; downstream just sees
			// a response with no extra parts.
		}
	}
	return &adkmodel.LLMResponse{
		Content: &genai.Content{
			Role:  "model",
			Parts: parts,
		},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(usage.InputTokens),
			CandidatesTokenCount: int32(usage.OutputTokens),
			TotalTokenCount:      int32(usage.TotalTokens),
		},
	}
}
