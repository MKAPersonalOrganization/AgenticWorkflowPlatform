#!/usr/bin/env python3
"""
package_deps.py — structural package-coupling signal via goda
(https://github.com/loov/goda).

Reports, per Go package inside the current module:

  - ``fan_in`` — how many intra-module packages import this one,
  - ``fan_out`` — how many intra-module packages this one imports,
  - ``instability`` — Martin's I = fan_out / (fan_in + fan_out);
    0 = stable (only depended on), 1 = unstable (only depends on
    others). Roughly, foundation/ packages should sit near 0 and
    cmd/ near 1 — anything in the middle with high fan_in and high
    fan_out is a "pivot",
  - ``cut_pkgs`` — number of packages that would become unreachable
    from the rest of the module if this package vanished (goda's
    ``Cut.PackageCount``). High cut_pkgs = everything downstream
    goes through here,
  - ``cut_loc`` — total Go lines behind that cut,
  - ``loc`` / ``size`` — the package's own code,
  - aggregated per-group view when a ``--grouping`` file is passed
    (same format as ``scripts/architecture/go_grouping.txt``).

Schema ``package_deps/1``.

Why a 7th signal: ``file_coupling.py`` measures *temporal* coupling
(which files change together in git history). That misses the
*structural* story — a package may have been imported by 40 others
for years and never change. Structural package coupling tells you
where the module's dependency graph concentrates, which is what
actually blocks refactors. goda's ``cut`` analysis in particular
surfaces single-point-of-reach packages that no file-level signal
can see.

Usage:
    scripts/architecture/package_deps.py                              # scan current module
    scripts/architecture/package_deps.py --json out/package_deps.json # structured report
    scripts/architecture/package_deps.py --top 40                     # show top N entries
    scripts/architecture/package_deps.py --expr './modules/...'       # restrict to subtree
    scripts/architecture/package_deps.py --grouping \\
        scripts/architecture/go_grouping.txt                          # also roll up by group

Requires the ``goda`` binary on PATH:
    go install github.com/loov/goda@latest

goda already excludes stdlib by default; this script never passes
``-std`` so results are always intra-module.
"""
from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path


SCHEMA_VERSION = "package_deps/1"

# Verdict thresholds. Each rule is a direct report of a threshold; no
# composite scoring, no editorialising. The reader applies judgment.
#
#   HIGH    cut_pkgs ≥ _HIGH_CUT                (single-point-of-reach for many pkgs)
#   MEDIUM  fan_in ≥ _MEDIUM_FAN_IN             (widely depended on — API surface)
#           OR fan_out ≥ _MEDIUM_FAN_OUT        (broad consumer — orchestrator)
#           OR cut_pkgs ≥ _MEDIUM_CUT
#   LOW     any in-module edges at all
#   FINE    leaf / isolated (no intra-module in or out edges)
_HIGH_CUT = 10
_MEDIUM_CUT = 3
_MEDIUM_FAN_IN = 10
_MEDIUM_FAN_OUT = 15


def _find_goda_cli() -> str | None:
    return shutil.which("goda")


def _run_goda(args: list[str]) -> str:
    """Invoke goda with the given args, return stdout. Exits the
    process on failure — goda output is the whole payload of this
    script, so a partial run would be misleading."""
    exe = _find_goda_cli()
    if exe is None:
        print("package_deps.py requires the goda binary on PATH.\n"
              "  go install github.com/loov/goda@latest",
              file=sys.stderr)
        sys.exit(1)
    try:
        out = subprocess.run(
            [exe, *args],
            capture_output=True, text=True, check=False,
        )
    except OSError as e:
        print(f"package_deps.py: goda invocation failed: {e}", file=sys.stderr)
        sys.exit(1)
    if out.returncode != 0 and not out.stdout:
        # goda prints useful messages on stderr when the expression is
        # unparseable or the Go build fails — surface them verbatim.
        print(out.stderr or "(goda produced no output)", file=sys.stderr)
        sys.exit(out.returncode or 1)
    return out.stdout


# goda subcommand wrappers ------------------------------------------------
#
# We run goda three times because each subcommand gives us a different
# view of the graph and none of them subsumes the others:
#
#   list   — per-package facts (LOC, size) for packages in the expression.
#   cut    — cut analysis: how much of the graph is gated behind each
#            package. Also reports InDegree (fan_in across the whole
#            universe goda considered, including std if enabled).
#   graph  — the actual edge list, from which we compute intra-module
#            fan_in and fan_out. goda's own InDegree can include stdlib
#            pseudo-imports when -std is on, so we re-derive fan_in from
#            edges to guarantee "fan_in counts only our packages".

def _list_packages(expr: str) -> dict[str, dict]:
    """Per-package {id: {loc, size}}. Uses raw Go.Lines (int) and the
    human-formatted Go.Size goda prints (e.g. "42.0KB"); the latter is
    kept as-is because goda doesn't expose a raw-bytes accessor in the
    template context and reformatting would be lossy."""
    text = _run_goda([
        "list", "-f", "{{.ID}}|{{.Go.Lines}}|{{.Go.Size}}", expr,
    ])
    out: dict[str, dict] = {}
    for raw in text.splitlines():
        parts = raw.split("|")
        if len(parts) < 3:
            continue
        pkg = parts[0].strip()
        if not pkg:
            continue
        try:
            loc = int(parts[1])
        except ValueError:
            loc = 0
        out[pkg] = {"loc": loc, "size": parts[2].strip()}
    return out


# ``cut`` output columns (our format string):
#   ID | InDegree | Cut.PackageCount | Cut.AllFiles.Size | Cut.Go.Lines
# Cut.Go.Lines is a raw int; Cut.AllFiles.Size is human-formatted.
def _cut_analysis(expr: str) -> dict[str, dict]:
    text = _run_goda([
        "cut", "-f",
        "{{.ID}}|{{.InDegree}}|{{.Cut.PackageCount}}|{{.Cut.AllFiles.Size}}|{{.Cut.Go.Lines}}",
        expr,
    ])
    out: dict[str, dict] = {}
    for raw in text.splitlines():
        parts = raw.split("|")
        if len(parts) < 5:
            continue
        pkg = parts[0].strip()
        if not pkg:
            continue
        try:
            in_degree = int(parts[1])
            cut_pkgs = int(parts[2])
            cut_loc = int(parts[4])
        except ValueError:
            continue
        out[pkg] = {
            "cut_in_degree": in_degree,
            "cut_pkgs": cut_pkgs,
            "cut_size": parts[3].strip(),
            "cut_loc": cut_loc,
        }
    return out


def _edges(expr: str) -> list[tuple[str, str]]:
    """Intra-module import edges as (src, dst) tuples. With no ``-std``
    flag, goda only emits edges between packages selected by the
    expression, so every edge is intra-module by construction."""
    text = _run_goda(["graph", "-type", "edges", expr])
    edges: list[tuple[str, str]] = []
    for raw in text.splitlines():
        parts = raw.split()
        if len(parts) != 2:
            continue
        edges.append((parts[0], parts[1]))
    return edges


# Grouping ----------------------------------------------------------------
#
# Same parser semantics as the other scripts that consume
# go_grouping.txt: each non-blank, non-comment line is
#     <regex>  =>  <group name>
# evaluated in order against the package path relative to the module
# prefix (so a rule like `^modules/chat/.*$` matches
# `aicompass/modules/chat/...`). First match wins; the catch-all
# `^.*$ => Other` line at the bottom handles the rest.
_GROUP_LINE = re.compile(r"^(.+?)\s*=>\s*(.+)$")


def _load_grouping(path: Path) -> list[tuple[re.Pattern, str]]:
    rules: list[tuple[re.Pattern, str]] = []
    for line in path.read_text().splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        m = _GROUP_LINE.match(stripped)
        if not m:
            continue
        try:
            rules.append((re.compile(m.group(1)), m.group(2).strip()))
        except re.error as e:
            print(f"package_deps.py: skipping invalid grouping regex {m.group(1)!r}: {e}",
                  file=sys.stderr)
    return rules


def _module_prefix(packages: list[str]) -> str:
    """Guess the Go module prefix from the shortest common first
    segment across listed packages. `aicompass` / `aicompass/foo/bar`
    → `aicompass`. We prefer go.mod when present but fall back to this
    so the tool still works in monorepos run against sub-expressions.
    """
    if not packages:
        return ""
    mod = Path("go.mod")
    if mod.exists():
        for line in mod.read_text().splitlines():
            if line.startswith("module "):
                return line[len("module "):].strip()
    # Fall back: first path segment shared by all.
    first_segments = {p.split("/", 1)[0] for p in packages}
    if len(first_segments) == 1:
        return next(iter(first_segments))
    return ""


def _classify_package(pkg: str, prefix: str, rules: list[tuple[re.Pattern, str]]) -> str:
    """Return the group for a package, or the short package suffix
    when no rules are configured. Rules match against the
    module-relative path (``modules/chat/xyz``)."""
    rel = pkg[len(prefix) + 1:] if prefix and pkg.startswith(prefix + "/") else pkg
    if not rules:
        return rel or pkg
    for pattern, group in rules:
        if pattern.search(rel):
            return group
    return "Other"


# Analysis ----------------------------------------------------------------

def _verdict(fan_in: int, fan_out: int, cut_pkgs: int) -> str:
    if cut_pkgs >= _HIGH_CUT:
        return "HIGH"
    if (fan_in >= _MEDIUM_FAN_IN
            or fan_out >= _MEDIUM_FAN_OUT
            or cut_pkgs >= _MEDIUM_CUT):
        return "MEDIUM"
    if fan_in > 0 or fan_out > 0:
        return "LOW"
    return "FINE"


def _instability(fan_in: int, fan_out: int) -> float | None:
    total = fan_in + fan_out
    if total == 0:
        return None
    return round(fan_out / total, 3)


def analyse(expr: str, grouping: list[tuple[re.Pattern, str]] | None = None) -> dict:
    """Run goda and return a structured result with per-package
    entries, per-group rollups (optional), and top-level stats."""
    listing = _list_packages(expr)
    cuts = _cut_analysis(expr)
    edges = _edges(expr)

    fan_in: dict[str, int] = defaultdict(int)
    fan_out: dict[str, int] = defaultdict(int)
    importers_of: dict[str, set[str]] = defaultdict(set)
    imports_of: dict[str, set[str]] = defaultdict(set)
    for src, dst in edges:
        fan_out[src] += 1
        fan_in[dst] += 1
        importers_of[dst].add(src)
        imports_of[src].add(dst)

    # Union of all packages we know about from every source. `list`
    # gives us the module packages; `cut` sometimes reports packages
    # that appear only as dependency roots; `edges` can mention both.
    all_pkgs = set(listing) | set(cuts) | set(fan_in) | set(fan_out)

    prefix = _module_prefix(sorted(all_pkgs))
    rules = grouping or []

    entries: list[dict] = []
    for pkg in sorted(all_pkgs):
        fi = fan_in.get(pkg, 0)
        fo = fan_out.get(pkg, 0)
        cut = cuts.get(pkg, {})
        info = listing.get(pkg, {})
        cut_pkgs = cut.get("cut_pkgs", 0)
        entry = {
            "id": pkg,
            "group": _classify_package(pkg, prefix, rules),
            "verdict": _verdict(fi, fo, cut_pkgs),
            "fan_in": fi,
            "fan_out": fo,
            "instability": _instability(fi, fo),
            "cut_pkgs": cut_pkgs,
            "cut_loc": cut.get("cut_loc", 0),
            "cut_size": cut.get("cut_size", ""),
            "loc": info.get("loc", 0),
            "size": info.get("size", ""),
        }
        # Stable ordering — importers/imports lists are used by
        # downstream consumers (refactor_priority-style merges) to
        # render tooltips or cross-refs, so we sort rather than rely
        # on set iteration order.
        if importers_of.get(pkg):
            entry["top_importers"] = sorted(importers_of[pkg])[:10]
        if imports_of.get(pkg):
            entry["top_imports"] = sorted(imports_of[pkg])[:10]
        entries.append(entry)

    groups = _aggregate_groups(entries) if rules else []

    return {
        "prefix": prefix,
        "entries": entries,
        "groups": groups,
        "edge_count": len(edges),
    }


def _aggregate_groups(entries: list[dict]) -> list[dict]:
    """Roll per-package entries up by ``group``. Fan-in/fan-out at the
    group level is the count of *external* edges (edges whose source
    group differs from their destination group) — a group with heavy
    internal churn but clean external edges should look stable here."""
    by_group: dict[str, dict] = defaultdict(lambda: {
        "packages": 0,
        "loc": 0,
        "cut_pkgs_max": 0,
        "fan_in_sum": 0,
        "fan_out_sum": 0,
        "verdict_counts": defaultdict(int),
    })
    for e in entries:
        g = by_group[e["group"]]
        g["packages"] += 1
        g["loc"] += e["loc"]
        g["cut_pkgs_max"] = max(g["cut_pkgs_max"], e["cut_pkgs"])
        g["fan_in_sum"] += e["fan_in"]
        g["fan_out_sum"] += e["fan_out"]
        g["verdict_counts"][e["verdict"]] += 1

    out: list[dict] = []
    for name, g in by_group.items():
        vc = dict(g["verdict_counts"])
        out.append({
            "name": name,
            "packages": g["packages"],
            "loc": g["loc"],
            "max_cut_pkgs": g["cut_pkgs_max"],
            "fan_in_sum": g["fan_in_sum"],
            "fan_out_sum": g["fan_out_sum"],
            "verdict_counts": vc,
        })
    out.sort(key=lambda x: (-x["max_cut_pkgs"], -x["fan_in_sum"], x["name"]))
    return out


# Printing ----------------------------------------------------------------

def _print_packages(entries: list[dict], top: int) -> None:
    entries_sorted = sorted(
        entries,
        key=lambda e: (
            -e["cut_pkgs"],
            -e["fan_in"],
            -e["fan_out"],
            e["id"],
        ),
    )
    shown = [e for e in entries_sorted if e["verdict"] != "FINE"][:top]
    if not shown:
        print("No packages above the coupling thresholds. 🎉")
        return
    hdr = (f"{'Verdict':<7}  {'FanIn':>5}  {'FanOut':>6}  {'Inst':>5}  "
           f"{'CutPkgs':>7}  {'CutLOC':>7}  {'LOC':>6}  Package")
    print(hdr)
    print("-" * len(hdr))
    for e in shown:
        inst = f"{e['instability']:.2f}" if e["instability"] is not None else "-"
        print(f"{e['verdict']:<7}  {e['fan_in']:>5}  {e['fan_out']:>6}  "
              f"{inst:>5}  {e['cut_pkgs']:>7}  {e['cut_loc']:>7}  "
              f"{e['loc']:>6}  {e['id']}")


def _print_groups(groups: list[dict], top: int) -> None:
    if not groups:
        return
    print()
    hdr = (f"{'Group':<30}  {'Pkgs':>4}  {'LOC':>7}  "
           f"{'MaxCut':>6}  {'FanIn':>5}  {'FanOut':>6}  Verdicts")
    print(hdr)
    print("-" * len(hdr))
    for g in groups[:top]:
        vc = ",".join(f"{k}={v}" for k, v in sorted(g["verdict_counts"].items()))
        print(f"{g['name']:<30}  {g['packages']:>4}  {g['loc']:>7}  "
              f"{g['max_cut_pkgs']:>6}  {g['fan_in_sum']:>5}  "
              f"{g['fan_out_sum']:>6}  {vc}")


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Package-level structural coupling signal via goda.",
    )
    ap.add_argument("--expr", default="./...",
                    help="goda package expression to scan "
                         "(default: ./... — every package in the current module). "
                         "See `goda help expr` for the full DSL.")
    ap.add_argument("--top", type=int, default=30,
                    help="how many offenders to print on stdout (default 30)")
    ap.add_argument("--json", type=str,
                    help="write structured report to this path "
                         f"(schema {SCHEMA_VERSION}).")
    ap.add_argument("--include-all", action="store_true",
                    help="include FINE packages in the JSON output "
                         "(default: only packages with at least one edge)")
    ap.add_argument("--grouping", type=str,
                    help="path to a go_grouping.txt-style file; when "
                         "present, emits a per-group aggregate in "
                         "addition to per-package entries.")
    args = ap.parse_args()

    grouping: list[tuple[re.Pattern, str]] = []
    if args.grouping:
        path = Path(args.grouping)
        if not path.exists():
            print(f"package_deps.py: grouping file not found: {path}", file=sys.stderr)
            return 1
        grouping = _load_grouping(path)

    result = analyse(args.expr, grouping=grouping)
    _print_packages(result["entries"], args.top)
    _print_groups(result["groups"], args.top)

    if args.json:
        out_entries = result["entries"] if args.include_all else [
            e for e in result["entries"] if e["verdict"] != "FINE"
        ]
        ranked = sorted(
            out_entries,
            key=lambda e: (-e["cut_pkgs"], -e["fan_in"], -e["fan_out"]),
        )
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "goda",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "expr": args.expr,
                "include_fine": bool(args.include_all),
                "grouping": args.grouping,
            },
            "thresholds": {
                "high_cut_pkgs": _HIGH_CUT,
                "medium_cut_pkgs": _MEDIUM_CUT,
                "medium_fan_in": _MEDIUM_FAN_IN,
                "medium_fan_out": _MEDIUM_FAN_OUT,
            },
            "module_prefix": result["prefix"],
            "stats": {
                "packages_scanned": len(result["entries"]),
                "packages_reported": len(out_entries),
                "edges": result["edge_count"],
            },
            "ranked": ranked[:50],
            "all": out_entries,
            "groups": result["groups"],
        }
        Path(args.json).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.json}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
