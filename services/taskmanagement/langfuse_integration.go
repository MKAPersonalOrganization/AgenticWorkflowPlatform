package taskmanagement

import "school-with-ai/foundation/observability/langfuse"

// LangfuseConfig holds configuration for Langfuse client
type LangfuseConfig struct {
	BaseURL   string
	PublicKey string
	SecretKey string
}

// LangfuseTracer wraps the Langfuse tracer for use in orchestration
type LangfuseTracer struct {
	tracer *langfuse.Tracer
}

// NewLangfuseTracer creates a new Langfuse tracer
func NewLangfuseTracer(cfg LangfuseConfig) *LangfuseTracer {
	client := langfuse.NewClient(langfuse.Config{
		BaseURL:   cfg.BaseURL,
		PublicKey: cfg.PublicKey,
		SecretKey: cfg.SecretKey,
	})

	return &LangfuseTracer{
		tracer: langfuse.NewTracer(client),
	}
}

// GetTracer returns the underlying Langfuse tracer
func (lt *LangfuseTracer) GetTracer() *langfuse.Tracer {
	if lt == nil {
		return nil
	}
	return lt.tracer
}
