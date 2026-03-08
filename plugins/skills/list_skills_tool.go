package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"school-with-ai/foundation/agent"
)

// listSkillsSchema is the JSON schema for the list_skills tool.
var listSkillsSchema = &agent.Schema{
	Type: agent.TypeObject,
	Description: `Lists all skills available to the current agent. Use this tool to discover which skills you can activate before calling activate_skill.

Use this tool when:
- You want to know what specialized skills are available
- The user asks what skills or capabilities the agent has
- You need to find the correct skill name before activating it
- You want to check if a particular domain skill exists

Returns a list of skill names, descriptions, and categories.`,
	Properties: map[string]*agent.Schema{
		"category": {
			Type:        agent.TypeString,
			Description: "Optional category filter. If provided, only skills in this category are returned. Leave empty to list all skills.",
		},
	},
}

// ListSkillsTool implements agent.ExecutableTool for discovering available skills.
type ListSkillsTool struct {
	resolver SkillResolver
}

func (t *ListSkillsTool) Name() string          { return "list_skills" }
func (t *ListSkillsTool) Description() string   { return listSkillsSchema.Description }
func (t *ListSkillsTool) Schema() *agent.Schema { return listSkillsSchema }

// listSkillEntry represents a single skill in the list response.
type listSkillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Icon        string `json:"icon,omitempty"`
	IsCustom    bool   `json:"is_custom"`
}

// listSkillsResult is the structured result returned by list_skills.
type listSkillsResult struct {
	Status string           `json:"status"`
	Count  int              `json:"count"`
	Skills []listSkillEntry `json:"skills"`
}

// listSkillsArgs is the input for the list_skills tool.
type listSkillsArgs struct {
	Category string `json:"category"`
}

func (t *ListSkillsTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	if t.resolver == nil {
		return agent.ErrorResult(fmt.Errorf("skill system is not configured")), nil
	}

	var parsed listSkillsArgs
	// Ignore decode errors — all fields are optional.
	_ = args.Decode(&parsed)
	categoryFilter := strings.TrimSpace(parsed.Category)

	userID := args.UserID
	agentID := ""
	if args.Metadata != nil {
		if aid, ok := args.Metadata["agent_id"].(string); ok {
			agentID = aid
		}
	}

	if agentID == "" {
		return agent.ErrorResult(fmt.Errorf("agent context is required to list skills")), nil
	}

	skills, err := t.resolver.ListSkillsForAgent(ctx, userID, agentID)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to list skills: %w", err)), nil
	}

	entries := make([]listSkillEntry, 0, len(skills))
	for _, sk := range skills {
		if categoryFilter != "" && !strings.EqualFold(sk.Category, categoryFilter) {
			continue
		}
		entries = append(entries, listSkillEntry{
			Name:        sk.Name,
			Description: sk.Description,
			Category:    sk.Category,
			Icon:        sk.Icon,
			IsCustom:    sk.IsCustom,
		})
	}

	result := listSkillsResult{
		Status: "ok",
		Count:  len(entries),
		Skills: entries,
	}

	resultJSON, _ := json.Marshal(result)
	return agent.Result{Content: []any{string(resultJSON)}}, nil
}
