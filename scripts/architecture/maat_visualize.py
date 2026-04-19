#!/usr/bin/env python3
"""
Code-Maat Visualization Dashboard
==================================
Reads CSV reports from .maat/ and generates visual charts.

Usage:
    python3 scripts/architecture/maat_visualize.py [--maat-dir .maat] [--output .maat/dashboard.png]

Charts generated:
    1. Hotspot Treemap — files sized by revision frequency
    2. Coupling Network — paired files that change together
    3. Code Age Distribution — stability histogram
    4. Churn Scatter — added vs deleted (bubble = commits)
    5. Fragmentation — most fragmented files
    6. Sum of Coupling — architectural bottlenecks
"""

import argparse
import os
import sys
from pathlib import Path

import matplotlib
matplotlib.use("Agg")  # headless backend

import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
from matplotlib.colors import LinearSegmentedColormap
import pandas as pd
import numpy as np

try:
    import squarify
    HAS_SQUARIFY = True
except ImportError:
    HAS_SQUARIFY = False

FILTER_MISSING = False


# ──────────────────────────────────────────────────────────────────
# Theme
# ──────────────────────────────────────────────────────────────────

BG_COLOR = "#0d1117"
CARD_BG = "#161b22"
TEXT_COLOR = "#e6edf3"
TEXT_MUTED = "#7d8590"
ACCENT_CYAN = "#58a6ff"
ACCENT_GREEN = "#3fb950"
ACCENT_ORANGE = "#d29922"
ACCENT_RED = "#f85149"
ACCENT_PURPLE = "#bc8cff"
ACCENT_PINK = "#f778ba"
GRID_COLOR = "#21262d"

# Gradient for heatmaps
HEAT_COLORS = ["#0d1117", "#1a3a5c", "#1f6feb", "#58a6ff", "#d29922", "#f85149"]
HEAT_CMAP = LinearSegmentedColormap.from_list("maat_heat", HEAT_COLORS, N=256)

plt.rcParams.update({
    "figure.facecolor": BG_COLOR,
    "axes.facecolor": CARD_BG,
    "axes.edgecolor": GRID_COLOR,
    "axes.labelcolor": TEXT_COLOR,
    "axes.titlepad": 14,
    "text.color": TEXT_COLOR,
    "xtick.color": TEXT_MUTED,
    "ytick.color": TEXT_MUTED,
    "grid.color": GRID_COLOR,
    "grid.alpha": 0.4,
    "font.family": "sans-serif",
    "font.size": 10,
})


def short_name(path: str, max_len: int = 30) -> str:
    """Shorten file paths for labels."""
    if len(path) <= max_len:
        return path
    parts = Path(path).parts
    if len(parts) <= 2:
        return path[-max_len:]
    return f"…/{'/'.join(parts[-2:])}"


def load_csv(maat_dir: str, name: str) -> pd.DataFrame | None:
    """Load a CSV from the maat directory, return None if missing."""
    path = os.path.join(maat_dir, name)
    if not os.path.exists(path):
        print(f"  ⚠  Skipping {name} (not found)")
        return None
    try:
        df = pd.read_csv(path)
        if FILTER_MISSING:
            if "entity" in df.columns:
                df = df[df["entity"].apply(lambda x: os.path.exists(x) if ":" not in str(x) else True)]
            if "coupled" in df.columns:
                df = df[df["coupled"].apply(lambda x: os.path.exists(x) if ":" not in str(x) else True)]
        # Always remove 'Other' from the plots
        if "entity" in df.columns:
            df = df[df["entity"] != "Other"]
        if "coupled" in df.columns:
            df = df[df["coupled"] != "Other"]
        if df.empty:
            print(f"  ⚠  Skipping {name} (empty after filtering)")
            return None
        return df
    except pd.errors.EmptyDataError:
        print(f"  ⚠  Skipping {name} (file is empty)")
        return None


# ──────────────────────────────────────────────────────────────────
# Chart 1: Hotspot Treemap
# ──────────────────────────────────────────────────────────────────

def plot_hotspots(ax, maat_dir):
    df = load_csv(maat_dir, "hotspots.csv")
    if df is None:
        ax.text(0.5, 0.5, "No hotspot data", ha="center", va="center",
                transform=ax.transAxes, color=TEXT_MUTED, fontsize=14)
        return

    top = df.nlargest(25, "n-revs").copy()
    top["label"] = top["entity"].apply(lambda x: short_name(x, 25))

    if HAS_SQUARIFY:
        sizes = top["n-revs"].values
        labels = [f"{l}\n{v}" for l, v in zip(top["label"], top["n-revs"])]

        norm = plt.Normalize(vmin=sizes.min(), vmax=sizes.max())
        colors = [HEAT_CMAP(norm(s)) for s in sizes]

        squarify.plot(
            sizes=sizes, label=labels, color=colors,
            ax=ax, alpha=0.9, text_kwargs={"fontsize": 7, "color": TEXT_COLOR},
            bar_kwargs={"linewidth": 1.5, "edgecolor": BG_COLOR},
        )
        ax.set_title("[HOT] Hotspot Treemap \u2014 Files by Revision Frequency",
                      fontsize=13, fontweight="bold", color=ACCENT_CYAN)
    else:
        # Fallback: horizontal bar chart
        ax.barh(top["label"][::-1], top["n-revs"][::-1], color=ACCENT_CYAN, alpha=0.85)
        ax.set_title("[HOT] Hotspots \u2014 Top 25 by Revision Frequency",
                      fontsize=13, fontweight="bold", color=ACCENT_CYAN)
        ax.set_xlabel("Revisions")

    ax.set_xticks([])
    ax.set_yticks([])


# ──────────────────────────────────────────────────────────────────
# Chart 2: Coupling Heatmap
# ──────────────────────────────────────────────────────────────────

def plot_coupling(ax, maat_dir):
    df = load_csv(maat_dir, "coupling.csv")
    if df is None or df.empty:
        ax.text(0.5, 0.5, "No coupling data", ha="center", va="center",
                transform=ax.transAxes, color=TEXT_MUTED, fontsize=14)
        return

    top = df.nlargest(15, "degree").copy()
    top["pair"] = (
        top["entity"].apply(lambda x: short_name(x, 20))
        + "\n↔ "
        + top["coupled"].apply(lambda x: short_name(x, 20))
    )

    colors = [ACCENT_RED if d >= 80 else ACCENT_ORANGE if d >= 50 else ACCENT_GREEN
              for d in top["degree"]]

    bars = ax.barh(range(len(top)), top["degree"].values, color=colors, alpha=0.85)
    ax.set_yticks(range(len(top)))
    ax.set_yticklabels(top["pair"].values, fontsize=7)
    ax.set_xlabel("Coupling Degree (%)")
    ax.set_title("[LINK] Logical Coupling \u2014 Files That Change Together",
                  fontsize=13, fontweight="bold", color=ACCENT_ORANGE)
    ax.invert_yaxis()
    ax.set_xlim(0, 105)

    # Add degree labels on bars
    for bar, deg in zip(bars, top["degree"].values):
        ax.text(bar.get_width() + 1, bar.get_y() + bar.get_height() / 2,
                f"{deg}%", va="center", fontsize=8, color=TEXT_MUTED)

    # Legend
    legend_patches = [
        mpatches.Patch(color=ACCENT_RED, label="≥80% (critical)"),
        mpatches.Patch(color=ACCENT_ORANGE, label="50-79% (watch)"),
        mpatches.Patch(color=ACCENT_GREEN, label="30-49% (low)"),
    ]
    ax.legend(handles=legend_patches, loc="lower right", fontsize=7,
              facecolor=CARD_BG, edgecolor=GRID_COLOR, labelcolor=TEXT_MUTED)


# ──────────────────────────────────────────────────────────────────
# Chart 3: Code Age Distribution
# ──────────────────────────────────────────────────────────────────

def plot_code_age(ax, maat_dir):
    df = load_csv(maat_dir, "code-age.csv")
    if df is None:
        ax.text(0.5, 0.5, "No code-age data", ha="center", va="center",
                transform=ax.transAxes, color=TEXT_MUTED, fontsize=14)
        return

    ages = df["age-months"]
    max_age = int(ages.max())
    bins = range(0, max_age + 2)

    counts, edges, patches = ax.hist(ages, bins=bins, color=ACCENT_CYAN,
                                      alpha=0.7, edgecolor=BG_COLOR, linewidth=0.5)

    # Color bins: recent = red, old = green
    for patch, edge in zip(patches, edges):
        if edge <= 1:
            patch.set_facecolor(ACCENT_RED)
            patch.set_alpha(0.85)
        elif edge <= 2:
            patch.set_facecolor(ACCENT_ORANGE)
            patch.set_alpha(0.8)
        else:
            patch.set_facecolor(ACCENT_GREEN)
            patch.set_alpha(0.7)

    ax.set_xlabel("Age (months since last change)")
    ax.set_ylabel("Number of Files")
    ax.set_title("[AGE] Code Age Distribution \u2014 Stability Map",
                  fontsize=13, fontweight="bold", color=ACCENT_GREEN)
    ax.grid(axis="y", alpha=0.3)

    legend_patches = [
        mpatches.Patch(color=ACCENT_RED, label="< 1 month (volatile)"),
        mpatches.Patch(color=ACCENT_ORANGE, label="1-2 months"),
        mpatches.Patch(color=ACCENT_GREEN, label="> 2 months (stable)"),
    ]
    ax.legend(handles=legend_patches, loc="upper right", fontsize=8,
              facecolor=CARD_BG, edgecolor=GRID_COLOR, labelcolor=TEXT_MUTED)


# ──────────────────────────────────────────────────────────────────
# Chart 4: Churn Scatter (added vs deleted)
# ──────────────────────────────────────────────────────────────────

def plot_churn(ax, maat_dir):
    df = load_csv(maat_dir, "churn.csv")
    if df is None:
        ax.text(0.5, 0.5, "No churn data", ha="center", va="center",
                transform=ax.transAxes, color=TEXT_MUTED, fontsize=14)
        return

    top = df.nlargest(40, "commits").copy()

    # Scale bubble sizes
    size_scale = top["commits"] / top["commits"].max() * 500

    scatter = ax.scatter(
        top["added"], top["deleted"],
        s=size_scale,
        c=top["commits"],
        cmap=HEAT_CMAP,
        alpha=0.75,
        edgecolors=ACCENT_CYAN,
        linewidths=0.5,
    )

    # Label top 8
    top_labeled = top.nlargest(8, "commits")
    for _, row in top_labeled.iterrows():
        ax.annotate(
            short_name(row["entity"], 22),
            xy=(row["added"], row["deleted"]),
            xytext=(8, 8), textcoords="offset points",
            fontsize=6.5, color=TEXT_MUTED,
            arrowprops=dict(arrowstyle="-", color=GRID_COLOR, lw=0.5),
        )

    # Diagonal line (added == deleted means constant rewriting)
    lim = max(top["added"].max(), top["deleted"].max()) * 1.1
    ax.plot([0, lim], [0, lim], "--", color=ACCENT_RED, alpha=0.3, linewidth=1)
    ax.text(lim * 0.65, lim * 0.72, "churn zone\n(rewrite risk)", fontsize=7,
            color=ACCENT_RED, alpha=0.5, style="italic", rotation=35)

    ax.set_xlabel("Lines Added")
    ax.set_ylabel("Lines Deleted")
    ax.set_title("[CHURN] Code Churn \u2014 Added vs Deleted (bubble = commits)",
                  fontsize=13, fontweight="bold", color=ACCENT_PURPLE)
    ax.grid(True, alpha=0.2)

    cbar = plt.colorbar(scatter, ax=ax, pad=0.02, shrink=0.8)
    cbar.set_label("Commits", color=TEXT_MUTED, fontsize=8)
    cbar.ax.yaxis.set_tick_params(color=TEXT_MUTED)
    plt.setp(cbar.ax.yaxis.get_ticklabels(), color=TEXT_MUTED, fontsize=7)



# ──────────────────────────────────────────────────────────────────
# Chart 7: Fragmentation
# ──────────────────────────────────────────────────────────────────

def plot_fragmentation(ax, maat_dir):
    df = load_csv(maat_dir, "fragmentation.csv")
    if df is None:
        ax.text(0.5, 0.5, "No fragmentation data", ha="center", va="center",
                transform=ax.transAxes, color=TEXT_MUTED, fontsize=14)
        return

    top = df.nlargest(20, "fractal-value").copy()
    top["label"] = top["entity"].apply(lambda x: short_name(x, 30))
    top = top.sort_values("fractal-value")

    colors = [ACCENT_RED if f >= 0.6 else ACCENT_ORANGE if f >= 0.4 else ACCENT_GREEN
              for f in top["fractal-value"]]

    bars = ax.barh(top["label"], top["fractal-value"], color=colors, alpha=0.85)

    for bar, val in zip(bars, top["fractal-value"]):
        ax.text(bar.get_width() + 0.01, bar.get_y() + bar.get_height() / 2,
                f"{val:.2f}", va="center", fontsize=7, color=TEXT_MUTED)

    ax.set_xlabel("Fragmentation Score (0 = single owner, 1 = evenly split)")
    ax.set_title("[FRAG] Code Fragmentation \u2014 Diffused Responsibility",
                  fontsize=13, fontweight="bold", color=ACCENT_ORANGE)
    ax.set_xlim(0, 1.0)
    ax.grid(axis="x", alpha=0.2)


# ──────────────────────────────────────────────────────────────────
# Chart 8: Sum of Coupling
# ──────────────────────────────────────────────────────────────────

def plot_soc(ax, maat_dir):
    df = load_csv(maat_dir, "soc.csv")
    if df is None:
        ax.text(0.5, 0.5, "No SoC data", ha="center", va="center",
                transform=ax.transAxes, color=TEXT_MUTED, fontsize=14)
        return

    top = df.nlargest(20, "soc").copy()
    top["label"] = top["entity"].apply(lambda x: short_name(x, 30))
    top = top.sort_values("soc")

    # Color by severity
    max_soc = top["soc"].max()
    norm = plt.Normalize(vmin=top["soc"].min(), vmax=max_soc)
    colors = [HEAT_CMAP(norm(v)) for v in top["soc"]]

    bars = ax.barh(top["label"], top["soc"], color=colors, alpha=0.9)

    for bar, val in zip(bars, top["soc"]):
        ax.text(bar.get_width() + max_soc * 0.01, bar.get_y() + bar.get_height() / 2,
                f"{val:,}", va="center", fontsize=7, color=TEXT_MUTED)

    ax.set_xlabel("Sum of Coupling")
    ax.set_title("[SoC] Sum of Coupling \u2014 Architectural Chokepoints",
                  fontsize=13, fontweight="bold", color=ACCENT_RED)
    ax.grid(axis="x", alpha=0.2)


# ──────────────────────────────────────────────────────────────────
# Dashboard Assembly
# ──────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Code-Maat Visualization Dashboard")
    parser.add_argument("--maat-dir", default=".maat", help="Directory containing CSV reports")
    parser.add_argument("--output", default=".maat/dashboard.png", help="Output image path")
    parser.add_argument("--filter-missing", action="store_true", help="Filter out entities that no longer exist on disk")
    args = parser.parse_args()

    global FILTER_MISSING
    FILTER_MISSING = args.filter_missing

    maat_dir = args.maat_dir
    output = args.output

    if not os.path.isdir(maat_dir):
        print(f"Error: {maat_dir} not found. Run 'task analyze' first.")
        sys.exit(1)

    # Load summary for title
    summary = load_csv(maat_dir, "summary.csv")
    summary_text = ""
    if summary is not None:
        stats = dict(zip(summary["statistic"], summary["value"]))
        summary_text = (
            f"  |  {stats.get('number-of-commits', '?')} commits  •  "
            f"{stats.get('number-of-entities', '?')} files  •  "
            f"{stats.get('number-of-authors', '?')} authors"
        )

    print(f"Generating dashboard from {maat_dir}...")

    fig = plt.figure(figsize=(24, 21))
    fig.patch.set_facecolor(BG_COLOR)

    # Title
    fig.suptitle(
        f"Code-Maat Analysis Dashboard{summary_text}",
        fontsize=20, fontweight="bold", color=ACCENT_CYAN,
        y=0.98,
    )

    # 3 rows × 2 columns layout
    gs = fig.add_gridspec(3, 2, hspace=0.35, wspace=0.35,
                          left=0.06, right=0.96, top=0.95, bottom=0.04)

    # Row 1
    ax1 = fig.add_subplot(gs[0, 0])
    plot_hotspots(ax1, maat_dir)

    ax2 = fig.add_subplot(gs[0, 1])
    plot_coupling(ax2, maat_dir)

    # Row 2
    ax3 = fig.add_subplot(gs[1, 0])
    plot_code_age(ax3, maat_dir)

    ax4 = fig.add_subplot(gs[1, 1])
    plot_churn(ax4, maat_dir)

    # Row 3
    ax5 = fig.add_subplot(gs[2, 0])
    plot_fragmentation(ax5, maat_dir)

    ax6 = fig.add_subplot(gs[2, 1])
    plot_soc(ax6, maat_dir)

    # Save dashboard
    fig.savefig(output, dpi=150, facecolor=BG_COLOR, bbox_inches="tight")
    plt.close(fig)

    file_size = os.path.getsize(output) / 1024
    print(f"✓ Dashboard saved to {output} ({file_size:.0f} KB)")

    # Save individual plots
    plots = {
        "hotspots": plot_hotspots,
        "coupling": plot_coupling,
        "code_age": plot_code_age,
        "churn": plot_churn,
        "fragmentation": plot_fragmentation,
        "soc": plot_soc
    }

    plots_dir = os.path.join(os.path.dirname(output), "plots")
    os.makedirs(plots_dir, exist_ok=True)
    basename = os.path.basename(output).replace("dashboard_", "").replace(".png", "")

    for name, func in plots.items():
        fig_single = plt.figure(figsize=(10, 7))
        fig_single.patch.set_facecolor(BG_COLOR)
        ax_single = fig_single.add_subplot(111)
        
        # We catch exceptions just in case a plot fails or is completely empty
        try:
            func(ax_single, maat_dir)
            out_path = os.path.join(plots_dir, f"{basename}_{name}.png")
            fig_single.savefig(out_path, dpi=130, facecolor=BG_COLOR, bbox_inches="tight")
        except Exception as e:
            print(f"  ⚠  Could not save individual plot {name}: {e}")
        finally:
            plt.close(fig_single)

    print(f"✓ Individual plots saved to {plots_dir}/")


if __name__ == "__main__":
    main()
