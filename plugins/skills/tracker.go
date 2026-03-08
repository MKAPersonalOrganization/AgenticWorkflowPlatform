package skills

import (
	"fmt"
	"strings"
	"sync"
)

// ActivatedSkill holds the information for an activated skill within a thread.
type ActivatedSkill struct {
	Name         string
	Instructions string
}

// SkillActivationTracker tracks which skills have been activated per thread.
// It is goroutine-safe and is shared between the activate_skill tool (writer)
// and the agent config builder (reader) to inject activated skill instructions
// into the system prompt.
//
// This is an in-memory structure — activations are lost on server restart.
// That's acceptable because:
//  1. Skills are cheap to re-activate (one tool call)
//  2. Long-lived threads already have the activation in history
//  3. The system prompt injection is a quality optimisation, not a correctness requirement
type SkillActivationTracker struct {
	// mu protects the skills map.
	mu sync.RWMutex
	// skills maps threadID → list of activated skills (order preserved).
	skills map[string][]ActivatedSkill
}

// NewSkillActivationTracker creates a new tracker.
func NewSkillActivationTracker() *SkillActivationTracker {
	return &SkillActivationTracker{
		skills: make(map[string][]ActivatedSkill),
	}
}

// Activate records a skill activation for the given thread.
// If the skill is already activated (by name), it is replaced with the new instructions.
func (t *SkillActivationTracker) Activate(threadID string, skill ActivatedSkill) {
	t.mu.Lock()
	defer t.mu.Unlock()

	existing := t.skills[threadID]
	for i, s := range existing {
		if s.Name == skill.Name {
			existing[i] = skill // replace
			return
		}
	}
	t.skills[threadID] = append(existing, skill)
}

// GetActiveSkills returns all activated skills for the given thread.
// Returns nil if none are activated.
func (t *SkillActivationTracker) GetActiveSkills(threadID string) []ActivatedSkill {
	t.mu.RLock()
	defer t.mu.RUnlock()

	skills := t.skills[threadID]
	if len(skills) == 0 {
		return nil
	}
	// Return a copy to avoid data races.
	out := make([]ActivatedSkill, len(skills))
	copy(out, skills)
	return out
}

// BuildPromptAddition returns a formatted string suitable for appending to
// the system prompt, containing all activated skill instructions for the thread.
// Returns empty string if no skills are activated.
func (t *SkillActivationTracker) BuildPromptAddition(threadID string) string {
	skills := t.GetActiveSkills(threadID)
	if len(skills) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Activated Skills\n\n")
	b.WriteString("The following skills have been activated for this conversation. Follow their instructions carefully.\n\n")
	for _, s := range skills {
		fmt.Fprintf(&b, "<activated_skill name=%q>\n<instructions>\n%s\n</instructions>\n</activated_skill>\n\n", s.Name, s.Instructions)
	}
	return b.String()
}

// Clear removes all activated skills for a thread (e.g. on thread deletion).
func (t *SkillActivationTracker) Clear(threadID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.skills, threadID)
}
