#!/usr/bin/env python3
"""
assertion_density.py — test-quality signal that pairs with coverage.py.

``coverage.py`` reports which lines were executed by the test suite.
It says nothing about whether the tests actually *checked* anything —
a test that calls code and asserts nothing is a regression net with
holes. The `testing-quality literature <https://testing.googleblog.com>`_
and Kent Beck's early TDD writing both land on the same point:
**coverage is a necessary but not sufficient test-quality signal**. A
package at 90% coverage with 5 assertions is weaker than one at 60%
with 300.

This tool reads every ``*_test.go`` file in the module and counts
*assertion-like calls* — the places a test commits to a claim the
framework can fail on. The Go idiom is plural; we recognise:

  - ``t.Error``, ``t.Errorf``, ``t.Fatal``, ``t.Fatalf`` (stdlib testing)
  - ``require.*`` / ``assert.*`` (github.com/stretchr/testify)
  - ``Expect(...)`` / ``Ω(...)`` (gomega)
  - ``t.Run`` is NOT counted — it's a sub-test boundary, not an
    assertion. Sub-tests contain their own assertions which we count
    inside their body.
  - Bare ``if got != want { t.Errorf(...) }`` counts once (the
    ``t.Errorf`` call), not twice — we count the assertion CALL, not
    the surrounding condition.

Derived metrics per file:
  - ``test_loc``            non-blank, non-comment lines in the file
  - ``test_funcs``          count of ``^func Test``
  - ``assertions``          total matched assertion calls
  - ``assertions_per_loc``  density (0..1)
  - ``assertions_per_func`` mean per test function
  - ``verdict``             tier (HIGH = likely fake-tested, FINE = healthy)

Verdict thresholds — "HIGH" means attention required, matching other
scripts' direction.

  HIGH    test_funcs > 0 AND (assertions_per_func < 1 OR assertions_per_loc < 0.02)
  MEDIUM  assertions_per_func < 3 OR assertions_per_loc < 0.05
  LOW     assertions_per_func < 6
  FINE    otherwise

The cross-signal use: pair with ``coverage.py``. A file at coverage
verdict FINE (≥ 80% covered) but assertion_density verdict HIGH is a
clear "fake-tested" flag that neither signal alone can produce.

Schema: ``assertion-density/1``.

Usage:
    scripts/architecture/assertion_density.py --json out/assertion.json
    scripts/architecture/assertion_density.py --paths ./modules/evaluation/ --top 20
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path


SCHEMA_VERSION = "assertion-density/1"

# We only scan test files. Generated / vendored stuff excluded.
_TEST_FILE_RE = re.compile(r"_test\.go$")
_EXCLUDE_DIRS = re.compile(
    r"(^|/)(vendor|node_modules|testdata|_tmp|books|\.hotspots|\.READ|scripts/out)/"
)

# The canonical Go testing assertion set. Each pattern matches a
# function CALL (not a method-reference or import) — we look for the
# identifier followed by ``(`` so plain variables named ``Error``
# (unusual but possible) don't false-positive.
_ASSERTION_PATTERNS = [
    # stdlib testing: t.Error, t.Errorf, t.Fatal, t.Fatalf, t.Check
    (re.compile(r"\b[a-zA-Z_]+\.(?:Error|Errorf|Fatal|Fatalf|Check)\b\s*\("), "stdlib"),
    # testify: require.*(t, …) or assert.*(t, …)
    (re.compile(r"\b(?:require|assert)\.[A-Z][A-Za-z0-9_]*\b\s*\("), "testify"),
    # gomega: Expect(…).To(…) counts the Expect call once.
    (re.compile(r"\bExpect\b\s*\("), "gomega"),
    (re.compile(r"\bΩ\s*\("), "gomega"),
]

_TEST_FUNC_RE = re.compile(r"^func\s+(?:\(\w+\s+\*?\w+\)\s+)?(?:Test|Benchmark|Example)[A-Z_][\w_]*\s*\(", re.M)


# -- Core analysis ---------------------------------------------------------

def strip_comments_and_blanks(text: str) -> tuple[list[str], str]:
    """Return (kept_lines, stripped_text).

    We strip ``//`` line comments and ``/* ... */`` block comments
    before counting assertions so regex matches can't trip on pattern-
    shaped comments. test_loc is the kept-line count (non-blank,
    non-comment).
    """
    # Remove /* block comments */ (greedy, but DOTALL). This runs in
    # O(len(text)) via re.sub.
    text = re.sub(r"/\*.*?\*/", "", text, flags=re.DOTALL)
    kept: list[str] = []
    for line in text.splitlines():
        stripped = line.lstrip()
        if not stripped:
            continue
        if stripped.startswith("//"):
            continue
        # Drop everything after ``//`` (inline comment) before keeping.
        code_part = re.split(r"\s*//", line, maxsplit=1)[0]
        if code_part.strip():
            kept.append(code_part)
    return kept, "\n".join(kept)


def analyze_file(path: Path) -> dict:
    """Return assertion-density metrics for a single ``*_test.go`` file."""
    try:
        raw = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return {"path": str(path), "test_loc": 0, "assertions": 0,
                "test_funcs": 0, "assertions_per_loc": 0.0,
                "assertions_per_func": 0.0, "by_family": {}}

    kept_lines, stripped_text = strip_comments_and_blanks(raw)
    test_loc = len(kept_lines)
    test_funcs = len(_TEST_FUNC_RE.findall(raw))

    family_counts: dict[str, int] = defaultdict(int)
    total = 0
    for pattern, family in _ASSERTION_PATTERNS:
        hits = pattern.findall(stripped_text)
        family_counts[family] += len(hits)
        total += len(hits)

    per_loc = (total / test_loc) if test_loc > 0 else 0.0
    per_func = (total / test_funcs) if test_funcs > 0 else 0.0

    return {
        "path": str(path),
        "test_loc": test_loc,
        "test_funcs": test_funcs,
        "assertions": total,
        "assertions_per_loc": round(per_loc, 4),
        "assertions_per_func": round(per_func, 2),
        "by_family": dict(family_counts),
    }


def classify(record: dict) -> str:
    """Apply verdict thresholds.

    A file with zero test functions is skipped (verdict "NO_TESTS");
    this happens for helpers_test.go or test-support files that define
    fixtures used by sibling tests but contain no ``func Test*``
    themselves.
    """
    if record["test_funcs"] == 0:
        return "NO_TESTS"
    per_func = record["assertions_per_func"]
    per_loc = record["assertions_per_loc"]
    if per_func < 1 or per_loc < 0.02:
        return "HIGH"
    if per_func < 3 or per_loc < 0.05:
        return "MEDIUM"
    if per_func < 6:
        return "LOW"
    return "FINE"


# -- Aggregation -----------------------------------------------------------

def aggregate_by_package(records: list[dict]) -> list[dict]:
    """Per-package rollup: sums + density ratios over the whole package."""
    by_pkg: dict[str, dict] = defaultdict(lambda: {
        "test_loc": 0, "test_funcs": 0, "assertions": 0,
        "by_family": defaultdict(int), "file_count": 0,
    })
    for r in records:
        pkg = str(Path(r["path"]).parent) or "."
        agg = by_pkg[pkg]
        agg["test_loc"] += r["test_loc"]
        agg["test_funcs"] += r["test_funcs"]
        agg["assertions"] += r["assertions"]
        agg["file_count"] += 1
        for fam, n in r.get("by_family", {}).items():
            agg["by_family"][fam] += n

    out: list[dict] = []
    for pkg, agg in by_pkg.items():
        per_loc = (agg["assertions"] / agg["test_loc"]) if agg["test_loc"] else 0.0
        per_func = (agg["assertions"] / agg["test_funcs"]) if agg["test_funcs"] else 0.0
        synthetic_record = {
            "path": pkg,
            "test_funcs": agg["test_funcs"],
            "assertions_per_loc": per_loc,
            "assertions_per_func": per_func,
        }
        out.append({
            "package": pkg,
            "file_count": agg["file_count"],
            "test_loc": agg["test_loc"],
            "test_funcs": agg["test_funcs"],
            "assertions": agg["assertions"],
            "assertions_per_loc": round(per_loc, 4),
            "assertions_per_func": round(per_func, 2),
            "by_family": dict(agg["by_family"]),
            "verdict": classify(synthetic_record),
        })
    return out


# -- Walking + reporting ---------------------------------------------------

def list_test_files(root: Path) -> list[Path]:
    out: list[Path] = []
    for base, dirs, files in os.walk(root):
        rel = Path(base).relative_to(root).as_posix()
        rel = "" if rel == "." else rel + "/"
        dirs[:] = [d for d in dirs
                   if not _EXCLUDE_DIRS.search(rel + d + "/")]
        for name in files:
            if not _TEST_FILE_RE.search(name):
                continue
            out.append(root / (rel + name))
    return out


def print_stdout(records: list[dict], packages: list[dict], top: int) -> None:
    if not records:
        print("No test files found.")
        return
    total_loc = sum(r["test_loc"] for r in records)
    total_assert = sum(r["assertions"] for r in records)
    total_funcs = sum(r["test_funcs"] for r in records)
    overall_per_loc = round(total_assert / total_loc, 4) if total_loc else 0.0
    overall_per_func = round(total_assert / total_funcs, 2) if total_funcs else 0.0
    print(f"Test corpus: {len(records)} files  ·  {total_loc:,} test LOC  ·  "
          f"{total_funcs} test funcs  ·  {total_assert:,} assertions")
    print(f"Overall: {overall_per_func:.2f} assertions/func  ·  "
          f"{overall_per_loc:.3f} assertions/LOC")
    verdict_hist = Counter(r["verdict"] for r in records)
    print("File verdict distribution: " +
          "  ".join(f"{v}={verdict_hist[v]}"
                   for v in ("HIGH", "MEDIUM", "LOW", "FINE", "NO_TESTS") if verdict_hist[v]))
    print()

    # Worst-first: HIGH → MEDIUM → LOW → FINE, then by lowest per-func.
    severity_order = {"HIGH": 0, "MEDIUM": 1, "LOW": 2, "NO_TESTS": 3, "FINE": 4}
    ranked = sorted(records, key=lambda r: (severity_order.get(r["verdict"], 99),
                                             r["assertions_per_func"],
                                             r["path"]))
    print(f"── Top {min(top, len(ranked))} files (worst-first) ──")
    print(f"  {'Verdict':<8}  {'Funcs':>5}  {'Asrt':>4}  {'/func':>6}  {'/LOC':>5}  Path")
    for r in ranked[:top]:
        print(f"  {r['verdict']:<8}  {r['test_funcs']:>5}  {r['assertions']:>4}  "
              f"{r['assertions_per_func']:>6.2f}  {r['assertions_per_loc']:>5.3f}  {r['path']}")


# -- Driver ----------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=__doc__.split("Usage:")[0].strip(),
    )
    ap.add_argument("--paths", nargs="*", default=None,
                    help="restrict scan to one or more directories (default: repo root)")
    ap.add_argument("--top", type=int, default=25,
                    help="how many files to print on stdout (default 25)")
    ap.add_argument("-o", "--output", "--json", dest="output",
                    help="write JSON report to this path")
    args = ap.parse_args()

    repo_root = Path.cwd()
    roots = [Path(p) for p in args.paths] if args.paths else [repo_root]
    files: list[Path] = []
    for root in roots:
        files.extend(list_test_files(root))

    records: list[dict] = []
    for f in files:
        rec = analyze_file(f)
        try:
            rec["path"] = str(f.resolve().relative_to(repo_root.resolve()))
        except ValueError:
            rec["path"] = str(f)
        rec["verdict"] = classify(rec)
        records.append(rec)
    packages = aggregate_by_package(records)

    print_stdout(records, packages, args.top)

    if args.output:
        verdict_hist = Counter(r["verdict"] for r in records)
        pkg_verdict_hist = Counter(p["verdict"] for p in packages)
        severity_order = {"HIGH": 0, "MEDIUM": 1, "LOW": 2, "NO_TESTS": 3, "FINE": 4}
        ranked = sorted(records, key=lambda r: (severity_order.get(r["verdict"], 99),
                                                r["assertions_per_func"], r["path"]))
        ranked_pkg = sorted(packages, key=lambda r: (severity_order.get(r["verdict"], 99),
                                                     r["assertions_per_func"], r["package"]))
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "assertion_density",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "excludes": [_EXCLUDE_DIRS.pattern],
                "test_file_pattern": _TEST_FILE_RE.pattern,
                "assertion_families": [fam for _, fam in _ASSERTION_PATTERNS],
            },
            "thresholds": {
                "HIGH": "per_func < 1 OR per_loc < 0.02",
                "MEDIUM": "per_func < 3 OR per_loc < 0.05",
                "LOW": "per_func < 6",
                "FINE": "otherwise",
            },
            "stats": {
                "test_files": len(records),
                "test_loc_total": sum(r["test_loc"] for r in records),
                "test_funcs_total": sum(r["test_funcs"] for r in records),
                "assertions_total": sum(r["assertions"] for r in records),
                "overall_per_func": round(
                    sum(r["assertions"] for r in records)
                    / max(1, sum(r["test_funcs"] for r in records)), 2,
                ),
                "overall_per_loc": round(
                    sum(r["assertions"] for r in records)
                    / max(1, sum(r["test_loc"] for r in records)), 4,
                ),
                "file_verdict_counts": dict(verdict_hist),
                "package_verdict_counts": dict(pkg_verdict_hist),
            },
            "files": {"all": records, "ranked": ranked},
            "packages": {"all": packages, "ranked": ranked_pkg},
        }
        Path(args.output).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.output}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
