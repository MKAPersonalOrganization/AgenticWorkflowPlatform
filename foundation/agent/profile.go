package agent

import "context"

// StudentProfile contains the academic profile data needed by the agent
// middleware layer to enrich LLM context. This is a foundation-level DTO —
// it contains only the fields the agent SDK needs, decoupled from the
// module-level identity.StudentProfile type.
//
// See TODO-2 in MODULARITY_HANDOFF.md.
type StudentProfile struct {
	UserID        string
	LearningStyle string
	Struggles     []string
	Strengths     []string
	Goals         string
}

// StudentProfileProvider abstracts access to student profiles so that
// foundation-layer middleware does not import module types.
// The concrete implementation lives in modules/userprofile/identity and is
// wired via the composition root (main.go / adapters/custom/).
type StudentProfileProvider interface {
	GetStudentProfile(ctx context.Context, userID string) (*StudentProfile, error)
}
