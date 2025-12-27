package openai

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/ssestream"
	"school-with-ai/internal/session"
)

// Client wraps the OpenAI client with additional functionality
type Client struct {
	client *openai.Client
	model  string
}

// NewClient creates a new OpenAI client
// Requires OPENAI_API_KEY environment variable to be set
func NewClient() (*Client, error) {
	client := openai.NewClient()

	return &Client{
		client: &client,
		model:  string(openai.ChatModelGPT4oMini), // Use GPT-4o-mini for cost efficiency
	}, nil
}

// NewClientWithAPIKey creates a new OpenAI client with an explicit API key
func NewClientWithAPIKey(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key cannot be empty")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &Client{
		client: &client,
		model:  string(openai.ChatModelGPT4oMini),
	}, nil
}

// SetModel changes the model used for completions
func (c *Client) SetModel(model string) {
	c.model = model
}

// BuildMessages converts session messages to OpenAI message format
func BuildMessages(messages []session.Message) []openai.ChatCompletionMessageParamUnion {
	params := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			params = append(params, openai.UserMessage(msg.Content))
		case "assistant":
			// Check if assistant message has tool calls
			if len(msg.ToolCalls) > 0 {
				// Convert tool calls to OpenAI format
				toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Arguments: tc.Function.Arguments,
								Name:      tc.Function.Name,
							},
						},
					})
				}

				// Build assistant message with tool calls
				var asst openai.ChatCompletionAssistantMessageParam
				if msg.Content != "" {
					asst.Content.OfString = openai.String(msg.Content)
				}
				asst.Role = "assistant"
				asst.ToolCalls = toolCalls

				params = append(params, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &asst,
				})
			} else {
				params = append(params, openai.AssistantMessage(msg.Content))
			}
		case "tool":
			// Tool result message (content first, then toolCallID)
			params = append(params, openai.ToolMessage(msg.Content, msg.ToolCallID))
		case "system":
			params = append(params, openai.SystemMessage(msg.Content))
		}
	}

	return params
}

// CreateStreamingCompletion creates a streaming chat completion with optional tools
func (c *Client) CreateStreamingCompletion(
	ctx context.Context,
	messages []session.Message,
	tools []openai.ChatCompletionToolUnionParam,
) *ssestream.Stream[openai.ChatCompletionChunk] {
	params := BuildMessages(messages)

	streamParams := openai.ChatCompletionNewParams{
		Messages: params,
		Model:    openai.ChatModel(c.model),
	}

	// Add tools if provided
	if len(tools) > 0 {
		streamParams.Tools = tools
	}

	stream := c.client.Chat.Completions.NewStreaming(ctx, streamParams)

	return stream
}

// GenerateThreadTitle generates a concise title for a conversation thread
// based on the first few messages exchanged
func (c *Client) GenerateThreadTitle(ctx context.Context, messages []session.Message) (string, error) {
	// Create a system prompt for title generation
	titlePrompt := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful assistant that generates concise, descriptive titles for conversations. Generate a title that captures the main topic or question. Keep it under 50 characters. Return ONLY the title text, nothing else."),
	}

	// Add the conversation messages (limited to first 2-3 exchanges for context)
	maxMessages := 4
	if len(messages) > maxMessages {
		messages = messages[:maxMessages]
	}
	titlePrompt = append(titlePrompt, BuildMessages(messages)...)

	// Make a non-streaming completion request
	completion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages:    titlePrompt,
		Model:       openai.ChatModel(c.model),
		MaxTokens:   openai.Int(20), // Short title
		Temperature: openai.Float(0.7),
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate title: %w", err)
	}

	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("no title generated")
	}

	title := completion.Choices[0].Message.Content

	// Trim quotes if present
	if len(title) > 0 && title[0] == '"' && title[len(title)-1] == '"' {
		title = title[1 : len(title)-1]
	}

	return title, nil
}
