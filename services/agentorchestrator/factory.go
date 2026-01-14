package agentorchestrator

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/services"
)

// Factory implements services.AgentFactory.
type Factory struct {
	log        *logger.Logger
	chatClient ChatClient
	writeModel core.EventStore
}

// NewFactory creates a new AgentFactory.
func NewFactory(log *logger.Logger, chatClient ChatClient, writeModel core.EventStore) *Factory {
	return &Factory{
		log:        log,
		chatClient: chatClient,
		writeModel: writeModel,
	}
}

// Create creates a new AgentOrchestrator with the given configuration and options.
func (f *Factory) Create(config core.AgentConfig, reqCtxCapture **core.RequestContext, toolTracer core.ToolTracer) services.AgentOrchestrator {
	opts := []Option{
		WithConfig(config),
		WithRequestContextCapture(reqCtxCapture),
	}
	if toolTracer != nil {
		opts = append(opts, WithToolTracer(toolTracer))
	}

	return New(f.log, f.chatClient, f.writeModel, opts...)
}
