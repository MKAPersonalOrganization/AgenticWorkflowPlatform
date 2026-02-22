package tenants

import "github.com/google/uuid"

// CreateTenantRequest represents the payload to create a new tenant.
type CreateTenantRequest struct {
	Name                      string  `json:"name"`
	Slug                      *string `json:"slug,omitempty"`
	Description               *string `json:"description,omitempty"`
	Plan                      *string `json:"plan,omitempty"`
	DefaultLLMModel           *string `json:"default_llm_model,omitempty"`
	DefaultLLMProvider        *string `json:"default_llm_provider,omitempty"`
	DefaultEmbeddingModel     *string `json:"default_embedding_model,omitempty"`
	DefaultEmbeddingProvider  *string `json:"default_embedding_provider,omitempty"`
	DefaultEmbeddingDimension *int    `json:"default_embedding_dimension,omitempty"`
}

// UpdateTenantRequest represents the payload to update an existing tenant.
type UpdateTenantRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Plan        *string `json:"plan,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

// TenantResponse represents a single tenant returned by the API.
type TenantResponse struct {
	ID                        uuid.UUID `json:"id"`
	Name                      string    `json:"name"`
	Slug                      string    `json:"slug"`
	Plan                      string    `json:"plan"`
	IsActive                  bool      `json:"is_active"`
	MaxWorkspaces             int       `json:"max_workspaces"`
	DefaultLLMModel           string    `json:"default_llm_model"`
	DefaultLLMProvider        string    `json:"default_llm_provider"`
	DefaultLLMFullID          string    `json:"default_llm_full_id"`
	DefaultEmbeddingModel     string    `json:"default_embedding_model"`
	DefaultEmbeddingProvider  string    `json:"default_embedding_provider"`
	DefaultEmbeddingDimension int       `json:"default_embedding_dimension"`
	DefaultEmbeddingFullID    string    `json:"default_embedding_full_id"`
	CreatedAt                 string    `json:"created_at"`
	UpdatedAt                 string    `json:"updated_at"`
}

// TenantListResponse represents a paginated list of tenants.
type TenantListResponse struct {
	Items  []TenantResponse `json:"items"`
	Total  int              `json:"total"`
	Offset int              `json:"offset"`
	Limit  int              `json:"limit"`
}
