package openai

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
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
			params = append(params, openai.AssistantMessage(msg.Content))
		case "system":
			params = append(params, openai.SystemMessage(msg.Content))
		}
	}

	return params
}

// CreateStreamingCompletion creates a streaming chat completion
func (c *Client) CreateStreamingCompletion(ctx context.Context, messages []session.Message) *ssestream.Stream[openai.ChatCompletionChunk] {
	params := BuildMessages(messages)

	stream := c.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Messages: params,
		Model:    openai.ChatModel(c.model),
	})

	return stream
}
