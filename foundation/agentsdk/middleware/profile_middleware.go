package middleware

import (
	"fmt"
	"strings"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/userprofile/identity"
	"school-with-ai/shared"
)

// NewStudentProfileInjectorMiddleware returns a Chat-level Middleware that
// prepends the student's profile as a system message before each LLM call.
func NewStudentProfileInjectorMiddleware(repo identity.StudentProfileRepository) agent.Middleware {
	return func(next agent.Handler) agent.Handler {
		return agent.HandlerFunc(func(c *agent.Context) error {
			if !c.IsChat() {
				return next.Handle(c)
			}

			user := shared.GetUserFromContext(c.Context)
			if user == nil {
				return next.Handle(c)
			}

			profile, err := repo.GetByUserID(c.Context, user.ID)
			if err != nil {
				// Proceed without profile on error.
				return next.Handle(c)
			}

			if profile == nil {
				// Use basic user details when no profile exists.
				if user.HasName() {
					msg := agent.Message{
						Role:    agent.RoleSystem,
						Content: []agent.Content{{Type: agent.ContentTypeText, Text: fmt.Sprintf("# Student Profile\n- **Name:** %s", user.GetFullName())}},
					}
					c.Chat.Messages = append([]agent.Message{msg}, c.Chat.Messages...)
				}
				return next.Handle(c)
			}

			// Build Markdown representation
			var sb strings.Builder
			sb.WriteString("# Student Profile\n")

			name := user.GetFullName()
			if name == "" {
				name = "Student"
			}
			sb.WriteString(fmt.Sprintf("- **Name:** %s\n", name))

			if profile.LearningStyle != "" {
				sb.WriteString(fmt.Sprintf("- **Learning Style:** %s\n", profile.LearningStyle))
			}

			if len(profile.Struggles) > 0 {
				sb.WriteString(fmt.Sprintf("- **Current Struggles:** %s\n", strings.Join(profile.Struggles, ", ")))
			}

			if len(profile.Strengths) > 0 {
				sb.WriteString(fmt.Sprintf("- **Strengths:** %s\n", strings.Join(profile.Strengths, ", ")))
			}

			if profile.Goals != "" {
				sb.WriteString(fmt.Sprintf("- **Goals:** %s\n", profile.Goals))
			}

			msg := agent.Message{
				Role:    agent.RoleSystem,
				Content: []agent.Content{{Type: agent.ContentTypeText, Text: sb.String()}},
			}
			c.Chat.Messages = append([]agent.Message{msg}, c.Chat.Messages...)

			return next.Handle(c)
		})
	}
}
