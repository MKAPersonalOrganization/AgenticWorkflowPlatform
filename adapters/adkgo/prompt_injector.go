package adkgo

import (
	"aicompass/foundation/auth"
	"context"
	"fmt"
	"strings"
)

// WorkspacePromptPermissions lists enabled knowledge-base workspaces for an agent.
type WorkspacePromptPermissions interface {
	GetEnabledWorkspaces(ctx context.Context, userID, agentName string) ([]string, error)
}

// KnowledgeBasePromptInjector injects knowledge-base instructions into prompts.
type KnowledgeBasePromptInjector struct {
	workspaceSvc WorkspacePromptPermissions
}

// NewKnowledgeBasePromptInjector creates a prompt injector for KB-enabled agents.
func NewKnowledgeBasePromptInjector(workspaceSvc WorkspacePromptPermissions) *KnowledgeBasePromptInjector {
	return &KnowledgeBasePromptInjector{workspaceSvc: workspaceSvc}
}

func (k *KnowledgeBasePromptInjector) BuildPromptAddition(ctx context.Context, agentName, _ string) string {
	user := auth.GetUserFromContext(ctx)
	if user == nil {
		return ""
	}
	enabledWorkspaceIDs, err := k.workspaceSvc.GetEnabledWorkspaces(ctx, user.ID, agentName)
	if err != nil || len(enabledWorkspaceIDs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Knowledge Bases\n\nYou have access to the following knowledge base workspaces. Use the query_knowledge_base tool to search them:\n")
	for _, wsID := range enabledWorkspaceIDs {
		fmt.Fprintf(&b, "- Workspace ID: %s\n", wsID)
	}
	b.WriteString("\nWhen the user asks about topics that may be covered by these knowledge bases, use the query_knowledge_base tool with the appropriate namespace (workspace ID) and a relevant query to retrieve information.")
	return b.String()
}
