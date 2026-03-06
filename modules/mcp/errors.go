package mcp

import "errors"

// Sentinel errors for the MCP connector subsystem.
// Handlers use errors.Is() to map these to HTTP status codes, following the
// same pattern as other services in the codebase (rag/documents, artifact, etc.)
// and aligned with agentstack's typed exception hierarchy.
var (
	// ErrConnectorNotFound is returned when a connector ID does not exist or
	// does not belong to the requesting user.
	ErrConnectorNotFound = errors.New("mcp: connector not found")

	// ErrDuplicateConnector is returned when a user tries to register a
	// server URL that they already have. Mirrors agentstack's DuplicateEntityError.
	ErrDuplicateConnector = errors.New("mcp: connector already exists for this URL")

	// ErrConnectorLimitReached is returned when the per-user connector cap
	// has been hit.
	ErrConnectorLimitReached = errors.New("mcp: connector limit reached")

	// ErrInvalidConnectorState is returned when an operation is attempted on
	// a connector that is not in a valid state for that operation. Mirrors
	// agentstack's state validation before disconnect/connect.
	ErrInvalidConnectorState = errors.New("mcp: invalid connector state for this operation")

	// ErrConnectorNotConnected is returned when a tool call or refresh is
	// attempted on a connector without an active MCP session.
	ErrConnectorNotConnected = errors.New("mcp: connector is not connected")

	// ErrOAuthStateInvalid is returned when an OAuth callback carries an
	// unknown or expired state parameter.
	ErrOAuthStateInvalid = errors.New("mcp: invalid or expired OAuth state")
)
