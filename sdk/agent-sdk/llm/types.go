package llm

// ChatRole represents the role of a message participant in a conversation.
// This mirrors Microsoft.Extensions.AI.ChatRole from the .NET agent framework.
type ChatRole string

const (
	// ChatRoleUser represents a message from the user/human.
	ChatRoleUser ChatRole = "user"

	// ChatRoleAssistant represents a message from the AI assistant.
	ChatRoleAssistant ChatRole = "assistant"

	// ChatRoleSystem represents a system message (instructions, context).
	ChatRoleSystem ChatRole = "system"

	// ChatRoleTool represents a tool/function result message.
	ChatRoleTool ChatRole = "tool"
)

// String returns the string representation of the ChatRole.
func (r ChatRole) String() string {
	return string(r)
}

// IsValid returns true if the role is a known valid role.
func (r ChatRole) IsValid() bool {
	switch r {
	case ChatRoleUser, ChatRoleAssistant, ChatRoleSystem, ChatRoleTool:
		return true
	default:
		return false
	}
}
