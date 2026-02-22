package auth

// LoginRequest represents the payload to login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the login response with tokens.
type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int64    `json:"expires_in"`
	RefreshToken string   `json:"refresh_token"`
	User         UserInfo `json:"user"`
}

// UserInfo represents user information safe for API responses.
type UserInfo struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// RefreshTokenRequest represents the payload to refresh a token.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenResponse represents the refresh token response.
type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

// GetMeResponse represents the current user info response.
type GetMeResponse struct {
	User UserInfo `json:"user"`
}

// CreateApiKeyRequest represents the payload to create an API key.
type CreateApiKeyRequest struct {
	Name      *string  `json:"name,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
	ExpiresIn *string  `json:"expires_in,omitempty"`
}

// CreateApiKeyResponse represents the response from creating an API key.
type CreateApiKeyResponse struct {
	KeyID     string   `json:"key_id"`
	ApiKey    string   `json:"api_key"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt *string  `json:"expires_at,omitempty"`
	CreatedAt string   `json:"created_at"`
}

// ApiKeySummary represents a summary of an API key (without the secret).
type ApiKeySummary struct {
	KeyID      string   `json:"key_id"`
	Name       string   `json:"name"`
	Prefix     *string  `json:"prefix,omitempty"`
	Scopes     []string `json:"scopes"`
	IsActive   bool     `json:"is_active"`
	LastUsedAt *string  `json:"last_used_at,omitempty"`
	ExpiresAt  *string  `json:"expires_at,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

// ListApiKeysResponse represents a paginated list of API keys.
type ListApiKeysResponse struct {
	Keys       []ApiKeySummary `json:"keys"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalPages int             `json:"total_pages"`
}

// RevokeApiKeyResponse represents the response from revoking an API key.
type RevokeApiKeyResponse struct {
	KeyID   string `json:"key_id"`
	Message string `json:"message"`
}

// CreateUserRequest represents the payload to create a new user.
type CreateUserRequest struct {
	Username string  `json:"username"`
	Email    string  `json:"email"`
	Password string  `json:"password"`
	Role     *string `json:"role,omitempty"`
}

// CreateUserResponse represents the create user response.
type CreateUserResponse struct {
	User      UserInfo `json:"user"`
	CreatedAt string   `json:"created_at"`
}

// ListUsersResponse represents a paginated list of users.
type ListUsersResponse struct {
	Users      []UserInfo `json:"users"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
}
