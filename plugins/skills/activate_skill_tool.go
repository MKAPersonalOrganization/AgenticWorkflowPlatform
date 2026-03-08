package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/skillstore"
)

// activateSkillSchema is the JSON schema for the activate_skill tool.
// The description tells the LLM how and when to use it.
var activateSkillSchema = &agent.Schema{
	Type: agent.TypeObject,
	Description: `Activates a specialized agent skill by name. When activated, the skill's instructions are loaded into your context, giving you specialized expertise for a particular domain or task.

Use this tool when:
- The user asks for help with a specific domain (statistics, creative writing, code review, etc.)
- You need specialized methodology or frameworks for a task
- A skill would significantly improve the quality of your response

After activation, follow the skill's instructions for all subsequent responses in that domain.`,
	Properties: map[string]*agent.Schema{
		"name": {
			Type:        agent.TypeString,
			Description: "The name of the skill to activate. Must match one of the available skills.",
		},
	},
	Required: []string{"name"},
}

// ActivateSkillTool implements agent.ExecutableTool for the activate_skill tool.
// It follows the Gemini CLI pattern: look up a skill by name, return its
// instructions wrapped in XML tags for the LLM to follow.
type ActivateSkillTool struct {
	resolver SkillResolver
}

func (t *ActivateSkillTool) Name() string          { return "activate_skill" }
func (t *ActivateSkillTool) Description() string   { return activateSkillSchema.Description }
func (t *ActivateSkillTool) Schema() *agent.Schema { return activateSkillSchema }

// activateSkillArgs is the input structure for the activate_skill tool.
type activateSkillArgs struct {
	Name string `json:"name"`
}

// activateSkillResult is the structured result returned to both the LLM and the UI.
type activateSkillResult struct {
	Status       string `json:"status"`
	SkillName    string `json:"skill_name"`
	Category     string `json:"category"`
	Description  string `json:"description"`
	Instructions string `json:"instructions"`
	Message      string `json:"message"`
}

func (t *ActivateSkillTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var parsed activateSkillArgs
	if err := args.Decode(&parsed); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
	}

	skillName := strings.TrimSpace(parsed.Name)
	if skillName == "" {
		return agent.ErrorResult(fmt.Errorf("skill name is required")), nil
	}

	// If no resolver is configured, return a helpful error
	if t.resolver == nil {
		return agent.ErrorResult(fmt.Errorf("skill system is not configured")), nil
	}

	// Look up the skill by name from the user's permitted skills for this agent
	userID := args.UserID
	agentID := "" // Will be populated from session metadata if available
	if args.Metadata != nil {
		if aid, ok := args.Metadata["agent_id"].(string); ok {
			agentID = aid
		}
	}

	if agentID == "" {
		return agent.ErrorResult(fmt.Errorf("agent context is required to activate skills")), nil
	}

	// Look up from agent-permitted skills (exact match first)
	s, err := t.resolver.GetSkillByNameForAgent(ctx, userID, agentID, skillName)
	if err == nil && s != nil {
		result := buildActivationResult(s)
		resultJSON, _ := json.Marshal(result)
		return agent.Result{Content: []any{string(resultJSON)}}, nil
	}

	// Try a case-insensitive search across all agent skills
	skills, listErr := t.resolver.ListSkillsForAgent(ctx, userID, agentID)
	if listErr != nil {
		return agent.ErrorResult(fmt.Errorf("failed to look up skills: %w", listErr)), nil
	}

	for i := range skills {
		if strings.EqualFold(skills[i].Name, skillName) {
			result := buildActivationResult(&skills[i])
			resultJSON, _ := json.Marshal(result)
			return agent.Result{Content: []any{string(resultJSON)}}, nil
		}
	}

	// Build available skills list for the error message
	var available []string
	for _, sk := range skills {
		available = append(available, sk.Name)
	}
	if len(available) == 0 {
		return agent.ErrorResult(fmt.Errorf(
			"skill %q not found. No skills are available for this agent. Ask the user to assign skills to this agent first.",
			skillName,
		)), nil
	}
	return agent.ErrorResult(fmt.Errorf(
		"skill %q not found. Available skills: %s",
		skillName, strings.Join(available, ", "),
	)), nil
}

// buildActivationResult creates the structured result following the Gemini CLI
// pattern of wrapping skill instructions in XML tags.
func buildActivationResult(s *skillstore.UserSkill) activateSkillResult {
	// Build the XML-wrapped instructions (Gemini CLI pattern)
	xmlInstructions := fmt.Sprintf(
		"<activated_skill name=%q>\n<instructions>\n%s\n</instructions>\n</activated_skill>",
		s.Name, s.Instructions,
	)

	return activateSkillResult{
		Status:       "activated",
		SkillName:    s.Name,
		Category:     s.Category,
		Description:  s.Description,
		Instructions: xmlInstructions,
		Message:      fmt.Sprintf("Skill %q activated. Follow the instructions above for all subsequent responses in this domain.", s.Name),
	}
}
