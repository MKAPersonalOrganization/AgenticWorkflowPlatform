#!/usr/bin/env python3
"""
balanced_coupling.py — composite score from *Balancing Coupling in
Software Design* (Khononov, Ch. 10).

The book defines a single 1–10 number per dependency edge:

    Balance = max( |Strength − Distance| + 1,  1 + 10 − Volatility )

Intuition (the book's core claim): an edge is healthy when strength
and distance match — a tightly-coupled local pair (strength 7,
distance 3) is fine; a loose API call to a stdlib function (strength
1, distance 10) is fine. The edges that hurt are the ones where
strength and distance are far apart:

    - Strength 10 across distance 10: a hack into a distant stranger.
      Balance = |10 − 10| + 1 = 1 … but then volatility kicks the
      floor up if the other module changes often.
    - Strength 3, distance 7: a domain-type reference into a different
      top-level subtree. Balance = |3 − 7| + 1 = 5. Medium.
    - Strength 10, distance 3: intrusive access into a same-subtree
      cousin. Balance = |10 − 3| + 1 = 8. Bad.

Volatility protects against "this looks fine but my neighbour rewrites
itself every week". A target package at volatility 10 forces the edge
score to at least 1 + 10 − 10 = 1 (low floor, irrelevant), while a
target at volatility 1 (perfectly stable) forces at least 10 (all
edges look bad — correct: everything couples to a stable thing, but
the FLOOR says you can't pretend volatile-safety when it's actually
stable).

Wait — read the floor carefully. ``1 + 10 − Volatility`` rewards
volatile code (high volatility = low floor). The book's framing: if
the target is volatile, strength/distance mismatches hurt less
because the shape is going to change anyway. If the target is
rock-stable, even a big mismatch doesn't hurt because the thing
doesn't change. So the composite is actually ``max(structure, 1 +
10 − volatility_of_target)`` where the second term rewards
*instability* in an odd way — it sets a FLOOR that's lower for
high-churn targets. In practice the ``|S − D| + 1`` term dominates
for well-chosen structure.

Inputs (all JSON from sibling tools):
  --distance   coupling_distance.py  --json
  --strength   coupling_strength.py  --json
  --hotspots   go_hotspots.py        --json   (for per-package volatility)

We derive VOLATILITY from the target package's revision count over
the same window the hotspots report used. The raw count gets
quantile-mapped to 1–10 so volatility comparisons are relative to
*this* repo's change pace, not an absolute threshold.

Output:
  stdout       compact worst-first table
  --json FILE  per-edge balance + the 3 input dimensions

Schema: ``balanced-coupling/1``. No further consumers in the suite
yet; this is the reader's final aggregate view.

Usage:
    scripts/architecture/balanced_coupling.py \\
        --distance  .hotspots/coupling_distance.json \\
        --strength  .hotspots/coupling_strength.json \\
        --hotspots  .hotspots/hotspots_repo.json \\
        --json      .hotspots/balanced_coupling.json
"""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path


SCHEMA_VERSION = "balanced-coupling/1"


# Input loading ------------------------------------------------------------

def load_distance(path: str) -> dict[tuple[str, str], dict]:
    """Return {(source, target): edge_record}. The edge record keeps the
    fields we need downstream (distance + reason + within_module).
    """
    d = json.loads(Path(path).read_text())
    out: dict[tuple[str, str], dict] = {}
    for e in d.get("edges", []):
        key = (e["source"], e["target"])
        out[key] = e
    return out


def load_strength(path: str) -> dict[tuple[str, str], dict]:
    d = json.loads(Path(path).read_text())
    out: dict[tuple[str, str], dict] = {}
    for e in d.get("edges", []):
        key = (e["source"], e["target"])
        out[key] = e
    return out


def load_hotspots_volatility(path: str) -> dict[str, int]:
    """Return {package_import_path: revisions} from go_hotspots.py.

    The hotspots report contains ``kind=package`` entries at the top
    level. We fold ``metrics.revisions`` out of each. Packages absent
    from the report default to 0 revisions in the consumer.
    """
    d = json.loads(Path(path).read_text())
    out: dict[str, int] = {}
    for h in d.get("hotspots", []):
        if h.get("kind") != "package":
            continue
        import_path = h.get("import_path") or h.get("name") or ""
        if not import_path:
            continue
        revs = ((h.get("metrics") or {}).get("revisions") or 0)
        out[import_path] = revs
    return out


def load_git_volatility(module_prefix: str, since: str) -> dict[str, int]:
    """Compute per-package revision counts directly from git log.

    The hotspots report only carries the top-N packages, so ~100
    packages in this repo default to revisions=0 and hit the
    volatility=1 floor in the book's formula — which makes the
    composite score useless on most edges.

    We run ``git log --since=<since> --name-only --no-merges`` once
    and count distinct commits touching each package directory. The
    key uses the module-prefix import-path shape so it joins with
    the strength/distance edge maps.
    """
    args = ["git", "log", f"--since={since}", "--name-only", "--no-merges",
            "--pretty=format:__COMMIT__"]
    res = subprocess.run(args, check=True, capture_output=True, text=True)
    pkg_commits: dict[str, set[int]] = defaultdict(set)
    commit_idx = 0
    for line in res.stdout.splitlines():
        if line.startswith("__COMMIT__"):
            commit_idx += 1
            continue
        line = line.strip()
        if not line or not line.endswith(".go"):
            continue
        if "_test.go" in line or "_templ.go" in line:
            continue
        if line.startswith(".READ/") or line.startswith("vendor/"):
            continue
        # Map file path to its package's import path.
        parent = line.rsplit("/", 1)[0] if "/" in line else ""
        pkg = f"{module_prefix}/{parent}" if parent else module_prefix
        pkg_commits[pkg].add(commit_idx)
    return {pkg: len(commits) for pkg, commits in pkg_commits.items()}


def read_module_prefix() -> str:
    go_mod = Path("go.mod")
    if not go_mod.exists():
        return ""
    for line in go_mod.read_text().splitlines():
        line = line.strip()
        if line.startswith("module "):
            return line.split(None, 1)[1].strip()
    return ""


# Volatility normalization -------------------------------------------------

def normalize_volatility(revs_by_pkg: dict[str, int]) -> dict[str, int]:
    """Map raw revision counts to 1–10 using quantile bucketing.

    Why quantiles, not linear: revision counts are heavily right-
    skewed — one or two packages with 70+ revs drag the mean up and
    flatten everything else. Quantiles give us a uniform 1–10
    distribution that's comparable across repos.

    Implementation: sort packages by revisions, assign buckets in
    order. Packages with 0 revisions (not seen in the window) get
    volatility 1 — by definition they're as stable as it gets.
    """
    if not revs_by_pkg:
        return {}
    sorted_items = sorted(revs_by_pkg.items(), key=lambda kv: kv[1])
    n = len(sorted_items)
    out: dict[str, int] = {}
    for idx, (pkg, revs) in enumerate(sorted_items):
        if revs == 0:
            out[pkg] = 1
            continue
        # percentile rank 0..1, mapped to 1..10 with 10 = highest.
        pct = idx / max(1, n - 1)
        out[pkg] = max(1, min(10, int(round(1 + pct * 9))))
    return out


# Balance computation ------------------------------------------------------

def balance(strength: int, distance: int, volatility: int) -> dict:
    """Apply the book's equation. Returns the number + both candidate
    terms so readers can tell which arm of ``max(…)`` won.
    """
    structural_term = abs(strength - distance) + 1
    volatility_term = 1 + (10 - volatility)
    score = max(structural_term, volatility_term)
    return {
        "balance": score,
        "structural_term": structural_term,
        "volatility_term": volatility_term,
        "dominant": "structural" if structural_term >= volatility_term else "volatility",
    }


# Reporting ----------------------------------------------------------------

def print_stdout(edges: list[dict], top: int) -> None:
    if not edges:
        print("No edges to report — did the inputs match on (source, target) keys?")
        return
    total = len(edges)
    hist = Counter(e["balance"] for e in edges)

    # 1–10 histogram
    print(f"Edges scored: {total}")
    print("Balance histogram (higher = worse):")
    for b in range(1, 11):
        count = hist.get(b, 0)
        if count == 0:
            continue
        bar = "█" * min(40, int(40 * count / total))
        print(f"  balance={b:>2}  {count:>5}  {bar}")
    print()

    worst = sorted(
        edges,
        key=lambda e: (-e["balance"], -e["strength"], -e["distance"], e["source"], e["target"]),
    )
    print(f"── Top {min(top, len(worst))} worst-balance edges ──")
    hdr = (f"  {'B':>2}  {'S':>2}  {'D':>2}  {'V':>2}  "
           f"{'dom':<10}  Source → Target")
    print(hdr)
    for e in worst[:top]:
        print(f"  {e['balance']:>2}  {e['strength']:>2}  {e['distance']:>2}  {e['volatility']:>2}  "
              f"{e['dominant']:<10}  {e['source']}  →  {e['target']}")


# Driver -------------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=__doc__.split("Inputs")[0].strip(),
    )
    ap.add_argument("--distance", required=True, help="coupling_distance.py --json output")
    ap.add_argument("--strength", required=True, help="coupling_strength.py --json output")
    ap.add_argument("--hotspots", help="go_hotspots.py --json output (optional; if omitted "
                                       "revisions are computed inline via git log)")
    ap.add_argument("--since", default="6 months ago",
                    help="git log window for the inline volatility pass (default '6 months ago'); "
                         "ignored when --hotspots is supplied")
    ap.add_argument("--internal-only", action="store_true",
                    help="skip external-dependency edges (distance=10, no strength data)")
    ap.add_argument("--top", type=int, default=25,
                    help="how many rows per stdout table (default 25)")
    ap.add_argument("--json", type=str,
                    help="write JSON report to this path")
    args = ap.parse_args()

    dist_by_edge = load_distance(args.distance)
    strength_by_edge = load_strength(args.strength)
    # Prefer inline git log over hotspots because hotspots only carries
    # the top-N packages; the tail defaults to volatility=1 and hits
    # the formula's floor for every edge, making the score unusable.
    if args.hotspots:
        revs_by_pkg = load_hotspots_volatility(args.hotspots)
    else:
        module_prefix = read_module_prefix()
        if not module_prefix:
            raise SystemExit("error: could not read module path from go.mod")
        revs_by_pkg = load_git_volatility(module_prefix, args.since)
    vol_by_pkg = normalize_volatility(revs_by_pkg)

    # Inner-join: edges that have both a distance AND a strength are
    # the ones we can score. External edges appear in distance but not
    # in strength (we can't walk external source), so they get dropped
    # unless the caller explicitly opted in (there's no opt-in here —
    # external edges have no useful strength number). --internal-only
    # is the default behaviour already and the flag is reserved for
    # future override.
    scored: list[dict] = []
    for key, dist_edge in dist_by_edge.items():
        strength_edge = strength_by_edge.get(key)
        if strength_edge is None:
            continue
        target = key[1]
        volatility = vol_by_pkg.get(target, 1)
        bal = balance(strength_edge["strength"], dist_edge["distance"], volatility)
        scored.append({
            "source": key[0],
            "target": key[1],
            "strength": strength_edge["strength"],
            "strength_tier": strength_edge["tier"],
            "distance": dist_edge["distance"],
            "distance_reason": dist_edge["reason"],
            "volatility": volatility,
            "target_revisions": revs_by_pkg.get(target, 0),
            "contract_refs": strength_edge.get("contract_refs", 0),
            "model_refs": strength_edge.get("model_refs", 0),
            "common_refs": strength_edge.get("common_refs", 0),
            "intrusive_refs": strength_edge.get("intrusive_refs", 0),
            "func_calls": strength_edge.get("func_calls", 0),
            **bal,
        })

    print_stdout(scored, args.top)

    if args.json:
        hist = Counter(e["balance"] for e in scored)
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "balanced_coupling",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "formula": "max(|Strength - Distance| + 1, 1 + 10 - Volatility)",
            "inputs": {
                "distance": args.distance,
                "strength": args.strength,
                "hotspots": args.hotspots,
            },
            "stats": {
                "edges_scored": len(scored),
                "balance_histogram": {str(k): v for k, v in hist.items()},
                "mean_balance": round(sum(e["balance"] for e in scored) / len(scored), 2) if scored else 0.0,
                "edges_with_balance_ge_7": sum(1 for e in scored if e["balance"] >= 7),
                "edges_with_balance_ge_5": sum(1 for e in scored if e["balance"] >= 5),
            },
            "edges": sorted(
                scored,
                key=lambda e: (-e["balance"], -e["strength"], -e["distance"], e["source"], e["target"]),
            ),
        }
        Path(args.json).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.json}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
