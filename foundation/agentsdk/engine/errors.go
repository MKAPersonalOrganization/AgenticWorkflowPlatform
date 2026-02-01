package engine

import "errors"

// ErrNoAgentSelected is returned when no target agent can be determined for a request.
var ErrNoAgentSelected = errors.New("no target agent determined and no sticky session found")
