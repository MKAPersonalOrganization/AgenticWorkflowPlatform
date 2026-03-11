package chatcontracts

import "context"

// ToolPermissionService abstracts the subset of the permissions module
// that the chat module requires for tool-set and individual-tool toggles.
// The concrete *tool.Service satisfies this interface implicitly.
type ToolPermissionService interface {
	// GetEnabledToolSets returns enabled tool-set IDs for the given user/agent.
	GetEnabledToolSets(ctx context.Context, userID, agentID string) ([]string, error)

	// HasPermission checks whether a specific tool is enabled.
	HasPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) bool

	// SetToolSetPermission enables or disables a tool set for an agent.
	SetToolSetPermission(ctx context.Context, userID, agentID, toolSetID string, enabled bool) error

	// SetPermission enables or disables an individual tool within a tool set.
	SetPermission(ctx context.Context, userID, agentID, toolSetID, toolID string, enabled bool) error
}

// StudentProfile represents the extended academic profile of a student.
// This is a local DTO mirroring identity.StudentProfile so that the chat
// module does not import modules/userprofile/identity directly.
type StudentProfile struct {
	UserID        string   `json:"user_id"`
	LearningStyle string   `json:"learning_style"`
	Struggles     []string `json:"struggles"`
	Strengths     []string `json:"strengths"`
	Goals         string   `json:"goals"`
}

// StudentProfileRepository reads student profiles.
// The concrete identity.StudentProfileRepository satisfies this interface
// with a thin adapter that maps identity.StudentProfile → chatcontracts.StudentProfile.
type StudentProfileRepository interface {
	GetByUserID(ctx context.Context, userID string) (*StudentProfile, error)
}
