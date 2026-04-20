#!/usr/bin/env python3
"""
coverage.py — per-file / per-package Go test coverage as an architectural signal.

Complements ``bugfix_density.py``: that tool tells you which files absorb
fixes; this one tells you whether those files are exercised by tests at
all. A file at 25% fix-pct *and* 10% coverage is a very different risk
than 25% fix-pct + 90% coverage. Coverage alone doesn't mean code is
tested well (assertions matter); but *absence* of coverage is a very
loud signal that something is unreached by the existing suite.

Pipeline:
  1. Run ``go test -coverprofile=<tmp>`` on the scope (or reuse a
     pre-generated profile via ``--coverprofile``).
  2. Parse the coverprofile block lines:
         <pkg>/<file>.go:startLine.col,endLine.col numStmt count
     Sum per-file: covered_statements / total_statements.
  3. Walk the repo for production .go files, cross-reference: files
     that never appear in the profile get a ``NO_TESTS`` verdict (their
     package has no ``*_test.go`` file, so nothing could have covered
     them anyway). This distinguishes "0% because the feature's
     untested" from "0% because nothing in the module tests that path".
  4. Aggregate per-package by summing files within each import path.
  5. Classify with four severity tiers and emit JSON + stdout.

Verdict thresholds — worst → best, "HIGH = reader should pay attention":
  HIGH     < 20%
  MEDIUM   20% ≤ c < 50%
  LOW      50% ≤ c < 80%
  FINE     c ≥ 80%
  NO_TESTS file's package has no ``*_test.go`` — untested by construction

These match the tier naming of ``cyclomatic.py`` / ``package_deps.py``
so ``refactor_priority.py`` can fold the verdict through without special
casing. Numerical direction is inverted (low coverage = high severity)
but the tier semantics — "attention required" → "fine" — is consistent.

JSON schema: ``coverage/1``. Top-level shape mirrors bugfix_density,
cohesion, package_deps: ``filters`` / ``thresholds`` / ``stats`` plus
``all`` (full dataset) and ``ranked`` (pre-sorted for stdout consumers).
Per-file and per-package rollups live side-by-side so a consumer can
pick its granularity.

Usage:
    scripts/architecture/coverage.py                        # run go test -cover on ./...
    scripts/architecture/coverage.py --paths ./modules/...  # scope to a subtree
    scripts/architecture/coverage.py --json out/coverage.json
    scripts/architecture/coverage.py --coverprofile existing.out   # reuse a profile
    scripts/architecture/coverage.py --top 30               # widen stdout table
    scripts/architecture/coverage.py --by package --top 20  # per-package ranking

Failures in ``go test`` (compile errors, test failures) surface on
stderr; whatever coverage was produced before the failure is still
parsed. Use ``--strict`` to fail the run when the test command exits
non-zero.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path


SCHEMA_VERSION = "coverage/1"

# Default excludes mirror the sibling scripts so the path keys match.
# Test files themselves are the source of coverage, not a target of it.
_EXCLUDE_PATTERNS = re.compile(
    r"(^|/)(vendor|node_modules|testdata|_tmp|books|\.hotspots|\.READ|scripts/out)/"
    r"|_test\.go$|_templ\.go$|\.pb\.go$|_mock\.go$|_mocks\.go$"
    r"|(?:^|/)zz_generated|bindata\.go$"
)

# Tier thresholds in percent of statements covered. Tuned so a module
# at 50% looks Elevated (LOW) but not alarming (MEDIUM), and 80% is the
# neighbourhood where "this is adequately exercised" holds for most
# domains.
_TIER_HIGH = 20.0
_TIER_MEDIUM = 50.0
_TIER_LOW = 80.0


def _classify(pct: float) -> str:
    if pct < _TIER_HIGH:
        return "HIGH"
    if pct < _TIER_MEDIUM:
        return "MEDIUM"
    if pct < _TIER_LOW:
        return "LOW"
    return "FINE"


# Coverage-profile ingestion ----------------------------------------------

# Coverprofile line:
#   <import-path>/<file>.go:<sLine>.<sCol>,<eLine>.<eCol> <numStmt> <count>
# We keep the line-range so we can dedupe blocks across multiple test
# runs — `go test -coverpkg=./...` emits the same block once per test
# package that compiled it in, which inflates raw totals by ~40× on
# this repo. Atomic mode means the correct aggregation is max(count)
# per block, then sum across unique blocks.
_COVER_LINE_RE = re.compile(
    r"^(?P<path>[^:]+):(?P<range>\d+\.\d+,\d+\.\d+)\s+(?P<stmts>\d+)\s+(?P<count>\d+)$"
)


def read_module_prefix(repo_root: Path) -> str:
    """Return the go.mod module path (e.g. 'aicompass') or '' if not found.

    We strip this prefix off the paths coming out of the coverprofile so
    they match the repo-relative keys the other scripts use (the
    synthesizer joins across all tools on that key).
    """
    go_mod = repo_root / "go.mod"
    if not go_mod.exists():
        return ""
    for line in go_mod.read_text().splitlines():
        line = line.strip()
        if line.startswith("module "):
            return line.split(None, 1)[1].strip()
    return ""


def run_go_test(paths: list[str], coverprofile: Path, extra_args: list[str],
                strict: bool, cwd: Path) -> None:
    """Run ``go test -coverprofile=... -coverpkg=<paths> <paths>``.

    ``-coverpkg`` is set so a package whose tests live elsewhere (or not
    at all) still shows up as "seen with 0% covered" rather than being
    silently absent from the profile. Without it, the script couldn't
    tell "never covered" from "no tests ever ran here".
    """
    target = paths or ["./..."]
    coverpkg = ",".join(target)
    cmd = [
        "go", "test",
        "-coverprofile", str(coverprofile),
        "-coverpkg", coverpkg,
        "-covermode", "atomic",
        *extra_args,
        *target,
    ]
    print(f"$ {' '.join(cmd)}", file=sys.stderr)
    result = subprocess.run(cmd, cwd=str(cwd))
    if result.returncode != 0:
        msg = f"go test exited with code {result.returncode}"
        if strict:
            raise SystemExit(msg)
        print(f"warning: {msg} — parsing whatever coverage was produced",
              file=sys.stderr)


def parse_coverprofile(profile_path: Path, module_prefix: str
                       ) -> dict[str, dict]:
    """Return {repo_relative_path: {covered_statements, total_statements}}.

    Two-pass:
      1. dedupe (path, line-range) → max(count, stmts). With
         ``-coverpkg=./...`` Go emits the same block once per test
         package that compiled it — usually identical stmts but with
         different counts depending on which test hit the code. Atomic
         mode's canonical semantics are "one block, one count = max
         across runs".
      2. sum max-counts per file into covered/total.
    """
    if not profile_path.exists():
        raise SystemExit(f"coverprofile not found: {profile_path}")

    # block_key → (stmts, max_count). Path is part of the key so the
    # same line range in two files doesn't collide.
    blocks: dict[tuple[str, str], tuple[int, int]] = {}
    for raw in profile_path.read_text().splitlines():
        raw = raw.strip()
        if not raw or raw.startswith("mode:"):
            continue
        m = _COVER_LINE_RE.match(raw)
        if not m:
            continue
        path = m["path"]
        if module_prefix and path.startswith(module_prefix + "/"):
            path = path[len(module_prefix) + 1:]
        if _EXCLUDE_PATTERNS.search(path):
            continue
        key = (path, m["range"])
        stmts = int(m["stmts"])
        count = int(m["count"])
        existing = blocks.get(key)
        if existing is None:
            blocks[key] = (stmts, count)
        else:
            # Keep the larger statement count (should be identical) and
            # the max of the hit counts — atomic-mode aggregation.
            blocks[key] = (max(existing[0], stmts), max(existing[1], count))

    by_file: dict[str, dict] = defaultdict(lambda: {
        "covered_statements": 0,
        "total_statements": 0,
    })
    for (path, _), (stmts, count) in blocks.items():
        entry = by_file[path]
        entry["total_statements"] += stmts
        if count > 0:
            entry["covered_statements"] += stmts
    return by_file


# Repo walk for NO_TESTS detection ----------------------------------------

def discover_go_files(repo_root: Path) -> list[str]:
    """Walk the repo for production .go files, applying the same excludes
    used elsewhere. Returns paths relative to repo_root.

    We use this to surface files that never appear in the coverprofile
    because their package has no ``*_test.go`` file at all — a distinct
    failure mode from "tested but uncovered", and useful to break out as
    its own NO_TESTS tier.
    """
    found: list[str] = []
    for root, dirs, files in os.walk(repo_root):
        rel_root = Path(root).relative_to(repo_root).as_posix()
        rel_root = "" if rel_root == "." else rel_root + "/"
        # Fast-path prune obvious non-Go trees.
        dirs[:] = [d for d in dirs
                   if not _EXCLUDE_PATTERNS.search(rel_root + d + "/")]
        for name in files:
            if not name.endswith(".go"):
                continue
            rel = rel_root + name
            if _EXCLUDE_PATTERNS.search(rel):
                continue
            found.append(rel)
    return sorted(found)


def packages_without_tests(all_files: list[str]) -> set[str]:
    """Return the set of package directories whose production files
    exist but which have no ``*_test.go`` file.

    Package = directory of the file. We have to walk the filesystem for
    this because the coverprofile silently omits packages with no tests.
    """
    by_dir_prod: dict[str, list[str]] = defaultdict(list)
    by_dir_test: set[str] = set()
    # Second pass: include _test.go files too so we can detect their
    # presence; discover_go_files() filtered them out.
    for root, _dirs, files in os.walk("."):
        rel_root = Path(root).as_posix()
        rel_root = "" if rel_root == "." else rel_root.lstrip("./") + "/"
        for name in files:
            if not name.endswith(".go"):
                continue
            rel = rel_root + name
            if "/testdata/" in rel or rel.startswith("testdata/"):
                continue
            if rel.startswith("vendor/") or rel.startswith(".READ/"):
                continue
            if name.endswith("_test.go"):
                by_dir_test.add(rel_root.rstrip("/") or ".")
            else:
                by_dir_prod[rel_root.rstrip("/") or "."].append(rel)
    return {d for d in by_dir_prod if d not in by_dir_test}


# Aggregation --------------------------------------------------------------

def _pct(covered: int, total: int) -> float:
    return round(100 * covered / total, 1) if total else 0.0


def build_file_records(
    from_profile: dict[str, dict], all_files: list[str],
    no_test_pkgs: set[str],
) -> list[dict]:
    """Merge profile data with the repo walk.

    - Files in the profile get their actual covered / total / pct.
    - Files on disk but absent from the profile get 0/0, pct 0.0.
      We still emit them so the report is complete. Their verdict is
      either NO_TESTS (package has no tests at all) or HIGH (package
      has tests but this file was never hit).
    - Files in the profile but not on disk (stale build, gofmt move,
      etc.) are kept for transparency; they won't hurt anything.
    """
    records: list[dict] = []
    seen: set[str] = set()
    for path, entry in from_profile.items():
        total = entry["total_statements"]
        covered = entry["covered_statements"]
        pct = _pct(covered, total)
        pkg_dir = str(Path(path).parent) or "."
        verdict = _classify(pct) if total > 0 else (
            "NO_TESTS" if pkg_dir in no_test_pkgs else "HIGH"
        )
        records.append({
            "path": path,
            "package": pkg_dir,
            "covered_statements": covered,
            "total_statements": total,
            "coverage_pct": pct,
            "verdict": verdict,
            "in_profile": True,
        })
        seen.add(path)

    for path in all_files:
        if path in seen:
            continue
        pkg_dir = str(Path(path).parent) or "."
        verdict = "NO_TESTS" if pkg_dir in no_test_pkgs else "HIGH"
        records.append({
            "path": path,
            "package": pkg_dir,
            "covered_statements": 0,
            "total_statements": 0,
            "coverage_pct": 0.0,
            "verdict": verdict,
            "in_profile": False,
        })
    return records


def build_package_records(file_records: list[dict]) -> list[dict]:
    """Aggregate file records by package-directory."""
    by_pkg: dict[str, dict] = defaultdict(lambda: {
        "covered_statements": 0,
        "total_statements": 0,
        "file_count": 0,
        "files_untested": 0,
        "files_in_profile": 0,
    })
    for r in file_records:
        p = by_pkg[r["package"]]
        p["covered_statements"] += r["covered_statements"]
        p["total_statements"] += r["total_statements"]
        p["file_count"] += 1
        if r["in_profile"]:
            p["files_in_profile"] += 1
        if r["verdict"] in ("NO_TESTS", "HIGH"):
            p["files_untested"] += 1

    out: list[dict] = []
    for pkg, agg in by_pkg.items():
        total = agg["total_statements"]
        covered = agg["covered_statements"]
        pct = _pct(covered, total)
        # A whole package with 0 statements (pure types/constants) or
        # 0 files-in-profile is NO_TESTS at package level too. Otherwise
        # classify numerically.
        if total == 0 or agg["files_in_profile"] == 0:
            verdict = "NO_TESTS"
        else:
            verdict = _classify(pct)
        out.append({
            "path": pkg,  # directory key, same format as file.package
            "coverage_pct": pct,
            "covered_statements": covered,
            "total_statements": total,
            "file_count": agg["file_count"],
            "files_in_profile": agg["files_in_profile"],
            "files_untested": agg["files_untested"],
            "verdict": verdict,
        })
    return out


# Reporting ----------------------------------------------------------------
#
# Worst-first sort weighs *uncovered statements* most heavily — a 0/961
# package (961 uncovered stmts) is a bigger refactor signal than a
# types-only file at 0/0 (0 uncovered stmts). This naturally sinks
# pure-contract NO_TESTS files without a secondary tier tiebreaker.
# When uncovered counts tie, prefer the lower coverage percentage.


def _stdout_sort_key(r: dict) -> tuple:
    uncovered = r["total_statements"] - r["covered_statements"]
    return (
        -uncovered,
        r["coverage_pct"],  # ties → lowest coverage first
        r["path"],
    )


_STDOUT_SORT_KEY = _stdout_sort_key


def print_stdout(file_records: list[dict], package_records: list[dict],
                 mode: str, top: int, min_stmts: int) -> None:
    total_stmts = sum(r["total_statements"] for r in file_records)
    covered_stmts = sum(r["covered_statements"] for r in file_records)
    overall = _pct(covered_stmts, total_stmts)
    tiers = defaultdict(int)
    for r in file_records:
        tiers[r["verdict"]] += 1
    dist = "  ".join(f"{v}={tiers[v]}" for v in ("HIGH", "MEDIUM", "LOW", "FINE", "NO_TESTS"))
    print(f"Overall: {covered_stmts:,}/{total_stmts:,} statements covered "
          f"= {overall:.1f}% ({len(file_records)} files)")
    print(f"Tier distribution (files): {dist}\n")

    if mode in ("file", "both"):
        rows = [r for r in file_records if r["total_statements"] >= min_stmts
                or r["verdict"] == "NO_TESTS"]
        rows.sort(key=_STDOUT_SORT_KEY)
        print(f"── Top {min(top, len(rows))} file entries (worst first) ──")
        print(f"  {'Verdict':<9} {'Cov%':>5} {'covered':>7}/{'total':<5}  Path")
        for r in rows[:top]:
            if r["verdict"] == "NO_TESTS":
                cov_str = "—"
            else:
                cov_str = f"{r['coverage_pct']:.1f}"
            print(f"  {r['verdict']:<9} {cov_str:>5} "
                  f"{r['covered_statements']:>7}/{r['total_statements']:<5}  {r['path']}")
        print()

    if mode in ("package", "both"):
        rows = list(package_records)
        rows.sort(key=_STDOUT_SORT_KEY)
        print(f"── Top {min(top, len(rows))} package entries (worst first) ──")
        print(f"  {'Verdict':<9} {'Cov%':>5} {'covered':>7}/{'total':<5} "
              f"{'files':>5}  Package")
        for r in rows[:top]:
            cov_str = "—" if r["verdict"] == "NO_TESTS" else f"{r['coverage_pct']:.1f}"
            print(f"  {r['verdict']:<9} {cov_str:>5} "
                  f"{r['covered_statements']:>7}/{r['total_statements']:<5} "
                  f"{r['file_count']:>5}  {r['path']}")


# Driver -------------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=__doc__.split("Usage:")[0].strip(),
    )
    ap.add_argument("--paths", nargs="+", default=["./..."],
                    help="go test target(s) (default ./...). Passed to -coverpkg too.")
    ap.add_argument("--coverprofile", type=str,
                    help="reuse an existing coverprofile; skips running go test")
    ap.add_argument("--extra-args", default="",
                    help="extra args passed to 'go test' (space-split, e.g. '-race -short')")
    ap.add_argument("--strict", action="store_true",
                    help="fail if 'go test' exits non-zero (default: parse partial output)")
    ap.add_argument("--by", choices=["file", "package", "both"], default="both",
                    help="stdout ranking granularity (JSON always contains both)")
    ap.add_argument("--min-stmts", type=int, default=1,
                    help="in stdout file table, hide files with fewer than N statements (default 1)")
    ap.add_argument("--top", type=int, default=25,
                    help="how many rows to print per table (default 25)")
    ap.add_argument("--json", type=str,
                    help="write JSON report to this path")
    ap.add_argument("--exclude", action="append", default=[],
                    help="extra regex exclude for file paths (repeatable)")
    args = ap.parse_args()

    repo_root = Path.cwd()
    module_prefix = read_module_prefix(repo_root)

    # Assemble the effective exclude pattern (defaults + user-supplied).
    global _EXCLUDE_PATTERNS
    if args.exclude:
        extra = "|".join(args.exclude)
        _EXCLUDE_PATTERNS = re.compile(
            f"({_EXCLUDE_PATTERNS.pattern})|({extra})"
        )

    # Gather coverage data.
    if args.coverprofile:
        profile_path = Path(args.coverprofile)
    else:
        tmp = tempfile.NamedTemporaryFile(
            prefix="coverage_", suffix=".out", delete=False,
        )
        tmp.close()
        profile_path = Path(tmp.name)
        extra = args.extra_args.split() if args.extra_args else []
        run_go_test(args.paths, profile_path, extra, args.strict, repo_root)

    by_file = parse_coverprofile(profile_path, module_prefix)

    # Surface NO_TESTS files via filesystem walk.
    all_files = discover_go_files(repo_root)
    no_test_pkgs = packages_without_tests(all_files)
    file_records = build_file_records(by_file, all_files, no_test_pkgs)
    package_records = build_package_records(file_records)

    print_stdout(file_records, package_records, args.by, args.top, args.min_stmts)

    if args.json:
        total_stmts = sum(r["total_statements"] for r in file_records)
        covered_stmts = sum(r["covered_statements"] for r in file_records)
        tiers = defaultdict(int)
        for r in file_records:
            tiers[r["verdict"]] += 1
        pkg_tiers = defaultdict(int)
        for r in package_records:
            pkg_tiers[r["verdict"]] += 1

        # Pre-sorted ranked views make stdout-style consumers cheap.
        ranked_files = sorted(file_records, key=_STDOUT_SORT_KEY)
        ranked_packages = sorted(package_records, key=_STDOUT_SORT_KEY)

        report = {
            "schema": SCHEMA_VERSION,
            "tool": "coverage",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "paths": args.paths,
                "excludes": [_EXCLUDE_PATTERNS.pattern],
                "extra_args": args.extra_args,
                "module_prefix": module_prefix,
                "coverprofile": str(profile_path) if args.coverprofile else None,
            },
            "thresholds": {
                "HIGH_pct": _TIER_HIGH,
                "MEDIUM_pct": _TIER_MEDIUM,
                "LOW_pct": _TIER_LOW,
                "FINE_pct": "≥ LOW",
            },
            "stats": {
                "total_statements": total_stmts,
                "covered_statements": covered_stmts,
                "overall_coverage_pct": _pct(covered_stmts, total_stmts),
                "files_tracked": len(file_records),
                "files_in_profile": sum(1 for r in file_records if r["in_profile"]),
                "packages_tracked": len(package_records),
                "file_verdicts": dict(tiers),
                "package_verdicts": dict(pkg_tiers),
            },
            "files": {
                "all": file_records,
                "ranked": ranked_files,
            },
            "packages": {
                "all": package_records,
                "ranked": ranked_packages,
            },
        }
        Path(args.json).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.json}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
