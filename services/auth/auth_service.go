package auth

import (
	"context"
	"school-with-ai/shared"
)

// authService implements the services.AuthService interface.
type authService struct{}

// NewAuthService creates a new authentication service.
func NewAuthService() *authService {
	return &authService{}
}

// GetUserFromContext extracts authenticated user from request context.
func (s *authService) GetUserFromContext(ctx context.Context) *shared.User {
	return shared.GetUserFromContext(ctx)
}

// IsAuthenticated checks if request context has valid authentication.
func (s *authService) IsAuthenticated(ctx context.Context) bool {
	return s.GetUserFromContext(ctx) != nil
}
