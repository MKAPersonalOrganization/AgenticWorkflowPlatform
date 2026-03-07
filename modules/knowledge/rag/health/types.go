package health

// HealthResponse matches the Rust HealthResponse exactly.
type HealthResponse struct {
	Status            string           `json:"status"`
	Version           string           `json:"version"`
	BuildInfo         *BuildInfo       `json:"build_info,omitempty"`
	StorageMode       string           `json:"storage_mode"`
	WorkspaceID       string           `json:"workspace_id"`
	Components        ComponentHealth  `json:"components"`
	LLMProviderName   *string          `json:"llm_provider_name,omitempty"`
	Schema            *SchemaHealth    `json:"schema,omitempty"`
	Providers         *ProvidersHealth `json:"providers,omitempty"`
	PDFStorageEnabled *bool            `json:"pdf_storage_enabled,omitempty"`
}

// BuildInfo matches the Rust BuildInfo exactly.
type BuildInfo struct {
	GitHash        string `json:"git_hash"`
	GitBranch      string `json:"git_branch"`
	BuildTimestamp string `json:"build_timestamp"`
	BuildNumber    string `json:"build_number"`
}

// SchemaHealth matches the Rust SchemaHealth exactly.
type SchemaHealth struct {
	LatestVersion     *int64  `json:"latest_version"`
	MigrationsApplied int     `json:"migrations_applied"`
	LastAppliedAt     *string `json:"last_applied_at,omitempty"`
}

// ComponentHealth matches the Rust ComponentHealth exactly.
type ComponentHealth struct {
	KVStorage     bool `json:"kv_storage"`
	VectorStorage bool `json:"vector_storage"`
	GraphStorage  bool `json:"graph_storage"`
	LLMProvider   bool `json:"llm_provider"`
}

// LlmProviderHealth matches the Rust LlmProviderHealth exactly.
type LlmProviderHealth struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

// EmbeddingProviderHealth matches the Rust EmbeddingProviderHealth exactly.
type EmbeddingProviderHealth struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	Dimension int    `json:"dimension"`
}

// ProvidersHealth matches the Rust ProvidersHealth exactly.
type ProvidersHealth struct {
	LLM       LlmProviderHealth       `json:"llm"`
	Embedding EmbeddingProviderHealth `json:"embedding"`
}
