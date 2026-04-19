#!/usr/bin/env python3
"""
refactor_priority.py — cross-signal data report.

Collects per-file measurements from several analyses and presents them
in one document. Makes NO judgments: no scores, no tiers, no effort
estimates, no priority rankings, no suggested actions.

Inputs (all JSON from sibling tools in scripts/architecture/):
  --hotspots      go_hotspots.py --json (repeatable)
  --trends        complexity_trend.py --discover --json
  --coupling      file_coupling.py -o
  --bugfix        bugfix_density.py -o (optional)
  --lcom4         cohesion.py --json (optional; Go files only)
  --cyclomatic    cyclomatic.py --json (optional; multi-language)

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


def load_lcom4(path: str | None, min_lcom4: int = 2) -> dict[str, dict]:
    """Return by-file map of LCOM4 findings for Go files.

    Only files whose current max LCOM4 meets the threshold are kept — a
    cohesive file (LCOM4=1) is not a signal worth counting. The shape
    mirrors the other loaders: a flat per-file record with the bits the
    collator and reporters need (current max, delta, worst type, method
    count, verdict).

    The raw ``types`` list is preserved so detail blocks can surface
    which type is fracturing and what its disjoint method groups are —
    that's the insight LCOM4 provides that other signals can't.
    """
    if not path:
        return {}
    d = json.loads(Path(path).read_text())
    entries = d.get("all") or d.get("ranked") or []
    out: dict[str, dict] = {}
    for e in entries:
        last = e.get("last") or {}
        max_lcom4 = last.get("max_lcom4") or 0
        if max_lcom4 < min_lcom4:
            continue
        metrics = e.get("metrics") or {}
        types = last.get("types") or []
        # The "worst" type = highest LCOM4; ties broken by method count.
        worst = max(types, key=lambda t: (t.get("lcom4", 0), t.get("methods", 0)), default=None)
        out[e["path"]] = {
            "path": e["path"],
            "verdict": e.get("verdict", ""),
            "max_lcom4": max_lcom4,
            "lcom4_delta": metrics.get("lcom4_delta", 0),
            "total_methods": last.get("total_methods", 0),
            "worst_type": worst,
            "types": types,
            "score": e.get("score", 0),
        }
    return out


def load_cyclomatic(path: str | None, min_verdict: str = "LOW") -> dict[str, dict]:
    """Return by-file map of cyclomatic complexity findings (from lizard).

    Keeps files whose verdict is at least as severe as ``min_verdict``
    — FINE files (every function under the threshold) aren't a signal
    worth counting, matching how the other loaders filter. Preserves
    the top-offender list so detail blocks can name-and-line the
    specific complex functions.
    """
    if not path:
        return {}
    severity_rank = {"FINE": 0, "LOW": 1, "MEDIUM": 2, "HIGH": 3}
    floor = severity_rank.get(min_verdict, 1)
    d = json.loads(Path(path).read_text())
    # cyclomatic.py's JSON always carries `all`; `ranked` is just a
    # pre-sorted slice for stdout-style consumers.
    entries = d.get("all") or []
    out: dict[str, dict] = {}
    for e in entries:
        verdict = e.get("verdict", "FINE")
        if severity_rank.get(verdict, 0) < floor:
            continue
        # Normalise the path so it matches what the other loaders use.
        # cyclomatic.py reports paths as lizard sees them (e.g. "./foo.go");
        # other tools use "foo.go". Strip the leading "./".
        path_key = e["path"]
        if path_key.startswith("./"):
            path_key = path_key[2:]
        out[path_key] = {
            "path": path_key,
            "verdict": verdict,
            "max_ccn": e.get("max_ccn", 0),
            "mean_ccn": e.get("mean_ccn", 0.0),
            "high_ccn_count": e.get("high_ccn_count", 0),
            "function_count": e.get("function_count", 0),
            "total_nloc": e.get("total_nloc", 0),
            "top_functions": e.get("top_functions", []),
        }
    return out


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
    lcom4_by_file: dict[str, dict] | None = None,
    cyclomatic_by_file: dict[str, dict] | None = None,
) -> list[dict]:
    """Merge per-file data from every signal. No ranking, no scoring — just
    a record listing which analyses touched each file and what they measured.
    """
    lcom4_by_file = lcom4_by_file or {}
    cyclomatic_by_file = cyclomatic_by_file or {}
    all_paths = (
        set(hotspots_by_file) | set(trends_by_file) | set(coupling_by_file)
        | set(bugfix_by_file) | set(lcom4_by_file) | set(cyclomatic_by_file)
    )
    records: list[dict] = []
    for path in all_paths:
        hs = hotspots_by_file.get(path)
        tr = trends_by_file.get(path)
        cp = coupling_by_file.get(path) or []
        bf = bugfix_by_file.get(path)
        lc = lcom4_by_file.get(path)
        cy = cyclomatic_by_file.get(path)

        present_in: list[str] = []
        if hs: present_in.append("hotspot")
        if tr: present_in.append("trend")
        if cp: present_in.append("coupling")
        if bf: present_in.append("bugfix")
        if lc: present_in.append("lcom4")
        if cy: present_in.append("cyclomatic")

        records.append({
            "path": path,
            "present_in": present_in,
            "signal_count": len(present_in),
            "hotspot": hs,
            "trend": tr,
            "coupling": cp,
            "bugfix": bf,
            "lcom4": lc,
            "cyclomatic": cy,
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
           f"{'Cpl':>3}  {'LCOM4':>5}  {'CCN':>5}  Path")
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
        lc = r["lcom4"]
        lcom4_txt = ""
        if lc:
            d = lc.get("lcom4_delta") or 0
            lcom4_txt = f"{lc['max_lcom4']}" + (f"(+{d})" if d > 0 else "")
        cy = r["cyclomatic"]
        # Display as "maxCCN/highCount" so both the ceiling and the
        # spread are visible at a glance (e.g. "66/3" tells a very
        # different story from "66/1").
        ccn_txt = f"{cy['max_ccn']}/{cy['high_ccn_count']}" if cy else ""
        print(f"{r['signal_count']:<3}  {str(commits):>7}  {fix_pct:>5}  "
              f"{_trunc(trend_verdict, 12):<12}  {dmean:>6}  {maxi:>4}  "
              f"{str(cpl):>3}  {lcom4_txt:>5}  {ccn_txt:>5}  {r['path']}")


def to_markdown(records: list[dict], packages: list[dict], meta: dict,
                md_min_signals: int = 2, md_min_commits: int = 3,
                plot_paths: dict[str, Path] | None = None,
                md_dir: Path | None = None) -> str:
    """Render the markdown data report.

    If ``plot_paths`` is given, each entry (keyed by diagram id) is
    embedded as an image reference near the relevant section. ``md_dir``
    is the directory where the markdown file will be written; image
    paths are rewritten to be relative to that directory when possible,
    so the report stays portable.
    """
    plot_paths = plot_paths or {}

    def _img_src(p: Path) -> str:
        # Prefer a path relative to the markdown file so the report
        # renders correctly when served from another directory. Fall
        # back to the absolute resolved path if the plot lives outside
        # the markdown's tree.
        if md_dir is None:
            return str(p)
        abs_p = p.resolve()
        try:
            return str(abs_p.relative_to(md_dir))
        except ValueError:
            import os
            try:
                return os.path.relpath(abs_p, md_dir)
            except ValueError:
                return str(abs_p)

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
    lines.append("| LCOM4 | `cohesion.py` | Max number of disjoint method groups across any Go type in the file. 1 = cohesive. ≥ 2 = the type could be split into that many types with no shared state or calls. Go-only; ignores receiverless functions; known false-positives on types that delegate to an embedded interface. |")
    lines.append("| Max CCN / High CCN | `cyclomatic.py` | Max per-function cyclomatic complexity in the file and how many functions in the file are ≥ threshold (default 10). McCabe's complexity counts decision points (if/for/switch/&&/||) — at CCN ≥ 10 a function becomes hard to test exhaustively; ≥ 25 is usually a split candidate. Multi-language (Go, JS, TS, Python, …); skips templ. |")
    lines.append("")

    # Top packages — orientation.
    lines.append("## Top package hotspots (orientation)\n")
    lines.append("| Score | Revs | LOC | Package |")
    lines.append("|---:|---:|---:|---|")
    for p in packages[:10]:
        lines.append(f"| {p['score']:,} | {p['revisions']} | {p['loc']:,} | `{p['name']}` |")
    lines.append("")

    # Embed the cross-signal scatter here — it's the big-picture view that
    # belongs right after the orientation table. Only included when the
    # renderer was actually run (plot_paths has the key).
    if "scatter" in plot_paths:
        lines.append("## Cross-signal scatter\n")
        lines.append(
            "Every file that has both a hotspot score and a complexity-trend "
            "classification, placed by its churn (x) and structural rot (y). "
            "Marker color = number of analyses the file appears in; marker "
            "size = bug-fix percentage. Quadrant labels are hints, not "
            "judgments — the reader decides what matters."
        )
        lines.append(f"\n![cross-signal scatter]({_img_src(plot_paths['scatter'])})\n")

    def _visible(r: dict) -> bool:
        if r["signal_count"] < md_min_signals:
            return False
        # Use the highest commit count from any signal as the filter
        # metric. cyclomatic is a snapshot measurement with no commit
        # count of its own, so it doesn't contribute to this check —
        # a file lit up ONLY by cyclomatic still needs one other
        # signal with ≥ min_commits to appear in the markdown.
        c = 0
        if r["bugfix"]:
            c = max(c, r["bugfix"]["total_commits"])
        if r["trend"]:
            c = max(c, r["trend"].get("commits", 0))
        if r["hotspot"]:
            c = max(c, max((f["revisions"] for f in r["hotspot"]["functions"]), default=0))
        return c >= md_min_commits

    visible = [r for r in records if _visible(r)]

    # Signal matrix sits right before the overview table, so the reader
    # can glance across rows to see which files are lit up in multiple
    # analyses before diving into the tabular data.
    if "matrix" in plot_paths:
        lines.append("## Signal matrix\n")
        lines.append(
            "Top files by signal count, one column per analysis. Cell color "
            "is normalized per column (darker = stronger relative signal in "
            "that column); the number inside is the raw value. A file that's "
            "dark across multiple columns is lit up by several analyses — "
            "whether that matters is a human judgment."
        )
        lines.append(f"\n![signal matrix]({_img_src(plot_paths['matrix'])})\n")

    lines.append(f"## File overview ({len(visible)} files)\n")
    lines.append("| File | Signals | Commits | Fix count | Fix % | Trend verdict | Δmean | Max indent | Hotspot score | Coupling partners | LCOM4 (Δ) | Max CCN / High |")
    lines.append("|---|:-:|---:|---:|---:|:-:|---:|---:|---:|---:|:-:|:-:|")
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
        lc = r["lcom4"]
        if lc:
            d = lc.get("lcom4_delta") or 0
            lcom4_cell = f"{lc['max_lcom4']}" + (f" (+{d})" if d > 0 else (f" ({d})" if d < 0 else ""))
        else:
            lcom4_cell = ""
        cy = r["cyclomatic"]
        ccn_cell = f"{cy['max_ccn']} / {cy['high_ccn_count']}" if cy else ""
        lines.append(
            f"| `{r['path']}` | {r['signal_count']} | "
            f"{commits} | {fix_n} | {fix_p} | {verdict} | "
            f"{dmean} | {maxi} | {hs_score} | {cpl} | {lcom4_cell} | {ccn_cell} |"
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
    lines.append(
        "- `cyclomatic` (lizard) counts control-flow branches per "
        "function. It cannot see semantic complexity (a single "
        "40-line switch counts the same as 40 split-out handlers); "
        "it also doesn't understand Go's generics or templ syntax, so "
        "some files will be absent."
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

    if r["lcom4"]:
        lc = r["lcom4"]
        delta = lc.get("lcom4_delta") or 0
        delta_txt = f" (Δ {delta:+d} over the window)" if delta else ""
        out.append(
            f"**lcom4** — max LCOM4 {lc['max_lcom4']}{delta_txt}, "
            f"{lc.get('total_methods', 0)} methods total across "
            f"{len(lc.get('types', []))} type(s). Verdict: **{lc.get('verdict', 'n/a')}**."
        )
        # Surface only types whose LCOM4 ≥ 2 — the cohesive ones aren't
        # the story. For each, show how the methods split into disjoint
        # groups so the reader can see whether the split lines up with
        # a real seam (e.g., two unrelated responsibilities) or is a
        # false positive (e.g., thin delegates over an embedded type).
        worst = [t for t in lc.get("types", []) if t.get("lcom4", 0) >= 2]
        worst.sort(key=lambda t: (-t.get("lcom4", 0), -t.get("methods", 0)))
        for t in worst[:3]:
            line = t.get("first_line") or 0
            loc_ref = f" (L{line})" if line else ""
            out.append(f"- `{t['type']}`{loc_ref} — LCOM4={t['lcom4']}, "
                       f"{t['methods']} methods, groups of "
                       f"{[len(g) for g in t.get('groups', [])]}")
            # Pool the groups into a readable hint: full membership for small
            # groups, a count+sample for large ones.
            for g in t.get("groups", []):
                if len(g) <= 6:
                    out.append(f"  - {{{', '.join(g)}}}")
                else:
                    out.append(f"  - {{{', '.join(g[:4])}, … +{len(g) - 4} more}}")
        out.append("")

    if r["cyclomatic"]:
        cy = r["cyclomatic"]
        out.append(
            f"**cyclomatic** — verdict **{cy.get('verdict', 'n/a')}** "
            f"(max CCN {cy['max_ccn']}, mean {cy['mean_ccn']:.1f}, "
            f"{cy['high_ccn_count']} function(s) ≥ threshold; "
            f"{cy['function_count']} total, {cy['total_nloc']} NLOC)."
        )
        # Name-and-line the top complex functions so the reader knows
        # exactly where to look rather than reading the whole file.
        top = cy.get("top_functions", [])
        for fn in top[:5]:
            params = fn.get("parameters", 0)
            param_hint = f", params={params}" if params else ""
            out.append(
                f"- `{fn.get('name', fn.get('long_name', '?'))}` (L{fn['line']}) "
                f"— CCN={fn['ccn']}, NLOC={fn['nloc']}{param_hint}"
            )
        out.append("")

    return out


# Plots --------------------------------------------------------------------
#
# Diagrams here are deliberately cross-signal — they show relationships the
# per-analysis scripts can't: "which files light up across multiple signals"
# (signal matrix), "where does each file sit on the churn × structural-rot
# plane" (scatter). The per-signal scripts (go_hotspots PNG, complexity_trend
# per-file PNGs, file_coupling network) already cover single-signal views,
# so we don't duplicate them.
#
# All diagrams use the same headless matplotlib import pattern as
# complexity_trend.py; if matplotlib isn't installed the plotting flags are
# silently no-ops (the textual report is the source of truth).

_CMAP_RED = "#d7263d"
_CMAP_AMBER = "#e09f3e"
_CMAP_BLUE = "#3a7ca5"


def _try_import_mpl():
    try:
        import matplotlib
        matplotlib.use("Agg")
        import matplotlib.pyplot as plt
        return plt
    except ImportError:
        return None


def _apply_plot_style(plt) -> None:
    plt.rcParams.update({
        "font.family": "DejaVu Sans",
        "font.size": 9,
        "axes.titlesize": 10,
        "axes.labelsize": 9,
        "axes.edgecolor": "#b0b0b0",
        "axes.linewidth": 0.8,
        "axes.grid": True,
        "grid.color": "#dcdcdc",
        "grid.linewidth": 0.6,
        "grid.alpha": 0.8,
        "xtick.color": "#404040",
        "ytick.color": "#404040",
        "xtick.labelsize": 8,
        "ytick.labelsize": 8,
        "figure.facecolor": "white",
        "axes.facecolor": "#fafbfc",
        "legend.frameon": True,
        "legend.framealpha": 0.95,
        "legend.edgecolor": "#cccccc",
        "legend.fontsize": 8,
    })


def _file_metric_tuple(r: dict) -> tuple[float, float, float, float, float, float]:
    """Return the per-file signal values as a tuple.

    Picks one representative number per analysis so the matrix/scatter
    can plot them side-by-side. Returns raw values; normalization is
    handled per-diagram.

    LCOM4 is surfaced as ``max_lcom4 - 1`` so that LCOM4=1 (cohesive)
    is 0 — the matrix column then reads as "how many extra
    responsibilities does this file carry". Files that don't appear in
    the LCOM4 report (not Go, or below threshold) get 0. CCN is
    reported as ``max_ccn`` directly (a pure scalar) since the tool's
    threshold is external and we want the matrix column to reflect
    absolute complexity.
    """
    hs_score = float(r["hotspot"]["total_score"]) if r["hotspot"] else 0.0
    tm = (r["trend"] or {}).get("metrics") or {}
    # Clamp negative Δmean to 0 for the matrix (REFACTORED files look cool,
    # DETERIORATING look hot). The scatter keeps the signed value.
    dmean = float(tm.get("mean_delta") or 0.0)
    fix_pct = float(r["bugfix"]["fix_pct"]) if r["bugfix"] else 0.0
    cpl = float(len(r["coupling"]) if r["coupling"] else 0)
    lc = r.get("lcom4")
    lcom4_excess = float(max(0, (lc.get("max_lcom4") or 0) - 1)) if lc else 0.0
    cy = r.get("cyclomatic")
    max_ccn = float(cy.get("max_ccn") or 0) if cy else 0.0
    return hs_score, dmean, fix_pct, cpl, lcom4_excess, max_ccn


def render_signal_matrix(records: list[dict], out_path: Path,
                         top_n: int = 30, min_signals: int = 2) -> Path | None:
    """Heatmap: rows = top files by signal count, cols = the four analyses.

    Cell intensity = that file's relative strength in that signal (rescaled
    0..1 per column). Lets the reader see at a glance which files are
    lit up across every column vs just one. Columns and their scaling:

    - hotspot: total_score, linear scale (log would over-compress the tail).
    - trend:   Δmean clamped to [0, max] — DETERIORATING direction only,
      since a REFACTORED file showing up in this matrix would be misleading.
    - bugfix:  fix_pct as-is.
    - coupling: partner count, linear.

    Intentionally not a composite score: readers compose their own priority
    by looking across the row.
    """
    plt = _try_import_mpl()
    if plt is None:
        return None
    visible = [r for r in records if r["signal_count"] >= min_signals]
    if not visible:
        return None

    rows = []
    for r in visible:
        hs, dmean, fix, cpl, lc, ccn = _file_metric_tuple(r)
        # Clamp Δmean to the deteriorating direction for the matrix.
        rows.append((r["path"], r["signal_count"], hs, max(0.0, dmean),
                     fix, cpl, lc, ccn))

    # Rank rows by "number of non-zero signals then weighted signal sum"
    # so the top of the matrix is the most-lit-up files. LCOM4 and
    # Δmean get the same 100× weight because they're on very small
    # scales; CCN uses a 3× weight so a CCN=30 file doesn't outrank a
    # file hot on four other signals.
    rows.sort(key=lambda t: (
        -t[1],
        -(t[2] + 100 * t[3] + t[4] + t[5] + 100 * t[6] + 3 * t[7]),
    ))
    rows = rows[:top_n]
    if not rows:
        return None

    _apply_plot_style(plt)
    import numpy as np
    data = np.array(
        [[r[2], r[3], r[4], r[5], r[6], r[7]] for r in rows], dtype=float,
    )
    # Per-column max for normalisation. Guard against all-zero column.
    col_max = data.max(axis=0)
    col_max[col_max == 0] = 1.0
    norm = data / col_max

    labels = [r[0] for r in rows]
    cols = ["hotspot\nscore", "Δmean\n(deterioration)", "bug-fix %",
            "coupling\npartners", "LCOM4\n(excess)", "max CCN"]
    sig_counts = [r[1] for r in rows]

    # Figure size scales with row count so rows stay legible.
    row_h = 0.30
    fig_h = max(5.0, 1.5 + row_h * len(rows))
    fig, ax = plt.subplots(figsize=(11, fig_h))
    fig.subplots_adjust(left=0.42, right=0.92, top=0.92, bottom=0.08)

    im = ax.imshow(norm, aspect="auto", cmap="YlOrRd", vmin=0.0, vmax=1.0)
    ax.set_xticks(range(len(cols)))
    ax.set_xticklabels(cols)
    ax.set_yticks(range(len(labels)))
    ax.set_yticklabels(labels, fontsize=7.5)
    ax.tick_params(axis="y", length=0)
    ax.tick_params(axis="x", length=0, pad=6)
    ax.grid(False)

    # Annotate each cell with the raw (un-normalized) value so the reader
    # can read off actual numbers, not just relative intensity.
    for i, row in enumerate(rows):
        vals = [row[2], row[3], row[4], row[5], row[6], row[7]]
        for j, v in enumerate(vals):
            if v == 0:
                txt = "—"
            elif j == 0:
                txt = f"{int(v):,}"
            elif j == 1:
                txt = f"{v:+.2f}"
            elif j == 2:
                txt = f"{v:.0f}%"
            elif j == 4:
                # LCOM4 column stores "excess" (max_lcom4 - 1); print back
                # the original LCOM4 so a reader doesn't have to translate.
                txt = f"{int(v) + 1}"
            else:
                txt = f"{int(v)}"
            # Pick text color by cell darkness (imshow YlOrRd: high = dark).
            color = "white" if norm[i, j] > 0.55 else "#303030"
            ax.text(j, i, txt, ha="center", va="center",
                    fontsize=7.2, color=color, fontweight="semibold")

    # Signal-count badge on the right of each row (6 analyses now).
    for i, n in enumerate(sig_counts):
        ax.text(5.7, i, f"[{n}/6]",
                ha="left", va="center", fontsize=7, color="#606060",
                fontstyle="italic")

    ax.set_title(
        "Cross-signal matrix — per-file intensity across six analyses\n"
        f"(top {len(rows)} files, signals≥{min_signals}; cells normalized per column; "
        "raw values overlaid; [n/6] = how many analyses mentioned the file)",
        fontsize=10, loc="left", pad=14,
    )
    cbar = fig.colorbar(im, ax=ax, shrink=0.55, pad=0.02, aspect=20)
    cbar.set_label("normalized intensity (per column)", fontsize=8)
    cbar.ax.tick_params(labelsize=7)

    out_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(out_path, dpi=150)
    plt.close(fig)
    return out_path


def render_signal_scatter(records: list[dict], out_path: Path,
                          min_signals: int = 1, label_top: int = 15) -> Path | None:
    """Scatter: x = hotspot score (log), y = Δmean indent (signed).

    Marker size encodes bug-fix % (5..25% → 40..400 pt²).
    Marker color encodes signal_count (1=grey, 2=blue, 3=amber, 4=red).
    Quadrant annotations help the reader parse the plot without a legend.

    Why this view: each axis comes from a different analysis, so the
    scatter shows interactions that per-analysis plots can't:
      - upper-right  = high churn AND structurally rotting (your usual
        refactor candidates)
      - upper-left   = rotting but not a hotspot (quiet ticking bomb)
      - lower-right  = hotspot but flat shape (stable workhorse)
      - lower-left   = background noise
    Signed Δmean keeps REFACTORED files visible (negative Δmean = below
    the horizontal axis).
    """
    plt = _try_import_mpl()
    if plt is None:
        return None

    visible = [r for r in records if r["signal_count"] >= min_signals]
    pts = []
    for r in visible:
        hs, dmean, fix, cpl, lc, ccn = _file_metric_tuple(r)
        # A file needs both a hotspot score AND a trend signal to appear —
        # otherwise we can't place it on the plane honestly.
        if hs <= 0 or r["trend"] is None:
            continue
        pts.append({
            "path": r["path"], "signal_count": r["signal_count"],
            "hs": hs, "dmean": dmean, "fix": fix, "cpl": cpl,
            "lcom4_excess": lc, "ccn": ccn,
        })
    if not pts:
        return None

    _apply_plot_style(plt)
    fig, ax = plt.subplots(figsize=(11, 7.5))
    fig.subplots_adjust(left=0.08, right=0.97, top=0.88, bottom=0.12)

    # Color ramps with signal count up to 6 (hotspot + trend + coupling +
    # bugfix + lcom4 + cyclomatic). Points with an LCOM4 signal get a
    # dark ring so the structural-cohesion flag pops out visually
    # without needing its own axis — you can read "this file has
    # hotspot + trend + LCOM4" at a glance from color + outline.
    color_by_sig = {1: "#9ca3af", 2: _CMAP_BLUE, 3: _CMAP_AMBER,
                    4: _CMAP_RED, 5: "#7b1133", 6: "#4c0516"}
    xs = [p["hs"] for p in pts]
    ys = [p["dmean"] for p in pts]
    sizes = [40 + min(360, p["fix"] * 12) for p in pts]
    colors = [color_by_sig.get(p["signal_count"], "#9ca3af") for p in pts]
    edges = ["#222" if p["lcom4_excess"] > 0 else "white" for p in pts]
    widths = [1.6 if p["lcom4_excess"] > 0 else 0.8 for p in pts]
    ax.scatter(xs, ys, s=sizes, c=colors, alpha=0.75,
               edgecolor=edges, linewidth=widths)

    ax.set_xscale("symlog", linthresh=100)
    ax.set_xlabel("hotspot score (go_hotspots)  ·  log axis")
    ax.set_ylabel("Δmean indent (complexity_trend, signed)")
    ax.axhline(0.0, color="#888", linewidth=0.9, linestyle="--", alpha=0.8)
    ax.axhline(0.08, color=_CMAP_RED, linewidth=0.7, linestyle=":", alpha=0.7)
    ax.text(ax.get_xlim()[1], 0.083, "deteriorating threshold (Δmean > 0.08)",
            ha="right", va="bottom", fontsize=7.5, color=_CMAP_RED, alpha=0.9)

    # Quadrant labels — anchored to the plot's data range, not viewport.
    xr_min, xr_max = max(1, min(xs)), max(xs)
    yr_min, yr_max = min(ys), max(ys)
    y_top = yr_max if yr_max > 0 else 0.05
    y_bot = yr_min if yr_min < 0 else -0.05
    quad_args = dict(fontsize=8, color="#606060", alpha=0.85, fontstyle="italic")
    ax.text(xr_max, y_top, "hotspot · rotting", ha="right", va="top", **quad_args)
    ax.text(xr_min, y_top, "quiet rot", ha="left", va="top", **quad_args)
    ax.text(xr_max, y_bot, "hotspot · flat", ha="right", va="bottom", **quad_args)
    ax.text(xr_min, y_bot, "noise", ha="left", va="bottom", **quad_args)

    # Label the top N points by a crude (hotspot × max(Δmean, 0)) ordering —
    # intentionally not a score in the report sense; just picks the corner
    # of the scatter a human eye would look at first. File names are
    # shortened to their tail two path segments to avoid overplotting.
    pts_ranked = sorted(
        pts,
        key=lambda p: -(p["hs"] * max(0.01, p["dmean"] + 0.2)),
    )[:label_top]
    for p in pts_ranked:
        short = "/".join(p["path"].rsplit("/", 2)[-2:])
        ax.annotate(
            short, xy=(p["hs"], p["dmean"]),
            xytext=(5, 5), textcoords="offset points",
            fontsize=7, color="#303030", alpha=0.9,
        )

    # Legend for color (signal count), size (fix%), and outline (LCOM4).
    legend_markers = []
    from matplotlib.lines import Line2D
    for n in (2, 3, 4, 5, 6):
        legend_markers.append(Line2D(
            [0], [0], marker="o", linestyle="", markersize=8,
            markerfacecolor=color_by_sig[n], markeredgecolor="white",
            label=f"{n} signals",
        ))
    legend_markers.append(Line2D(
        [0], [0], marker="o", linestyle="", markersize=6,
        markerfacecolor="#aaa", markeredgecolor="white",
        label="bug-fix 5%",
    ))
    legend_markers.append(Line2D(
        [0], [0], marker="o", linestyle="", markersize=14,
        markerfacecolor="#aaa", markeredgecolor="white",
        label="bug-fix 25%",
    ))
    legend_markers.append(Line2D(
        [0], [0], marker="o", linestyle="", markersize=10,
        markerfacecolor="#ddd", markeredgecolor="#222", markeredgewidth=1.6,
        label="LCOM4 ≥ 2 (outlined)",
    ))
    ax.legend(handles=legend_markers, loc="lower right", ncol=2,
              fontsize=8, title="marker key", title_fontsize=8)

    ax.set_title(
        "Cross-signal scatter — where each file sits on churn × structural rot\n"
        f"({len(pts)} files with both hotspot + trend signals; marker size = bug-fix %)",
        fontsize=10, loc="left", pad=12,
    )

    out_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(out_path, dpi=150)
    plt.close(fig)
    return out_path


# Driver -------------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--hotspots", required=True, action="append",
                    help="go_hotspots.py --json output (repeatable)")
    ap.add_argument("--trends", required=True, help="complexity_trend.py --json output")
    ap.add_argument("--coupling", required=True, help="file_coupling.py -o output")
    ap.add_argument("--bugfix", help="bugfix_density.py -o output (optional)")
    ap.add_argument("--lcom4", help="cohesion.py --json output (optional; Go files only)")
    ap.add_argument("--cyclomatic", help="cyclomatic.py --json output (optional; multi-language)")
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
    ap.add_argument("--out-plots", type=str, default=None,
                    help="directory to write cross-signal PNGs into "
                         "(signal_matrix.png, signal_scatter.png). When set "
                         "and --out-md is also set, the plots are embedded "
                         "inline in the markdown report.")
    ap.add_argument("--matrix-top-n", type=int, default=30,
                    help="max rows in the signal matrix heatmap (default 30)")
    args = ap.parse_args()

    try:
        hotspots_by_file, packages = load_hotspots(args.hotspots)
        trends_by_file = load_trends(args.trends)
        coupling_by_file = load_coupling(args.coupling, args.cooccur_floor)
        bugfix_by_file = load_bugfix(args.bugfix)
        lcom4_by_file = load_lcom4(args.lcom4)
        cyclomatic_by_file = load_cyclomatic(args.cyclomatic)
    except FileNotFoundError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1

    records = collate(hotspots_by_file, trends_by_file, coupling_by_file,
                      bugfix_by_file, lcom4_by_file, cyclomatic_by_file)
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

    # Render cross-signal diagrams. Matplotlib is optional — if it's not
    # installed we print a hint and skip; the textual report stands alone.
    plot_paths: dict[str, Path] = {}
    if args.out_plots:
        out_dir = Path(args.out_plots)
        if _try_import_mpl() is None:
            print("warning: matplotlib unavailable — skipping --out-plots",
                  file=sys.stderr)
        else:
            matrix_path = render_signal_matrix(
                records, out_dir / "signal_matrix.png",
                top_n=args.matrix_top_n, min_signals=args.md_min_signals,
            )
            if matrix_path:
                plot_paths["matrix"] = matrix_path
                print(f"Signal matrix: {matrix_path}")
            scatter_path = render_signal_scatter(
                records, out_dir / "signal_scatter.png",
                min_signals=1,
            )
            if scatter_path:
                plot_paths["scatter"] = scatter_path
                print(f"Signal scatter: {scatter_path}")

    if args.out_md:
        md_path = Path(args.out_md)
        Path(args.out_md).write_text(to_markdown(
            records, packages, meta,
            md_min_signals=args.md_min_signals,
            md_min_commits=args.md_min_commits,
            plot_paths=plot_paths,
            md_dir=md_path.parent.resolve(),
        ))
        print(f"Markdown: {args.out_md}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
