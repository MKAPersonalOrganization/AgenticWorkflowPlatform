package tenants

import (
	"time"

	"github.com/google/uuid"
)

// CreateTenantRequest represents the payload to create a new tenant.
type CreateTenantRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug,omitempty"`
}

// UpdateTenantRequest represents the payload to update an existing tenant.
type UpdateTenantRequest struct {
	Name *string `json:"name,omitempty"`
	Slug *string `json:"slug,omitempty"`
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
	DefaultEmbeddingModel     string    `json:"default_embedding_model"`
	DefaultEmbeddingProvider  string    `json:"default_embedding_provider"`
	DefaultEmbeddingDimension int       `json:"default_embedding_dimension"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

// TenantListResponse represents a paginated list of tenants.
type TenantListResponse struct {
	Items      []TenantResponse `json:"items"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
}
