#!/usr/bin/env python3
"""
cyclomatic.py — architectural complexity signals via Lizard
(https://github.com/terryyin/lizard).

Reports, per file:

  - cyclomatic complexity (CCN) per function + the top offenders,
  - a second pass with lizard extensions for ``modified_ccn``
    (switch counts as 1, so an event-dispatch `Apply()` doesn't look
    as scary as a 30-branch tangled function), ``max_nested_structures``
    (NS), and ``exit_count`` (number of return paths — a flat CCN
    function with 10 early returns is still hairy),
  - duplicate code blocks — clones of ≥ 6-line sequences across the
    scan. On this codebase that surfaces the 6× repeated
    ``NewCustomEvent`` wrapper in ``modules/chat/handlers/common/converter.go``
    and similar card-template repetition in templ/Alpine files.

Schema ``cyclomatic/2`` is additive over ``cyclomatic/1``: previous
consumers (refactor_priority.py) still read ``max_ccn``,
``high_ccn_count``, ``top_functions``, etc. unchanged; the new fields
(``duplicate_blocks``, ``modified_ccn``, ``max_ns``, ``max_exit_count``)
are optional per entry.

Why a 6th signal: complexity_trend.py measures whitespace indent as a
proxy for structural nesting, which misses real branching complexity
(a file of flat `if/else if/else` chains looks fine to the indent
metric). Cyclomatic complexity counts branches directly per function,
at the LLM-calls / tool-use / control-flow level that actually matters
for reasoning about a function. Duplicate detection catches
copy-paste regressions that no other signal in this directory can see.

Usage:
    scripts/architecture/cyclomatic.py                              # fast: CCN + NS/exit/modified
    scripts/architecture/cyclomatic.py --json out/cyclomatic.json   # emit structured report
    scripts/architecture/cyclomatic.py --top 30                     # show top N offenders
    scripts/architecture/cyclomatic.py --threshold 15 --top 20      # stricter high-CCN floor
    scripts/architecture/cyclomatic.py modules/ adapters/ --duplicates \
        --json out/cyclomatic.json                                  # scoped duplicate detection

Duplicate detection is opt-in (``--duplicates``) because lizard's
clone detector is O(tokens²) and scanning the whole repo can take
minutes. Scope it to specific subtrees (``modules/``, ``adapters/``,
``assets/js/``) for usable iteration times — a few seconds each.

Defaults cover the language set we actually ship (Go, JS, TS, Python,
templ is not supported by lizard — keep that in mind when reading
aggregate numbers). Excludes vendor/, *_templ.go, _test.go by
default to match the other signals in scripts/architecture/.
"""
from __future__ import annotations

import argparse
import csv
import io
import json
import re
import shutil
import subprocess
import sys
from collections import defaultdict
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


SCHEMA_VERSION = "cyclomatic/2"

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


# Lizard CLI support ------------------------------------------------------
#
# The Python API (`lizard.analyze(..., exts=[...])`) is unreliable for
# Go when non-trivial extensions (ns, nd, duplicate) are plugged in —
# their state machines assume C/Java-style token streams, and on Go
# they can corrupt the function boundaries (we observed names being
# lost entirely in local tests). Falling back to the CLI is the
# pragmatic fix: lizard's CLI extension wiring is the better-trodden
# path, and we can parse its CSV / text output deterministically.
#
# Two extra passes run only when requested (default on). Both are
# tolerant: if the lizard binary isn't on PATH, or the subprocess
# fails, we log and continue with whatever we already have.

def _find_lizard_cli() -> str | None:
    """Locate the lizard executable. Returns the path or None."""
    return shutil.which("lizard")


# Exclusion globs passed to lizard's CLI (-x) so it doesn't walk into
# vendor trees, generated artefacts, reference copies, etc. The
# Python-API pass applies _EXCLUDE_PATTERNS to the RESULTS; this one
# has to prune BEFORE lizard starts tokenising, because duplicate
# detection is O(tokens²) and scanning vendor drags to a crawl.
_LIZARD_EXCLUDE_GLOBS = [
    "*/vendor/*",
    "*/node_modules/*",
    "*/testdata/*",
    "*/.READ/*",
    "*/scripts/out/*",
    "*/_tmp/*",
    "*/books/*",
    "*/dist/*",
    "*/build/*",
    "*.min.js",
    "*.min.css",
    "*_templ.go",
    "*_mock.go",
    "*_mocks.go",
    "*.pb.go",
    "*_test.go",
]


def _run_lizard_cli(args: list[str], paths: list[str]) -> str | None:
    """Invoke lizard with given args + paths, return stdout text on
    success. Prepends ``-x`` exclude globs so we don't tokenise the
    reference copies under ``.READ/`` or the ``vendor/`` tree — on
    large repos the difference is between 2 s and 2 min."""
    exe = _find_lizard_cli()
    if exe is None:
        return None
    exclude_args: list[str] = []
    for g in _LIZARD_EXCLUDE_GLOBS:
        exclude_args.extend(["-x", g])
    try:
        out = subprocess.run(
            [exe, *exclude_args, *args, *paths],
            capture_output=True, text=True, check=False,
        )
    except OSError as e:
        print(f"cyclomatic.py: lizard CLI invocation failed: {e}", file=sys.stderr)
        return None
    # lizard exits non-zero when warnings are hit (that's the normal
    # CI-gate behaviour); stdout is still well-formed, so we accept it.
    return out.stdout


# Extensions that collect extra per-function metrics in the CSV
# output:
#   -ENS          → max_nested_structures (nesting-depth proxy)
#   -Eexitcount   → exit_count (number of return paths)
#   -Emodified    → CCN column becomes modified-CCN (switch counted as 1)
#
# We run these together in one pass so Go's tricky lexer only has to
# cope once. The CCN column is `modified_ccn`; the standard CCN comes
# from the separate Python-API pass.
def _collect_extensions(paths: list[str]) -> dict[tuple[str, int], dict]:
    """Return a dict keyed by (normalised_path, start_line) → per-fn
    extension metrics. Empty when lizard CLI isn't available or fails."""
    out = _run_lizard_cli(
        ["-ENS", "-Eexitcount", "-Emodified", "--csv"],
        paths,
    )
    if not out:
        return {}
    # Lizard CSV columns, as of v1.17.x:
    # NLOC, CCN, token_count, param_count, length, func_ref, filename,
    # name, long_name, start_line, end_line, NS, exits
    per_fn: dict[tuple[str, int], dict] = {}
    reader = csv.reader(io.StringIO(out))
    for row in reader:
        if len(row) < 13:
            continue
        try:
            file_path = row[6].replace("\\", "/").lstrip("./")
            start_line = int(row[9])
            ns = int(row[11])
            exits = int(row[12])
            modified_ccn = int(row[1])
        except (ValueError, IndexError):
            continue
        per_fn[(file_path, start_line)] = {
            "max_nested_structures": ns,
            "exit_count": exits,
            "modified_ccn": modified_ccn,
        }
    return per_fn


# Duplicate detection via -Eduplicate. The output is not in the CSV /
# XML streams — lizard prints a "Duplicates" section to stdout. Parse
# it with a small state machine:
#
#   Duplicates
#   ===================================
#   Duplicate block:
#   --------------------------
#   path/a.go:68 ~ 73
#   path/a.go:101 ~ 106
#   path/b.go:12 ~ 17
#   ^^^^^^^^^^^^^^^^^^^^^^^^^^
#
#   Duplicate block:
#   ...
#
# Each block lists ≥ 2 locations; a clone "count" is implicit in how
# many locations share the block. We attribute each location to its
# file — the per-file "duplicate_blocks" field is the list of clones
# that touch that file, with (start, end) line ranges.
_DUP_SECTION_HEADER = re.compile(r"^Duplicates\b")
_DUP_BLOCK_START = re.compile(r"^Duplicate block:\s*$")
_DUP_LINE = re.compile(r"^(.+?):(\d+)\s*~\s*(\d+)\s*$")


def _collect_duplicates(paths: list[str]) -> dict[str, list[dict]]:
    """Return {file → [clone_ref, ...]}. Each clone_ref is
    ``{start, end, occurrences, siblings}`` where ``siblings`` is the
    set of other file paths that share this clone (including
    same-file repeats). Empty when lizard CLI isn't available."""
    out = _run_lizard_cli(["-Eduplicate"], paths)
    if not out:
        return {}
    lines = out.splitlines()
    i = 0
    # Skip ahead to the "Duplicates" section header.
    while i < len(lines) and not _DUP_SECTION_HEADER.match(lines[i]):
        i += 1
    per_file: dict[str, list[dict]] = defaultdict(list)
    while i < len(lines):
        if not _DUP_BLOCK_START.match(lines[i]):
            i += 1
            continue
        # Collect every "path:start ~ end" line until the "^^^^^..."
        # terminator or a blank line.
        i += 1
        block_locations: list[tuple[str, int, int]] = []
        while i < len(lines):
            line = lines[i].strip()
            if not line or line.startswith("^"):
                break
            m = _DUP_LINE.match(line)
            if m:
                path = m.group(1).replace("\\", "/").lstrip("./")
                block_locations.append((path, int(m.group(2)), int(m.group(3))))
            i += 1
        if len(block_locations) >= 2:
            occurrences = len(block_locations)
            for path, start, end in block_locations:
                siblings = [
                    f"{p}:{s}-{e}"
                    for p, s, e in block_locations
                    if (p, s, e) != (path, start, end)
                ]
                per_file[path].append({
                    "start": start,
                    "end": end,
                    "span": end - start + 1,
                    "occurrences": occurrences,
                    "siblings": siblings,
                })
    return dict(per_file)


def _verdict(max_ccn: int, high_count: int, duplicate_blocks: int = 0) -> str:
    # Duplicate blocks in a file are an independent "worth looking at"
    # signal — a flat CCN file with 5 clones is also actionable. They
    # escalate LOW → MEDIUM and add no-CCN files into LOW.
    if max_ccn >= _SEVERE_CCN or high_count >= 3:
        return "HIGH"
    if max_ccn >= _ELEVATED_CCN or high_count >= 1 or duplicate_blocks >= 3:
        return "MEDIUM"
    if max_ccn > 0 or duplicate_blocks > 0:
        return "LOW"
    return "FINE"


def _aggregate_file(
    result,
    threshold: int,
    ext_by_fn: dict[tuple[str, int], dict] | None = None,
    dup_blocks: list[dict] | None = None,
) -> dict | None:
    """Roll a single lizard FileInfo into one JSON entry.

    Returns None for files that produced no function records (lizard
    skips unsupported languages silently, so we filter them out here
    rather than reporting CCN=0 for HTML, YAML, etc.).

    ``ext_by_fn`` — optional NS/exit/modified-CCN metrics keyed by
    (normalised_path, start_line). Each matched function is annotated
    with those extra fields.
    ``dup_blocks`` — optional list of duplicate blocks that touch this
    file (see :func:`_collect_duplicates`). Added as-is to the entry
    plus summary counts for the top-level fields.
    """
    fns = list(result.function_list)
    if not fns:
        return None
    ext_by_fn = ext_by_fn or {}
    # Normalise the file path once so lookups match whatever the CLI
    # emits for this filename.
    norm_path = result.filename.replace("\\", "/").lstrip("./")
    ccns = [fn.cyclomatic_complexity for fn in fns]
    max_ccn = max(ccns)
    mean_ccn = round(sum(ccns) / len(ccns), 2)
    high_ccn_count = sum(1 for c in ccns if c >= threshold)
    total_nloc = sum(fn.nloc for fn in fns)

    # Per-function top-5 offenders — now carries extension fields when
    # available. Fall back silently when the CLI pass didn't run or a
    # function didn't line up across passes.
    top = sorted(fns, key=lambda x: -x.cyclomatic_complexity)[:5]
    top_fns = []
    for fn in top:
        exts = ext_by_fn.get((norm_path, fn.start_line), {})
        entry = {
            "name": fn.name,
            "long_name": fn.long_name,
            "ccn": fn.cyclomatic_complexity,
            "nloc": fn.nloc,
            "line": fn.start_line,
            "parameters": len(fn.parameters),
        }
        if exts:
            entry["modified_ccn"] = exts["modified_ccn"]
            entry["max_nested_structures"] = exts["max_nested_structures"]
            entry["exit_count"] = exts["exit_count"]
        top_fns.append(entry)

    # File-level roll-ups for the extension metrics. These join against
    # every function (not just the top 5) so the numbers reflect the
    # whole file.
    max_ns = 0
    max_exits = 0
    max_modified_ccn = 0
    for fn in fns:
        exts = ext_by_fn.get((norm_path, fn.start_line))
        if not exts:
            continue
        max_ns = max(max_ns, exts["max_nested_structures"])
        max_exits = max(max_exits, exts["exit_count"])
        max_modified_ccn = max(max_modified_ccn, exts["modified_ccn"])

    dup_blocks = dup_blocks or []
    dup_count = len(dup_blocks)
    # Total lines implicated in clones — a rough "how much of this
    # file is copy-pasted from elsewhere" indicator.
    dup_line_span = sum(b.get("span", 0) for b in dup_blocks)

    entry = {
        "path": result.filename,
        "verdict": _verdict(max_ccn, high_ccn_count, dup_count),
        "max_ccn": max_ccn,
        "mean_ccn": mean_ccn,
        "high_ccn_count": high_ccn_count,
        "function_count": len(fns),
        "total_nloc": total_nloc,
        "top_functions": top_fns,
    }
    # Only emit extension fields when we actually gathered them — keeps
    # the JSON lean and lets consumers detect "no extensions were run"
    # by their absence.
    if max_ns or max_exits or max_modified_ccn:
        entry["max_ns"] = max_ns
        entry["max_exit_count"] = max_exits
        entry["max_modified_ccn"] = max_modified_ccn
        # Complexity gap: how much of the raw CCN comes from the
        # switch-as-many-branches expansion vs genuinely tangled
        # control flow. A big gap (e.g. CCN=30, modified=13) tells you
        # "this is a dispatcher — refactor-by-splitting won't help".
        entry["ccn_switch_gap"] = max(0, max_ccn - max_modified_ccn)
    if dup_blocks:
        entry["duplicate_blocks"] = dup_blocks
        entry["duplicate_block_count"] = dup_count
        entry["duplicate_line_span"] = dup_line_span
    return entry


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


def analyse(
    paths: list[str],
    threshold: int,
    include_extensions: bool = True,
    include_duplicates: bool = False,
) -> list[dict]:
    """Run lizard over `paths` and return one dict per file with ≥ 1
    function. Files whose language lizard doesn't parse (YAML, HTML,
    templ…) are silently omitted.

    Two optional CLI passes annotate the results with additional
    signals when available:

      - ``include_extensions`` (default ON): NS / exit_count /
        modified_ccn per function. Cheap — one CSV pass with a token-
        stream walker.
      - ``include_duplicates`` (default OFF): cloned-block detection
        per file. Expensive — lizard does O(tokens²) cross-file
        hashing, which is minutes on repos with big asset trees.
        Opt in with ``--duplicates`` on the CLI when you want clone
        data in the report.

    Both are best-effort: if the lizard CLI isn't on PATH or the
    subprocess fails, the extra signals are silently omitted from
    the result instead of failing the whole run.
    """
    ext_by_fn: dict[tuple[str, int], dict] = {}
    dup_by_file: dict[str, list[dict]] = {}
    if include_extensions:
        ext_by_fn = _collect_extensions(paths)
    if include_duplicates:
        dup_by_file = _collect_duplicates(paths)

    entries: list[dict] = []
    for r in _iter_file_results(paths):
        norm_path = r.filename.replace("\\", "/").lstrip("./")
        entry = _aggregate_file(
            r, threshold,
            ext_by_fn=ext_by_fn,
            dup_blocks=dup_by_file.get(norm_path, []),
        )
        if entry is not None:
            entries.append(entry)
    return entries


def _print_table(entries: list[dict], top: int) -> None:
    entries_sorted = sorted(
        entries,
        key=lambda e: (
            -e["max_ccn"],
            -e.get("duplicate_block_count", 0),
            -e["high_ccn_count"],
            e["path"],
        ),
    )
    shown = [e for e in entries_sorted if e["verdict"] != "FINE"][:top]
    if not shown:
        print("No files above the complexity threshold. 🎉")
        return
    hdr = (f"{'Verdict':<7}  {'MaxCCN':>6}  {'ModCCN':>6}  {'NS':>3}  "
           f"{'Exits':>5}  {'HighCCN':>7}  {'Dups':>4}  "
           f"{'Fns':>4}  {'NLOC':>5}  Path")
    print(hdr)
    print("-" * len(hdr))
    for e in shown:
        mod = str(e.get("max_modified_ccn") or "-").rjust(6)
        ns = str(e.get("max_ns") or "-").rjust(3)
        ex = str(e.get("max_exit_count") or "-").rjust(5)
        dups = str(e.get("duplicate_block_count") or "-").rjust(4)
        print(f"{e['verdict']:<7}  {e['max_ccn']:>6}  {mod}  {ns}  "
              f"{ex}  {e['high_ccn_count']:>7}  {dups}  "
              f"{e['function_count']:>4}  {e['total_nloc']:>5}  {e['path']}")


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
    ap.add_argument("--duplicates", action="store_true",
                    help="run the cloned-block detection pass. Off by "
                         "default: lizard's duplicate detector is "
                         "O(tokens²) and can take minutes on repos "
                         "with big asset trees. Scope the scan to a "
                         "subtree (e.g. 'modules/' 'adapters/') when "
                         "you want to enable it.")
    ap.add_argument("--no-extensions", action="store_true",
                    help="skip the NS / exit_count / modified_ccn pass. "
                         "You get raw CCN + function NLOC only — cheaper "
                         "but you lose switch-dispatch detection.")
    args = ap.parse_args()

    entries = analyse(
        args.paths,
        args.threshold,
        include_extensions=not args.no_extensions,
        include_duplicates=args.duplicates,
    )
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
                "extensions_pass": not args.no_extensions,
                "duplicates_pass": args.duplicates,
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
