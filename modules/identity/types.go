package identity

import "school-with-ai/domain"

// ──────────────────────────────────────────────────────────────────────────────
// Type aliases — canonical types remain in domain/ for Phase 4 because
// components/profile imports domain.StudentProfile, which would create an
// import cycle if the type lived here. Once component templates are updated
// (Phase 6), these types will move into this package and the aliases removed.
// ──────────────────────────────────────────────────────────────────────────────

// StudentProfile is an alias for domain.StudentProfile.
type StudentProfile = domain.StudentProfile

// StudentProfileRepository is an alias for domain.StudentProfileRepository.
type StudentProfileRepository = domain.StudentProfileRepository
