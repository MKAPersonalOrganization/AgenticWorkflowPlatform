#!/usr/bin/env python3
"""
commit_kinds.py — conventional-commit prefix classification per file.

Why: ``bugfix_density.py`` tells you which files absorb ``fix:``-flavored
commits, but nothing about the other 75% of history. A file with 37
``feat:`` and 0 ``refactor:`` is actively growing (don't pile on); a file
with 12 ``refactor:`` and 3 ``fix:`` is being actively improved (refactor
fatigue risk); a file with 9 ``fix:`` and 2 ``feat:`` is a bug magnet
that's never been preventatively repaired. The three pictures look
identical to ``bugfix_density`` alone — their `fix_pct` can even match.

What it does:

  1. Scan ``git log`` in a time window (reusing bugfix_density's
     parser shape so the two signals draw from the same corpus).
  2. Classify each commit subject by its conventional-commit prefix
     (``feat:``, ``fix:``, ``refactor:``, ``chore:``, ``docs:``,
     ``test:``, ``perf:``, ``build:``, ``ci:``, ``style:``, ``revert:``),
     with the scope suffix ``feat(foo):`` also accepted.
  3. Roll per-file kind counts + dominant kind + Shannon entropy of
     the kind distribution (0 = monoculture, log2(K) ≈ 3.5 bits for a
     uniform K-kind mix).
  4. Emit a verdict tier for quick filtering:
       HIGH     fix_pct ≥ 50% and ≥ 5 commits  — bug magnet
       MEDIUM   fix_pct ≥ 30% and ≥ 5 commits
       LOW      fix_pct ≥ 10%  OR  refactor_pct ≥ 30%
       FINE     predominantly feat/chore/docs — healthy growth
       NONE     commits but no classifiable prefix (legacy-style messages)
       QUIET    < min_commits in the window (not a signal either way)

Overlap with bugfix_density: both flag fix-flavored commits but with
different matchers. ``bugfix_density`` keyword-matches across the whole
subject (``"bugfix"``, ``"hotfix"``, ``"typo"``, ``"crash"``, …).
``commit_kinds`` prefix-matches at the start of the subject. Projects
that use conventional commits get near-identical ``fix_pct`` from both;
projects that don't diverge — bugfix_density picks up ``"Fixed foo"``,
commit_kinds picks up ``"fix(auth): foo"``. Running both triangulates.

Emits a versioned JSON report (schema: commit-kinds/1).
"""
from __future__ import annotations

import argparse
import json
import math
import re
import subprocess
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path


SCHEMA_VERSION = "commit-kinds/1"

# Conventional-commit kinds we recognise. ``revert`` folds into ``fix``
# because a revert-commit is definitionally a repair ("this change
# broke something; undo it"), matching bugfix_density's treatment.
_KIND_CANONICAL = {
    "feat":     "feat",
    "feature":  "feat",
    "fix":      "fix",
    "bugfix":   "fix",
    "hotfix":   "fix",
    "revert":   "fix",
    "refactor": "refactor",
    "refact":   "refactor",
    "rework":   "refactor",
    "chore":    "chore",
    "deps":     "chore",
    "docs":     "docs",
    "doc":      "docs",
    "test":     "test",
    "tests":    "test",
    "perf":     "perf",
    "build":    "build",
    "ci":       "ci",
    "style":    "style",
    "fmt":      "style",
}

# Canonical kinds in display order. "other" is the catch-all for subjects
# that carry no recognisable prefix.
_KIND_ORDER = ["feat", "fix", "refactor", "chore", "docs", "test",
               "perf", "build", "ci", "style", "other"]

# Matches "kind:" or "kind(scope):" at the start of the subject,
# case-insensitive. Captures kind in group 1.
_PREFIX_RE = re.compile(r"^\s*([a-zA-Z]+)(?:\([^)]*\))?\s*[:!]\s*", re.IGNORECASE)


def classify_subject(subject: str) -> str:
    """Return the canonical kind for a commit subject, or ``other``."""
    m = _PREFIX_RE.match(subject)
    if not m:
        return "other"
    raw = m.group(1).lower()
    return _KIND_CANONICAL.get(raw, "other")


def _entropy(counts: dict[str, int]) -> float:
    """Shannon entropy in bits over a count distribution. 0 if
    ``counts`` is empty or single-kind.
    """
    total = sum(counts.values())
    if total == 0:
        return 0.0
    h = 0.0
    for n in counts.values():
        if n == 0:
            continue
        p = n / total
        h -= p * math.log2(p)
    return round(h, 3)


# Git ingestion (same shape as bugfix_density.parse_git_log) ---------------

def parse_git_log(since: str, until: str | None,
                  excludes: list[re.Pattern]) -> list[dict]:
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
                "kind": classify_subject(subject),
                "files": [],
            }
        elif raw.strip() and current is not None:
            path = raw.strip()
            if any(pat.search(path) for pat in excludes):
                continue
            current["files"].append(path)
    if current is not None:
        commits.append(current)
    return commits


# Aggregation --------------------------------------------------------------

def aggregate_per_file(commits: list[dict]) -> dict[str, dict]:
    """Roll up per-file kind counts + recent-subject samples per kind."""
    by_file: dict[str, dict] = defaultdict(lambda: {
        "total_commits": 0,
        "kinds": Counter(),
        "recent_subjects": defaultdict(list),  # kind → [{hash,date,subject}]
    })
    for c in commits:
        for path in c["files"]:
            entry = by_file[path]
            entry["total_commits"] += 1
            entry["kinds"][c["kind"]] += 1
            samples = entry["recent_subjects"][c["kind"]]
            if len(samples) < 3:
                samples.append({
                    "hash": c["hash"],
                    "date": c["date"],
                    "subject": c["subject"],
                })
    out: dict[str, dict] = {}
    for path, entry in by_file.items():
        kinds = dict(entry["kinds"])
        total = entry["total_commits"]
        # Derived fields: dominant kind (ties break alphabetically),
        # percentages for the main three pivot kinds, entropy, prefix
        # match rate (1 - other/total).
        dominant = max(kinds.items(), key=lambda kv: (kv[1], -_KIND_ORDER.index(kv[0])))[0] if kinds else "other"
        fix_pct = round(100 * kinds.get("fix", 0) / total, 1) if total else 0.0
        feat_pct = round(100 * kinds.get("feat", 0) / total, 1) if total else 0.0
        refactor_pct = round(100 * kinds.get("refactor", 0) / total, 1) if total else 0.0
        prefix_match_rate = round(100 * (1 - kinds.get("other", 0) / total), 1) if total else 0.0
        out[path] = {
            "path": path,
            "total_commits": total,
            "kinds": {k: kinds.get(k, 0) for k in _KIND_ORDER if kinds.get(k, 0)},
            "dominant": dominant,
            "fix_pct": fix_pct,
            "feat_pct": feat_pct,
            "refactor_pct": refactor_pct,
            "prefix_match_rate": prefix_match_rate,
            "entropy_bits": _entropy(kinds),
            "recent_subjects": {k: v for k, v in entry["recent_subjects"].items() if v},
        }
    return out


def _verdict(entry: dict, min_commits: int) -> str:
    total = entry["total_commits"]
    if total < min_commits:
        return "QUIET"
    if entry["prefix_match_rate"] < 25:
        return "NONE"
    if entry["fix_pct"] >= 50 and total >= 5:
        return "HIGH"
    if entry["fix_pct"] >= 30 and total >= 5:
        return "MEDIUM"
    if entry["fix_pct"] >= 10 or entry["refactor_pct"] >= 30:
        return "LOW"
    return "FINE"


# Reporting ----------------------------------------------------------------

def _compact_kinds(kinds: dict[str, int]) -> str:
    # "feat 9, fix 3, refactor 2" — skip zero-count kinds for brevity.
    return ", ".join(f"{k} {kinds[k]}" for k in _KIND_ORDER if kinds.get(k))


def print_stdout(records: list[dict], top: int, min_commits: int) -> None:
    by_verdict: Counter = Counter(r["verdict"] for r in records)
    print(f"Classified {sum(r['total_commits'] for r in records):,} commit-file "
          f"touches across {len(records)} files.")
    print(f"Verdict distribution: " +
          "  ".join(f"{v}={by_verdict[v]}" for v in ("HIGH","MEDIUM","LOW","FINE","NONE","QUIET") if by_verdict[v]))

    # Rank worst-first: HIGH first by fix count, then MEDIUM, then files
    # with highest refactor_pct so "being actively reshaped" is visible.
    severity = {"HIGH": 0, "MEDIUM": 1, "LOW": 2, "NONE": 3, "FINE": 4, "QUIET": 5}
    rows = [r for r in records if r["total_commits"] >= min_commits]
    rows.sort(key=lambda r: (
        severity.get(r["verdict"], 99),
        -r["kinds"].get("fix", 0),
        -r["total_commits"],
        r["path"],
    ))
    print(f"\n── Top {min(top, len(rows))} files (worst-first) ──")
    print(f"  {'Verdict':<7}  {'Commits':>7}  {'Fix%':>5}  {'Feat%':>5}  "
          f"{'Refac%':>6}  {'Dom':<9}  {'Entropy':>7}  Kinds")
    for r in rows[:top]:
        print(f"  {r['verdict']:<7}  {r['total_commits']:>7}  "
              f"{r['fix_pct']:>4.0f}%  {r['feat_pct']:>4.0f}%  "
              f"{r['refactor_pct']:>5.0f}%  {r['dominant']:<9}  "
              f"{r['entropy_bits']:>7.2f}  {_compact_kinds(r['kinds'])}")
        # Path on its own line so long paths don't break the table.
        print(f"    {r['path']}")


# Driver -------------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=__doc__.split("What it does:")[0].strip(),
    )
    ap.add_argument("--since", default="6 months ago")
    ap.add_argument("--until", default=None)
    ap.add_argument("--min-commits", type=int, default=3,
                    help="hide files with fewer than N commits in stdout (default 3)")
    ap.add_argument("--exclude", action="append", default=[],
                    help="regex excluding matching file paths (repeatable)")
    ap.add_argument("--top", type=int, default=25,
                    help="how many files to print on stdout (default 25)")
    ap.add_argument("-o", "--output", "--json", dest="output",
                    help="write JSON report to this path")
    args = ap.parse_args()

    default_excludes = [
        r"_templ\.go$", r"_mock[s]?\.go$", r"\.pb\.go$", r"_gen\.go$",
        r"zz_generated", r"bindata\.go$",
        r"^\.hotspots/", r"^vendor/", r"^node_modules/", r"^testdata/",
        r"^\.READ/",
    ]
    excludes = [re.compile(p) for p in default_excludes + args.exclude]

    commits = parse_git_log(args.since, args.until, excludes)
    by_file = aggregate_per_file(commits)

    # Verdict pass after aggregation so we have the percentages available.
    records = list(by_file.values())
    for r in records:
        r["verdict"] = _verdict(r, args.min_commits)

    print(f"Window: since={args.since!r}  until={args.until or '(now)'}")
    corpus_kinds = Counter(c["kind"] for c in commits)
    print(f"Corpus: {len(commits)} commits — " + _compact_kinds(corpus_kinds))
    print(f"Files with ≥{args.min_commits} commits: "
          f"{sum(1 for r in records if r['total_commits'] >= args.min_commits)}\n")

    print_stdout(records, args.top, args.min_commits)

    if args.output:
        # Sort the persisted file list by the same severity order stdout
        # uses so consumers that just take `all[:N]` get usable output.
        severity = {"HIGH": 0, "MEDIUM": 1, "LOW": 2, "NONE": 3, "FINE": 4, "QUIET": 5}
        ranked = sorted(records, key=lambda r: (
            severity.get(r["verdict"], 99),
            -r["kinds"].get("fix", 0),
            -r["total_commits"],
            r["path"],
        ))
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "commit_kinds",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "since": args.since,
                "until": args.until,
                "excludes": [p.pattern for p in excludes],
                "min_commits": args.min_commits,
            },
            "thresholds": {
                "HIGH_fix_pct": 50,
                "MEDIUM_fix_pct": 30,
                "LOW_fix_pct": 10,
                "LOW_refactor_pct": 30,
                "min_commits_for_severity": 5,
            },
            "stats": {
                "total_commits": len(commits),
                "files_tracked": len(records),
                "corpus_kinds": dict(corpus_kinds),
                "verdict_counts": {v: c for v, c in Counter(r["verdict"] for r in records).items()},
            },
            "all": records,
            "ranked": ranked,
        }
        Path(args.output).write_text(json.dumps(report, indent=2, default=str))
        print(f"\nJSON report: {args.output}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
