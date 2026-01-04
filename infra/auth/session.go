package auth

import (
	"context"

	"school-with-ai/domain"

	client "github.com/ory/client-go"
)

// GetUserFromContext retrieves the authenticated user from the request context
func GetUserFromContext(ctx context.Context) *domain.User {
	if user, ok := ctx.Value(UserContextKey).(*domain.User); ok {
		return user
	}
	return nil
}

// GetSessionFromContext retrieves the Kratos session from the request context
func GetSessionFromContext(ctx context.Context) *client.Session {
	if session, ok := ctx.Value(SessionContextKey).(*client.Session); ok {
		return session
	}
	return nil
}

// IsAuthenticated checks if the context has an authenticated user
func IsAuthenticated(ctx context.Context) bool {
	return GetUserFromContext(ctx) != nil
}
