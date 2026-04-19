// Package toolreqs is the exclusive authority for per-user, per-agent
// tool requirements. It owns two surfaces:
//
//   - [PermissionStore] — PostgreSQL + Redis-backed persistence for
//     requirement configs (allow / block / require_confirmation per tool
//     or tool-set).
//   - [Service] — runtime decision-making used by the adk adapter:
//     permission checks, confirmation lookups, and the block middleware
//     factory.
//
// Every tool access decision the adapter makes flows through one of
// these two types. HTTP handlers that only read/write config take a
// *PermissionStore directly.
package toolreqs
