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
