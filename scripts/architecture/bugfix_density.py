#!/usr/bin/env python3
"""
bugfix_density.py — per-file bug/fix commit density from git log subjects.

Scans `git log --name-only` in a time window, classifies each commit subject
as fix-flavored or not via a keyword regex, then aggregates per-file:
  - total_commits    total commits touching the file in the window
  - fix_commits      commits whose subject looks like a fix
  - fix_pct          fix_commits / total_commits
  - recent_fix_subjects  up to 5 recent fix-subject samples (qualitative)

Emits a versioned JSON report (schema: bugfix-density/1).

Rationale: change frequency alone conflates feature development with rot. A
file that sees 40 commits because two team-mates are building a feature on
it is not a refactor target. A file that sees 10 commits of which 4 are
"fix" commits IS. This tool gives the cleanest single-signal answer to the
question "how much of this file's churn is triggered by things going wrong?".

Keyword list was chosen for high precision (few false positives) over recall:
  fix, fixes, fixed, fixing, fixup, bug, bugs, bugfix, bugfixes, hotfix,
  regression, broken, crash, crashes, crashed, hang, hangs, freeze,
  freezes, revert, reverts, typo, typos, deadlock, race (race condition),
  leak (mem leak), oom

We DON'T include "patch", "issue", "error" because they frequently mean
non-fix things ("apply patch" can mean a feature patch; "error handling"
can be new code).

Word boundaries matter:
  - "prefix" does NOT match "fix" (no word boundary between 'pre' and 'fix').
  - "affix", "suffix" same. Tested.
"""
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path


SCHEMA_VERSION = "bugfix-density/1"

# High-precision keywords. Grouped with \b on both sides so 'prefix',
# 'suffix', 'refactor' etc. don't trigger false positives.
FIX_KEYWORDS = [
    "fix", "fixes", "fixed", "fixing", "fixup",
    "bug", "bugs", "bugfix", "bugfixes",
    "hotfix",
    "regression", "regressions",
    "broken",
    "crash", "crashes", "crashed",
    "hang", "hangs",
    "freeze", "freezes",
    "revert", "reverts", "reverted",
    "typo", "typos",
    "deadlock",
    # "race condition" — match 'race' only when followed by 'condition' to
    # avoid matching 'race' as a verb/noun in other contexts.
    # Implemented via a separate check below.
    "leak", "leaks",
    "oom",
]

FIX_RE = re.compile(
    r"\b(" + "|".join(re.escape(k) for k in FIX_KEYWORDS) + r")\b",
    re.IGNORECASE,
)

RACE_CONDITION_RE = re.compile(r"\brace\s+condition(s)?\b", re.IGNORECASE)


def is_fix_subject(subject: str) -> bool:
    """Return True if the commit subject looks like a bug/fix commit."""
    if FIX_RE.search(subject):
        return True
    if RACE_CONDITION_RE.search(subject):
        return True
    return False


def parse_git_log(since: str, until: str | None,
                  excludes: list[re.Pattern]) -> tuple[list[dict], dict]:
    """Return (commits, totals). Each commit is
    {hash, date, subject, is_fix, files: [path, ...]}."""
    args = ["git", "log", "--pretty=format:__COMMIT__%x1f%H%x1f%cs%x1f%s",
            "--name-only", "--no-merges"]
    if since:
        args += ["--since", since]
    if until:
        args += ["--until", until]

    out = subprocess.run(args, check=True, capture_output=True, text=True)
    commits: list[dict] = []
    current: dict | None = None
    for raw in out.stdout.splitlines():
        if raw.startswith("__COMMIT__"):
            if current is not None:
                commits.append(current)
            _marker, h, date, subject = raw.split("\x1f", 3)
            current = {
                "hash": h[:12],
                "date": date,
                "subject": subject,
                "is_fix": is_fix_subject(subject),
                "files": [],
            }
        elif raw.strip() and current is not None:
            path = raw.strip()
            if any(pat.search(path) for pat in excludes):
                continue
            current["files"].append(path)
    if current is not None:
        commits.append(current)

    totals = {
        "total_commits": len(commits),
        "fix_commits": sum(1 for c in commits if c["is_fix"]),
    }
    totals["fix_pct_overall"] = (
        round(100 * totals["fix_commits"] / totals["total_commits"], 1)
        if totals["total_commits"] else 0.0
    )
    return commits, totals


def aggregate_per_file(commits: list[dict]) -> dict[str, dict]:
    """Roll up (total_commits, fix_commits, recent_fix_subjects) per file."""
    by_file: dict[str, dict] = defaultdict(lambda: {
        "total_commits": 0,
        "fix_commits": 0,
        "recent_fix_subjects": [],
        "last_fix_date": None,
    })
    # commits come newest-first from git log; iterate in that order so recent
    # subjects are sorted newest-first.
    for c in commits:
        for path in c["files"]:
            entry = by_file[path]
            entry["total_commits"] += 1
            if c["is_fix"]:
                entry["fix_commits"] += 1
                if len(entry["recent_fix_subjects"]) < 5:
                    entry["recent_fix_subjects"].append({
                        "hash": c["hash"],
                        "date": c["date"],
                        "subject": c["subject"],
                    })
                if entry["last_fix_date"] is None or c["date"] > entry["last_fix_date"]:
                    entry["last_fix_date"] = c["date"]

    # Finalize: compute pct, add path field.
    out = {}
    for path, e in by_file.items():
        e["path"] = path
        e["fix_pct"] = (
            round(100 * e["fix_commits"] / e["total_commits"], 1)
            if e["total_commits"] else 0.0
        )
        out[path] = e
    return out


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--since", default="6 months ago")
    ap.add_argument("--until", default=None)
    ap.add_argument("--min-commits", type=int, default=3,
                    help="skip files with fewer than N commits in stdout summary (default 3)")
    ap.add_argument("--exclude", action="append", default=[],
                    help="regex excluding matching file paths (repeatable)")
    ap.add_argument("--top", type=int, default=25,
                    help="how many files to print on stdout (default 25)")
    ap.add_argument("-o", "--output", type=str,
                    help="write JSON report to this path")
    ap.add_argument("--sort", choices=["pct", "count", "recent"],
                    default="count",
                    help="stdout ranking: by fix_pct, fix_count, or last_fix_date (default count)")
    args = ap.parse_args()

    # Default excludes mirror go_hotspots conventions so this tool produces
    # comparable file paths.
    default_excludes = [
        r"_templ\.go$", r"_mock[s]?\.go$", r"\.pb\.go$", r"_gen\.go$",
        r"zz_generated", r"bindata\.go$",
        r"^\.hotspots/", r"^vendor/", r"^node_modules/", r"^testdata/",
        r"^\.READ/",
    ]
    excludes = [re.compile(p) for p in default_excludes + args.exclude]

    commits, totals = parse_git_log(args.since, args.until, excludes)
    by_file = aggregate_per_file(commits)

    # Stdout summary.
    filtered = [e for e in by_file.values() if e["total_commits"] >= args.min_commits]
    if args.sort == "pct":
        filtered.sort(key=lambda e: (-e["fix_pct"], -e["fix_commits"]))
    elif args.sort == "recent":
        filtered.sort(key=lambda e: (e["last_fix_date"] or "", -e["fix_commits"]), reverse=True)
    else:
        filtered.sort(key=lambda e: (-e["fix_commits"], -e["fix_pct"]))

    print(f"Window: since={args.since!r}  until={args.until or '(now)'}")
    print(f"Corpus: {totals['total_commits']} commits "
          f"({totals['fix_commits']} fix-flavored, "
          f"{totals['fix_pct_overall']:.1f}% overall)")
    print(f"Files with ≥{args.min_commits} commits: {len(filtered)}\n")

    print(f"Top {min(args.top, len(filtered))} by {args.sort}:")
    print(f"  {'fixes':>5}/{'total':<5} {'pct':>5}  {'last fix':<10}  path")
    for e in filtered[:args.top]:
        last = e["last_fix_date"] or "—"
        print(f"  {e['fix_commits']:>5}/{e['total_commits']:<5} "
              f"{e['fix_pct']:>4.0f}%  {last:<10}  {e['path']}")

    if args.output:
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "bugfix_density",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "since": args.since,
                "until": args.until,
                "excludes": [p.pattern for p in excludes],
            },
            "keywords": FIX_KEYWORDS + ["race condition"],
            "stats": {
                **totals,
                "files_tracked": len(by_file),
                "files_with_any_fix": sum(1 for e in by_file.values() if e["fix_commits"] > 0),
            },
            "files": sorted(by_file.values(),
                            key=lambda e: (-e["fix_commits"], -e["fix_pct"])),
        }
        Path(args.output).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.output}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
