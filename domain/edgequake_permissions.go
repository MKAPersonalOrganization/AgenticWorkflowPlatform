package domain

// This file re-exports shared kernel types for backward compatibility.
// New code should import "school-with-ai/shared" directly.

import "school-with-ai/shared"

// TenantContext is a type alias to the shared kernel TenantContext.
type TenantContext = shared.TenantContext

// EdgequakeRole is a type alias to the shared kernel EdgequakeRole.
type EdgequakeRole = shared.EdgequakeRole

// Role constants re-exported from shared.
const (
	RoleAdmin    = shared.RoleAdmin
	RoleUser     = shared.RoleUser
	RoleReadonly = shared.RoleReadonly
)
