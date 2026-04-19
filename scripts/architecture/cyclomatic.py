#!/usr/bin/env python3
"""
cyclomatic.py — architectural complexity signal via Lizard
(https://github.com/terryyin/lizard).

Reports per-function cyclomatic complexity (CCN) across the repo and
aggregates it to the file level so the cross-signal synthesizer
(refactor_priority.py) can consume it alongside hotspot / trend /
cohesion / coupling / bugfix signals.

Why a 6th signal: complexity_trend.py measures whitespace indent as a
proxy for structural nesting, which misses real branching complexity
(a file of flat `if/else if/else` chains looks fine to the indent
metric). Cyclomatic complexity counts branches directly per function,
at the LLM-calls / tool-use / control-flow level that actually matters
for reasoning about a function.

Usage:
    scripts/architecture/cyclomatic.py                              # scan repo, print top offenders
    scripts/architecture/cyclomatic.py --json out/cyclomatic.json   # emit structured report
    scripts/architecture/cyclomatic.py --top 30                     # show top N offenders
    scripts/architecture/cyclomatic.py --threshold 15 --top 20      # stricter high-CCN floor

Defaults cover the language set we actually ship (Go, JS, TS, Python,
templ is not supported by lizard — keep that in mind when reading
aggregate numbers). Excludes vendor/, *_templ.go, _test.go by
default to match the other signals in scripts/architecture/.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable

try:
    import lizard  # type: ignore
except ImportError:
    print("cyclomatic.py requires lizard.\n"
          "  pip install lizard\n"
          "  or install all deps: pip install -r scripts/architecture/requirements.txt",
          file=sys.stderr)
    sys.exit(1)


SCHEMA_VERSION = "cyclomatic/1"

# Excludes mirror the other architecture scripts — test files rot
# differently from production, generated files have artificial CCN,
# vendored code isn't ours to refactor.
_EXCLUDE_PATTERNS = re.compile(
    r"(^|/)(vendor|node_modules|testdata|_tmp|books|scripts/out)/"
    r"|_test\.go$|_templ\.go$|\.pb\.go$|_mock\.go$|_mocks\.go$"
    r"|(?:^|/)zz_generated|bindata\.go$"
    r"|\.min\.(js|css)$"
)

# Verdict thresholds. Tuned for "the reader should pay attention when a
# file hits these"; everything is data — no score, no priority.
#
#   HIGH    one or more functions at CCN ≥ 25, OR ≥ 3 functions at CCN ≥ threshold
#   MEDIUM  one function at CCN ≥ 15, OR 1–2 functions at CCN ≥ threshold
#   LOW     one function at CCN ≥ threshold (default 10) but under 15
#   FINE    every function under threshold
_SEVERE_CCN = 25
_ELEVATED_CCN = 15


def _is_excluded(path: str) -> bool:
    return bool(_EXCLUDE_PATTERNS.search(path))


def _verdict(max_ccn: int, high_count: int) -> str:
    if max_ccn >= _SEVERE_CCN or high_count >= 3:
        return "HIGH"
    if max_ccn >= _ELEVATED_CCN or high_count >= 1:
        return "MEDIUM"
    if max_ccn > 0:
        return "LOW"
    return "FINE"


def _aggregate_file(result, threshold: int) -> dict | None:
    """Roll a single lizard FileInfo into one JSON entry.

    Returns None for files that produced no function records (lizard
    skips unsupported languages silently, so we filter them out here
    rather than reporting CCN=0 for HTML, YAML, etc.).
    """
    fns = list(result.function_list)
    if not fns:
        return None
    ccns = [fn.cyclomatic_complexity for fn in fns]
    max_ccn = max(ccns)
    mean_ccn = round(sum(ccns) / len(ccns), 2)
    high_ccn_count = sum(1 for c in ccns if c >= threshold)
    total_nloc = sum(fn.nloc for fn in fns)
    # Top 5 offenders for the detail view; callers that don't need them
    # can ignore the field.
    top = sorted(fns, key=lambda x: -x.cyclomatic_complexity)[:5]
    top_fns = [
        {
            "name": fn.name,
            "long_name": fn.long_name,
            "ccn": fn.cyclomatic_complexity,
            "nloc": fn.nloc,
            "line": fn.start_line,
            "parameters": len(fn.parameters),
        }
        for fn in top
    ]
    return {
        "path": result.filename,
        "verdict": _verdict(max_ccn, high_ccn_count),
        "max_ccn": max_ccn,
        "mean_ccn": mean_ccn,
        "high_ccn_count": high_ccn_count,
        "function_count": len(fns),
        "total_nloc": total_nloc,
        "top_functions": top_fns,
    }


def _iter_file_results(paths: Iterable[str]) -> Iterable:
    """Yield lizard FileInfo records for every non-excluded path that
    lizard can parse. `paths` may contain files or directories.
    """
    for r in lizard.analyze(list(paths)):
        if not r.filename:
            continue
        normalized = r.filename.replace("\\", "/")
        # lizard walks directories itself, so we filter *here* instead of
        # pre-walking. Normalising first matters on non-POSIX paths.
        if _is_excluded(normalized):
            continue
        yield r


def analyse(paths: list[str], threshold: int) -> list[dict]:
    """Run lizard over `paths` and return one dict per file with ≥ 1
    function. Files whose language lizard doesn't parse (YAML, HTML,
    templ…) are silently omitted."""
    entries: list[dict] = []
    for r in _iter_file_results(paths):
        entry = _aggregate_file(r, threshold)
        if entry is not None:
            entries.append(entry)
    return entries


def _print_table(entries: list[dict], top: int) -> None:
    entries_sorted = sorted(
        entries,
        key=lambda e: (-e["max_ccn"], -e["high_ccn_count"], e["path"]),
    )
    shown = [e for e in entries_sorted if e["verdict"] != "FINE"][:top]
    if not shown:
        print("No files above the complexity threshold. 🎉")
        return
    hdr = f"{'Verdict':<7}  {'MaxCCN':>6}  {'MeanCCN':>7}  {'HighCCN':>7}  {'Fns':>4}  {'NLOC':>5}  Path"
    print(hdr)
    print("-" * len(hdr))
    for e in shown:
        print(f"{e['verdict']:<7}  {e['max_ccn']:>6}  {e['mean_ccn']:>7.1f}  "
              f"{e['high_ccn_count']:>7}  {e['function_count']:>4}  "
              f"{e['total_nloc']:>5}  {e['path']}")


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Per-function cyclomatic complexity signal via lizard.",
    )
    ap.add_argument("paths", nargs="*", default=["."],
                    help="files or directories to scan (default: current dir)")
    ap.add_argument("--threshold", type=int, default=10,
                    help="per-function CCN above which we count a function "
                         "as 'high' (default 10; CCN ≥ 10 is the classic "
                         "McCabe inflection point where a function starts "
                         "to resist tests).")
    ap.add_argument("--top", type=int, default=25,
                    help="how many offenders to print on stdout (default 25)")
    ap.add_argument("--json", type=str,
                    help="write structured report to this path "
                         "(schema cyclomatic/1). refactor_priority.py "
                         "consumes this via --cyclomatic.")
    ap.add_argument("--include-all", action="store_true",
                    help="include FINE files in the JSON output (default: "
                         "only files with at least one LOW-or-worse function)")
    args = ap.parse_args()

    entries = analyse(args.paths, args.threshold)
    _print_table(entries, args.top)

    if args.json:
        out = entries if args.include_all else [
            e for e in entries if e["verdict"] != "FINE"
        ]
        ranked = sorted(
            out, key=lambda e: (-e["max_ccn"], -e["high_ccn_count"]),
        )
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "lizard",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "threshold": args.threshold,
                "paths": args.paths,
                "include_fine": bool(args.include_all),
            },
            "stats": {
                "files_scanned": len(entries),
                "files_reported": len(out),
            },
            # `ranked` is a convenience slice (top offenders first);
            # `all` carries the full set so downstream tools can filter
            # without re-running lizard.
            "ranked": ranked[:50],
            "all": out,
        }
        Path(args.json).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.json}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
