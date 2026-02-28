package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"runtime"
	"strings"

	"github.com/avast/retry-go/v4"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

// GeminiClient implements the LLM interface for Gemini.
type GeminiClient struct {
	client *genai.Client
	model  string
}

// NewGeminiClient creates a new Gemini client.
func NewGeminiClient(ctx context.Context, apiKey string, model string) (*GeminiClient, error) {
	cfg := &genai.ClientConfig{
		APIKey: apiKey,
	}

	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, &Error{
			Op:   "gemini.NewClient",
			Kind: KindInternal,
			Err:  err,
		}
	}

	return &GeminiClient{
		client: client,
		model:  model,
	}, nil
}

// Generate implements the LLM interface.
func (c *GeminiClient) Generate(
	ctx context.Context,
	req *Request,
) iter.Seq2[*Response, error] {
	return func(yield func(*Response, error) bool) {
		// Convert messages to genai.Content
		contents := c.buildContents(req.Messages)

		// Build config
		config := c.buildConfig(req)

		// Set headers to match reference implementation
		if config.HTTPOptions == nil {
			config.HTTPOptions = &genai.HTTPOptions{}
		}
		if config.HTTPOptions.Headers == nil {
			config.HTTPOptions.Headers = make(map[string][]string)
		}
		versionHeader := fmt.Sprintf("google-adk/0.0.1 gl-go/%s", strings.TrimPrefix(runtime.Version(), "go"))
		config.HTTPOptions.Headers["x-goog-api-client"] = []string{versionHeader}
		config.HTTPOptions.Headers["user-agent"] = []string{versionHeader}

		// Select model
		modelName := c.model
		if req.Extensions != nil {
			if m, ok := req.Extensions["model"].(string); ok {
				modelName = m
			}
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
						yield(nil, c.mapError("gemini.GenerateStream", err))
						return nil
					}
					firstChunk = false

					// Iterate over candidates
					for _, candidate := range chunk.Candidates {
						// Handle Content
						if candidate.Content != nil {
							for _, part := range candidate.Content.Parts {
								if part.Text != "" {
									resp := &Response{
										Content: []Content{TextContent{Text: part.Text}},
										Partial: true,
										Extensions: Metadata{
											"response_id": responseID,
											"model":       modelName,
										},
									}
									if !yield(resp, nil) {
										return nil
									}
								}

								if part.FunctionCall != nil {
									argsJSON, _ := json.Marshal(part.FunctionCall.Args)

									callID := part.FunctionCall.ID
									if callID == "" {
										callID = "call_" + uuid.New().String()
									}

									fc := FunctionCall{
										ID:        callID,
										Name:      part.FunctionCall.Name,
										Arguments: json.RawMessage(argsJSON),
									}

									resp := &Response{
										Content: []Content{fc},
										Partial: true,
										Extensions: Metadata{
											"response_id": responseID,
											"model":       modelName,
										},
									}
									if !yield(resp, nil) {
										return nil
									}
								}
							}
						}

						// Finish Reason
						if candidate.FinishReason != "" {
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
								reason = FinishReasonToolUse
							} else {
								reason = c.mapFinishReason(candidate.FinishReason)
							}

							resp := &Response{
								FinishReason: reason,
								Partial:      false,
								Extensions: Metadata{
									"response_id": responseID,
									"model":       modelName,
									"provider":    "gemini",
								},
							}
							if !yield(resp, nil) {
								return nil
							}
						}
					}

					// Handle Usage
					if chunk.UsageMetadata != nil {
						usage := &Usage{
							InputTokens:  int(chunk.UsageMetadata.PromptTokenCount),
							OutputTokens: int(chunk.UsageMetadata.CandidatesTokenCount),
							TotalTokens:  int(chunk.UsageMetadata.TotalTokenCount),
						}
						resp := &Response{
							Usage:   usage,
							Partial: true,
							Extensions: Metadata{
								"response_id": responseID,
								"model":       modelName,
							},
						}
						if !yield(resp, nil) {
							return nil
						}
					}
				}
				return nil
			},
			retry.Attempts(3),
			retry.DelayType(retry.BackOffDelay),
			retry.RetryIf(func(err error) bool {
				return strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Resource exhausted")
			}),
		)
		if err != nil {
			yield(nil, c.mapError("gemini.Generate", err))
		}
	}
}

func (c *GeminiClient) mapError(op string, err error) error {
	if err == nil {
		return nil
	}

	kind := KindInternal
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "429") || strings.Contains(errStr, "Resource exhausted"):
		kind = KindRateLimit
	case strings.Contains(errStr, "400") || strings.Contains(errStr, "invalid"):
		kind = KindInvalidInput
	case strings.Contains(errStr, "404") || strings.Contains(errStr, "not found"):
		kind = KindNotFound
	}

	return &Error{
		Op:   op,
		Kind: kind,
		Err:  err,
		Context: map[string]any{
			"model": c.model,
		},
	}
}

func (c *GeminiClient) buildContents(messages []Message) []*genai.Content {
	var contents []*genai.Content
	callIDToName := make(map[string]string)

	for _, msg := range messages {
		var parts []*genai.Part

		for _, content := range msg.Content {
			switch v := content.(type) {
			case TextContent:
				parts = append(parts, &genai.Part{Text: v.Text})
			case FunctionCall:
				if v.ID != "" {
					callIDToName[v.ID] = v.Name
				}
				var args map[string]any
				_ = json.Unmarshal(v.Arguments, &args)
				parts = append(parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						ID:   v.ID,
						Name: v.Name,
						Args: args,
					},
				})
			case FunctionResult:
				var resultData map[string]interface{}
				if err := json.Unmarshal(v.Result, &resultData); err != nil {
					resultData = map[string]interface{}{"result": string(v.Result)}
				}

				name := v.Name
				if name == "" {
					name = callIDToName[v.CallID]
				}

				parts = append(parts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name:     name,
						ID:       v.CallID,
						Response: resultData,
					},
				})
			case ImageURLContent:
				// Gemini supports inline images via data URIs
				if strings.HasPrefix(v.URL, "data:") {
					// Parse data URI: data:image/png;base64,<data>
					if idx := strings.Index(v.URL, ","); idx > 0 {
						header := v.URL[5:idx] // e.g. "image/png;base64"
						mimeType := strings.Split(header, ";")[0]
						b64Data := v.URL[idx+1:]
						decoded, err := base64.StdEncoding.DecodeString(b64Data)
						if err == nil {
							parts = append(parts, &genai.Part{
								InlineData: &genai.Blob{
									MIMEType: mimeType,
									Data:     decoded,
								},
							})
						}
					}
				}
			case AudioContent:
				// Gemini supports inline audio via InlineData blobs
				decoded, err := base64.StdEncoding.DecodeString(v.Data)
				if err == nil {
					mimeType := "audio/" + v.Format
					parts = append(parts, &genai.Part{
						InlineData: &genai.Blob{
							MIMEType: mimeType,
							Data:     decoded,
						},
					})
				}
			case FileContent:
				// Gemini supports inline files via InlineData blobs
				decoded, err := base64.StdEncoding.DecodeString(v.Data)
				if err == nil {
					mimeType := v.MimeType
					if mimeType == "" {
						mimeType = "application/octet-stream"
					}
					parts = append(parts, &genai.Part{
						InlineData: &genai.Blob{
							MIMEType: mimeType,
							Data:     decoded,
						},
					})
				}
			}
		}

		if len(parts) > 0 {
			role := "user"
			if msg.Role == RoleAssistant {
				role = "model"
			} else if msg.Role == RoleTool {
				role = "function"
			} else if msg.Role == RoleSystem {
				continue
			}

			contents = append(contents, &genai.Content{
				Role:  role,
				Parts: parts,
			})
		}
	}

	if len(contents) == 0 {
		contents = append(contents, &genai.Content{
			Role:  "user",
			Parts: []*genai.Part{{Text: "Handle the requests as specified in the System Instruction."}},
		})
	} else {
		last := contents[len(contents)-1]
		if last.Role != "user" && last.Role != "function" {
			contents = append(contents, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: "Continue processing previous requests as instructed. Exit or provide a summary if no more outputs are needed."}},
			})
		}
	}

	return contents
}

func (c *GeminiClient) buildConfig(req *Request) *genai.GenerateContentConfig {
	cfg := &genai.GenerateContentConfig{}

	if req.Parameters != nil {
		opts := req.Parameters
		if opts.Temperature != nil {
			t := float32(*opts.Temperature)
			cfg.Temperature = &t
		}
		if opts.TopP != nil {
			p := float32(*opts.TopP)
			cfg.TopP = &p
		}
		if opts.MaxTokens > 0 {
			cfg.MaxOutputTokens = int32(opts.MaxTokens)
		}
		if len(opts.StopSequences) > 0 {
			cfg.StopSequences = opts.StopSequences
		}
	}

	if req.System != "" {
		cfg.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: req.System}},
		}
	}

	// Tools mapping
	if len(req.Tools) > 0 {
		var funcs []*genai.FunctionDeclaration
		for _, tool := range req.Tools {
			var paramsMap map[string]any
			if tool.Parameters != nil {
				data, _ := json.Marshal(tool.Parameters)
				_ = json.Unmarshal(data, &paramsMap)
			}

			funcs = append(funcs, &genai.FunctionDeclaration{
				Name:                 tool.Name,
				Description:          tool.Description,
				ParametersJsonSchema: paramsMap,
			})
		}
		cfg.Tools = []*genai.Tool{{
			FunctionDeclarations: funcs,
		}}

		if req.Extensions != nil {
			if toolMode, ok := req.Extensions["tool_mode"].(string); ok {
				switch toolMode {
				case "required":
					cfg.ToolConfig = &genai.ToolConfig{
						FunctionCallingConfig: &genai.FunctionCallingConfig{
							Mode: genai.FunctionCallingConfigModeAny,
						},
					}
				case "none":
					cfg.ToolConfig = &genai.ToolConfig{
						FunctionCallingConfig: &genai.FunctionCallingConfig{
							Mode: genai.FunctionCallingConfigModeNone,
						},
					}
				}
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
	default:
		return FinishReason(reason)
	}
}
