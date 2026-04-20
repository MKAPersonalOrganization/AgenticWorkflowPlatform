#!/usr/bin/env python3
"""
dead_code.py — unreachable / unused Go symbols per file.

Why: ``coverage.py`` shows which code is untested; this script shows
which code is *unused*. A file with 100 dead-code findings isn't a
refactor target — it's a delete target. Coverage + dead-code together
are the tightest "simplify by subtraction" signal in the suite.

Wraps two orthogonal Go analyzers that each catch what the other misses:

  1. ``golang.org/x/tools/cmd/deadcode`` — call-graph traversal from
     every ``main`` entry point. Reports FUNCTIONS/METHODS that no
     reachable path calls. Highest precision for "safe to delete" (the
     call graph is sound); modulo reflection, build-tag gating, and
     plugin-style dynamic dispatch.

  2. ``staticcheck -checks U1000`` — AST/type-level check for unused
     struct fields, constants, variables, types, and receiver methods
     that deadcode's function-focused walk doesn't surface. Also
     catches unused parameters in private functions. Lower precision
     for methods implementing interfaces (U1000 generally respects
     that, but edge cases exist).

Tool versions (for reference, latest as of writing):
    go install golang.org/x/tools/cmd/deadcode@latest
    go install honnef.co/go/tools/cmd/staticcheck@latest

Outputs per-file:
  - deadcode_count    count of unreachable-function findings
  - unused_count      count of U1000 findings (fields/consts/vars/methods)
  - total_findings    sum — the headline "deletable surface" count
  - findings          list of {line, kind, symbol, tool} for detail

Verdict tiers:
  HIGH    ≥ 5 findings (likely a forgotten branch of the module)
  MEDIUM  3-4 findings
  LOW     1-2 findings
  FINE    file is clean

Emits a versioned JSON report (schema: dead-code/1).

Usage:
    scripts/architecture/dead_code.py                             # scan ./...
    scripts/architecture/dead_code.py --paths ./modules/... ./adapters/...
    scripts/architecture/dead_code.py --json out/dead.json
    scripts/architecture/dead_code.py --skip-staticcheck          # deadcode only, faster
    scripts/architecture/dead_code.py --skip-deadcode             # staticcheck only
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path


SCHEMA_VERSION = "dead-code/1"

# Excludes mirror the other scripts so the path keys match. deadcode
# and staticcheck emit paths relative to the module root (same format
# the synthesizer joins on), but we filter at the output-parsing stage
# to drop generated / vendored hits the user can't act on.
_EXCLUDE_PATTERNS = re.compile(
    r"(^|/)(vendor|node_modules|testdata|_tmp|books|\.hotspots|\.READ|scripts/out)/"
    r"|_templ\.go$|\.pb\.go$|_mock\.go$|_mocks\.go$"
    r"|(?:^|/)zz_generated|bindata\.go$"
)

_TIER_HIGH = 5
_TIER_MEDIUM = 3
_TIER_LOW = 1


def _classify(total: int) -> str:
    if total >= _TIER_HIGH:
        return "HIGH"
    if total >= _TIER_MEDIUM:
        return "MEDIUM"
    if total >= _TIER_LOW:
        return "LOW"
    return "FINE"


# Tool runners -------------------------------------------------------------

def _resolve_binary(name: str) -> str | None:
    """Return the first binary found on PATH or in $GOPATH/bin."""
    resolved = shutil.which(name)
    if resolved:
        return resolved
    gopath = os.environ.get("GOPATH") or os.path.expanduser("~/go")
    candidate = Path(gopath) / "bin" / name
    return str(candidate) if candidate.exists() else None


def run_deadcode(paths: list[str], cwd: Path) -> tuple[list[str], bool]:
    """Run deadcode, return (lines, ok).

    deadcode's default output is one finding per line:
      ``path/to/file.go:L:C: unreachable func: <symbol>``
    We pass it through unchanged and parse the leading path/line.
    """
    binary = _resolve_binary("deadcode")
    if not binary:
        print("warning: 'deadcode' not on PATH or in GOPATH/bin — "
              "install with `go install golang.org/x/tools/cmd/deadcode@latest`",
              file=sys.stderr)
        return [], False
    target = paths or ["./..."]
    cmd = [binary, *target]
    print(f"$ {' '.join(cmd)}", file=sys.stderr)
    res = subprocess.run(cmd, cwd=str(cwd), capture_output=True, text=True)
    # deadcode exits 0 even when it finds issues — exit nonzero means
    # the build broke. We surface that as a warning and keep whatever
    # output we have.
    if res.returncode != 0 and not res.stdout:
        print(f"warning: deadcode exited {res.returncode}\n{res.stderr}",
              file=sys.stderr)
        return [], False
    return res.stdout.splitlines(), True


def run_staticcheck(paths: list[str], cwd: Path) -> tuple[list[dict], bool]:
    """Run staticcheck with JSON output filtered to U1000.

    JSON mode is critical — staticcheck's text format is ambiguous for
    machine parsing, but its ``-f json`` emits one object per finding:
      {"code":"U1000","severity":"warning","location":{"file":"...","line":N,"column":N},
       "message":"func `foo` is unused"}
    """
    binary = _resolve_binary("staticcheck")
    if not binary:
        print("warning: 'staticcheck' not on PATH or in GOPATH/bin — "
              "install with `go install honnef.co/go/tools/cmd/staticcheck@latest`",
              file=sys.stderr)
        return [], False
    target = paths or ["./..."]
    cmd = [binary, "-checks", "U1000", "-f", "json", *target]
    print(f"$ {' '.join(cmd)}", file=sys.stderr)
    res = subprocess.run(cmd, cwd=str(cwd), capture_output=True, text=True)
    # staticcheck exits 1 when it finds issues — expected. Only treat
    # missing stdout as a failure.
    if not res.stdout and res.returncode != 0:
        print(f"warning: staticcheck exited {res.returncode}\n{res.stderr}",
              file=sys.stderr)
        return [], False
    findings: list[dict] = []
    for raw in res.stdout.splitlines():
        raw = raw.strip()
        if not raw:
            continue
        try:
            findings.append(json.loads(raw))
        except json.JSONDecodeError:
            continue
    return findings, True


# Output parsing -----------------------------------------------------------

# deadcode text format: "<file>:<line>:<col>: unreachable func: <symbol>"
_DEADCODE_RE = re.compile(
    r"^(?P<file>[^:]+):(?P<line>\d+):\d+:\s+unreachable\s+(?P<kind>\w+):\s+(?P<symbol>.+)$"
)


def parse_deadcode(lines: list[str], module_prefix: str
                   ) -> list[dict]:
    """Return [{path, line, kind, symbol, tool}]."""
    out: list[dict] = []
    for raw in lines:
        m = _DEADCODE_RE.match(raw)
        if not m:
            continue
        path = m["file"]
        # Normalise: deadcode can emit absolute paths or module-prefixed
        # paths. Strip the repo root + any module-prefix leaking through
        # (paranoid; usually not needed).
        path = _normalise_path(path, module_prefix)
        if _EXCLUDE_PATTERNS.search(path):
            continue
        out.append({
            "path": path,
            "line": int(m["line"]),
            "kind": m["kind"],          # "func", "method" in practice
            "symbol": m["symbol"].strip(),
            "tool": "deadcode",
        })
    return out


def parse_staticcheck(findings: list[dict], module_prefix: str
                      ) -> list[dict]:
    """Return [{path, line, kind, symbol, tool}] for U1000 entries."""
    out: list[dict] = []
    for f in findings:
        if f.get("code") != "U1000":
            continue
        loc = f.get("location") or {}
        path = loc.get("file", "")
        path = _normalise_path(path, module_prefix)
        if not path or _EXCLUDE_PATTERNS.search(path):
            continue
        msg = f.get("message", "")
        # U1000 messages are like "func `Foo` is unused" or "field `bar`
        # is unused". Yank the backtick-quoted symbol + the leading kind
        # word so we report something human-readable.
        kind, symbol = _split_u1000_message(msg)
        out.append({
            "path": path,
            "line": loc.get("line", 0),
            "kind": kind,
            "symbol": symbol,
            "tool": "staticcheck",
        })
    return out


_U1000_SPLIT = re.compile(
    r"^(?P<kind>\w+)\s+`(?P<symbol>[^`]+)`\s+is\s+(?P<state>\w+)",
    re.IGNORECASE,
)


def _split_u1000_message(msg: str) -> tuple[str, str]:
    m = _U1000_SPLIT.match(msg)
    if not m:
        return "unused", msg
    return m.group("kind").lower(), m.group("symbol")


def _normalise_path(path: str, module_prefix: str) -> str:
    if not path:
        return path
    # Absolute paths → repo-relative. Both tools usually already return
    # module-relative paths, but be safe when run from a subdirectory.
    repo = str(Path.cwd()) + "/"
    if path.startswith(repo):
        path = path[len(repo):]
    if module_prefix and path.startswith(module_prefix + "/"):
        path = path[len(module_prefix) + 1:]
    return path


def read_module_prefix(repo_root: Path) -> str:
    go_mod = repo_root / "go.mod"
    if not go_mod.exists():
        return ""
    for line in go_mod.read_text().splitlines():
        if line.strip().startswith("module "):
            return line.strip().split(None, 1)[1].strip()
    return ""


# Aggregation --------------------------------------------------------------

def aggregate(all_findings: list[dict]) -> list[dict]:
    by_file: dict[str, dict] = defaultdict(lambda: {
        "deadcode_count": 0,
        "unused_count": 0,
        "findings": [],
    })
    for f in all_findings:
        entry = by_file[f["path"]]
        if f["tool"] == "deadcode":
            entry["deadcode_count"] += 1
        else:
            entry["unused_count"] += 1
        entry["findings"].append(f)

    out: list[dict] = []
    for path, entry in by_file.items():
        total = entry["deadcode_count"] + entry["unused_count"]
        # Sort findings by line so detail blocks read top-to-bottom.
        entry["findings"].sort(key=lambda f: (f["line"], f["tool"], f["symbol"]))
        out.append({
            "path": path,
            "deadcode_count": entry["deadcode_count"],
            "unused_count": entry["unused_count"],
            "total_findings": total,
            "verdict": _classify(total),
            "findings": entry["findings"],
        })
    return out


# Reporting ----------------------------------------------------------------

def print_stdout(records: list[dict], top: int) -> None:
    from collections import Counter
    tiers = Counter(r["verdict"] for r in records)
    total_findings = sum(r["total_findings"] for r in records)
    total_deadcode = sum(r["deadcode_count"] for r in records)
    total_unused = sum(r["unused_count"] for r in records)
    print(f"Total findings: {total_findings:,} "
          f"(deadcode={total_deadcode:,}  staticcheck U1000={total_unused:,}) "
          f"across {len(records)} files.")
    dist = "  ".join(f"{v}={tiers[v]}" for v in ("HIGH","MEDIUM","LOW","FINE") if tiers[v])
    print(f"Tier distribution: {dist}\n")

    ranked = sorted(records, key=lambda r: (-r["total_findings"], r["path"]))
    print(f"── Top {min(top, len(ranked))} files (most findings first) ──")
    print(f"  {'Verdict':<7}  {'Total':>5}  {'Dead':>4}  {'Unused':>6}  Path")
    for r in ranked[:top]:
        print(f"  {r['verdict']:<7}  {r['total_findings']:>5}  "
              f"{r['deadcode_count']:>4}  {r['unused_count']:>6}  {r['path']}")


# Driver -------------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=__doc__.split("Usage:")[0].strip(),
    )
    ap.add_argument("--paths", nargs="+", default=["./..."],
                    help="Go paths to scan (default ./...)")
    ap.add_argument("--skip-deadcode", action="store_true",
                    help="skip the deadcode call-graph pass")
    ap.add_argument("--skip-staticcheck", action="store_true",
                    help="skip the staticcheck U1000 pass")
    ap.add_argument("--top", type=int, default=25,
                    help="how many files to print on stdout (default 25)")
    ap.add_argument("-o", "--output", "--json", dest="output",
                    help="write JSON report to this path")
    ap.add_argument("--exclude", action="append", default=[],
                    help="extra regex exclude for file paths (repeatable)")
    args = ap.parse_args()

    repo_root = Path.cwd()
    module_prefix = read_module_prefix(repo_root)

    global _EXCLUDE_PATTERNS
    if args.exclude:
        extra = "|".join(args.exclude)
        _EXCLUDE_PATTERNS = re.compile(
            f"({_EXCLUDE_PATTERNS.pattern})|({extra})"
        )

    findings: list[dict] = []
    tools_run: list[str] = []
    if not args.skip_deadcode:
        lines, ok = run_deadcode(args.paths, repo_root)
        if ok:
            tools_run.append("deadcode")
            findings.extend(parse_deadcode(lines, module_prefix))
    if not args.skip_staticcheck:
        raw, ok = run_staticcheck(args.paths, repo_root)
        if ok:
            tools_run.append("staticcheck")
            findings.extend(parse_staticcheck(raw, module_prefix))

    if not tools_run:
        print("error: both analyzers failed or were skipped — no data to report",
              file=sys.stderr)
        return 1

    records = aggregate(findings)
    print_stdout(records, args.top)

    if args.output:
        # ranked view for consumers: worst-first, then alpha.
        ranked = sorted(records, key=lambda r: (-r["total_findings"], r["path"]))
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "dead_code",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "paths": args.paths,
                "excludes": [_EXCLUDE_PATTERNS.pattern],
                "tools_run": tools_run,
                "module_prefix": module_prefix,
            },
            "thresholds": {
                "HIGH_total_findings": _TIER_HIGH,
                "MEDIUM_total_findings": _TIER_MEDIUM,
                "LOW_total_findings": _TIER_LOW,
            },
            "stats": {
                "total_findings": sum(r["total_findings"] for r in records),
                "deadcode_findings": sum(r["deadcode_count"] for r in records),
                "unused_findings": sum(r["unused_count"] for r in records),
                "files_with_findings": len(records),
                "verdict_counts": {v: sum(1 for r in records if r["verdict"] == v)
                                   for v in ("HIGH", "MEDIUM", "LOW", "FINE")},
            },
            "all": records,
            "ranked": ranked,
        }
        Path(args.output).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.output}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
