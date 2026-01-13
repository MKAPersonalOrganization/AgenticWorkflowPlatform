package kratos

// This file re-exports session functions from domain for backward compatibility
// New code should use domain.GetUserFromContext(), domain.GetSessionFromContext(), etc.

import (
	"school-with-ai/domain"
)

// GetUserFromContext retrieves the authenticated user from the request context
// Deprecated: Use domain.GetUserFromContext instead.
var GetUserFromContext = domain.GetUserFromContext

// GetSessionFromContext retrieves the Kratos session from the request context
// Deprecated: Use domain.GetSessionFromContext instead.
var GetSessionFromContext = domain.GetSessionFromContext

// IsAuthenticated checks if the context has an authenticated user
// Deprecated: Use domain.IsAuthenticated instead.
var IsAuthenticated = domain.IsAuthenticated
