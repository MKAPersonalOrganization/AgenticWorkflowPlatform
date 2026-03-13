package middleware

import (
	"fmt"
	"sort"
	"strings"

	"school-with-ai/foundation/agent"
	"school-with-ai/shared"
)

// NewUserContextInjectorMiddleware returns a Chat-level Middleware that
// prepends the user's context as a system message before each LLM call.
//
// It depends on [agent.UserContextProvider], a foundation-level interface,
// rather than importing domain modules directly. The concrete implementation
// is wired in main.go / adapters/custom/.
func NewUserContextInjectorMiddleware(provider agent.UserContextProvider) agent.Middleware {
	return func(next agent.Handler) agent.Handler {
		return agent.HandlerFunc(func(c *agent.Context) error {
			if !c.IsChat() {
				return next.Handle(c)
			}

			user := shared.GetUserFromContext(c.Context)
			if user == nil {
				return next.Handle(c)
			}

			uctx, err := provider.GetUserContext(c.Context, user.ID)
			if err != nil {
				// Proceed without user context on error.
				return next.Handle(c)
			}

			if uctx == nil {
				// Use basic user details when no context exists.
				if user.HasName() {
					msg := agent.Message{
						Role:    agent.RoleSystem,
						Content: []agent.Content{{Type: agent.ContentTypeText, Text: fmt.Sprintf("# User Context\n- **Name:** %s", user.GetFullName())}},
					}
					c.Chat.Messages = append([]agent.Message{msg}, c.Chat.Messages...)
				}
				return next.Handle(c)
			}

			// Build Markdown representation from generic UserContext.
			var sb strings.Builder
			sb.WriteString("# User Context\n")

			name := user.GetFullName()
			if name == "" {
				name = uctx.UserID
			}
			if name != "" {
				sb.WriteString(fmt.Sprintf("- **Name:** %s\n", name))
			}

			// Render all attributes in stable key order.
			if len(uctx.Attributes) > 0 {
				keys := make([]string, 0, len(uctx.Attributes))
				for k := range uctx.Attributes {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					sb.WriteString(fmt.Sprintf("- **%s:** %s\n", formatLabel(k), uctx.Attributes[k]))
				}
			}

			// Render all tags in stable key order.
			if len(uctx.Tags) > 0 {
				keys := make([]string, 0, len(uctx.Tags))
				for k := range uctx.Tags {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					if vals := uctx.Tags[k]; len(vals) > 0 {
						sb.WriteString(fmt.Sprintf("- **%s:** %s\n", formatLabel(k), strings.Join(vals, ", ")))
					}
				}
			}

			if uctx.Goals != "" {
				sb.WriteString(fmt.Sprintf("- **Goals:** %s\n", uctx.Goals))
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

// formatLabel converts a snake_case key to a Title Case label.
// e.g., "learning_style" → "Learning Style"
func formatLabel(key string) string {
	parts := strings.Split(key, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
