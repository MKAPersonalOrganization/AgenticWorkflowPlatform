package custom

import (
	"context"

	"school-with-ai/modules/skillstore"
	skillsplugin "school-with-ai/plugins/skills"
)

// SkillResolverSource is the subset of skillstore.Module that the adapter needs.
type SkillResolverSource interface {
	ListSkillsForAgent(ctx context.Context, userID, agentID string) ([]skillstore.UserSkill, error)
	GetSkillByNameForAgent(ctx context.Context, userID, agentID, skillName string) (*skillstore.UserSkill, error)
}

// SkillResolverAdapter bridges skillstore.Module to the skills plugin's
// SkillResolver interface, converting skillstore.UserSkill → skills.Skill.
type SkillResolverAdapter struct {
	Source SkillResolverSource
}

func (a *SkillResolverAdapter) ListSkillsForAgent(ctx context.Context, userID, agentID string) ([]skillsplugin.Skill, error) {
	raw, err := a.Source.ListSkillsForAgent(ctx, userID, agentID)
	if err != nil {
		return nil, err
	}
	out := make([]skillsplugin.Skill, len(raw))
	for i, s := range raw {
		out[i] = mapSkill(s)
	}
	return out, nil
}

func (a *SkillResolverAdapter) GetSkillByNameForAgent(ctx context.Context, userID, agentID, skillName string) (*skillsplugin.Skill, error) {
	s, err := a.Source.GetSkillByNameForAgent(ctx, userID, agentID, skillName)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}
	out := mapSkill(*s)
	return &out, nil
}

func mapSkill(s skillstore.UserSkill) skillsplugin.Skill {
	return skillsplugin.Skill{
		Name:         s.Name,
		Description:  s.Description,
		Instructions: s.Instructions,
		Category:     s.Category,
		Icon:         s.Icon,
		IsCustom:     s.IsCustom,
	}
}
