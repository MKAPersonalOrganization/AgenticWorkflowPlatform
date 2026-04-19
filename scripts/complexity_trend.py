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

SCHEMA_VERSION = "complexity-trend/1"

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
    """Whitespace complexity stats for a file blob."""
    indents = []
    for line in text.splitlines():
        if not line.strip():
            continue
        indents.append(logical_indent(line))
    if not indents:
        return {"n": 0, "total": 0.0, "mean": 0.0, "sd": 0.0, "max": 0.0}
    return {
        "n": len(indents),
        "total": round(sum(indents), 2),
        "mean": round(statistics.fmean(indents), 3),
        "sd": round(statistics.pstdev(indents) if len(indents) > 1 else 0.0, 3),
        "max": round(max(indents), 2),
    }


def revisions_for(path: str, since: str) -> list[tuple[str, int]]:
    """(hash, unix_ts) pairs that touched `path` in period, oldest first.

    `--follow` doesn't compose cleanly with `--since/--reverse`, so we fetch
    the full followed history and filter in Python.
    """
    ts_out = subprocess.run(
        ["date", "-d", since, "+%s"], capture_output=True, text=True,
    )
    cutoff = int(ts_out.stdout.strip()) if ts_out.returncode == 0 else 0

    out = subprocess.run(
        ["git", "log", "--follow", "--pretty=format:%H %ct", "--", path],
        check=True, capture_output=True, text=True,
    )
    revs = []
    for line in out.stdout.splitlines():
        if not line.strip():
            continue
        h, ts = line.split()
        if int(ts) >= cutoff:
            revs.append((h, int(ts)))
    revs.reverse()  # oldest first
    return revs


def path_at_revision(path: str, rev: str) -> str | None:
    """Resolve current `path` to its name at `rev` (handles renames)."""
    out = subprocess.run(
        ["git", "log", "--follow", "--name-only", "--pretty=format:", "-1", rev, "--", path],
        capture_output=True, text=True,
    )
    for line in out.stdout.splitlines():
        if line.strip():
            return line.strip()
    return path


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


def sample_history(path: str, revs: list[tuple[str, int]], max_samples: int) -> list[Sample]:
    if len(revs) > max_samples:
        step = len(revs) / max_samples
        idxs = sorted({int(i * step) for i in range(max_samples)} | {len(revs) - 1})
        revs = [revs[i] for i in idxs if i < len(revs)]
    samples = []
    for rev, ts in revs:
        rev_path = path_at_revision(path, rev) or path
        blob = blob_at(rev_path, rev)
        if blob is None:
            continue
        s = complexity_stats(blob)
        if s["n"] == 0:
            continue
        samples.append(Sample(rev[:9], ts, s["n"], s["total"], s["mean"], s["sd"], s["max"]))
    return samples


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


def compute_bands(samples: list[Sample]) -> list[tuple[int, int, str]]:
    """Return [(start_idx, end_idx_inclusive, verdict), ...] with adjacent
    same-verdict segments merged into a single band."""
    if len(samples) < 2:
        return []
    bands: list[tuple[int, int, str]] = []
    cur_verdict = classify_segment(samples[0], samples[1])
    cur_start = 0
    for i in range(1, len(samples) - 1):
        v = classify_segment(samples[i], samples[i + 1])
        if v != cur_verdict:
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
        delta_txt = (f"Δ total {metrics['total_delta_pct']:+.0f}%   "
                     f"Δ LOC {metrics['loc_delta_pct']:+.0f}%   "
                     f"Δ mean indent {metrics['mean_delta']:+.2f}   "
                     f"peak-dip {metrics['refactor_dip_pct']:.0f}%")
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
    ax2b.step(dates, maxes, color=MAX_COLOR, linewidth=1.6,
              where="post", label="max indent")
    ax2b.scatter(dates, maxes, color=MAX_COLOR, marker="^", s=16, zorder=4)
    ax2b.set_ylabel("max indent", color=MAX_COLOR)
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
    if not quiet:
        print(f"\n=== {path} ===")
        print(f"  revisions: {len(revs)}  samples: {len(samples)}")
        print(f"  FIRST  n={samples[0].n:5d}  total={samples[0].total:7.1f}  mean={samples[0].mean:.3f}  max={samples[0].maxi:.2f}")
        print(f"  LAST   n={samples[-1].n:5d}  total={samples[-1].total:7.1f}  mean={samples[-1].mean:.3f}  max={samples[-1].maxi:.2f}")
        if metrics:
            print(f"  Δ LOC: {metrics['loc_delta_pct']:+.1f}%   Δ total: {metrics['total_delta_pct']:+.1f}%   "
                  f"Δ mean: {metrics['mean_delta']:+.3f}   peak-dip: {metrics['refactor_dip_pct']:.1f}%")
        print(f"  VERDICT: {verdict}")
    return samples, verdict, metrics


def _build_entry(
    path: str, n_commits: int, loc: int,
    samples: list[Sample], verdict: str, metrics: dict,
) -> dict:
    """Assemble a JSON-serializable entry describing one file's trend."""
    bands = compute_bands(samples)
    score = deterioration_score(metrics)
    rscore = recent_score(metrics)
    return {
        "path": path,
        "is_test": is_test_file(path),
        "verdict": verdict,
        "score": round(score, 1),
        "recent_score": round(rscore, 1),
        "active_decay": is_active_decay(metrics),
        "commits": n_commits,
        "loc": loc,
        "samples": len(samples),
        "metrics": metrics,
        "first": {
            "rev": samples[0].rev, "ts": samples[0].ts,
            "n": samples[0].n, "total": samples[0].total,
            "mean": samples[0].mean, "max": samples[0].maxi,
        },
        "last": {
            "rev": samples[-1].rev, "ts": samples[-1].ts,
            "n": samples[-1].n, "total": samples[-1].total,
            "mean": samples[-1].mean, "max": samples[-1].maxi,
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

    # Rank by combined score: full-period + 1.5× recent (privileges current
    # decay without ignoring long-term rot).
    for e in deteriorating:
        e["combined_score"] = round(e["score"] + 1.5 * e["recent_score"], 1)
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
                    help="write a full structured report (all screened files) to this path")
    args = ap.parse_args()

    if args.discover:
        return discover_mode(args)

    if not args.paths:
        ap.error("provide file paths or use --discover")

    out_dir = Path(args.out)
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

    return rc


if __name__ == "__main__":
    sys.exit(main())
