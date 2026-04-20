#!/usr/bin/env python3
"""
coupling_distance.py — "distance" dimension from the Balanced Coupling
framework (Khononov, *Balancing Coupling in Software Design*, Ch. 8).

The book's DISTANCE metric is an ordinal 1–10 measure of how far apart
two modules sit in the codebase / organisation:

  1  — same class/method (not applicable at package granularity)
  2  — same package / file (same directory)
  3  — same package tree, deep siblings or parent↔child
  4  — same sub-tree, one level of divergence
  5  — same top-level module directory (e.g. two packages under modules/)
  7  — different top-level directories inside the same Go module
       (e.g. adapters/* ↔ modules/*)
  9  — across services (needs semantic analysis — not detected here;
       reserved for a future signal that inspects A2A/MCP/HTTP clients)
  10 — external Go module (different vendor / upstream dependency)

Input: every (source, target) import edge reported by ``go list``. We
don't re-use package_deps.py's JSON because that's capped at the top
importers/imports per package — for distance we want the complete edge
graph so the report counts every dependency, not just the loudest.

Output per edge:
  source, target, distance, within_module, external, shared_depth, reason

Output per package (source):
  outgoing edge count, min/max/mean distance, external_ratio, and a
  distance-bucket histogram. Useful for spotting packages that reach
  widely across the codebase vs. ones that stay inside their own tree.

Schema: ``coupling-distance/1``. Consumed by ``balanced_coupling.py``
(the composite score from the same book, Ch. 10).

Usage:
    scripts/architecture/coupling_distance.py --json out/distance.json
    scripts/architecture/coupling_distance.py --internal-only --top 20
    scripts/architecture/coupling_distance.py --paths ./modules/... --json out.json

Notes:
- Distance depends only on the import graph + the module path; it is
  completely deterministic. Run-to-run stability is expected.
- Stdlib imports are ignored (no "fmt" / "net/http" entries). They are
  always distance 10 and would drown out the signal.
"""
from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path


SCHEMA_VERSION = "coupling-distance/1"

# Distance thresholds (book Ch. 8, adapted for Go's package-tree shape).
# SERVICE and EXTERNAL are anchors; the rest are derived from shared-
# prefix depth on path segments.
_DIST_SAME = 2
_DIST_PARENT_CHILD = 3          # one pkg is ancestor of the other
_DIST_SIBLINGS_DEEP = 3         # shared prefix ≥ 3 segments below module root
_DIST_SIBLINGS_MID = 4          # shared prefix = 2 segments
_DIST_SIBLINGS_TOP = 5          # shared prefix = 1 segment
_DIST_CROSS_TOP_LEVEL = 7       # no shared prefix inside module
_DIST_EXTERNAL = 10


def read_module_prefix(repo_root: Path) -> str:
    """Return the go.mod module path (e.g. 'aicompass'). Empty when
    absent — all packages will then be treated as external, which is
    useless, so the caller should raise.
    """
    go_mod = repo_root / "go.mod"
    if not go_mod.exists():
        return ""
    for line in go_mod.read_text().splitlines():
        line = line.strip()
        if line.startswith("module "):
            return line.split(None, 1)[1].strip()
    return ""


# Stdlib import paths have no ``/`` *and* no ``.`` in their first
# segment (e.g. ``fmt``, ``net/http``). goda excludes them by default;
# ``go list`` doesn't, so we filter manually. Technically any
# single-segment import without a dot is stdlib, plus "cmd/*".
_STDLIB_FIRST_SEGMENT_RE = re.compile(r"^[a-z0-9_]+(/|$)")


def is_stdlib_import(path: str) -> bool:
    if not path:
        return True
    first = path.split("/", 1)[0]
    # Real packages have a "." in the domain segment (e.g. github.com,
    # golang.org). Everything without that *and* without a custom
    # module_prefix is stdlib.
    return "." not in first


# Import-graph ingestion ---------------------------------------------------

def run_go_list(paths: list[str], cwd: Path) -> str:
    """Run ``go list -f '…' <paths>`` and return raw stdout.

    The template emits one line per package:
        <ImportPath>|<Import1>;<Import2>;…
    """
    if not shutil.which("go"):
        raise SystemExit("error: 'go' not on PATH")
    target = paths or ["./..."]
    cmd = ["go", "list", "-f",
           "{{.ImportPath}}|{{range .Imports}}{{.}};{{end}}",
           *target]
    print(f"$ {' '.join(cmd)}", file=sys.stderr)
    res = subprocess.run(cmd, cwd=str(cwd), capture_output=True, text=True)
    if res.returncode != 0:
        raise SystemExit(f"go list exited {res.returncode}\n{res.stderr}")
    return res.stdout


def parse_go_list_output(raw: str, module_prefix: str) -> list[tuple[str, str]]:
    """Return the flat edge list [(source, target), ...], excluding
    stdlib targets.

    We don't dedupe here — ``go list`` already emits each edge once.
    """
    edges: list[tuple[str, str]] = []
    for line in raw.splitlines():
        line = line.strip()
        if not line or "|" not in line:
            continue
        src, imports_blob = line.split("|", 1)
        for imp in imports_blob.split(";"):
            imp = imp.strip()
            if not imp:
                continue
            if is_stdlib_import(imp) and not imp.startswith(module_prefix):
                continue
            edges.append((src, imp))
    return edges


# Distance computation -----------------------------------------------------

def _components(pkg: str, module_prefix: str) -> list[str] | None:
    """Return the path segments *below* module_prefix, or None when the
    package is external. The module root itself returns []."""
    if module_prefix and pkg == module_prefix:
        return []
    if module_prefix and pkg.startswith(module_prefix + "/"):
        return pkg[len(module_prefix) + 1:].split("/")
    return None  # external


def distance(source: str, target: str, module_prefix: str) -> dict:
    """Compute the 1–10 distance for one edge.

    Returns a dict with the numeric value + the attributes that drove
    the classification. The ``reason`` field is a short tag callers can
    use for grouping ("siblings_top", "cross_top_level", …).
    """
    a = _components(source, module_prefix)
    b = _components(target, module_prefix)
    within_module = a is not None and b is not None
    external = b is None  # target outside the module

    if not within_module:
        return {
            "distance": _DIST_EXTERNAL,
            "within_module": False,
            "external": True,
            "shared_depth": 0,
            "reason": "external_dependency",
        }

    if a == b:
        return {
            "distance": _DIST_SAME,
            "within_module": True,
            "external": False,
            "shared_depth": min(len(a), len(b)),
            "reason": "same_package",
        }

    # Longest common prefix on path segments below module_prefix.
    shared = 0
    min_depth = min(len(a), len(b))
    while shared < min_depth and a[shared] == b[shared]:
        shared += 1

    # One pkg is an ancestor of the other → shared == the shorter one's depth.
    if shared == min_depth:
        return {
            "distance": _DIST_PARENT_CHILD,
            "within_module": True,
            "external": False,
            "shared_depth": shared,
            "reason": "ancestor_or_descendant",
        }

    # Bucket by how many segments they agree on before diverging.
    if shared >= 3:
        d, reason = _DIST_SIBLINGS_DEEP, "deep_siblings"
    elif shared == 2:
        d, reason = _DIST_SIBLINGS_MID, "mid_siblings"
    elif shared == 1:
        d, reason = _DIST_SIBLINGS_TOP, "same_top_level_dir"
    else:
        d, reason = _DIST_CROSS_TOP_LEVEL, "cross_top_level"

    return {
        "distance": d,
        "within_module": True,
        "external": False,
        "shared_depth": shared,
        "reason": reason,
    }


# Aggregation --------------------------------------------------------------

def aggregate_by_source(edges: list[dict]) -> list[dict]:
    """Per-source summary: edge count + distance distribution + reason
    breakdown. Useful for ranking packages that reach the widest.
    """
    by_src: dict[str, list[dict]] = defaultdict(list)
    for e in edges:
        by_src[e["source"]].append(e)

    out: list[dict] = []
    for src, es in by_src.items():
        dists = [e["distance"] for e in es]
        reason_counts = Counter(e["reason"] for e in es)
        external_count = sum(1 for e in es if e["external"])
        out.append({
            "package": src,
            "outgoing_edges": len(es),
            "min_distance": min(dists),
            "max_distance": max(dists),
            "mean_distance": round(sum(dists) / len(dists), 2),
            "external_edge_count": external_count,
            "external_edge_ratio": round(external_count / len(es), 2),
            "reason_counts": dict(reason_counts),
            "distance_histogram": dict(Counter(dists)),
        })
    # Sort by widest reach (highest mean distance * edge count) so the
    # top of the list is "packages that pull heavily from far places".
    out.sort(key=lambda r: (-(r["mean_distance"] * r["outgoing_edges"]), r["package"]))
    return out


# Reporting ----------------------------------------------------------------

def print_stdout(edges: list[dict], per_source: list[dict],
                 module_prefix: str, top: int,
                 internal_only: bool) -> None:
    if internal_only:
        edges = [e for e in edges if e["within_module"]]
    total = len(edges)
    if total == 0:
        print("No edges to report.")
        return

    # Histogram across all edges.
    hist = Counter(e["distance"] for e in edges)
    print(f"Module: {module_prefix or '(unset)'}")
    print(f"Edges: {total}  "
          f"(internal {sum(1 for e in edges if e['within_module'])}, "
          f"external {sum(1 for e in edges if e['external'])})\n")
    print("Distance histogram:")
    for d in sorted(hist):
        bar = "█" * min(40, int(40 * hist[d] / total))
        print(f"  d={d:>2}  {hist[d]:>5}  {bar}")
    print()

    # Top-N widest internal edges — the lens most refactor decisions
    # actually care about. External d=10 edges are dominant by count
    # but usually not actionable (they're library deps).
    internal = [e for e in edges if e["within_module"]]
    internal.sort(key=lambda e: (-e["distance"], e["source"], e["target"]))
    print(f"── Top {min(top, len(internal))} widest INTERNAL edges ──")
    print(f"  {'Dist':>4}  {'Reason':<25}  Source → Target")
    for e in internal[:top]:
        print(f"  {e['distance']:>4}  {e['reason']:<25}  {e['source']}  →  {e['target']}")
    print()

    # Top-N widest-reaching packages (outgoing mean × count).
    print(f"── Top {min(top, len(per_source))} widest-reaching packages ──")
    print(f"  {'Edges':>5}  {'Mean':>5}  {'Max':>3}  {'Ext':>3}  Package")
    for r in per_source[:top]:
        print(f"  {r['outgoing_edges']:>5}  {r['mean_distance']:>5.2f}  "
              f"{r['max_distance']:>3}  {r['external_edge_count']:>3}  {r['package']}")


# Driver -------------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=__doc__.split("Usage:")[0].strip(),
    )
    ap.add_argument("--paths", nargs="+", default=["./..."],
                    help="go list target(s) (default ./...)")
    ap.add_argument("--top", type=int, default=25,
                    help="how many rows per stdout table (default 25)")
    ap.add_argument("--internal-only", action="store_true",
                    help="omit external-dependency edges (d=10) from the histogram")
    ap.add_argument("--json", type=str,
                    help="write JSON report to this path")
    args = ap.parse_args()

    repo_root = Path.cwd()
    module_prefix = read_module_prefix(repo_root)
    if not module_prefix:
        raise SystemExit("error: could not read module path from go.mod")

    raw = run_go_list(args.paths, repo_root)
    edge_pairs = parse_go_list_output(raw, module_prefix)

    edges: list[dict] = []
    for src, tgt in edge_pairs:
        info = distance(src, tgt, module_prefix)
        edges.append({"source": src, "target": tgt, **info})

    per_source = aggregate_by_source(edges)
    print_stdout(edges, per_source, module_prefix, args.top, args.internal_only)

    if args.json:
        hist_all = Counter(e["distance"] for e in edges)
        internal = [e for e in edges if e["within_module"]]
        external = [e for e in edges if e["external"]]
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "coupling_distance",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "paths": args.paths,
                "module_prefix": module_prefix,
            },
            "thresholds": {
                "same_package": _DIST_SAME,
                "parent_or_child": _DIST_PARENT_CHILD,
                "deep_siblings": _DIST_SIBLINGS_DEEP,
                "mid_siblings": _DIST_SIBLINGS_MID,
                "top_level_siblings": _DIST_SIBLINGS_TOP,
                "cross_top_level": _DIST_CROSS_TOP_LEVEL,
                "external_dependency": _DIST_EXTERNAL,
            },
            "stats": {
                "total_edges": len(edges),
                "internal_edges": len(internal),
                "external_edges": len(external),
                "packages_with_outgoing_edges": len(per_source),
                "distance_histogram": dict(hist_all),
                "mean_distance_internal": round(
                    sum(e["distance"] for e in internal) / len(internal), 2,
                ) if internal else 0.0,
            },
            "edges": edges,
            "per_source": per_source,
        }
        Path(args.json).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.json}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
