package custom

import (
	"context"

	"school-with-ai/modules/chat/chatcontracts"
	"school-with-ai/modules/userprofile/identity"
)

// ChatProfileAdapter bridges identity.StudentProfileRepository to the
// chatcontracts.StudentProfileRepository interface so the chat module
// does not import modules/userprofile/identity directly.
type ChatProfileAdapter struct {
	Repo identity.StudentProfileRepository
}

// GetByUserID loads the profile from identity and maps it to the chat DTO.
func (a *ChatProfileAdapter) GetByUserID(ctx context.Context, userID string) (*chatcontracts.StudentProfile, error) {
	p, err := a.Repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &chatcontracts.StudentProfile{
		UserID:        p.UserID,
		LearningStyle: p.LearningStyle,
		Struggles:     p.Struggles,
		Strengths:     p.Strengths,
		Goals:         p.Goals,
	}, nil
}
