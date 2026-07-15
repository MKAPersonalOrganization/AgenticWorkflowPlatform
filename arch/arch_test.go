// Package arch mechanically enforces the layer and module boundaries that the
// ADRs in arch/adr document. The ADRs carry the why; these tests carry the
// what — a change that crosses a boundary fails the build instead of relying
// on reviewers (human or AI) to remember prose.
//
// Layering (low to high):
//
//	foundation  → imports no other aicompass tree
//	infra       → foundation
//	frontend    → foundation
//	plugins     → foundation, frontend
//	modules     → foundation, infra, frontend (cross-module only via allowlist)
//	adapters    → foundation, modules, plugins
//	main / cmd  → composition roots, unconstrained
package arch

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const modulePath = "aicompass"

// allowedPrefixes maps a source tree to the aicompass import prefixes its
// production code may use. Every tree may import itself.
var allowedPrefixes = map[string][]string{
	"foundation": {"foundation"},
	"infra":      {"foundation", "infra"},
	"frontend":   {"foundation", "frontend"},
	"plugins":    {"foundation", "frontend", "plugins"},
	"modules":    {"foundation", "infra", "frontend", "modules"},
	"adapters":   {"foundation", "modules", "plugins", "adapters"},
}

// allowedCrossModule enumerates the only module→module edges permitted inside
// modules/. Each entry is a deliberate, reviewed contract — adding one here
// should feel like signing your name. Everything else is a violation.
var allowedCrossModule = map[string][]string{
	// TenantContext is the shared tenancy value passed to the knowledge
	// (Edgequake) service by its consumers.
	"chat":        {"modules/knowledge/tenant"},
	"permissions": {"modules/knowledge/tenant"},
	// The templates catalog publishes into the agent-definition repository.
	"templates": {"modules/agentdefinition"},
}

// TestLayerBoundaries verifies every production Go file imports only from the
// trees its layer is allowed to see.
func TestLayerBoundaries(t *testing.T) {
	t.Parallel()

	for file, imports := range collectImports(t) {
		tree := strings.SplitN(file, string(filepath.Separator), 2)[0]
		allowed, constrained := allowedPrefixes[tree]
		if !constrained {
			continue // composition roots (main, cmd) and unlisted trees
		}
		for _, imp := range imports {
			if !prefixAllowed(imp, allowed) {
				t.Errorf("%s imports %q — the %s tree may only import %v",
					file, modulePath+"/"+imp, tree, allowed)
			}
		}
	}
}

// TestModuleIsolation verifies modules do not reach into each other except
// through the explicitly allowlisted contract packages.
func TestModuleIsolation(t *testing.T) {
	t.Parallel()

	for file, imports := range collectImports(t) {
		parts := strings.Split(file, string(filepath.Separator))
		if len(parts) < 2 || parts[0] != "modules" {
			continue
		}
		module := parts[1]
		for _, imp := range imports {
			if !strings.HasPrefix(imp, "modules/") {
				continue
			}
			if imp == "modules/"+module || strings.HasPrefix(imp, "modules/"+module+"/") {
				continue // own module
			}
			if prefixAllowed(imp, allowedCrossModule[module]) {
				continue
			}
			t.Errorf("%s imports %q — module %q may not import other modules"+
				" (allowlisted contracts: %v)",
				file, modulePath+"/"+imp, module, allowedCrossModule[module])
		}
	}
}

// prefixAllowed reports whether imp equals or lives under any allowed prefix.
func prefixAllowed(imp string, allowed []string) bool {
	for _, p := range allowed {
		if imp == p || strings.HasPrefix(imp, p+"/") {
			return true
		}
	}
	return false
}

// collectImports parses every production .go file in the repository and
// returns repo-relative file path → aicompass-internal imports (module path
// stripped). Test files, generated templ output, vendor, and tooling
// directories are excluded.
func collectImports(t *testing.T) map[string][]string {
	t.Helper()

	root := ".."
	fset := token.NewFileSet()
	out := map[string][]string{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", "bin", "node_modules", ".git", ".READ":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") ||
			strings.HasSuffix(path, "_templ.go") {
			return nil
		}

		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}

		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}

		var internal []string
		for _, spec := range f.Imports {
			imp := strings.Trim(spec.Path.Value, `"`)
			if imp == modulePath || !strings.HasPrefix(imp, modulePath+"/") {
				continue
			}
			internal = append(internal, strings.TrimPrefix(imp, modulePath+"/"))
		}
		if len(internal) > 0 {
			sort.Strings(internal)
			out[rel] = internal
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repository: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("no Go files found — collectImports is broken, not the architecture")
	}
	return out
}
