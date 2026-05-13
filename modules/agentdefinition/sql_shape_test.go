package agentdefinition

import (
	"regexp"
	"strings"
	"testing"
)

func TestGeneratedInsertAndUpdateParams(t *testing.T) {
	t.Parallel()

	got := insertAgentQuery
	// 13 leading + 11 JSON + 4 trailing = 28 placeholders.
	want := 13 + len(agentJSONColumns) + 4
	re := regexp.MustCompile(`\$\d+`)
	if n := len(re.FindAllString(got, -1)); n != want {
		t.Fatalf("INSERT placeholders = %d, want %d; query:\n%s", n, want, got)
	}
	// Update: 11 scalar + len(jsonCols) + 2 where = total
	got = updateAgentQuery
	want = 11 + len(agentJSONColumns) + 2
	if n := len(re.FindAllString(got, -1)); n != want {
		t.Fatalf("UPDATE placeholders = %d, want %d; query:\n%s", n, want, got)
	}
	// Sanity: ensure each JSON column name appears in both queries.
	for _, c := range agentJSONColumns {
		if !strings.Contains(insertAgentQuery, c.name) {
			t.Errorf("INSERT missing JSON column %q", c.name)
		}
		if !strings.Contains(updateAgentQuery, c.name) {
			t.Errorf("UPDATE missing JSON column %q", c.name)
		}
	}
}
