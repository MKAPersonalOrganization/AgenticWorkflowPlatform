package skillstore

import (
	"context"
	"fmt"

	"school-with-ai/foundation/logger"
)

// maxSkillsPerUser is the maximum number of skills a user can have in their library.
const maxSkillsPerUser = 50

// service manages user skills, catalog imports, and skill-agent permissions.
type service struct {
	log    *logger.Logger
	skills UserSkillRepository
	perms  SkillPermissionRepository
}

func newService(log *logger.Logger, skills UserSkillRepository, perms SkillPermissionRepository) *service {
	return &service{
		log:    log,
		skills: skills,
		perms:  perms,
	}
}

// ─── User Skill CRUD ────────────────────────────────────────────────────────

func (s *service) create(ctx context.Context, skill *UserSkill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if skill.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if skill.Instructions == "" {
		return fmt.Errorf("skill instructions are required")
	}

	// Enforce per-user skill limit
	count, err := s.skills.CountByUser(ctx, skill.UserID)
	if err != nil {
		return fmt.Errorf("checking skill count: %w", err)
	}
	if count >= maxSkillsPerUser {
		return fmt.Errorf("skill limit reached: maximum %d skills per user", maxSkillsPerUser)
	}

	s.log.Info(ctx, "creating user skill", "userID", skill.UserID, "name", skill.Name)

	if err := s.skills.Create(ctx, skill); err != nil {
		return fmt.Errorf("creating skill: %w", err)
	}

	return nil
}

func (s *service) update(ctx context.Context, skill *UserSkill) error {
	if skill.ID == "" {
		return fmt.Errorf("skill ID is required")
	}
	if skill.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if skill.Instructions == "" {
		return fmt.Errorf("skill instructions are required")
	}

	s.log.Info(ctx, "updating user skill", "skillID", skill.ID, "userID", skill.UserID)

	if err := s.skills.Update(ctx, skill); err != nil {
		return fmt.Errorf("updating skill: %w", err)
	}

	return nil
}

func (s *service) delete(ctx context.Context, id, userID string) error {
	s.log.Info(ctx, "deleting user skill", "skillID", id, "userID", userID)

	if err := s.skills.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("deleting skill: %w", err)
	}

	return nil
}

func (s *service) getByID(ctx context.Context, id, userID string) (*UserSkill, error) {
	return s.skills.GetByID(ctx, id, userID)
}

func (s *service) listByUser(ctx context.Context, userID string) ([]UserSkill, error) {
	return s.skills.ListByUser(ctx, userID)
}

func (s *service) search(ctx context.Context, userID, query string) ([]UserSkill, error) {
	return s.skills.Search(ctx, userID, query)
}

// ─── Skill-Agent Permissions ────────────────────────────────────────────────

func (s *service) grantPermission(ctx context.Context, userID, agentID, skillID string) error {
	// Verify skill exists and belongs to user
	_, err := s.skills.GetByID(ctx, skillID, userID)
	if err != nil {
		return fmt.Errorf("skill not found: %w", err)
	}

	s.log.Info(ctx, "granting skill permission", "userID", userID, "agentID", agentID, "skillID", skillID)

	perm := &SkillAgentPermission{
		UserID:  userID,
		AgentID: agentID,
		SkillID: skillID,
		Enabled: true,
	}
	return s.perms.Grant(ctx, perm)
}

func (s *service) revokePermission(ctx context.Context, userID, agentID, skillID string) error {
	s.log.Info(ctx, "revoking skill permission", "userID", userID, "agentID", agentID, "skillID", skillID)
	return s.perms.Revoke(ctx, userID, agentID, skillID)
}

func (s *service) listPermissionsBySkill(ctx context.Context, userID, skillID string) ([]SkillAgentPermission, error) {
	return s.perms.ListBySkill(ctx, userID, skillID)
}

func (s *service) listPermissionsByAgent(ctx context.Context, userID, agentID string) ([]SkillAgentPermission, error) {
	return s.perms.ListByAgent(ctx, userID, agentID)
}

// listSkillsForAgent returns the full UserSkill objects that an agent is permitted to use.
// Used by the activate_skill tool at runtime.
func (s *service) listSkillsForAgent(ctx context.Context, userID, agentID string) ([]UserSkill, error) {
	skillIDs, err := s.perms.ListGrantedSkillIDs(ctx, userID, agentID)
	if err != nil {
		return nil, fmt.Errorf("listing granted skill IDs: %w", err)
	}

	if len(skillIDs) == 0 {
		return nil, nil
	}

	// Fetch full skill details for each granted skill
	var skills []UserSkill
	for _, sid := range skillIDs {
		skill, err := s.skills.GetByID(ctx, sid, userID)
		if err != nil {
			// Skill may have been deleted after permission was granted; skip it
			continue
		}
		skills = append(skills, *skill)
	}

	return skills, nil
}

// getSkillByNameForAgent looks up a skill by name from the set of skills
// the agent is permitted to use. Used by the activate_skill tool.
func (s *service) getSkillByNameForAgent(ctx context.Context, userID, agentID, skillName string) (*UserSkill, error) {
	skills, err := s.listSkillsForAgent(ctx, userID, agentID)
	if err != nil {
		return nil, err
	}

	for i := range skills {
		if skills[i].Name == skillName {
			return &skills[i], nil
		}
	}

	return nil, fmt.Errorf("skill %q not found or not permitted for this agent", skillName)
}
