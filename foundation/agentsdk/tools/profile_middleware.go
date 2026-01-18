package tools

import (
	"context"
	"fmt"
	"strings"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
)

// NewStudentProfileInjectorMiddleware creates a middleware that injects the student profile.
func NewStudentProfileInjectorMiddleware(repo domain.StudentProfileRepository) core.Middleware {
	return core.MiddlewareFunc(func(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
		user := domain.GetUserFromContext(ctx)
		if user == nil {
			return next(ctx, reqCtx)
		}

		profile, err := repo.GetByUserID(ctx, user.ID)
		if err != nil {
			// Log error but don't fail the request?
			// Ideally we should log, but middleware func doesn't take logger directly.
			// Proceed without profile.
			return next(ctx, reqCtx)
		}

		if profile == nil {
			// Attempt to use User details if profile doesn't exist
			if user.HasName() {
				reqCtx.AddSystemPrompt(fmt.Sprintf("# Student Profile\n- **Name:** %s", user.GetFullName()))
			}
			return next(ctx, reqCtx)
		}

		// Build Markdown representation
		var sb strings.Builder
		sb.WriteString("# Student Profile\n")
		
		name := user.GetFullName()
		if name == "" {
			name = "Student" // or keep empty
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

		reqCtx.AddSystemPrompt(sb.String())

		return next(ctx, reqCtx)
	})
}
