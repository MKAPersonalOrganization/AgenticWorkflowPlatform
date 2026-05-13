package migrate

import (
	"strings"
	"testing"

	"github.com/ardanlabs/darwin/v3"
)

// TestMigrateSQLParses is a guardrail for pitfalls.md #13: it verifies
// that every `-- Version:` block in sql/migrate.sql is parseable by
// Darwin. A stray continuation comment starting with "-- ver" or
// "-- des" would cause ParseMigrations to return an empty slice — this
// test catches that at unit-test time rather than at cluster startup.
func TestMigrateSQLParses(t *testing.T) {
	t.Parallel()

	parsed := darwin.ParseMigrations(migrateDoc)
	versionLines := strings.Count(migrateDoc, "\n-- Version:")
	if strings.HasPrefix(migrateDoc, "-- Version:") {
		versionLines++
	}
	if len(parsed) == 0 {
		t.Fatalf("ParseMigrations returned 0 entries (expected %d) — a description continuation probably starts with '-- ver' or '-- des'", versionLines)
	}
	if len(parsed) != versionLines {
		t.Fatalf("ParseMigrations returned %d entries, want %d", len(parsed), versionLines)
	}
}
