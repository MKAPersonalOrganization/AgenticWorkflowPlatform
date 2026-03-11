package custom

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/userprofile/identity"
)

// StudentProfileAdapter bridges the module-level identity.StudentProfileRepository
// to the foundation-level agent.StudentProfileProvider interface.
//
// This adapter lives in adapters/custom/ (the composition layer) because it is
// explicitly allowed to import both foundation/ and modules/. The profile
// middleware in foundation/agentsdk/middleware/ now depends only on the
// agent.StudentProfileProvider interface, keeping foundation/ free of module imports.
//
// See TODO-2 in MODULARITY_HANDOFF.md.
type StudentProfileAdapter struct {
	Repo identity.StudentProfileRepository
}

// GetStudentProfile implements agent.StudentProfileProvider by delegating to the
// module's repository and converting the module-level type to the foundation-level DTO.
func (a *StudentProfileAdapter) GetStudentProfile(ctx context.Context, userID string) (*agent.StudentProfile, error) {
	profile, err := a.Repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, nil
	}
	return &agent.StudentProfile{
		UserID:        profile.UserID,
		LearningStyle: profile.LearningStyle,
		Struggles:     profile.Struggles,
		Strengths:     profile.Strengths,
		Goals:         profile.Goals,
	}, nil
}
