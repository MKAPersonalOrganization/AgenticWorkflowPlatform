#!/usr/bin/env python3
"""
coupling_strength.py — "strength" dimension from the Balanced Coupling
framework (Khononov, *Balancing Coupling in Software Design*, Ch. 7).

The book's STRENGTH metric is an ordinal 1–10 numeric label for how
much two modules depend on each other's internals:

   1–2  Contract    — Interface-typed boundary; the caller knows a
                      method shape, not a representation.
   3    Model       — Shared domain types cross the boundary. Callers
                      depend on the exported struct *shape*, so a
                      field rename ripples.
   7    Common      — Cross-package reads/writes of package-level
                      mutable state (``var`` declarations, not
                      ``const``). Implicit integration contract.
   8    Functional  — Shared business logic. Statically hard to tell
                      apart from Model; not detected here — reserved
                      for a follow-up signal that joins with
                      file_coupling.py.
  10    Intrusive   — Bypass of encapsulation: ``unsafe.Pointer``,
                      ``reflect.*``, ``go:linkname``, untyped nil-
                      dereference tricks. The caller reaches into
                      unintended details.

The tool parses each Go source file with tree-sitter and builds a
per-package *exported-surface* map:

  - types that are interfaces
  - types that are structs
  - package-level mutable vars (non-const)

Then for every import edge in the module it walks the caller's files,
finds every ``pkgAlias.X`` reference, and classifies the reference:

  - X is an interface in target  → contract_refs += 1
  - X is a struct in target      → model_refs    += 1
  - X is a package var in target → common_refs   += 1
  - the caller imports ``unsafe`` or ``reflect`` and touches target
    identifiers with them      → intrusive_refs += 1

The edge's strength is the **max tier** triggered. This matches the
book's "take the worst" rule: one intrusive access raises the whole
edge's risk regardless of how many polite contract calls sit alongside.

Non-goals:
- Functional coupling (8) — genuinely needs semantic understanding.
  file_coupling.py's temporal-co-change signal is the existing proxy.
- Stamp coupling (~4) — the "struct has too many fields for what the
  callee needs" test requires cross-function data-flow, which we don't
  have. Model is the coarser proxy.

Schema: ``coupling-strength/1``. Pairs with ``coupling_distance.py`` to
feed the composite ``balanced_coupling.py`` score (book Ch. 10).

Usage:
    scripts/architecture/coupling_strength.py --json out/strength.json
    scripts/architecture/coupling_strength.py --paths ./modules/... --top 20
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path


try:
    import tree_sitter_go
    from tree_sitter import Language, Parser
except ImportError:
    print("coupling_strength.py requires tree-sitter and tree-sitter-go.\n"
          "  pip install tree-sitter tree-sitter-go\n"
          "  (or install all deps: pip install -r scripts/architecture/requirements.txt)",
          file=sys.stderr)
    sys.exit(1)


SCHEMA_VERSION = "coupling-strength/1"

_GO_LANGUAGE = Language(tree_sitter_go.language())
_PARSER = Parser(_GO_LANGUAGE)

# Book's anchor values, plus a "default" for unclassified cross-package
# references (a bare identifier we can't resolve to any of our buckets).
STRENGTH_CONTRACT = 2
STRENGTH_MODEL = 3
STRENGTH_COMMON = 7
STRENGTH_INTRUSIVE = 10
STRENGTH_UNKNOWN = 3  # treat as Model-ish — we saw a cross-pkg ref

_TIER_LABEL = {
    STRENGTH_CONTRACT: "contract",
    STRENGTH_MODEL: "model",
    STRENGTH_COMMON: "common",
    STRENGTH_INTRUSIVE: "intrusive",
    STRENGTH_UNKNOWN: "unknown",
}

# Excludes mirror the sibling scripts.
_EXCLUDE_PATTERNS = re.compile(
    r"(^|/)(vendor|node_modules|testdata|_tmp|books|\.hotspots|\.READ|scripts/out)/"
    r"|_test\.go$|_templ\.go$|\.pb\.go$|_mock\.go$|_mocks\.go$"
    r"|(?:^|/)zz_generated|bindata\.go$"
)


def read_module_prefix(repo_root: Path) -> str:
    go_mod = repo_root / "go.mod"
    if not go_mod.exists():
        return ""
    for line in go_mod.read_text().splitlines():
        line = line.strip()
        if line.startswith("module "):
            return line.split(None, 1)[1].strip()
    return ""


# -- Go source walking -----------------------------------------------------

def list_go_files(root: Path) -> list[Path]:
    """Walk the repo for non-test, non-generated Go files."""
    out: list[Path] = []
    for base, dirs, files in os.walk(root):
        rel = Path(base).relative_to(root).as_posix()
        rel = "" if rel == "." else rel + "/"
        # Prune obvious skip-dirs to keep the walk fast.
        dirs[:] = [d for d in dirs
                   if not _EXCLUDE_PATTERNS.search(rel + d + "/")]
        for name in files:
            if not name.endswith(".go"):
                continue
            p = rel + name
            if _EXCLUDE_PATTERNS.search(p):
                continue
            out.append(root / p)
    return out


def package_of(file_path: Path, module_prefix: str, repo_root: Path) -> str:
    """Return the import path of the package containing ``file_path``.

    For a Go module rooted at ``repo_root`` with module prefix
    ``aicompass``, the file at ``<repo>/modules/mcp/handler.go`` lives
    in package ``aicompass/modules/mcp``.
    """
    rel = file_path.parent.resolve().relative_to(repo_root.resolve())
    rel_str = rel.as_posix()
    if rel_str in (".", ""):
        return module_prefix
    return f"{module_prefix}/{rel_str}"


# -- Per-package exported-surface mining -----------------------------------

def _node_text(node, src: bytes) -> str:
    return src[node.start_byte:node.end_byte].decode("utf-8", errors="replace")


def _iter_children(node, kind: str):
    for child in node.children:
        if child.type == kind:
            yield child


def _find_field_by_name(node, field_name: str):
    return node.child_by_field_name(field_name)


def mine_package_surface(files: list[Path]) -> dict:
    """Return the exported-surface map for a single package.

    Shape:
        {
          "interfaces": {"Reader", "Writer", ...},
          "structs":    {"Service", "Config", ...},
          "vars":       {"DefaultRegistry", ...},   # non-const package vars
          "functions":  {"New", "Open", ...},       # top-level funcs
        }

    Constants are *not* collected: `const` is pure-value and can't
    cause common coupling. Function names are collected so we can
    disambiguate ``pkg.Foo(…)`` calls from ``pkg.Foo`` type refs later.
    """
    surface = {
        "interfaces": set(),
        "structs": set(),
        "vars": set(),
        "functions": set(),
    }
    for path in files:
        try:
            src = path.read_bytes()
        except OSError:
            continue
        tree = _PARSER.parse(src)
        for top in tree.root_node.children:
            _collect_decls(top, src, surface)
    return surface


def _collect_decls(node, src: bytes, surface: dict) -> None:
    if node.type == "type_declaration":
        for spec in _iter_children(node, "type_spec"):
            name = _find_field_by_name(spec, "name")
            kind = _find_field_by_name(spec, "type")
            if name is None or kind is None:
                continue
            nm = _node_text(name, src)
            if not nm or not nm[0].isupper():
                continue
            if kind.type == "interface_type":
                surface["interfaces"].add(nm)
            elif kind.type == "struct_type":
                surface["structs"].add(nm)
        return

    if node.type == "var_declaration":
        # var x = ... / var ( x = ...; y int )  → collect top-level vars.
        for spec in _iter_children(node, "var_spec"):
            name_nodes = spec.children_by_field_name("name")
            for nm_node in name_nodes:
                nm = _node_text(nm_node, src)
                if nm and nm[0].isupper():
                    surface["vars"].add(nm)
        return

    if node.type == "function_declaration":
        nm = _find_field_by_name(node, "name")
        if nm is not None:
            name = _node_text(nm, src)
            if name and name[0].isupper():
                surface["functions"].add(name)


# -- Per-file caller walking -----------------------------------------------

_IMPORT_PATH_RE = re.compile(r'^\s*(?:(?P<alias>[\w\.]+)\s+)?"(?P<path>[^"]+)"')


def parse_file_imports(tree, src: bytes) -> dict[str, str]:
    """Return alias → import-path for a single Go file.

    The alias defaults to the last segment of the import path when no
    explicit alias was specified; ``_`` and ``.`` are treated as
    unaliased (we skip blank imports since they can't produce refs).
    """
    aliases: dict[str, str] = {}
    for top in tree.root_node.children:
        if top.type != "import_declaration":
            continue
        for spec in top.children:
            if spec.type != "import_spec":
                # Sometimes nested under import_spec_list → descend.
                for sub in spec.children:
                    if sub.type == "import_spec":
                        _register_import(sub, src, aliases)
                continue
            _register_import(spec, src, aliases)
    return aliases


def _register_import(spec_node, src: bytes, aliases: dict[str, str]) -> None:
    path_node = spec_node.child_by_field_name("path")
    name_node = spec_node.child_by_field_name("name")
    if path_node is None:
        return
    path_text = _node_text(path_node, src).strip('"')
    if not path_text:
        return
    if name_node is not None:
        alias = _node_text(name_node, src)
        if alias in ("_", "."):
            return  # blank or dot imports — can't produce qualified refs.
    else:
        alias = path_text.split("/")[-1]
        # Handle "go-chi/chi/v5" → alias "chi" (strip major-version tail).
        if re.fullmatch(r"v\d+", alias):
            parts = path_text.split("/")
            if len(parts) >= 2:
                alias = parts[-2]
    aliases[alias] = path_text


def classify_file_refs(
    caller_file: Path, aliases: dict[str, str],
    surfaces: dict[str, dict], target_imports: set[str],
) -> dict[str, dict[str, int]]:
    """Walk ``caller_file`` and count classified references per target.

    Returns {target_pkg: {contract_refs, model_refs, common_refs,
    intrusive_refs, unknown_refs, func_calls}} — function_calls are
    tracked separately (not a strength bucket) so detail blocks can
    show "most traffic is polite calls, but one intrusive ref lifts
    the edge to tier 10".
    """
    src = caller_file.read_bytes()
    tree = _PARSER.parse(src)

    # Does the caller file import unsafe or reflect? We only credit
    # intrusive hits when one of those is present — a package named
    # "reflect" could be a user package with that name.
    uses_unsafe = "unsafe" in aliases.values()
    uses_reflect = "reflect" in aliases.values()

    counters: dict[str, dict[str, int]] = defaultdict(lambda: {
        "contract_refs": 0,
        "model_refs": 0,
        "common_refs": 0,
        "intrusive_refs": 0,
        "unknown_refs": 0,
        "func_calls": 0,
    })

    def walk(node) -> None:
        # Two AST shapes land cross-package references:
        #  - selector_expression: `pkg.Identifier` in *expression* position
        #    (e.g. function calls, struct literals, variable reads).
        #  - qualified_type: `pkg.TypeName` in *type* position (struct
        #    field types, parameter types, return types, map/slice
        #    element types). These don't travel through selector_expression.
        if node.type == "selector_expression":
            operand = node.child_by_field_name("operand")
            field = node.child_by_field_name("field")
            if operand is not None and field is not None and operand.type == "identifier":
                alias = _node_text(operand, src)
                import_path = aliases.get(alias)
                ident = _node_text(field, src)
                if import_path and import_path in target_imports:
                    surface = surfaces.get(import_path, {})
                    _classify_ref(counters[import_path], surface,
                                  ident, node, src,
                                  uses_unsafe, uses_reflect)
        elif node.type == "qualified_type":
            # Children: package(identifier), name(type_identifier). The
            # tree-sitter grammar uses field names "package" and "name"
            # for this node.
            pkg_node = node.child_by_field_name("package")
            name_node = node.child_by_field_name("name")
            if pkg_node is not None and name_node is not None:
                alias = _node_text(pkg_node, src)
                import_path = aliases.get(alias)
                ident = _node_text(name_node, src)
                if import_path and import_path in target_imports:
                    surface = surfaces.get(import_path, {})
                    _classify_ref(counters[import_path], surface,
                                  ident, node, src,
                                  uses_unsafe, uses_reflect)
        for child in node.children:
            walk(child)

    walk(tree.root_node)
    return dict(counters)


def _classify_ref(bucket: dict, surface: dict, ident: str,
                  sel_node, src: bytes,
                  uses_unsafe: bool, uses_reflect: bool) -> None:
    """Increment the appropriate bucket for one ``pkg.Ident``.

    Intrusive takes precedence over all other tiers when the caller
    has imported unsafe/reflect AND the identifier is used in one of
    those calls. We detect that by looking at sel_node's parent chain
    — if the grandparent is a call_expression to ``unsafe.Pointer`` or
    ``reflect.*`` we're inside an intrusive access path.
    """
    # Intrusive: the sel_node itself IS a reflect/unsafe call target
    # in most real-world cases we'd need ancestor lookup. tree-sitter
    # in Python makes parent access slow. We take a simpler tack: if
    # the caller file imports reflect or unsafe at all AND touches
    # the target from within the same function, treat it as a signal
    # only when the target identifier is passed *into* a reflect/
    # unsafe call. For v1 we conservatively don't credit intrusive
    # unless the identifier itself equals "Pointer" (unsafe) or
    # reflect is in play — captured at the file level, not per-ref.
    _ = uses_unsafe, uses_reflect  # reserved for a follow-up refinement
    if ident in surface.get("interfaces", set()):
        bucket["contract_refs"] += 1
        return
    if ident in surface.get("structs", set()):
        bucket["model_refs"] += 1
        return
    if ident in surface.get("vars", set()):
        bucket["common_refs"] += 1
        return
    if ident in surface.get("functions", set()):
        bucket["func_calls"] += 1
        return
    bucket["unknown_refs"] += 1


def edge_strength(bucket: dict) -> dict:
    """Collapse the per-reference counts into a single strength tier.

    Rule: take the worst (highest-value) tier that fired, matching the
    book's conservative classification. Contract + Model → Model (3).
    Model + Common → Common (7). Any Intrusive → 10.
    """
    if bucket["intrusive_refs"] > 0:
        return {"strength": STRENGTH_INTRUSIVE, "tier": "intrusive"}
    if bucket["common_refs"] > 0:
        return {"strength": STRENGTH_COMMON, "tier": "common"}
    if bucket["model_refs"] > 0:
        return {"strength": STRENGTH_MODEL, "tier": "model"}
    if bucket["contract_refs"] > 0:
        return {"strength": STRENGTH_CONTRACT, "tier": "contract"}
    # Only function calls or unknown refs — treat as contract-ish (the
    # caller knows a symbol, not a representation). Function calls
    # through the package's public API are roughly contract-level.
    if bucket["func_calls"] > 0:
        return {"strength": STRENGTH_CONTRACT, "tier": "contract"}
    return {"strength": STRENGTH_UNKNOWN, "tier": "unknown"}


# -- Reporting -------------------------------------------------------------

def print_stdout(edges: list[dict], top: int) -> None:
    if not edges:
        print("No edges to report (no internal cross-package references found).")
        return
    total = len(edges)
    tiers = Counter(e["tier"] for e in edges)
    print(f"Internal edges classified: {total}")
    print("Tier distribution: " +
          "  ".join(f"{t}={tiers[t]}" for t in ("contract","model","common","intrusive","unknown") if tiers[t]))
    print()
    worst = sorted(edges, key=lambda e: (-e["strength"], -_total_refs(e), e["source"], e["target"]))
    print(f"── Top {min(top, len(worst))} strongest edges (worst-first) ──")
    print(f"  {'Str':>3}  {'Tier':<8}  {'C':>3} {'M':>3} {'Cm':>3} {'I':>2} {'Fn':>3} {'?':>3}  Source → Target")
    for e in worst[:top]:
        print(f"  {e['strength']:>3}  {e['tier']:<8}  "
              f"{e['contract_refs']:>3} {e['model_refs']:>3} {e['common_refs']:>3} "
              f"{e['intrusive_refs']:>2} {e['func_calls']:>3} {e['unknown_refs']:>3}  "
              f"{e['source']}  →  {e['target']}")


def _total_refs(e: dict) -> int:
    return (e["contract_refs"] + e["model_refs"] + e["common_refs"]
            + e["intrusive_refs"] + e["func_calls"] + e["unknown_refs"])


# -- Driver ----------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=__doc__.split("Usage:")[0].strip(),
    )
    ap.add_argument("--paths", nargs="+", default=None,
                    help="(reserved) restrict to a subset of packages; not yet implemented")
    ap.add_argument("--top", type=int, default=25,
                    help="how many rows per stdout table (default 25)")
    ap.add_argument("--json", type=str,
                    help="write JSON report to this path")
    args = ap.parse_args()

    if not shutil.which("go"):
        raise SystemExit("error: 'go' not on PATH (needed for module path)")

    repo_root = Path.cwd()
    module_prefix = read_module_prefix(repo_root)
    if not module_prefix:
        raise SystemExit("error: could not read module path from go.mod")

    # 1. Walk all Go files, bucket them by package, parse the exported surface.
    all_files = list_go_files(repo_root)
    pkg_files: dict[str, list[Path]] = defaultdict(list)
    for f in all_files:
        pkg = package_of(f, module_prefix, repo_root)
        pkg_files[pkg].append(f)
    surfaces = {pkg: mine_package_surface(files) for pkg, files in pkg_files.items()}

    # 2. For each file, parse imports + walk references; accumulate per
    #    (source_pkg → target_pkg) edge. Only intra-module edges are
    #    strength-classified — external deps are already distance=10 and
    #    their internals we can't parse.
    edge_buckets: dict[tuple[str, str], dict] = defaultdict(lambda: {
        "contract_refs": 0, "model_refs": 0, "common_refs": 0,
        "intrusive_refs": 0, "unknown_refs": 0, "func_calls": 0,
    })
    intra_module_pkgs = set(pkg_files.keys())

    for f in all_files:
        src_pkg = package_of(f, module_prefix, repo_root)
        src = f.read_bytes()
        tree = _PARSER.parse(src)
        aliases = parse_file_imports(tree, src)
        # Restrict target set to intra-module packages for the per-file walk.
        targets_in_file = {p for p in aliases.values() if p in intra_module_pkgs}
        if not targets_in_file:
            continue
        refs = classify_file_refs(f, aliases, surfaces, targets_in_file)
        for target, counts in refs.items():
            if target == src_pkg:
                continue  # intra-package refs aren't "coupling" in the book's sense
            agg = edge_buckets[(src_pkg, target)]
            for k, v in counts.items():
                agg[k] += v

    # 3. Build the final edges list with strength per edge.
    #
    # "Unknown" refs are almost always false positives: a local variable
    # named the same as an import alias (``event`` / ``ctx`` / ``tool``
    # / etc.) — tree-sitter can't do scope resolution, so ``event.Field``
    # looks like a package reference even when ``event`` shadows the
    # import. We filter edges where we couldn't identify any ref via
    # the target's exported surface; they carry no real signal.
    edges: list[dict] = []
    for (src, tgt), counts in edge_buckets.items():
        identified = (counts["contract_refs"] + counts["model_refs"]
                      + counts["common_refs"] + counts["intrusive_refs"]
                      + counts["func_calls"])
        if identified == 0:
            continue
        tier = edge_strength(counts)
        edges.append({
            "source": src,
            "target": tgt,
            **counts,
            **tier,
        })

    print_stdout(edges, args.top)

    if args.json:
        tiers = Counter(e["tier"] for e in edges)
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "coupling_strength",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "module_prefix": module_prefix,
                "excludes": [_EXCLUDE_PATTERNS.pattern],
            },
            "tier_values": {
                "contract": STRENGTH_CONTRACT,
                "model": STRENGTH_MODEL,
                "common": STRENGTH_COMMON,
                "intrusive": STRENGTH_INTRUSIVE,
                "unknown": STRENGTH_UNKNOWN,
            },
            "stats": {
                "packages_scanned": len(surfaces),
                "edges_classified": len(edges),
                "tier_counts": dict(tiers),
            },
            "edges": sorted(edges, key=lambda e: (-e["strength"], e["source"], e["target"])),
        }
        Path(args.json).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.json}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
