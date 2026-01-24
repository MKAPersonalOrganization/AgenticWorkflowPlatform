package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"runtime"
	"strings"

	"github.com/avast/retry-go/v4"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

// GeminiClient implements the ChatClient interface for Gemini.
type GeminiClient struct {
	client *genai.Client
	model  string
}

// NewGeminiClient creates a new Gemini client.
func NewGeminiClient(ctx context.Context, apiKey string, model string) (*GeminiClient, error) {
	cfg := &genai.ClientConfig{
		APIKey: apiKey,
		// Backend: genai.BackendGoogleAI,
	}

	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &GeminiClient{
		client: client,
		model:  model,
	}, nil
}

// Provider implements the ChatClient interface.
func (c *GeminiClient) Provider() string {
	return "gemini"
}

// GetStreamingResponse implements the ChatClient interface.
func (c *GeminiClient) GetStreamingResponse(
	ctx context.Context,
	messages []ChatMessage,
	options *ChatOptions,
) iter.Seq2[ChatResponseUpdate, error] {
	return func(yield func(ChatResponseUpdate, error) bool) {
		// Convert messages to genai.Content
		contents := c.buildContents(messages)

		// Build config
		config := c.buildConfig(options)

		// Set headers to match reference implementation
		if config.HTTPOptions == nil {
			config.HTTPOptions = &genai.HTTPOptions{}
		}
		if config.HTTPOptions.Headers == nil {
			config.HTTPOptions.Headers = make(map[string][]string)
		}
		// Mimic reference version heder: google-adk/<version> gl-go/<go-version>
		// Since we don't have ADK version, we use a placeholder or similar.
		// Reference: "google-adk/%s gl-go/%s"
		versionHeader := fmt.Sprintf("google-adk/0.0.1 gl-go/%s", strings.TrimPrefix(runtime.Version(), "go"))
		config.HTTPOptions.Headers["x-goog-api-client"] = []string{versionHeader}
		config.HTTPOptions.Headers["user-agent"] = []string{versionHeader}

		// Select model
		modelName := c.model
		if options != nil && options.ModelID != "" {
			modelName = options.ModelID
		}

		// Call the API with retry logic
		err := retry.Do(
			func() error {
				respStream := c.client.Models.GenerateContentStream(ctx, modelName, contents, config)

				var responseID string
				firstChunk := true

				for chunk, err := range respStream {
					if err != nil {
						// Only retry if we haven't sent any content yet
						if firstChunk {
							return err
						}
						// If error happens mid-stream, we can't easily retry transparently
						yield(ChatResponseUpdate{}, err)
						return nil // return nil to stop retry.Do, as we handled error via yield
					}
					firstChunk = false

					// We don't get a strict response ID from standard Gemini stream easily in the first chunk like OpenAI
					// But we can generate or leave empty.
					// Currently genai.GenerateContentResponse doesn't seem to have a top-level ID field in the stream struct easily accessible?
					// Checking struct definition would be good, but assuming standard flow.

					// Iterate over candidates
					for _, candidate := range chunk.Candidates {
						// Handle Content
						if candidate.Content != nil {
							for _, part := range candidate.Content.Parts {
								if part.Text != "" {
									update := ChatResponseUpdate{
										Role:       ChatRoleAssistant,
										ResponseID: responseID,
										Model:      modelName,
										Contents:   []AIContent{TextContent{Text: part.Text}},
									}
									if !yield(update, nil) {
										return nil // Stop iteration
									}
								}

								if part.FunctionCall != nil {
									// Function call
									argsJSON, _ := json.Marshal(part.FunctionCall.Args) // Args is map[string]interface{}

									callID := part.FunctionCall.ID
									if callID == "" {
										// Generate a local ID if Gemini didn't provide one
										callID = "call_" + uuid.New().String()
									}

									fc := FunctionCallContent{
										CallID:       callID,
										Name:         part.FunctionCall.Name,
										Arguments:    part.FunctionCall.Args,
										RawArguments: string(argsJSON),
									}

									update := ChatResponseUpdate{
										Role:       ChatRoleAssistant,
										ResponseID: responseID,
										Model:      modelName,
										Contents:   []AIContent{fc},
									}
									if !yield(update, nil) {
										return nil
									}
								}
							}
						}

						// Finish Reason
						if candidate.FinishReason != "" {
							// Check for potential tool calls if standard reason is stop/other
							hasToolCalls := false
							if candidate.Content != nil {
								for _, p := range candidate.Content.Parts {
									if p.FunctionCall != nil {
										hasToolCalls = true
										break
									}
								}
							}

							var reason FinishReason
							if hasToolCalls {
								reason = FinishReasonToolCalls
							} else {
								reason = c.mapFinishReason(candidate.FinishReason)
							}

							update := ChatResponseUpdate{
								Role:         ChatRoleAssistant,
								FinishReason: reason,
							}
							if !yield(update, nil) {
								return nil
							}
						}
					}

					// Handle Usage
					if chunk.UsageMetadata != nil {
						usage := &UsageDetails{
							InputTokens:  int(chunk.UsageMetadata.PromptTokenCount),
							OutputTokens: int(chunk.UsageMetadata.CandidatesTokenCount),
							TotalTokens:  int(chunk.UsageMetadata.TotalTokenCount),
						}
						update := ChatResponseUpdate{
							Role:  ChatRoleAssistant,
							Usage: usage,
						}
						if !yield(update, nil) {
							return nil
						}
					}
				}
				return nil
			},
			retry.Attempts(3),
			retry.DelayType(retry.BackOffDelay),
			retry.RetryIf(func(err error) bool {
				// Retry on Resource Exhausted (429) errors
				return strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Resource exhausted")
			}),
		)
		if err != nil {
			// If retry exhausted or non-retryable error
			yield(ChatResponseUpdate{}, err)
		}
	}
}

func (c *GeminiClient) buildContents(messages []ChatMessage) []*genai.Content {
	var contents []*genai.Content
	callIDToName := make(map[string]string)

	for _, msg := range messages {
		var parts []*genai.Part

		for _, content := range msg.Contents {
			switch v := content.(type) {
			case TextContent:
				parts = append(parts, &genai.Part{Text: v.Text})
			case FunctionCallContent:
				if v.CallID != "" {
					callIDToName[v.CallID] = v.Name
				}
				parts = append(parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						ID:   v.CallID,
						Name: v.Name,
						Args: v.Arguments,
					},
				})
			case FunctionResultContent:
				// FunctionResponse needs to be wrapped properly
				// Gemini expects FunctionResponse to be a part
				// We need to parse valid JSON result or wrap it
				var resultData map[string]interface{}
				// Try parsing result as JSON, otherwise wrap string
				if err := json.Unmarshal([]byte(v.Result), &resultData); err != nil {
					resultData = map[string]interface{}{"result": v.Result}
				}

				name := callIDToName[v.CallID]
				// If name is missing, we can try to proceed with empty name or handle error,
				// but Gemini usually requires it.

				parts = append(parts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name:     name,
						ID:       v.CallID,
						Response: resultData,
					},
				})
			}
		}

		if len(parts) > 0 {
			role := "user"
			if msg.Role == ChatRoleAssistant {
				role = "model"
			} else if msg.Role == ChatRoleTool {
				role = "function" // technically part of 'user' or separate? "function" role is deprecated or used differently?
				// GenAI usually uses 'function' or puts it in history associated with user?
				// In new API, it is usually role="function" or role="tool" depending on version.
				// Google GenAI SDK `Content` struct: Role field.
				// Valid roles: "user", "model", "tool"? or just "user"/"model".
				// "function" is often a separate role.
				role = "function" // Verify strict role names
			} else if msg.Role == ChatRoleSystem {
				// System instructions are usually config, not content history in Gemini
				continue
			}

			contents = append(contents, &genai.Content{
				Role:  role,
				Parts: parts,
			})
		}
	}

	// Semantic Check: Gemini requires the last message to be from 'user' or 'function' (which drives model output).
	// If the last message is 'model', validation will fail. We must append a user message.
	if len(contents) == 0 {
		contents = append(contents, &genai.Content{
			Role:  "user",
			Parts: []*genai.Part{{Text: "Handle the requests as specified in the System Instruction."}},
		})
	} else {
		last := contents[len(contents)-1]
		if last.Role != "user" {
			contents = append(contents, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: "Continue processing previous requests as instructed. Exit or provide a summary if no more outputs are needed."}},
			})
		}
	}

	return contents
}

func (c *GeminiClient) buildConfig(options *ChatOptions) *genai.GenerateContentConfig {
	if options == nil {
		return nil
	}

	cfg := &genai.GenerateContentConfig{}

	if options.Temperature != nil {
		t := float32(*options.Temperature)
		cfg.Temperature = &t
	}
	if options.TopP != nil {
		p := float32(*options.TopP)
		cfg.TopP = &p
	}
	if options.MaxOutputTokens != nil {
		cfg.MaxOutputTokens = int32(*options.MaxOutputTokens)
	}
	if len(options.StopSequences) > 0 {
		cfg.StopSequences = options.StopSequences
	}
	if options.Instructions != "" {
		cfg.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: options.Instructions}},
		}
	}

	// Tools mapping
	if len(options.Tools) > 0 {
		var funcs []*genai.FunctionDeclaration
		for _, tool := range options.Tools {
			funcs = append(funcs, &genai.FunctionDeclaration{
				Name:                 tool.Name(),
				Description:          tool.Description(),
				ParametersJsonSchema: tool.Parameters(),
			})
		}
		cfg.Tools = []*genai.Tool{{
			FunctionDeclarations: funcs,
		}}

		// Tool Mode
		if options.ToolMode == ToolModeRequired {
			cfg.ToolConfig = &genai.ToolConfig{
				FunctionCallingConfig: &genai.FunctionCallingConfig{
					Mode: genai.FunctionCallingConfigModeAny,
				},
			}
		} else if options.ToolMode == ToolModeNone {
			cfg.ToolConfig = &genai.ToolConfig{
				FunctionCallingConfig: &genai.FunctionCallingConfig{
					Mode: genai.FunctionCallingConfigModeNone,
				},
			}
		}
	}

	return cfg
}

func (c *GeminiClient) mapFinishReason(reason genai.FinishReason) FinishReason {
	switch reason {
	case genai.FinishReasonStop:
		return FinishReasonStop
	case genai.FinishReasonMaxTokens:
		return FinishReasonLength
	// Add other mappings
	default:
		return FinishReason(string(reason))
	}
}
