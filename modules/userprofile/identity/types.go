package identity

import "school-with-ai/domain"

// ──────────────────────────────────────────────────────────────────────────────
// Type aliases — canonical types remain in domain/ (shared kernel) because
// StudentProfile is consumed by 3+ packages across 2+ layers:
//   - modules/identity/ (owner)
//   - modules/chat/ (consumer)
//   - foundation/agentsdk/middleware/ (consumer)
//
// The import cycle that previously blocked moving these (components/profile →
// domain → identity) was broken in ADR 016 by refactoring components to accept
// a local DTO. The types remain in domain/ because they are legitimately
// shared-kernel types.
// ──────────────────────────────────────────────────────────────────────────────

// StudentProfile is an alias for domain.StudentProfile.
type StudentProfile = domain.StudentProfile

// StudentProfileRepository is an alias for domain.StudentProfileRepository.
type StudentProfileRepository = domain.StudentProfileRepository
