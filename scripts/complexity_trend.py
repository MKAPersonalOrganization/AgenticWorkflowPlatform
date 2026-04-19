#!/usr/bin/env python3
"""
complexity_trend.py — Chapter 5 of Your Code as a Crime Scene.
Whitespace-indentation complexity analysis over a file's git history.
Four-space / one-tab = 1 logical indent. Empty/blank lines ignored.
Usage:
    complexity_trend.py <path>... [--since "6 months ago"] [--samples N]
                        [--out DIR] [--no-plot] [--summary]
Writes one PNG per file into --out (default: scripts/out/complexity_trend/)
and, with --summary, a grid plot comparing all files side by side.
Classifies evolution as:
  - DETERIORATING: total complexity grows faster than LOC (structure thickens)
  - GROWING:       complexity and LOC grow together (more code, same shape)
  - REFACTORED:    complexity dropped vs earlier peak
  - STABLE:        neither grew significantly
"""
from __future__ import annotations
import argparse
import json
import math
import re
import statistics
import subprocess
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
SCHEMA_VERSION = "complexity-trend/2"
# Regex covering the usual test-file conventions: Go (_test.go), JS
# (.test.js/.spec.js/.test.ts), Python (test_*.py, *_test.py).
_TEST_PATTERN = re.compile(
    r"(_test\.go$|(?:\.test|\.spec)\.(?:jsx?|tsx?)$|(?:^|/)test_[^/]+\.py$|_test\.py$)"
)
def is_test_file(path: str) -> bool:
    return bool(_TEST_PATTERN.search(path))
VERDICT_COLORS = {
    "DETERIORATING": "#d7263d",
    "GROWING":       "#e09f3e",
    "REFACTORED":    "#2a9d8f",
    "STABLE":        "#3a7ca5",
    "INSUFFICIENT":  "#808080",
}
VERDICT_BLURB = {
    "DETERIORATING": "total indent grows faster than LOC",
    "GROWING":       "more code, same shape",
    "REFACTORED":    "complexity dropped vs peak",
    "STABLE":        "little structural change",
    "INSUFFICIENT":  "not enough samples",
}
LOC_COLOR = "#404040"
MEAN_COLOR = "#1b6ca8"
MAX_COLOR = "#e76f51"
def logical_indent(line: str) -> float:
    """Count leading whitespace as logical indent (1 tab or 4 spaces = 1)."""
    units = 0.0
    for ch in line:
        if ch == "\t":
            units += 1.0
        elif ch == " ":
            units += 0.25
        else:
            break
    return units
def complexity_stats(text: str) -> dict:
    """Whitespace complexity stats for a file blob.

    Returns the full stat dict plus a raw ``indents`` list so callers that
    aggregate across multiple files (see ``sample_group_history``) can
    recompute a true p95 / max over the merged population rather than
    trying to combine pre-aggregated percentiles.
    """
    indents = [logical_indent(line) for line in text.splitlines() if line.strip()]
    return _stats_from_indents(indents)


def _stats_from_indents(indents: list[float]) -> dict:
    """Derive the stat dict from an already-collected indent list.

    Split out so group aggregation can merge per-file indent lists before
    computing summary stats — the only correct way to get mean / p95 /
    max over a multi-file unit.
    """
    if not indents:
        return {"n": 0, "total": 0.0, "mean": 0.0, "sd": 0.0, "max": 0.0,
                "p95": 0.0, "indents": []}
    # p95: the 95th-percentile indent. Catches "deep pockets" (a handful of
    # heavily nested lines) that mean-indent smears away. For a file of
    # 500 lines this is the 475th-deepest line; for groups we pool all
    # files' indent lists first, so the percentile reflects the whole
    # unit, not the max across per-file percentiles.
    si = sorted(indents)
    p95_idx = max(0, min(len(si) - 1, int(math.ceil(0.95 * len(si))) - 1))
    return {
        "n": len(indents),
        "total": round(sum(indents), 2),
        "mean": round(statistics.fmean(indents), 3),
        "sd": round(statistics.pstdev(indents) if len(indents) > 1 else 0.0, 3),
        "max": round(max(indents), 2),
        "p95": round(si[p95_idx], 2),
        "indents": indents,
    }
def revisions_for(path: str, since: str) -> list[tuple[str, int]]:
    """(hash, unix_ts) pairs that touched `path` in period, oldest first.

    Uses git's own `--since` filter (approxidate) so the same relative-date
    strings ("6 months ago", "2 weeks ago") work cross-platform. Previously
    shelled out to GNU `date -d`, which silently no-ops on macOS/BSD and
    returned every revision regardless of the window.
    """
    out = subprocess.run(
        ["git", "log", "--follow", f"--since={since}",
         "--pretty=format:%H %ct", "--", path],
        check=True, capture_output=True, text=True,
    )
    revs = []
    for line in out.stdout.splitlines():
        if not line.strip():
            continue
        h, ts = line.split()
        revs.append((h, int(ts)))
    revs.reverse()  # oldest first
    return revs
def path_at_revision(path: str, rev: str) -> str | None:
    """Resolve current `path` to its name at `rev` (handles renames).

    Returns None if the file can't be located at rev — callers should skip
    the revision rather than read a wrong-file blob.

    The previous implementation used `git log --name-only -- path`, which
    filters which COMMITS are shown but still prints every filename those
    commits touched. When a rename commit also modified other files, the
    first printed name was often an unrelated file, and the caller would
    silently sample that blob as if it were `path` — corrupting the trend.
    """
    # Fast path: file exists under the same name at this revision.
    probe = subprocess.run(
        ["git", "cat-file", "-e", f"{rev}:{path}"],
        capture_output=True,
    )
    if probe.returncode == 0:
        return path

    # Rename path: walk `path`'s follow-history for rename events (R),
    # unwinding new→old until we find a name that exists at rev.
    out = subprocess.run(
        ["git", "log", "--follow", "--name-status", "--diff-filter=R",
         "--pretty=format:", "--", path],
        capture_output=True, text=True,
    )
    current = path
    for line in out.stdout.splitlines():
        parts = line.split("\t")
        if len(parts) >= 3 and parts[0].startswith("R") and parts[2] == current:
            current = parts[1]  # unwind to older name
            probe = subprocess.run(
                ["git", "cat-file", "-e", f"{rev}:{current}"],
                capture_output=True,
            )
            if probe.returncode == 0:
                return current
    return None  # couldn't locate; let caller skip this revision
def blob_at(path: str, rev: str) -> str | None:
    out = subprocess.run(
        ["git", "show", f"{rev}:{path}"],
        capture_output=True, text=True,
    )
    if out.returncode != 0:
        return None
    return out.stdout
@dataclass
class Sample:
    rev: str
    ts: int  # unix timestamp
    n: int
    total: float
    mean: float
    sd: float
    maxi: float
    p95: float = 0.0  # deep-pocket percentile; defaulted so older callers that
                      # construct Samples without it keep working
def sample_history(path: str, revs: list[tuple[str, int]], max_samples: int) -> list[Sample]:
    if len(revs) > max_samples:
        step = len(revs) / max_samples
        idxs = sorted({int(i * step) for i in range(max_samples)} | {len(revs) - 1})
        revs = [revs[i] for i in idxs if i < len(revs)]
    samples = []
    for rev, ts in revs:
        rev_path = path_at_revision(path, rev)
        if rev_path is None:
            continue  # couldn't locate file at this revision — skip rather
                      # than sample an unrelated blob
        blob = blob_at(rev_path, rev)
        if blob is None:
            continue
        s = complexity_stats(blob)
        if s["n"] == 0:
            continue
        samples.append(Sample(rev[:9], ts, s["n"], s["total"], s["mean"],
                              s["sd"], s["max"], s["p95"]))
    return samples


def sample_group_history(paths: list[str], revs: list[tuple[str, int]],
                         max_samples: int) -> list[Sample]:
    """Sample complexity for a *group* of files at each revision.

    At each sampled rev, every path's blob is read and its indent values
    are pooled into one population before summary stats are computed. Per
    the M18/refactor finding: multi-file refactors move complexity
    between siblings — the only honest measure is to treat the group as
    the unit and watch its mean / p95 / total move together.

    Files that didn't exist at a given rev contribute nothing rather
    than an error — the group's size at that point simply reflects what
    was committed.
    """
    if len(revs) > max_samples:
        step = len(revs) / max_samples
        idxs = sorted({int(i * step) for i in range(max_samples)} | {len(revs) - 1})
        revs = [revs[i] for i in idxs if i < len(revs)]
    samples: list[Sample] = []
    for rev, ts in revs:
        pooled: list[float] = []
        for path in paths:
            rev_path = path_at_revision(path, rev)
            if rev_path is None:
                continue
            blob = blob_at(rev_path, rev)
            if blob is None:
                continue
            pooled.extend(
                logical_indent(line) for line in blob.splitlines() if line.strip()
            )
        s = _stats_from_indents(pooled)
        if s["n"] == 0:
            continue
        samples.append(Sample(rev[:9], ts, s["n"], s["total"], s["mean"],
                              s["sd"], s["max"], s["p95"]))
    return samples


def revisions_for_group(paths: list[str], since: str) -> list[tuple[str, int]]:
    """Union of the revisions that touched any path in the group, oldest first.

    De-duplicated by commit hash. A commit that touched two files in the
    group is still one revision — we're measuring the group's own
    evolution.
    """
    seen: dict[str, int] = {}
    for p in paths:
        try:
            for h, ts in revisions_for(p, since):
                seen[h] = ts
        except subprocess.CalledProcessError:
            continue
    return sorted(seen.items(), key=lambda kv: kv[1])
def classify_segment(prev: Sample, curr: Sample) -> str:
    """Classify the slope between two consecutive samples.
    Uses tighter thresholds than `classify()` because segment-level deltas
    are noisier than full-period deltas.
    """
    dt_pct = (curr.total - prev.total) / prev.total * 100 if prev.total else 0
    dl_pct = (curr.n - prev.n) / prev.n * 100 if prev.n else 0
    dmean = curr.mean - prev.mean
    if dt_pct < -8:
        return "REFACTORED"
    if dmean > 0.03 or (dt_pct - dl_pct > 5 and dt_pct > 3):
        return "DETERIORATING"
    if abs(dt_pct) < 2 and abs(dl_pct) < 2:
        return "STABLE"
    return "GROWING"
def _is_strong_flip(prev_verdict: str, new_verdict: str,
                    prev_sample: Sample, curr_sample: Sample) -> bool:
    """Hysteresis: only flip to a new verdict when the slope clearly
    exceeds the new verdict's own threshold. Otherwise small wobbles
    around the threshold produce an alternating stripe pattern that
    misrepresents the underlying stability.
    """
    if prev_verdict == new_verdict:
        return False
    dt_pct = ((curr_sample.total - prev_sample.total) / prev_sample.total * 100
              if prev_sample.total else 0)
    dmean = curr_sample.mean - prev_sample.mean
    # Stronger-than-threshold gates for each direction.
    if new_verdict == "DETERIORATING":
        return dmean > 0.05 or dt_pct > 5
    if new_verdict == "REFACTORED":
        return dt_pct < -12
    if new_verdict == "STABLE":
        # Require both deltas to be very small before declaring stable,
        # so we don't label an active file stable after one quiet commit.
        return abs(dt_pct) < 1 and abs(dmean) < 0.02
    return True  # GROWING — default fall-through is fine
def compute_bands(samples: list[Sample]) -> list[tuple[int, int, str]]:
    """Return [(start_idx, end_idx_inclusive, verdict), ...] with adjacent
    same-verdict segments merged into a single band.
    Uses hysteresis so a two-sample wobble doesn't spawn a short STABLE or
    GROWING band between two DETERIORATING stretches.
    """
    if len(samples) < 2:
        return []
    bands: list[tuple[int, int, str]] = []
    cur_verdict = classify_segment(samples[0], samples[1])
    cur_start = 0
    for i in range(1, len(samples) - 1):
        v = classify_segment(samples[i], samples[i + 1])
        if v != cur_verdict and _is_strong_flip(cur_verdict, v, samples[i], samples[i + 1]):
            bands.append((cur_start, i, cur_verdict))
            cur_start = i
            cur_verdict = v
    bands.append((cur_start, len(samples) - 1, cur_verdict))
    return bands
def classify(samples: list[Sample]) -> tuple[str, dict]:
    if len(samples) < 2:
        return "INSUFFICIENT", {}
    first, last = samples[0], samples[-1]
    peak_total = max(s.total for s in samples)
    loc_delta_pct = (last.n - first.n) / first.n * 100 if first.n else 0
    total_delta_pct = (last.total - first.total) / first.total * 100 if first.total else 0
    mean_delta = last.mean - first.mean
    refactor_dip_pct = (peak_total - last.total) / peak_total * 100 if peak_total else 0
    # Recent-window deltas — answers "is this rotting right now?" as distinct
    # from "did this rot at some point in the last 6 months?". Use the last
    # ~30% of samples as the "recent" window, with a floor of 2 samples.
    window = max(2, len(samples) // 3)
    recent_first = samples[-window]
    recent_loc_pct = (last.n - recent_first.n) / recent_first.n * 100 if recent_first.n else 0
    recent_total_pct = (last.total - recent_first.total) / recent_first.total * 100 if recent_first.total else 0
    recent_mean_delta = last.mean - recent_first.mean
    metrics = {
        "loc_delta_pct": round(loc_delta_pct, 1),
        "total_delta_pct": round(total_delta_pct, 1),
        "mean_delta": round(mean_delta, 3),
        "refactor_dip_pct": round(refactor_dip_pct, 1),
        "first_mean": first.mean,
        "last_mean": last.mean,
        "first_max": first.maxi,
        "last_max": last.maxi,
        "samples": len(samples),
        # Recent-window (last ~30% of samples) — measures active decay.
        "recent_window": window,
        "recent_loc_delta_pct": round(recent_loc_pct, 1),
        "recent_total_delta_pct": round(recent_total_pct, 1),
        "recent_mean_delta": round(recent_mean_delta, 3),
    }
    if refactor_dip_pct > 12:
        return "REFACTORED", metrics
    if mean_delta > 0.08 or (total_delta_pct > loc_delta_pct + 15 and total_delta_pct > 10):
        return "DETERIORATING", metrics
    if abs(total_delta_pct) < 5 and abs(mean_delta) < 0.05:
        return "STABLE", metrics
    return "GROWING", metrics
# --- plotting -------------------------------------------------------------
def _try_import_mpl():
    try:
        import matplotlib
        matplotlib.use("Agg")  # headless
        import matplotlib.pyplot as plt
        import matplotlib.dates as mdates
        from matplotlib.patches import Patch
        from matplotlib.lines import Line2D
        return plt, mdates, Patch, Line2D
    except ImportError:
        return None, None, None, None
def _sanitize(path: str) -> str:
    # Normalize so "./foo/bar.go" → "foo/bar.go" (otherwise the sanitized
    # filename would start with "." and become hidden on Unix).
    import os
    normalized = os.path.normpath(path)
    return normalized.replace("/", "__").replace("\\", "__")
def _apply_style(plt) -> None:
    plt.rcParams.update({
        "font.family": "DejaVu Sans",
        "font.size": 10,
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
def _format_date_axis(ax, mdates) -> None:
    locator = mdates.AutoDateLocator(minticks=4, maxticks=7)
    ax.xaxis.set_major_locator(locator)
    ax.xaxis.set_major_formatter(mdates.ConciseDateFormatter(locator, show_offset=False))
def _verdict_badge(ax, verdict: str, metrics: dict) -> None:
    """Colored pill in the top-left of an axis."""
    color = VERDICT_COLORS.get(verdict, "#444")
    blurb = VERDICT_BLURB.get(verdict, "")
    label = verdict if not blurb else f"{verdict} — {blurb}"
    ax.text(
        0.005, 1.04, label,
        transform=ax.transAxes,
        ha="left", va="bottom",
        fontsize=9, fontweight="bold", color="white",
        bbox=dict(boxstyle="round,pad=0.35", facecolor=color, edgecolor="none"),
    )
    if metrics:
        # Include cost-per-commit when we have it — makes the verdict
        # quantitative ("each commit added X indent units") and
        # comparable across files/groups of different activity levels.
        cpc = metrics.get("cost_per_commit")
        cpc_txt = f"   cost/commit {cpc:+.2f}" if cpc is not None else ""
        delta_txt = (f"Δ total {metrics['total_delta_pct']:+.0f}%   "
                     f"Δ LOC {metrics['loc_delta_pct']:+.0f}%   "
                     f"Δ mean indent {metrics['mean_delta']:+.2f}   "
                     f"peak-dip {metrics['refactor_dip_pct']:.0f}%"
                     + cpc_txt)
        ax.text(
            0.995, 1.04, delta_txt,
            transform=ax.transAxes,
            ha="right", va="bottom",
            fontsize=8.5, color="#404040",
        )
def plot_file(path: str, samples: list[Sample], verdict: str, metrics: dict, out_dir: Path) -> Path | None:
    mods = _try_import_mpl()
    plt, mdates, _Patch, _Line2D = mods
    if plt is None:
        return None
    if len(samples) < 2:
        return None
    _apply_style(plt)
    dates = [datetime.fromtimestamp(s.ts, tz=timezone.utc) for s in samples]
    totals = [s.total for s in samples]
    locs = [s.n for s in samples]
    means = [s.mean for s in samples]
    maxes = [s.maxi for s in samples]
    v_color = VERDICT_COLORS.get(verdict, "#444")
    bands = compute_bands(samples)
    fig, (ax1, ax2) = plt.subplots(
        2, 1, figsize=(12, 7), sharex=True,
        gridspec_kw={"height_ratios": [3, 2], "hspace": 0.28},
    )
    fig.subplots_adjust(top=0.86, bottom=0.09, left=0.07, right=0.93)
    # Paint per-segment verdict bands as background shading on both panels.
    # The last band extends to the right edge so the final sample's local
    # slope is visible.
    for start_i, end_i, seg_verdict in bands:
        seg_color = VERDICT_COLORS.get(seg_verdict, "#888")
        for ax in (ax1, ax2):
            ax.axvspan(dates[start_i], dates[end_i],
                       color=seg_color, alpha=0.11, zorder=0, linewidth=0)
    # ── Top panel: total indent vs LOC ─────────────────────────────────
    ax1.plot(dates, totals, color=v_color, linewidth=2.4,
             marker="o", markersize=4.5, label="total indent (complexity)")
    ax1.set_ylabel("total indent", color=v_color)
    ax1.tick_params(axis="y", labelcolor=v_color)
    # Headroom so peak annotations and legend don't clip the top edge.
    ax1.margins(y=0.12)
    ax1b = ax1.twinx()
    ax1b.plot(dates, locs, color=LOC_COLOR, linewidth=1.6, linestyle="--",
              marker="s", markersize=3.3, label="LOC")
    ax1b.set_ylabel("LOC", color=LOC_COLOR)
    ax1b.tick_params(axis="y", labelcolor=LOC_COLOR)
    ax1b.grid(False)
    ax1b.margins(y=0.12)
    # Shared legend — combine handles from both y-axes.
    h1, l1 = ax1.get_legend_handles_labels()
    h2, l2 = ax1b.get_legend_handles_labels()
    ax1.legend(h1 + h2, l1 + l2, loc="upper left", bbox_to_anchor=(0.0, 1.0))
    # Peak-total marker (the highest point on the red curve).
    if totals:
        pk = max(range(len(totals)), key=lambda i: totals[i])
        if pk not in (0, len(totals) - 1):
            ax1.scatter([dates[pk]], [totals[pk]], color=v_color, s=60,
                        zorder=5, edgecolor="white", linewidth=1.2)
            ax1.annotate(f"peak {totals[pk]:.0f}",
                         xy=(dates[pk], totals[pk]),
                         xytext=(6, 8), textcoords="offset points",
                         fontsize=7.5, color=v_color)
    _verdict_badge(ax1, verdict, metrics)
    # ── Bottom panel: mean + max indent ───────────────────────────────
    ax2.plot(dates, means, color=MEAN_COLOR, linewidth=2.0,
             marker="o", markersize=3.5, label="mean indent")
    ax2.set_ylabel("mean indent", color=MEAN_COLOR)
    ax2.tick_params(axis="y", labelcolor=MEAN_COLOR)
    ax2b = ax2.twinx()
    # p95: deep-pocket percentile. Drawn as a dashed translucent line on
    # the same axis as max so the reader sees the gap between "the
    # deepest line" and "95% of lines are below this" — a large gap
    # means one local hotspot, a small gap means the whole file is
    # uniformly nested.
    p95s = [s.p95 for s in samples]
    ax2b.step(dates, maxes, color=MAX_COLOR, linewidth=1.6,
              where="post", label="max indent")
    ax2b.scatter(dates, maxes, color=MAX_COLOR, marker="^", s=16, zorder=4)
    ax2b.step(dates, p95s, color=MAX_COLOR, linewidth=1.1, linestyle="--",
              alpha=0.55, where="post", label="p95 indent")
    ax2b.set_ylabel("max / p95 indent", color=MAX_COLOR)
    ax2b.tick_params(axis="y", labelcolor=MAX_COLOR)
    ax2b.grid(False)
    h3, l3 = ax2.get_legend_handles_labels()
    h4, l4 = ax2b.get_legend_handles_labels()
    ax2.legend(h3 + h4, l3 + l4, loc="upper left", bbox_to_anchor=(0.0, 1.0))
    _format_date_axis(ax2, mdates)
    # ── Figure-level title + verdict color key + footer ──────────────
    fig.suptitle(path, fontsize=13, fontweight="bold", y=0.975, x=0.07, ha="left")
    # Verdict color key — shows what the background shading means. Placed
    # just below the title on the right so it mirrors the overall-verdict
    # pill on the left.
    _mpl = _try_import_mpl()
    _, _, Patch, _ = _mpl
    key_handles = [
        Patch(facecolor=VERDICT_COLORS[v], alpha=0.55, edgecolor="none", label=v.title())
        for v in ("DETERIORATING", "GROWING", "STABLE", "REFACTORED")
    ]
    fig.legend(
        handles=key_handles, loc="upper right",
        bbox_to_anchor=(0.935, 0.975), ncol=4, fontsize=8,
        title="segment verdict", title_fontsize=8,
        frameon=True, edgecolor="#cccccc",
    )
    footer = (f"{len(samples)} samples · "
              f"{dates[0].strftime('%b %d, %Y')} → {dates[-1].strftime('%b %d, %Y')} · "
              f"indent metric: 1 tab or 4 spaces = 1 unit")
    fig.text(0.07, 0.015, footer, fontsize=7.5, color="#707070", ha="left")
    out_dir.mkdir(parents=True, exist_ok=True)
    out_file = out_dir / f"{_sanitize(path)}.png"
    fig.savefig(out_file, dpi=150)
    plt.close(fig)
    return out_file
def plot_summary(results: list[tuple[str, list[Sample], str, dict]], out_dir: Path) -> Path | None:
    """Small-multiples grid: normalized complexity vs LOC per file, colored by verdict."""
    mods = _try_import_mpl()
    plt, mdates, Patch, Line2D = mods
    if plt is None:
        return None
    plotted = [(p, s, v, m) for p, s, v, m in results if len(s) >= 2]
    if not plotted:
        return None
    _apply_style(plt)
    # Sort tiles by verdict severity so the eye sweeps red→green→blue.
    order = {"DETERIORATING": 0, "GROWING": 1, "STABLE": 2, "REFACTORED": 3, "INSUFFICIENT": 4}
    plotted.sort(key=lambda r: (order.get(r[2], 9), -r[3].get("total_delta_pct", 0)))
    n = len(plotted)
    cols = min(3, n)
    rows = math.ceil(n / cols)
    fig, axes = plt.subplots(
        rows, cols, figsize=(5.0 * cols, 3.2 * rows), squeeze=False,
    )
    fig.subplots_adjust(top=0.90, bottom=0.05, left=0.04, right=0.98,
                        hspace=0.45, wspace=0.22)
    for idx, (path, samples, verdict, metrics) in enumerate(plotted):
        ax = axes[idx // cols][idx % cols]
        color = VERDICT_COLORS.get(verdict, "#444")
        dates = [datetime.fromtimestamp(s.ts, tz=timezone.utc) for s in samples]
        totals = [s.total for s in samples]
        locs = [s.n for s in samples]
        t0 = totals[0] or 1
        l0 = locs[0] or 1
        # Per-segment verdict bands behind the lines.
        for start_i, end_i, seg_verdict in compute_bands(samples):
            ax.axvspan(dates[start_i], dates[end_i],
                       color=VERDICT_COLORS.get(seg_verdict, "#888"),
                       alpha=0.11, zorder=0, linewidth=0)
        ax.plot(dates, [t / t0 for t in totals], color=color,
                linewidth=2.4, label="total indent (normalized)")
        ax.plot(dates, [l / l0 for l in locs], color=LOC_COLOR,
                linewidth=1.4, linestyle="--", label="LOC (normalized)")
        ax.axhline(1.0, color="#bbb", linewidth=0.7, zorder=0)
        # Shade the gap where complexity overtakes LOC — makes deterioration pop.
        ys_t = [t / t0 for t in totals]
        ys_l = [l / l0 for l in locs]
        ax.fill_between(dates, ys_l, ys_t,
                        where=[a > b for a, b in zip(ys_t, ys_l)],
                        color=color, alpha=0.10, interpolate=True)
        ax.set_title(path, fontsize=9, color="#222", pad=4)
        # Verdict pill inside the axes.
        ax.text(
            0.02, 0.95, verdict,
            transform=ax.transAxes,
            ha="left", va="top",
            fontsize=7.5, fontweight="bold", color="white",
            bbox=dict(boxstyle="round,pad=0.25", facecolor=color, edgecolor="none"),
        )
        if metrics:
            ax.text(
                0.98, 0.05,
                f"Δt {metrics['total_delta_pct']:+.0f}%  ΔL {metrics['loc_delta_pct']:+.0f}%",
                transform=ax.transAxes,
                ha="right", va="bottom",
                fontsize=7.5, color="#404040",
            )
        _format_date_axis(ax, mdates)
        ax.tick_params(axis="x", labelrotation=0, labelsize=7)
        ax.tick_params(axis="y", labelsize=7)
    # Hide unused cells.
    for k in range(len(plotted), rows * cols):
        axes[k // cols][k % cols].axis("off")
    # Figure-level legend: line styles + verdict colors.
    style_handles = [
        Line2D([0], [0], color="#444", linewidth=2.4, label="total indent (normalized to first)"),
        Line2D([0], [0], color=LOC_COLOR, linewidth=1.4, linestyle="--", label="LOC (normalized to first)"),
    ]
    verdict_handles = [
        Patch(facecolor=VERDICT_COLORS[v], edgecolor="none",
              label=f"{v} — {VERDICT_BLURB[v]}")
        for v in ("DETERIORATING", "GROWING", "STABLE", "REFACTORED")
    ]
    leg1 = fig.legend(handles=style_handles, loc="upper left",
                      bbox_to_anchor=(0.04, 0.945), ncol=2, fontsize=8)
    fig.add_artist(leg1)
    fig.legend(handles=verdict_handles, loc="upper right",
               bbox_to_anchor=(0.98, 0.945), ncol=4, fontsize=8)
    out_dir.mkdir(parents=True, exist_ok=True)
    out_file = out_dir / "_summary.png"
    fig.suptitle(
        "Complexity trend — normalized total indent vs LOC, per file",
        fontsize=14, fontweight="bold", y=0.985,
    )
    fig.savefig(out_file, dpi=150)
    plt.close(fig)
    return out_file
# --- discovery ------------------------------------------------------------
DEFAULT_EXTS = (".go", ".js", ".ts", ".tsx", ".jsx", ".py", ".templ",
                ".rb", ".java", ".kt", ".swift", ".rs", ".c", ".cc",
                ".cpp", ".h", ".hpp", ".cs")
GENERATED_PATTERNS = (
    "_templ.go", "_mock.go", "_mocks.go", ".pb.go", "_gen.go",
    "zz_generated", "bindata.go", ".min.js", "node_modules/",
    "vendor/", "testdata/",
)
def _is_generated(path: str) -> bool:
    return any(pat in path for pat in GENERATED_PATTERNS)
def discover_candidates(since: str, min_commits: int, min_loc: int,
                        max_candidates: int) -> list[tuple[str, int, int]]:
    """Return (path, commit_count, loc) for currently-existing files that have
    enough churn and size to be worth analyzing. Sorted by commit count desc."""
    out = subprocess.run(
        ["git", "log", "--since", since, "--name-only", "--pretty=format:"],
        check=True, capture_output=True, text=True,
    )
    counts: dict[str, int] = {}
    for line in out.stdout.splitlines():
        p = line.strip()
        if not p or not p.endswith(DEFAULT_EXTS) or _is_generated(p):
            continue
        counts[p] = counts.get(p, 0) + 1
    candidates: list[tuple[str, int, int]] = []
    for path, n_commits in counts.items():
        if n_commits < min_commits:
            continue
        fp = Path(path)
        if not fp.is_file():
            continue  # renamed/deleted since
        try:
            loc = sum(1 for ln in fp.read_text(errors="ignore").splitlines() if ln.strip())
        except OSError:
            continue
        if loc < min_loc:
            continue
        candidates.append((path, n_commits, loc))
    candidates.sort(key=lambda r: -r[1])
    return candidates[:max_candidates]
def deterioration_score(metrics: dict) -> float:
    """Rank how badly a file is deteriorating over the full period.
    Signals are size-normalized and gap is capped so a rewrite (tiny first
    blob → huge deltas) doesn't dominate. Mean-indent growth is the cleanest
    structural-decay signal.
    """
    if not metrics:
        return 0.0
    raw_gap = max(0.0, metrics["total_delta_pct"] - metrics["loc_delta_pct"])
    gap = min(raw_gap, 150.0)                           # cap rewrite noise
    mean_signal = 150.0 * max(0.0, metrics["mean_delta"])
    max_jump = 80.0 * max(0.0, metrics["last_max"] - metrics["first_max"])
    absolute_depth = 10.0 * metrics["last_mean"]        # already-deep matters
    return gap + mean_signal + max_jump + absolute_depth
def recent_score(metrics: dict) -> float:
    """Score deterioration *in the recent window only* — answers "is this
    rotting right now?" as opposed to "did this rot at some point?".
    Files that were deteriorating 4 months ago but have stabilized will
    score low here. Files that look calm on the full-period view but are
    actively decaying this week will score high.
    """
    if not metrics or metrics.get("recent_window", 0) < 2:
        return 0.0
    raw_gap = max(0.0, metrics["recent_total_delta_pct"] - metrics["recent_loc_delta_pct"])
    gap = min(raw_gap, 80.0)
    mean_signal = 200.0 * max(0.0, metrics["recent_mean_delta"])  # recent mean moves are stronger signal
    return gap + mean_signal
def is_active_decay(metrics: dict) -> bool:
    """Pragmatic flag: the file is actively decaying in the recent window."""
    if not metrics:
        return False
    return (
        metrics.get("recent_mean_delta", 0) > 0.03
        or (metrics.get("recent_total_delta_pct", 0) > metrics.get("recent_loc_delta_pct", 0) + 5
            and metrics.get("recent_total_delta_pct", 0) > 3)
    )
# --- driver ---------------------------------------------------------------
def _inject_cost_per_commit(metrics: dict, samples: list[Sample], n_revs: int) -> None:
    """Fold cost-per-commit measurements into ``metrics`` in place.

    `cost_per_commit` is how much structural indent the unit gained per
    actual git commit across the window — a direct answer to "how
    expensive is each commit in complexity terms?". Makes DETERIORATING
    comparable across files / groups of different activity levels.
    The ``recent_`` variant uses the recent window so active-decay
    signals can be ranked the same way.
    """
    if not metrics or not samples:
        return
    total_delta = samples[-1].total - samples[0].total
    metrics["cost_per_commit"] = round(total_delta / n_revs, 3) if n_revs else 0.0
    win = metrics.get("recent_window", 0)
    if win >= 2:
        recent_first = samples[-win]
        recent_delta = samples[-1].total - recent_first.total
        # Approximate the recent rev count by the fraction of samples in-window.
        # Exact rev count would require threading revs into metrics, but the
        # sample ratio is a faithful proxy for the same span of time.
        recent_revs = max(1, int(n_revs * win / max(1, len(samples))))
        metrics["recent_cost_per_commit"] = round(recent_delta / recent_revs, 3)
    else:
        metrics["recent_cost_per_commit"] = 0.0


def _format_first_last(s: Sample) -> str:
    return (f"n={s.n:5d}  total={s.total:7.1f}  mean={s.mean:.3f}  "
            f"p95={s.p95:.2f}  max={s.maxi:.2f}")


def run(path: str, since: str, max_samples: int, quiet: bool = False) -> tuple[list[Sample], str, dict] | None:
    revs = revisions_for(path, since)
    if not revs:
        if not quiet:
            print(f"{path}: no commits since {since}", file=sys.stderr)
        return None
    samples = sample_history(path, revs, max_samples)
    if not samples:
        if not quiet:
            print(f"{path}: could not read blobs", file=sys.stderr)
        return None
    verdict, metrics = classify(samples)
    _inject_cost_per_commit(metrics, samples, len(revs))
    if not quiet:
        print(f"\n=== {path} ===")
        print(f"  revisions: {len(revs)}  samples: {len(samples)}")
        print(f"  FIRST  {_format_first_last(samples[0])}")
        print(f"  LAST   {_format_first_last(samples[-1])}")
        if metrics:
            print(f"  Δ LOC: {metrics['loc_delta_pct']:+.1f}%   Δ total: {metrics['total_delta_pct']:+.1f}%   "
                  f"Δ mean: {metrics['mean_delta']:+.3f}   peak-dip: {metrics['refactor_dip_pct']:.1f}%")
            print(f"  cost/commit: {metrics.get('cost_per_commit', 0):+.3f}   "
                  f"recent cost/commit: {metrics.get('recent_cost_per_commit', 0):+.3f}")
        print(f"  VERDICT: {verdict}")
    return samples, verdict, metrics


def run_group(label: str, paths: list[str], since: str, max_samples: int,
              quiet: bool = False) -> tuple[list[Sample], str, dict] | None:
    """Run complexity analysis on a *group* of files as a single unit.

    Motivation: in multi-file refactors, complexity moves between siblings
    (see the types.go / agent_definition_repo.go refactor) — file-level
    verdicts can cancel out. Running the group as one unit with pooled
    indent values reveals whether the group's overall structural shape
    actually improved.
    """
    if not paths:
        if not quiet:
            print(f"group {label}: no paths provided", file=sys.stderr)
        return None
    revs = revisions_for_group(paths, since)
    if not revs:
        if not quiet:
            print(f"group {label}: no commits since {since}", file=sys.stderr)
        return None
    samples = sample_group_history(paths, revs, max_samples)
    if not samples:
        if not quiet:
            print(f"group {label}: could not read any blobs", file=sys.stderr)
        return None
    verdict, metrics = classify(samples)
    _inject_cost_per_commit(metrics, samples, len(revs))
    if not quiet:
        print(f"\n=== [group] {label} ({len(paths)} files) ===")
        print(f"  union revisions: {len(revs)}  samples: {len(samples)}")
        for p in paths:
            print(f"    • {p}")
        print(f"  FIRST  {_format_first_last(samples[0])}")
        print(f"  LAST   {_format_first_last(samples[-1])}")
        if metrics:
            print(f"  Δ LOC: {metrics['loc_delta_pct']:+.1f}%   Δ total: {metrics['total_delta_pct']:+.1f}%   "
                  f"Δ mean: {metrics['mean_delta']:+.3f}   peak-dip: {metrics['refactor_dip_pct']:.1f}%")
            print(f"  cost/commit: {metrics.get('cost_per_commit', 0):+.3f}   "
                  f"recent cost/commit: {metrics.get('recent_cost_per_commit', 0):+.3f}")
        print(f"  VERDICT: {verdict}")
    return samples, verdict, metrics
def _compute_confidence(samples: list[Sample], metrics: dict) -> tuple[str, list[str]]:
    """Return (level, caveats). Confidence captures how much trust the reader
    should place in the verdict — based on data volume, recency-window size,
    and detection of rewrites that would distort full-period deltas.
    We'd rather say "medium confidence with caveats" than silently present a
    low-signal verdict as decisive.
    """
    caveats: list[str] = []
    level = "high"
    n = len(samples)
    if n < 4:
        level = "low"
        caveats.append(f"only {n} samples — trend direction is speculative")
    elif n < 8:
        level = "medium" if level == "high" else level
        caveats.append(f"{n} samples is borderline — treat the slope as indicative, not definitive")
    win = metrics.get("recent_window", 0)
    if win < 3:
        caveats.append(
            f"recent window is {win} samples — the active_decay flag is "
            f"low-confidence (a single noisy commit can flip it)"
        )
        if level == "high":
            level = "medium"
    # Rewrite detection — if any consecutive sample pair shows >70% LOC
    # change, the "file" likely isn't the same code across the whole window
    # and the first/last delta compares apples to oranges.
    for i in range(1, len(samples)):
        prev, curr = samples[i - 1], samples[i]
        if prev.n == 0:
            continue
        if abs(curr.n - prev.n) / prev.n > 0.7:
            caveats.append(
                f"possible rewrite between {prev.rev} and {curr.rev} "
                f"(LOC jumped {prev.n}→{curr.n}) — full-period deltas may not "
                f"reflect continuous code evolution"
            )
            level = "low"
            break
    # Verdict-threshold proximity: deterioration that only barely crosses the
    # threshold deserves a caveat so it isn't read with the same force as a
    # clear-cut case.
    mean_delta = metrics.get("mean_delta", 0)
    if 0.08 <= mean_delta < 0.12:
        caveats.append(
            f"Δmean {mean_delta:+.2f} is close to the deterioration threshold (0.08); "
            f"consider this a soft signal"
        )
        if level == "high":
            level = "medium"
    return level, caveats
def _build_entry(
    path: str, n_commits: int, loc: int,
    samples: list[Sample], verdict: str, metrics: dict,
) -> dict:
    """Assemble a JSON-serializable entry describing one file's trend.

    `combined_score` is computed here (instead of being mutated into the
    ranked subset later) so that every entry in the JSON report has the
    same schema regardless of whether it ended up in the ranked set.
    Previously, mutating the ranked entries after the fact also mutated
    the shared references in `all_entries` — meaning some files had
    `combined_score` and others didn't, which broke schema assumptions
    for downstream tools.
    """
    bands = compute_bands(samples)
    score = deterioration_score(metrics)
    rscore = recent_score(metrics)
    confidence, caveats = _compute_confidence(samples, metrics)
    return {
        "path": path,
        "is_test": is_test_file(path),
        "verdict": verdict,
        "confidence": confidence,
        "caveats": caveats,
        "score": round(score, 1),
        "recent_score": round(rscore, 1),
        "combined_score": round(score + 1.5 * rscore, 1),
        "active_decay": is_active_decay(metrics),
        "commits": n_commits,
        "loc": loc,
        "samples": len(samples),
        "metrics": metrics,
        "first": {
            "rev": samples[0].rev, "ts": samples[0].ts,
            "n": samples[0].n, "total": samples[0].total,
            "mean": samples[0].mean, "p95": samples[0].p95,
            "max": samples[0].maxi,
        },
        "last": {
            "rev": samples[-1].rev, "ts": samples[-1].ts,
            "n": samples[-1].n, "total": samples[-1].total,
            "mean": samples[-1].mean, "p95": samples[-1].p95,
            "max": samples[-1].maxi,
        },
        "segments": [
            {
                "from_ts": samples[s].ts, "to_ts": samples[e].ts,
                "verdict": v,
            }
            for s, e, v in bands
        ],
    }
def discover_mode(args) -> int:
    """Scan the repo, rank files by deterioration, plot the top N.
    Now emits a structured JSON report covering every screened file (not
    only the top N) so downstream tools (refactor_priority.py, CI gates)
    can filter and diff without re-running the analysis.
    """
    print(f"Discovering candidates touched since {args.since!r} "
          f"(≥{args.min_commits} commits, ≥{args.min_loc} LOC)...")
    candidates = discover_candidates(args.since, args.min_commits, args.min_loc,
                                     args.max_candidates)
    # Test-file filter — default exclude; tests rot differently (a 1000-line
    # test file is usually fine, deep nesting in table-driven setup is fine).
    if not args.include_tests:
        before = len(candidates)
        candidates = [c for c in candidates if not is_test_file(c[0])]
        print(f"  → {len(candidates)} candidates to screen "
              f"({before - len(candidates)} test files excluded; use --include-tests to keep).")
    else:
        print(f"  → {len(candidates)} candidates to screen.")
    if not candidates:
        print("  no candidates found.", file=sys.stderr)
        return 1
    print(f"Screening with {args.screen_samples} samples per file...")
    all_entries: list[dict] = []
    deteriorating: list[dict] = []
    for i, (path, n_commits, loc) in enumerate(candidates, 1):
        try:
            r = run(path, args.since, args.screen_samples, quiet=True)
        except subprocess.CalledProcessError:
            continue
        if r is None:
            continue
        samples, verdict, metrics = r
        entry = _build_entry(path, n_commits, loc, samples, verdict, metrics)
        all_entries.append(entry)
        if verdict != "DETERIORATING":
            continue
        # Skip files whose first sample was essentially empty — usually a
        # rewrite or brand-new file, not a deterioration.
        if samples[0].n < 100 or samples[0].total < 80:
            continue
        deteriorating.append(entry)
        print(f"  [{i:3d}/{len(candidates)}] {path}  "
              f"commits={n_commits}  LOC={loc}  score={entry['score']:.1f}  "
              f"recent={entry['recent_score']:.1f}"
              + ("  🔥" if entry["active_decay"] else ""))
    # Rank by combined score (full-period + 1.5× recent) — computed at
    # entry-build time, so no mutation of shared references here.
    deteriorating.sort(key=lambda e: -e["combined_score"])
    top = deteriorating[:args.top]
    if not top:
        print("\nNo deteriorating files found. 🎉")
    else:
        print(f"\nTop {len(top)} deteriorating files (score = full + 1.5·recent):")
        for rank, e in enumerate(top, 1):
            flag = " 🔥 active" if e["active_decay"] else ""
            print(f"  {rank:2d}. {e['path']:<60}  "
                  f"total={e['combined_score']:6.1f}  "
                  f"(full={e['score']:5.1f}  recent={e['recent_score']:5.1f}){flag}  "
                  f"Δmean={e['metrics']['mean_delta']:+.2f}  "
                  f"max={e['metrics']['last_max']:.0f}")
    # JSON export — always on if --json path provided.
    if args.json:
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "complexity_trend",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "since": args.since,
                "min_commits": args.min_commits,
                "min_loc": args.min_loc,
                "max_candidates": args.max_candidates,
                "screen_samples": args.screen_samples,
                "include_tests": args.include_tests,
            },
            "stats": {
                "candidates_screened": len(all_entries),
                "deteriorating_found": len(deteriorating),
            },
            "ranked": top,
            "all": all_entries,
        }
        Path(args.json).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.json}")
    if args.no_plot or not top:
        return 0
    out_dir = Path(args.out)
    print(f"\nRendering {len(top)} plots at {args.samples} samples each...")
    final: list[tuple[str, list[Sample], str, dict]] = []
    for e in top:
        r = run(e["path"], args.since, args.samples, quiet=True)
        if r is None:
            continue
        samples, verdict, metrics = r
        final.append((e["path"], samples, verdict, metrics))
        img = plot_file(e["path"], samples, verdict, metrics, out_dir)
        if img:
            print(f"  wrote {img}")
    if len(final) > 1:
        img = plot_summary(final, out_dir)
        if img:
            print(f"\nsummary: {img}")
    return 0
def _group_json_entry(label: str, paths: list[str],
                      samples: list[Sample], verdict: str, metrics: dict) -> dict:
    """Shape the JSON record for one group. Mirrors ``_build_entry`` but
    carries group-specific fields (label + member paths) so downstream
    tools can tell file entries from group entries.
    """
    return {
        "kind": "group",
        "label": label,
        "paths": paths,
        "verdict": verdict,
        "samples": len(samples),
        "metrics": metrics,
        "first": {
            "rev": samples[0].rev, "ts": samples[0].ts,
            "n": samples[0].n, "total": samples[0].total,
            "mean": samples[0].mean, "p95": samples[0].p95,
            "max": samples[0].maxi,
        },
        "last": {
            "rev": samples[-1].rev, "ts": samples[-1].ts,
            "n": samples[-1].n, "total": samples[-1].total,
            "mean": samples[-1].mean, "p95": samples[-1].p95,
            "max": samples[-1].maxi,
        },
    }


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("paths", nargs="*",
                    help="files to analyze (omit when using --discover)")
    ap.add_argument("--since", default="6 months ago")
    ap.add_argument("--samples", type=int, default=20,
                    help="samples per file for final plots (default 20)")
    ap.add_argument("--out", default="scripts/out/complexity_trend",
                    help="output directory for PNGs")
    ap.add_argument("--no-plot", action="store_true", help="skip PNG generation")
    ap.add_argument("--summary", action="store_true",
                    help="also emit a combined small-multiples grid")
    # Group mode — collapses positional paths into a single unit so a
    # multi-file refactor's net structural change is measurable.
    ap.add_argument("--group", type=str, default=None,
                    help="label the positional paths as one multi-file group; "
                         "stats are pooled across files at every revision")
    # Discover mode.
    ap.add_argument("--discover", action="store_true",
                    help="auto-find and rank the top N deteriorating files")
    ap.add_argument("--top", type=int, default=10,
                    help="how many deteriorating files to plot (default 10)")
    ap.add_argument("--min-commits", type=int, default=8,
                    help="min commits in window to consider a file (default 8)")
    ap.add_argument("--min-loc", type=int, default=150,
                    help="min non-blank lines to consider a file (default 150)")
    ap.add_argument("--max-candidates", type=int, default=120,
                    help="cap on candidates to screen (default 120)")
    ap.add_argument("--screen-samples", type=int, default=6,
                    help="cheap-screening samples per file (default 6)")
    ap.add_argument("--include-tests", action="store_true",
                    help="keep test files in --discover results (default: exclude)")
    ap.add_argument("--json", type=str,
                    help="write a structured report to this path "
                         "(schema complexity-trend/2; works in all modes)")
    args = ap.parse_args()
    if args.discover:
        return discover_mode(args)
    if not args.paths:
        ap.error("provide file paths or use --discover")
    out_dir = Path(args.out)
    # ── Group mode ───────────────────────────────────────────────────
    if args.group:
        gr = run_group(args.group, args.paths, args.since, args.samples)
        if gr is None:
            return 1
        samples, verdict, metrics = gr
        group_label = f"[group] {args.group}"
        if not args.no_plot:
            img = plot_file(group_label, samples, verdict, metrics, out_dir)
            if img:
                print(f"  wrote {img}")
        if args.json:
            report = {
                "schema": SCHEMA_VERSION,
                "tool": "complexity_trend",
                "generated_at": datetime.now(timezone.utc).isoformat(),
                "filters": {"since": args.since, "mode": "group"},
                "groups": [_group_json_entry(args.group, args.paths, samples, verdict, metrics)],
                "all": [],
            }
            Path(args.json).write_text(json.dumps(report, indent=2))
            print(f"\nJSON report: {args.json}")
        return 0
    # ── Per-file mode (existing) ─────────────────────────────────────
    results: list[tuple[str, list[Sample], str, dict]] = []
    rc = 0
    for p in args.paths:
        try:
            r = run(p, args.since, args.samples)
        except subprocess.CalledProcessError as e:
            print(f"{p}: git error: {e}", file=sys.stderr)
            rc = 1
            continue
        if r is None:
            rc = 1
            continue
        samples, verdict, metrics = r
        results.append((p, samples, verdict, metrics))
        if not args.no_plot:
            img = plot_file(p, samples, verdict, metrics, out_dir)
            if img:
                print(f"  wrote {img}")
            elif len(samples) < 2:
                pass
            else:
                print("  matplotlib not available, skipping PNG", file=sys.stderr)
    if args.summary and not args.no_plot and len(results) > 1:
        img = plot_summary(results, out_dir)
        if img:
            print(f"\nsummary: {img}")
    # JSON export in per-file mode — emit per-file entries with the new
    # schema so downstream tools can diff trends without re-running.
    if args.json:
        file_entries = []
        for path, samples, verdict, metrics in results:
            # Match the discover-mode `all` shape so refactor_priority.py
            # can consume this output with the same reader.
            file_entries.append({
                "path": path,
                "kind": "file",
                "verdict": verdict,
                "samples": len(samples),
                "metrics": metrics,
                "first": {
                    "rev": samples[0].rev, "ts": samples[0].ts,
                    "n": samples[0].n, "total": samples[0].total,
                    "mean": samples[0].mean, "p95": samples[0].p95,
                    "max": samples[0].maxi,
                },
                "last": {
                    "rev": samples[-1].rev, "ts": samples[-1].ts,
                    "n": samples[-1].n, "total": samples[-1].total,
                    "mean": samples[-1].mean, "p95": samples[-1].p95,
                    "max": samples[-1].maxi,
                },
            })
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "complexity_trend",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {"since": args.since, "mode": "files"},
            "groups": [],
            "all": file_entries,
        }
        Path(args.json).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.json}")
    return rc
if __name__ == "__main__":
    sys.exit(main())