#!/usr/bin/env python3
"""
refactor_priority.py — cross-signal refactor ranker.

Takes JSON output from the three analyzers and synthesizes a prioritized
refactor plan. The key insight: one signal is opinion, two is a hypothesis,
three is a diagnosis. Files that appear in multiple signals are the
highest-value targets.

Inputs (all JSON from the sibling tools):
  --hotspots      go_hotspots.py --json
  --trends        complexity_trend.py --discover --json
  --coupling      file_coupling.py -o

Outputs:
  stdout          ranked summary table
  --out-md FILE   full markdown plan with evidence citations
  --out-json FILE machine-readable ranked report

Signal contributions (per-file score):
  hotspot      up to 100 — scaled by (score / top_score)
  deteriorating  0–100 — scaled by (combined_score / top_combined_score)
  active_decay   +40 bonus — currently rotting, not historically
  coupling       +25 per high-coupling partner (cooccur ≥ 8)
  depth_extreme  +20 when max indent ≥ 9
  is_test        ×0.3 multiplier — tests rot differently

Tiers:
  Tier 1 — score ≥ 80 AND ≥2 independent signals (cross-signal)
  Tier 2 — score ≥ 60 with 1 strong signal
  Tier 3 — score ≥ 30 (monitor)
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
    merge a package-mode repo scan with function-mode drill-ins. Hotspots can
    be at package or function granularity; we key functions by file path and
    aggregate."""
    by_file: dict[str, dict] = {}
    packages: list[dict] = []
    all_hs: list[dict] = []
    for path in paths:
        d = json.loads(Path(path).read_text())
        all_hs.extend(d.get("hotspots", []))
    if not all_hs:
        return by_file, packages
    # Compute top_score over the *function* entries only — package scores are
    # orders of magnitude larger and would crush normalization.
    fn_scores = [h["metrics"]["score"] for h in all_hs if h.get("kind") != "package"]
    top_score = max(fn_scores, default=1) or 1
    for h in all_hs:
        score = h["metrics"]["score"]
        normalized = 100.0 * score / top_score
        kind = h.get("kind")
        path = h.get("file_path")
        if kind == "package":
            packages.append({
                "name": h.get("import_path") or h.get("name"),
                "score": score,
                "revisions": h["metrics"]["revisions"],
                "loc": h["metrics"]["loc"],
                "normalized": normalized,
                "signals": h.get("signals", []),
                "suggested_actions": h.get("suggested_actions", []),
            })
            continue
        if not path:
            continue
        # Function-level hit — aggregate into its containing file.
        entry = by_file.setdefault(path, {
            "path": path,
            "total_score": 0,
            "top_function_score": 0,
            "functions": [],
            "signals": set(),
            "suggested_actions": set(),
        })
        entry["total_score"] += score
        if score > entry["top_function_score"]:
            entry["top_function_score"] = score
        entry["functions"].append({
            "name": h.get("name"),
            "lines": f"L{h.get('start_line')}-{h.get('end_line')}",
            "loc": h["metrics"]["loc"],
            "revisions": h["metrics"]["revisions"],
            "score": score,
        })
        for s in h.get("signals") or []:
            entry["signals"].add(s.get("kind"))
        for a in h.get("suggested_actions") or []:
            entry["suggested_actions"].add(a)

    # Normalize file-aggregate scores.
    if by_file:
        max_total = max(e["total_score"] for e in by_file.values())
        for e in by_file.values():
            e["hotspot_normalized"] = 100.0 * e["total_score"] / max_total if max_total else 0
            e["signals"] = sorted(e["signals"])
            e["suggested_actions"] = sorted(e["suggested_actions"])
    return by_file, packages


def load_trends(path: str) -> dict[str, dict]:
    """Return by-file map of deteriorating files from complexity_trend JSON."""
    d = json.loads(Path(path).read_text())
    ranked = d.get("ranked") or []
    if not ranked:
        return {}
    top = ranked[0].get("combined_score") or ranked[0]["score"]
    out: dict[str, dict] = {}
    for e in ranked:
        combined = e.get("combined_score") or e["score"]
        out[e["path"]] = {
            "path": e["path"],
            "is_test": e["is_test"],
            "verdict": e["verdict"],
            "score": e["score"],
            "recent_score": e.get("recent_score", 0),
            "combined_score": combined,
            "trend_normalized": 100.0 * combined / top if top else 0,
            "active_decay": e.get("active_decay", False),
            "commits": e["commits"],
            "loc": e["loc"],
            "mean_delta": e["metrics"]["mean_delta"],
            "last_max": e["metrics"]["last_max"],
            "total_delta_pct": e["metrics"]["total_delta_pct"],
            "loc_delta_pct": e["metrics"]["loc_delta_pct"],
        }
    return out


def load_coupling(path: str, cooc_floor: int = 8) -> dict[str, list[dict]]:
    """Return by-file map of high-coupling partners."""
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
        jaccard = p.get("jaccard", 0)
        for src, dst in ((f1, f2), (f2, f1)):
            by_file[src].append({
                "partner": dst,
                "cooccur": cooc,
                "jaccard": jaccard,
                "structural": p.get("structural", False),
            })
    return by_file


# Synthesis ----------------------------------------------------------------

def synthesize(
    hotspots_by_file: dict[str, dict],
    trends_by_file: dict[str, dict],
    coupling_by_file: dict[str, list[dict]],
) -> list[dict]:
    """Merge signals into a single per-file ranking."""
    all_paths = (
        set(hotspots_by_file) | set(trends_by_file) | set(coupling_by_file)
    )

    results = []
    for path in all_paths:
        hs = hotspots_by_file.get(path)
        tr = trends_by_file.get(path)
        cp = coupling_by_file.get(path) or []

        # Base contributions.
        hotspot_contrib = hs["hotspot_normalized"] if hs else 0
        trend_contrib = tr["trend_normalized"] if tr else 0
        coupling_contrib = min(100, 25 * len(cp))

        # Bonuses.
        active_bonus = 40 if (tr and tr["active_decay"]) else 0
        depth_bonus = 20 if (tr and tr["last_max"] >= 9) else 0

        # Base score — weighted sum. hotspot×1.0, trend×0.9, coupling×0.6.
        score = (
            1.0 * hotspot_contrib +
            0.9 * trend_contrib +
            0.6 * coupling_contrib +
            active_bonus + depth_bonus
        )

        is_test = (tr and tr["is_test"]) or "_test" in path
        if is_test:
            score *= 0.3

        sources: list[str] = []
        if hs:
            sources.append("hotspot")
        if tr:
            sources.append(f"trend:{tr['verdict']}")
        if cp:
            sources.append(f"coupling×{len(cp)}")

        results.append({
            "path": path,
            "score": round(score, 1),
            "is_test": is_test,
            "sources": sources,
            "hotspot": hs,
            "trend": tr,
            "coupling": cp,
        })

    results.sort(key=lambda r: -r["score"])
    return results


def tier(entry: dict) -> int:
    """1 (top) / 2 / 3 / 0 (filler)."""
    s = entry["score"]
    n_sources = len(entry["sources"])
    if s >= 80 and n_sources >= 2:
        return 1
    if s >= 60:
        return 2
    if s >= 30:
        return 3
    return 0


# Reporting ----------------------------------------------------------------

def print_stdout(results: list[dict], packages: list[dict]) -> None:
    print("\n═══ Top package hotspots (for orientation) ═══")
    for p in packages[:8]:
        print(f"  score={p['score']:7.0f}  revs={p['revisions']:3d}  loc={p['loc']:5d}  {p['name']}")

    print("\n═══ Cross-signal refactor ranking ═══")
    print(f"{'Tier':<4} {'Score':>6}  {'Sources':<30}  Path")
    for r in results:
        t = tier(r)
        if t == 0:
            continue
        tag = f"T{t}" + (" 🔥" if r["trend"] and r["trend"]["active_decay"] else "")
        sources = ", ".join(r["sources"])
        print(f"{tag:<5} {r['score']:6.1f}  {sources:<30}  {r['path']}")


def to_markdown(
    results: list[dict],
    packages: list[dict],
    meta: dict,
) -> str:
    tiers: dict[int, list[dict]] = defaultdict(list)
    for r in results:
        tiers[tier(r)].append(r)

    lines = []
    lines.append("# Refactoring Plan — Evidence-Based\n")
    lines.append(f"**Generated:** {meta['generated_at']}  **Window:** {meta['window']}\n")
    lines.append("Three independent signals cross-referenced to pick targets:\n")
    lines.append("| Tool | What it measures |")
    lines.append("|---|---|")
    lines.append("| `go_hotspots.py` | Change frequency × effective LOC |")
    lines.append("| `complexity_trend.py` | Whitespace-indent complexity per revision |")
    lines.append("| `file_coupling.py` | Temporal co-change (Jaccard of commit sets) |")
    lines.append("\nFiles appearing in **multiple** signals are prioritized — one signal is opinion, two is a hypothesis, three is a diagnosis.\n")

    lines.append("## Top package hotspots\n")
    lines.append("| Score | Revs | LOC | Package |")
    lines.append("|---:|---:|---:|---|")
    for p in packages[:10]:
        lines.append(f"| {p['score']:,} | {p['revisions']} | {p['loc']:,} | `{p['name']}` |")
    lines.append("")

    def _entry_block(r: dict) -> list[str]:
        out = []
        flag = " 🔥 actively decaying" if r["trend"] and r["trend"]["active_decay"] else ""
        out.append(f"### `{r['path']}`{flag}\n")
        out.append("**Evidence**\n")
        if r["hotspot"]:
            hs = r["hotspot"]
            out.append(
                f"- `go_hotspots`: file-aggregate score **{hs['total_score']:,}** "
                f"({len(hs['functions'])} hot functions)."
            )
            for f in sorted(hs["functions"], key=lambda x: -x["score"])[:3]:
                out.append(f"  - `{f['name']}` {f['lines']} — score {f['score']}, {f['revisions']} revs, {f['loc']} LOC")
            if hs["signals"]:
                out.append(f"  - Signals: {', '.join(hs['signals'])}.")
        if r["trend"]:
            tr = r["trend"]
            out.append(
                f"- `complexity_trend`: **{tr['verdict']}** — score {tr['combined_score']:.0f} "
                f"(full {tr['score']:.0f}, recent {tr['recent_score']:.0f}), "
                f"Δmean {tr['mean_delta']:+.2f}, Δtotal {tr['total_delta_pct']:+.0f}% vs ΔLOC {tr['loc_delta_pct']:+.0f}%, "
                f"max indent {tr['last_max']:.0f}, {tr['commits']} commits."
            )
        if r["coupling"]:
            top_partners = sorted(r["coupling"], key=lambda p: -p["cooccur"])[:3]
            out.append("- `file_coupling`: high-coupling partners —")
            for p in top_partners:
                marker = " (structural)" if p["structural"] else ""
                out.append(f"  - `{p['partner']}` — {p['cooccur']:.0f} co-changes, J={p['jaccard']:.0f}{marker}")
        if r["hotspot"] and r["hotspot"]["suggested_actions"]:
            out.append("\n**Suggested actions (from hotspot analyzer)**\n")
            for a in r["hotspot"]["suggested_actions"]:
                out.append(f"- {a}")
        out.append("")
        return out

    for t_num, title, blurb in [
        (1, "Tier 1 — cross-signal targets", "Appear in ≥ 2 independent signals. Highest leverage."),
        (2, "Tier 2 — single-signal, high magnitude", "One strong signal. Refactor when you're already in the area."),
        (3, "Tier 3 — monitor", "Scored but not urgent. Re-run quarterly."),
    ]:
        entries = tiers.get(t_num, [])
        if not entries:
            continue
        lines.append(f"## {title}\n")
        lines.append(f"_{blurb}_\n")
        for r in entries:
            lines.extend(_entry_block(r))

    lines.append("## Reproduce\n")
    lines.append("```bash")
    lines.append("task refactor:plan")
    lines.append("```\n")
    return "\n".join(lines)


# Driver -------------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--hotspots", required=True, action="append",
                    help="go_hotspots.py --json output (repeatable; e.g. one repo-wide + one per hot package)")
    ap.add_argument("--trends", required=True, help="complexity_trend.py --json output")
    ap.add_argument("--coupling", required=True, help="file_coupling.py -o output")
    ap.add_argument("--out-md", type=str, help="write markdown plan to this path")
    ap.add_argument("--out-json", type=str, help="write synthesized JSON ranking to this path")
    ap.add_argument("--cooccur-floor", type=int, default=8,
                    help="coupling co-occurrence threshold (default 8)")
    ap.add_argument("--window", default="6 months",
                    help="description for the markdown header (default '6 months')")
    args = ap.parse_args()

    try:
        hotspots_by_file, packages = load_hotspots(args.hotspots)
        trends_by_file = load_trends(args.trends)
        coupling_by_file = load_coupling(args.coupling, args.cooccur_floor)
    except FileNotFoundError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1

    results = synthesize(hotspots_by_file, trends_by_file, coupling_by_file)
    print_stdout(results, packages)

    meta = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "window": args.window,
    }

    if args.out_json:
        Path(args.out_json).write_text(json.dumps({
            "schema": "refactor-priority/1",
            "meta": meta,
            "packages": packages,
            "ranked": results,
        }, indent=2))
        print(f"\nJSON: {args.out_json}")

    if args.out_md:
        Path(args.out_md).write_text(to_markdown(results, packages, meta))
        print(f"Markdown: {args.out_md}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
