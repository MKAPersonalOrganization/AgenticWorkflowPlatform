// Package shared provides the shared kernel — cross-cutting types that are
// used by multiple bounded contexts. This package must remain dependency-free
// (no imports of domain/, modules/, or application-level packages).
//
// The types here were extracted from domain/ as part of Phase 3 of the
// modular-monolith migration (see arch/adr/012-modular-monolith-migration.md).
package shared
