package custom

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/userprofile/identity"
)

// UserContextAdapter bridges the module-level identity.StudentProfileRepository
// to the foundation-level agent.UserContextProvider interface.
//
// This adapter lives in adapters/custom/ (the composition layer) because it is
// explicitly allowed to import both foundation/ and modules/. The middleware
// in foundation/agentsdk/middleware/ depends only on agent.UserContextProvider,
// keeping foundation/ free of module imports.
type UserContextAdapter struct {
	Repo identity.StudentProfileRepository
}

// GetUserContext implements agent.UserContextProvider by delegating to the
// module's repository and converting the domain-level type to the generic DTO.
func (a *UserContextAdapter) GetUserContext(ctx context.Context, userID string) (*agent.UserContext, error) {
	profile, err := a.Repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, nil
	}

	attrs := make(map[string]string)
	if profile.LearningStyle != "" {
		attrs["learning_style"] = profile.LearningStyle
	}

	tags := make(map[string][]string)
	if len(profile.Struggles) > 0 {
		tags["struggles"] = profile.Struggles
	}
	if len(profile.Strengths) > 0 {
		tags["strengths"] = profile.Strengths
	}

	return &agent.UserContext{
		UserID:     profile.UserID,
		Attributes: attrs,
		Tags:       tags,
		Goals:      profile.Goals,
	}, nil
}
