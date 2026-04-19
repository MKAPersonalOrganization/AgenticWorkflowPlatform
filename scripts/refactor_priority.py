#!/usr/bin/env python3
"""
refactor_priority.py — cross-signal data report.

Collects per-file measurements from four analyses and presents them in one
document. Makes NO judgments: no scores, no tiers, no effort estimates, no
priority rankings, no suggested actions.

Inputs (all JSON from sibling tools):
  --hotspots      go_hotspots.py --json (repeatable)
  --trends        complexity_trend.py --discover --json
  --coupling      file_coupling.py -o
  --bugfix        bugfix_density.py -o (optional)

Outputs:
  stdout          compact table of files with ≥1 signal, sorted by number
                  of signals they appear in, then alphabetical
  --out-md FILE   markdown report with raw per-file data + detail blocks
  --out-json FILE machine-readable merged data

Philosophy: the tool's job is to surface evidence. The human reads the
evidence and decides what to refactor, in what order. Any prioritization
inherited from the tool would inherit its author's opinion about weights
and thresholds.
"""
from __future__ import annotations

import argparse
import json
import sys
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path


# Signal ingestion ---------------------------------------------------------

def load_hotspots(paths: list[str]) -> tuple[dict[str, dict], list[dict]]:
    """Return (by_file, packages). Accepts multiple JSON files so callers can
    merge a package-mode repo scan with function-mode drill-ins."""
    by_file: dict[str, dict] = {}
    packages: list[dict] = []
    all_hs: list[dict] = []
    for path in paths:
        d = json.loads(Path(path).read_text())
        all_hs.extend(d.get("hotspots", []))

    for h in all_hs:
        kind = h.get("kind")
        path = h.get("file_path")
        if kind == "package":
            packages.append({
                "name": h.get("import_path") or h.get("name"),
                "score": h["metrics"]["score"],
                "revisions": h["metrics"]["revisions"],
                "loc": h["metrics"]["loc"],
            })
            continue
        if not path:
            continue
        entry = by_file.setdefault(path, {
            "path": path,
            "total_score": 0,
            "functions": [],
            "signals": set(),
            "max_bug_fix_pct": 0.0,
        })
        entry["total_score"] += h["metrics"]["score"]
        bf_pct = h.get("bug_fix_pct") or 0.0
        if bf_pct > entry["max_bug_fix_pct"]:
            entry["max_bug_fix_pct"] = bf_pct
        entry["functions"].append({
            "name": h.get("name"),
            "lines": f"L{h.get('start_line')}-{h.get('end_line')}",
            "loc": h["metrics"]["loc"],
            "revisions": h["metrics"]["revisions"],
            "score": h["metrics"]["score"],
            "bug_fix_pct": bf_pct,
        })
        for s in h.get("signals") or []:
            entry["signals"].add(s.get("kind"))

    for e in by_file.values():
        e["signals"] = sorted(e["signals"])
    return by_file, packages


def load_trends(path: str) -> dict[str, dict]:
    """Return by-file map. Uses the `all` field which includes every screened
    file, not just those classified DETERIORATING."""
    d = json.loads(Path(path).read_text())
    entries = d.get("all") or d.get("ranked") or []
    return {e["path"]: e for e in entries}


def load_bugfix(path: str | None) -> dict[str, dict]:
    if not path:
        return {}
    d = json.loads(Path(path).read_text())
    return {f["path"]: f for f in d.get("files", [])}


def load_coupling(path: str, cooc_floor: int = 6) -> dict[str, list[dict]]:
    """Return by-file map of coupling partners."""
    d = json.loads(Path(path).read_text())
    pairs = d.get("top_pairs_normalized") or d.get("top_pairs") or []
    by_file: dict[str, list[dict]] = defaultdict(list)
    for p in pairs:
        cooc = p.get("cooccur", 0)
        if cooc < cooc_floor:
            continue
        f1, f2 = p.get("file1"), p.get("file2")
        if not (f1 and f2):
            continue
        for src, dst in ((f1, f2), (f2, f1)):
            by_file[src].append({
                "partner": dst,
                "cooccur": cooc,
                "jaccard": p.get("jaccard", 0),
                "structural": p.get("structural", False),
            })
    return by_file


# Collation ----------------------------------------------------------------

def collate(
    hotspots_by_file: dict[str, dict],
    trends_by_file: dict[str, dict],
    coupling_by_file: dict[str, list[dict]],
    bugfix_by_file: dict[str, dict],
) -> list[dict]:
    """Merge per-file data from every signal. No ranking, no scoring — just
    a record listing which analyses touched each file and what they measured.
    """
    all_paths = (
        set(hotspots_by_file) | set(trends_by_file) | set(coupling_by_file)
        | set(bugfix_by_file)
    )
    records: list[dict] = []
    for path in all_paths:
        hs = hotspots_by_file.get(path)
        tr = trends_by_file.get(path)
        cp = coupling_by_file.get(path) or []
        bf = bugfix_by_file.get(path)

        present_in: list[str] = []
        if hs: present_in.append("hotspot")
        if tr: present_in.append("trend")
        if cp: present_in.append("coupling")
        if bf: present_in.append("bugfix")

        records.append({
            "path": path,
            "present_in": present_in,
            "signal_count": len(present_in),
            "hotspot": hs,
            "trend": tr,
            "coupling": cp,
            "bugfix": bf,
        })

    # Order: files present in more analyses first (a factual count, not a
    # rank), then alphabetical within each group for a neutral presentation.
    records.sort(key=lambda r: (-r["signal_count"], r["path"]))
    return records


# Reporting ----------------------------------------------------------------

def _trunc(s: str, n: int) -> str:
    return s if len(s) <= n else s[: n - 1] + "…"


def print_stdout(records: list[dict], packages: list[dict]) -> None:
    print("\n── Top package hotspots (raw score, revs, LOC) ──")
    for p in packages[:10]:
        print(f"  score={p['score']:>7}  revs={p['revisions']:>3}  loc={p['loc']:>5}  {p['name']}")

    print(f"\n── Files with ≥1 signal ({len(records)} total) ──")
    hdr = (f"{'Sig':<3}  {'Commits':>7}  {'Fix%':>5}  "
           f"{'Trend':<12}  {'Δmean':>6}  {'MaxI':>4}  "
           f"{'Cpl':>3}  Path")
    print(hdr)
    print("-" * len(hdr))
    for r in records:
        if r["signal_count"] == 0:
            continue
        commits = (r["bugfix"]["total_commits"] if r["bugfix"]
                   else (r["trend"]["commits"] if r["trend"] else ""))
        fix_pct = f"{r['bugfix']['fix_pct']:.0f}" if r["bugfix"] else ""
        trend_verdict = r["trend"]["verdict"] if r["trend"] else ""
        tm = (r["trend"] or {}).get("metrics") or {}
        dmean = f"{tm['mean_delta']:+.2f}" if "mean_delta" in tm else ""
        maxi = f"{tm['last_max']:.0f}" if "last_max" in tm else ""
        cpl = len(r["coupling"]) if r["coupling"] else ""
        print(f"{r['signal_count']:<3}  {str(commits):>7}  {fix_pct:>5}  "
              f"{_trunc(trend_verdict, 12):<12}  {dmean:>6}  {maxi:>4}  "
              f"{str(cpl):>3}  {r['path']}")


def to_markdown(records: list[dict], packages: list[dict], meta: dict,
                md_min_signals: int = 2, md_min_commits: int = 3) -> str:
    lines: list[str] = []
    lines.append("# Code Signals — Data Report\n")
    lines.append(f"**Generated:** {meta['generated_at']}  **Window:** {meta['window']}\n")
    lines.append(
        "This document **collects measurements**. It does not rank, prioritize, "
        "or recommend. Files are grouped by how many analyses mentioned them "
        "(an objective count), and within each group listed alphabetically."
    )
    lines.append("")
    lines.append(
        f"**Markdown scope.** To keep this document readable, files shown "
        f"here must appear in ≥ {md_min_signals} analyses AND have ≥ {md_min_commits} "
        f"commits in the window. The JSON report emitted alongside this file "
        f"contains the full, unfiltered dataset."
    )
    lines.append("")

    lines.append("## What each column means\n")
    lines.append("| Column | Source | What it is |")
    lines.append("|---|---|---|")
    lines.append("| hotspot-score | `go_hotspots.py` | Revisions × effective LOC for Go packages/functions |")
    lines.append("| trend-verdict | `complexity_trend.py` | Classification by threshold rules: DETERIORATING (Δmean > 0.08 or Δtotal > ΔLOC+15%), REFACTORED (peak-dip > 12%), STABLE (|Δtotal| < 5% & |Δmean| < 0.05), GROWING (otherwise) |")
    lines.append("| Δmean | `complexity_trend.py` | Last sample mean indent minus first sample mean indent (whole window) |")
    lines.append("| max-indent | `complexity_trend.py` | Deepest whitespace-indent in the last sampled revision |")
    lines.append("| fix-pct | `bugfix_density.py` | Fraction of this file's commits whose subject line matched fix keywords (`fix`, `bug`, `hotfix`, `regression`, `crash`, `revert`, `typo`, etc., word-bounded) |")
    lines.append("| coupling count | `file_coupling.py` | Number of partner files that co-change with this one at cooccurrence ≥ 6 |")
    lines.append("")

    # Top packages — orientation.
    lines.append("## Top package hotspots (orientation)\n")
    lines.append("| Score | Revs | LOC | Package |")
    lines.append("|---:|---:|---:|---|")
    for p in packages[:10]:
        lines.append(f"| {p['score']:,} | {p['revisions']} | {p['loc']:,} | `{p['name']}` |")
    lines.append("")

    def _visible(r: dict) -> bool:
        if r["signal_count"] < md_min_signals:
            return False
        # Use the highest commit count from any signal as the filter metric.
        c = 0
        if r["bugfix"]:
            c = max(c, r["bugfix"]["total_commits"])
        if r["trend"]:
            c = max(c, r["trend"].get("commits", 0))
        if r["hotspot"]:
            c = max(c, max((f["revisions"] for f in r["hotspot"]["functions"]), default=0))
        return c >= md_min_commits

    visible = [r for r in records if _visible(r)]

    lines.append(f"## File overview ({len(visible)} files)\n")
    lines.append("| File | Signals | Commits | Fix count | Fix % | Trend verdict | Δmean | Max indent | Hotspot score | Coupling partners |")
    lines.append("|---|:-:|---:|---:|---:|:-:|---:|---:|---:|---:|")
    for r in visible:
        commits = (r["bugfix"]["total_commits"] if r["bugfix"]
                   else (r["trend"]["commits"] if r["trend"] else ""))
        fix_n = r["bugfix"]["fix_commits"] if r["bugfix"] else ""
        fix_p = f"{r['bugfix']['fix_pct']:.0f}" if r["bugfix"] else ""
        verdict = r["trend"]["verdict"] if r["trend"] else ""
        tm = (r["trend"] or {}).get("metrics") or {}
        dmean = f"{tm['mean_delta']:+.2f}" if "mean_delta" in tm else ""
        maxi = f"{tm['last_max']:.0f}" if "last_max" in tm else ""
        hs_score = f"{r['hotspot']['total_score']:,}" if r["hotspot"] else ""
        cpl = len(r["coupling"]) if r["coupling"] else ""
        lines.append(
            f"| `{r['path']}` | {r['signal_count']} | "
            f"{commits} | {fix_n} | {fix_p} | {verdict} | "
            f"{dmean} | {maxi} | {hs_score} | {cpl} |"
        )
    lines.append("")

    # Per-file detail blocks — grouped by signal count, over the same
    # visibility filter as the overview table.
    groups: dict[int, list[dict]] = defaultdict(list)
    for r in visible:
        groups[r["signal_count"]].append(r)

    for n in sorted(groups, reverse=True):
        lines.append(f"## Detail — files in {n} {'analyses' if n > 1 else 'analysis'}\n")
        for r in groups[n]:
            lines.extend(_detail_block(r))

    lines.append("## Notes on the data\n")
    lines.append(
        "- Whitespace-indent is a proxy for McCabe/Halstead complexity "
        "(Hindle–Godfrey–Holt, 2008); it correlates with but does not equal "
        "semantic complexity."
    )
    lines.append(
        "- `complexity_trend` only observes files currently on disk. Files "
        "deleted or rewritten do not appear — absence from this report is not "
        "evidence of health."
    )
    lines.append(
        "- `bugfix_density` classifies commits by keyword matches on the "
        "subject line. A commit whose subject lies (e.g. `feat:` for a fix) "
        "is not counted."
    )
    lines.append(
        "- `file_coupling` measures temporal co-change — it does not decide "
        "whether that coupling is intended or accidental."
    )
    lines.append("")

    lines.append("## Reproduce\n")
    lines.append("```bash")
    lines.append("task refactor:plan")
    lines.append("```\n")
    return "\n".join(lines)


def _detail_block(r: dict) -> list[str]:
    out: list[str] = []
    out.append(f"### `{r['path']}`  (signals: {', '.join(r['present_in'])})\n")

    if r["hotspot"]:
        hs = r["hotspot"]
        out.append(
            f"**go_hotspots** — file-aggregate score {hs['total_score']:,} "
            f"across {len(hs['functions'])} hot function(s):"
        )
        for f in sorted(hs["functions"], key=lambda x: -x["score"]):
            bf = f" · bug-fix {f['bug_fix_pct']:.0f}%" if f.get("bug_fix_pct") else ""
            out.append(f"- `{f['name']}` {f['lines']} — score {f['score']:,}, "
                       f"{f['revisions']} revs, {f['loc']} LOC{bf}")
        if hs["signals"]:
            out.append(f"\nHotspot signals: {', '.join(hs['signals'])}.")
        out.append("")

    if r["trend"]:
        tr = r["trend"]
        m = tr.get("metrics") or {}
        out.append(
            f"**complexity_trend** — classified **{tr['verdict']}** by "
            f"threshold rules (see column legend above)."
        )
        out.append(f"- samples: {tr.get('samples', 'n/a')}  ·  commits observed: {tr.get('commits', 'n/a')}  ·  LOC: {tr.get('loc', 'n/a')}")
        first = tr.get("first") or {}
        last = tr.get("last") or {}
        if first:
            out.append(
                f"- first sample: n={first.get('n', 'n/a')}  total={first.get('total', 'n/a')}  "
                f"mean={first.get('mean', 0):.2f}  max={first.get('max', 0):.0f}"
            )
        if last:
            out.append(
                f"- last sample:  n={last.get('n', 'n/a')}  total={last.get('total', 'n/a')}  "
                f"mean={last.get('mean', 0):.2f}  max={last.get('max', 0):.0f}"
            )
        if m:
            out.append(
                f"- Δtotal {m.get('total_delta_pct', 0):+.0f}%  ·  ΔLOC {m.get('loc_delta_pct', 0):+.0f}%  "
                f"·  Δmean indent {m.get('mean_delta', 0):+.2f}  ·  peak-dip {m.get('refactor_dip_pct', 0):.0f}%"
            )
            out.append(
                f"- recent window (last {m.get('recent_window', 0)} samples): "
                f"Δtotal {m.get('recent_total_delta_pct', 0):+.0f}%  ·  "
                f"Δmean {m.get('recent_mean_delta', 0):+.2f}"
            )
        out.append("")

    if r["bugfix"]:
        bf = r["bugfix"]
        out.append(
            f"**bugfix_density** — {bf['fix_commits']} fix-flavored commits "
            f"out of {bf['total_commits']} touches "
            f"({bf['fix_pct']:.0f}%). Last fix: {bf.get('last_fix_date') or 'n/a'}."
        )
        for s in bf.get("recent_fix_subjects", [])[:5]:
            subj = s["subject"]
            if len(subj) > 100:
                subj = subj[:97] + "..."
            out.append(f"- `{s['hash']}` ({s['date']}): {subj}")
        out.append("")

    if r["coupling"]:
        out.append(
            f"**file_coupling** — {len(r['coupling'])} partner(s) "
            f"co-changing at cooccurrence ≥ 6:"
        )
        for p in sorted(r["coupling"], key=lambda p: -p["cooccur"]):
            marker = " (structural Go import)" if p["structural"] else ""
            out.append(f"- `{p['partner']}` — {p['cooccur']:.0f} co-changes, J={p['jaccard']:.0f}{marker}")
        out.append("")

    return out


# Driver -------------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--hotspots", required=True, action="append",
                    help="go_hotspots.py --json output (repeatable)")
    ap.add_argument("--trends", required=True, help="complexity_trend.py --json output")
    ap.add_argument("--coupling", required=True, help="file_coupling.py -o output")
    ap.add_argument("--bugfix", help="bugfix_density.py -o output (optional)")
    ap.add_argument("--out-md", type=str, help="write markdown data report to this path")
    ap.add_argument("--out-json", type=str, help="write merged data JSON to this path")
    ap.add_argument("--cooccur-floor", type=int, default=6,
                    help="coupling co-occurrence threshold (default 6)")
    ap.add_argument("--md-min-signals", type=int, default=2,
                    help="in markdown, only show files present in ≥ N analyses (default 2); JSON always has the full dataset")
    ap.add_argument("--md-min-commits", type=int, default=3,
                    help="in markdown, only show files with ≥ N commits (default 3); JSON always has the full dataset")
    ap.add_argument("--window", default="6 months",
                    help="description for the markdown header (default '6 months')")
    args = ap.parse_args()

    try:
        hotspots_by_file, packages = load_hotspots(args.hotspots)
        trends_by_file = load_trends(args.trends)
        coupling_by_file = load_coupling(args.coupling, args.cooccur_floor)
        bugfix_by_file = load_bugfix(args.bugfix)
    except FileNotFoundError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1

    records = collate(hotspots_by_file, trends_by_file, coupling_by_file, bugfix_by_file)
    print_stdout(records, packages)

    meta = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "window": args.window,
    }

    if args.out_json:
        Path(args.out_json).write_text(json.dumps({
            "schema": "code-signals/1",
            "meta": meta,
            "packages": packages,
            "records": records,
        }, indent=2, default=str))
        print(f"\nJSON: {args.out_json}")

    if args.out_md:
        Path(args.out_md).write_text(to_markdown(
            records, packages, meta,
            md_min_signals=args.md_min_signals,
            md_min_commits=args.md_min_commits,
        ))
        print(f"Markdown: {args.out_md}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
