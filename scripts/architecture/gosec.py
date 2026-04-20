#!/usr/bin/env python3
"""
gosec.py — security signal for Go code via the gosec linter.

Wraps `securego/gosec <https://github.com/securego/gosec>`_, the
de-facto Go security scanner. gosec walks every ``*.go`` file and
emits findings tagged with CWE + severity (HIGH / MEDIUM / LOW) +
confidence. Common categories on a real codebase:

  - unhandled errors (``_ = w.Write(...)``)
  - weak cryptography (MD5 / SHA1 in security-sensitive contexts)
  - SQL string-concat (pre-``database/sql`` parameterised queries)
  - TLS configuration (InsecureSkipVerify, min-version)
  - hard-coded credentials
  - file-permission bits (world-writeable)
  - command injection via ``exec.Command``
  - deprecated / unsafe package use (``unsafe``, ``math/rand`` for crypto)

This is **distinct from every other signal in the suite**: no amount
of refactoring, coverage, coupling, or commit-kind analysis surfaces
"the file does `err := db.Query(); _ = err`". gosec is the one
production-risk lens we're missing.

Not based on the Khononov book — the book is about coupling, not
security — but the cross-signal join works: a package with high
hotspot score + high-severity gosec findings is a live
production-risk hotspot, not just a refactor-velocity one.

Install the tool (one-time):
    go install github.com/securego/gosec/v2/cmd/gosec@latest

Outputs per file:
  - ``severity_counts``    {HIGH: N, MEDIUM: M, LOW: L}
  - ``confidence_counts``  {HIGH / MEDIUM / LOW}
  - ``total_findings``     headline number for ranking
  - ``findings``           list of {line, severity, confidence, rule, cwe, details}
  - ``verdict``            HIGH (any HIGH-severity), MEDIUM (≥ 1 MEDIUM),
                           LOW (≥ 1 LOW), FINE (no findings; filtered out)

gosec considers HIGH-severity × HIGH-confidence the actionable top
tier; HIGH × LOW is typically a false-positive category. We expose
both fields so the reader can slice further.

Schema: ``gosec/1``.

Usage:
    scripts/architecture/gosec.py --json out/gosec.json
    scripts/architecture/gosec.py --paths ./modules/... ./adapters/... --top 30
    scripts/architecture/gosec.py --exclude-rules G104,G204   # skip noisy rules
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


SCHEMA_VERSION = "gosec/1"

_SEVERITY_RANK = {"HIGH": 3, "MEDIUM": 2, "LOW": 1, "INFO": 0}

# Excludes mirror the other scripts. gosec itself ignores vendor/ and
# testdata/ by default, but we still filter the parsed output so path
# keys match the other architecture-script reports.
_EXCLUDE_PATTERNS = re.compile(
    r"(^|/)(vendor|node_modules|testdata|_tmp|books|\.hotspots|\.READ|scripts/out)/"
    r"|_templ\.go$|\.pb\.go$|_mock\.go$|_mocks\.go$"
    r"|(?:^|/)zz_generated|bindata\.go$"
)


def _resolve_binary(name: str) -> str | None:
    resolved = shutil.which(name)
    if resolved:
        return resolved
    gopath = os.environ.get("GOPATH") or os.path.expanduser("~/go")
    candidate = Path(gopath) / "bin" / name
    return str(candidate) if candidate.exists() else None


# -- Tool invocation -------------------------------------------------------

def run_gosec(paths: list[str], cwd: Path,
              exclude_rules: list[str] | None = None,
              include_tests: bool = False) -> tuple[dict, bool]:
    """Run gosec with JSON output and return the parsed dict.

    gosec exits non-zero when it finds anything — we treat that as
    expected (the report contains the findings). Only an empty stdout
    plus non-zero exit is a true failure.
    """
    binary = _resolve_binary("gosec")
    if not binary:
        print("warning: 'gosec' not on PATH or in GOPATH/bin — "
              "install with `go install github.com/securego/gosec/v2/cmd/gosec@latest`",
              file=sys.stderr)
        return {}, False

    target = paths or ["./..."]
    cmd = [binary, "-fmt", "json", "-quiet"]
    if exclude_rules:
        cmd.extend(["-exclude", ",".join(exclude_rules)])
    if not include_tests:
        cmd.append("-tests=false")
    cmd.extend(target)
    print(f"$ {' '.join(cmd)}", file=sys.stderr)
    res = subprocess.run(cmd, cwd=str(cwd), capture_output=True, text=True)
    if not res.stdout and res.returncode != 0:
        print(f"warning: gosec exited {res.returncode}\n{res.stderr}",
              file=sys.stderr)
        return {}, False
    try:
        return json.loads(res.stdout), True
    except json.JSONDecodeError as e:
        print(f"warning: couldn't parse gosec JSON: {e}\n"
              f"first 200 bytes: {res.stdout[:200]!r}",
              file=sys.stderr)
        return {}, False


# -- Parsing + aggregation -------------------------------------------------

def _normalise_path(path: str, repo_root: Path) -> str:
    """gosec emits absolute paths — strip the repo prefix so path keys
    match the other architecture scripts (which use repo-relative
    paths throughout).
    """
    if not path:
        return path
    repo = str(repo_root.resolve()) + "/"
    if path.startswith(repo):
        return path[len(repo):]
    return path


def aggregate_issues(raw: dict, repo_root: Path) -> list[dict]:
    """Return per-file records with severity/confidence rollups."""
    issues = raw.get("Issues") or []
    by_file: dict[str, dict] = defaultdict(lambda: {
        "findings": [],
        "severity_counts": Counter(),
        "confidence_counts": Counter(),
    })
    for issue in issues:
        path = _normalise_path(issue.get("file", ""), repo_root)
        if not path or _EXCLUDE_PATTERNS.search(path):
            continue
        sev = (issue.get("severity") or "").upper()
        conf = (issue.get("confidence") or "").upper()
        entry = by_file[path]
        entry["severity_counts"][sev] += 1
        entry["confidence_counts"][conf] += 1
        line_raw = issue.get("line", "0")
        # gosec reports "23" or "23-24" (ranges); take the first.
        try:
            line = int(str(line_raw).split("-", 1)[0])
        except ValueError:
            line = 0
        entry["findings"].append({
            "line": line,
            "severity": sev,
            "confidence": conf,
            "rule_id": issue.get("rule_id", ""),
            "cwe_id": ((issue.get("cwe") or {}).get("id") or ""),
            "details": issue.get("details", ""),
        })

    records: list[dict] = []
    for path, entry in by_file.items():
        # Verdict = worst severity present.
        verdict = "FINE"
        for sev in ("HIGH", "MEDIUM", "LOW"):
            if entry["severity_counts"].get(sev, 0) > 0:
                verdict = sev
                break
        entry["findings"].sort(key=lambda f: (-_SEVERITY_RANK.get(f["severity"], 0), f["line"]))
        records.append({
            "path": path,
            "total_findings": sum(entry["severity_counts"].values()),
            "severity_counts": dict(entry["severity_counts"]),
            "confidence_counts": dict(entry["confidence_counts"]),
            "findings": entry["findings"],
            "verdict": verdict,
        })
    return records


# -- Reporting -------------------------------------------------------------

def print_stdout(records: list[dict], stats: dict, top: int) -> None:
    total_findings = sum(r["total_findings"] for r in records)
    if not records:
        print("No gosec findings (after exclusions).")
        return
    print(f"Corpus: files={stats.get('files', '?')}  "
          f"lines={stats.get('lines', '?')}  "
          f"nosec={stats.get('nosec', '?')}  "
          f"raw_found={stats.get('found', '?')}")
    print(f"After exclude patterns: {len(records)} files, {total_findings} findings")

    sev_tot = Counter()
    for r in records:
        for k, v in r["severity_counts"].items():
            sev_tot[k] += v
    print("Severity totals: " +
          "  ".join(f"{s}={sev_tot[s]}" for s in ("HIGH", "MEDIUM", "LOW", "INFO") if sev_tot[s]))
    print()

    ranked = sorted(
        records,
        key=lambda r: (-_SEVERITY_RANK.get(r["verdict"], 0),
                       -r["total_findings"], r["path"]),
    )
    print(f"── Top {min(top, len(ranked))} files (worst-first) ──")
    print(f"  {'Verdict':<8}  {'Total':>5}  {'H':>3}  {'M':>3}  {'L':>3}  Path")
    for r in ranked[:top]:
        h = r["severity_counts"].get("HIGH", 0)
        m = r["severity_counts"].get("MEDIUM", 0)
        l = r["severity_counts"].get("LOW", 0)
        print(f"  {r['verdict']:<8}  {r['total_findings']:>5}  "
              f"{h:>3}  {m:>3}  {l:>3}  {r['path']}")


# -- Driver ----------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=__doc__.split("Usage:")[0].strip(),
    )
    ap.add_argument("--paths", nargs="+", default=["./..."],
                    help="gosec target(s) (default ./...)")
    ap.add_argument("--exclude-rules", default="",
                    help="comma-separated gosec rule IDs to suppress (e.g. G104,G204)")
    ap.add_argument("--include-tests", action="store_true",
                    help="scan *_test.go files too (gosec skips them by default)")
    ap.add_argument("--top", type=int, default=25,
                    help="how many files to print on stdout (default 25)")
    ap.add_argument("-o", "--output", "--json", dest="output",
                    help="write JSON report to this path")
    args = ap.parse_args()

    repo_root = Path.cwd()
    exclude_rules = [r.strip() for r in args.exclude_rules.split(",") if r.strip()]
    raw, ok = run_gosec(args.paths, repo_root, exclude_rules, args.include_tests)
    if not ok:
        return 1

    stats = raw.get("Stats", {})
    records = aggregate_issues(raw, repo_root)
    print_stdout(records, stats, args.top)

    if args.output:
        rule_hist: Counter = Counter()
        cwe_hist: Counter = Counter()
        for r in records:
            for f in r["findings"]:
                rule_hist[f.get("rule_id", "")] += 1
                if f.get("cwe_id"):
                    cwe_hist[f["cwe_id"]] += 1
        ranked = sorted(
            records,
            key=lambda r: (-_SEVERITY_RANK.get(r["verdict"], 0),
                           -r["total_findings"], r["path"]),
        )
        total_by_sev: Counter = Counter()
        for r in records:
            for k, v in r["severity_counts"].items():
                total_by_sev[k] += v
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "gosec",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "paths": args.paths,
                "exclude_rules": exclude_rules,
                "include_tests": args.include_tests,
                "excludes": [_EXCLUDE_PATTERNS.pattern],
            },
            "gosec_version": raw.get("GosecVersion"),
            "tiers": {
                "HIGH": "file has ≥ 1 HIGH-severity finding",
                "MEDIUM": "worst severity is MEDIUM",
                "LOW": "worst severity is LOW",
                "FINE": "no findings (filtered out of the list)",
            },
            "stats": {
                "files_scanned": stats.get("files", 0),
                "lines_scanned": stats.get("lines", 0),
                "files_with_findings": len(records),
                "total_findings": sum(r["total_findings"] for r in records),
                "severity_totals": dict(total_by_sev),
                "top_rules": dict(rule_hist.most_common(10)),
                "top_cwes": dict(cwe_hist.most_common(10)),
                "verdict_counts": {v: sum(1 for r in records if r["verdict"] == v)
                                   for v in ("HIGH", "MEDIUM", "LOW")},
            },
            "all": records,
            "ranked": ranked,
        }
        Path(args.output).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.output}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
