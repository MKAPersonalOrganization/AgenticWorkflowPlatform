package a2a

import "errors"

// Sentinel errors for the A2A connector subsystem.
// Handlers use errors.Is() to map these to HTTP status codes, following the
// same pattern as the MCP module and agentstack's typed exception hierarchy.
var (
	// ErrConnectorNotFound is returned when a connector ID does not exist or
	// does not belong to the requesting user.
	ErrConnectorNotFound = errors.New("a2a: connector not found")

	// ErrDuplicateConnector is returned when a user tries to register an
	// agent URL that they already have.
	ErrDuplicateConnector = errors.New("a2a: connector already exists for this URL")

	// ErrConnectorLimitReached is returned when the per-user connector cap
	// has been hit.
	ErrConnectorLimitReached = errors.New("a2a: connector limit reached")

	// ErrInvalidConnectorState is returned when an operation is attempted on
	// a connector that is not in a valid state for that operation.
	ErrInvalidConnectorState = errors.New("a2a: invalid connector state for this operation")

	// ErrConnectorNotConnected is returned when a skill invocation is
	// attempted on a connector without a connected status.
	ErrConnectorNotConnected = errors.New("a2a: connector is not connected")

	// ErrAgentCardNotFound is returned when the agent card cannot be
	// discovered at the well-known URL.
	ErrAgentCardNotFound = errors.New("a2a: agent card not found at endpoint")

	// ErrAgentCardInvalid is returned when the agent card fails validation.
	ErrAgentCardInvalid = errors.New("a2a: agent card is invalid")

	// ErrTaskFailed is returned when the remote agent's task enters a
	// terminal failure state.
	ErrTaskFailed = errors.New("a2a: remote agent task failed")

	// ErrTaskTimeout is returned when waiting for a remote task response
	// exceeds the configured timeout.
	ErrTaskTimeout = errors.New("a2a: remote agent task timed out")
)
