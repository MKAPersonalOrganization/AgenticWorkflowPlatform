#!/usr/bin/env python3
"""Analyze linter CSV data and show trends."""

import csv
import sys
from pathlib import Path


def main():
    csv_path = Path("linter.csv")
    
    if not csv_path.exists():
        print("❌ No linter.csv file found - no snapshots have been captured yet")
        print("\nRun: go run tools/linter-viz/main.go -snapshot")
        sys.exit(1)
    
    with open(csv_path, 'r') as f:
        reader = csv.reader(f)
        rows = list(reader)
    
    if len(rows) < 2:
        print("❌ Not enough data - need at least one snapshot")
        sys.exit(1)
    
    headers = rows[0]
    first = [int(x) for x in rows[1]]
    latest = [int(x) for x in rows[-1]]
    num_snapshots = len(rows) - 1
    
    print("=" * 80)
    print(f"📊 Linter Analysis - {num_snapshots} snapshots")
    print("=" * 80)
    print()
    
    # Calculate changes
    changes = []
    for h, f, l in zip(headers, first, latest):
        change = l - f
        pct = ((l - f) / f * 100) if f > 0 else (100 if l > 0 else 0)
        changes.append((h, f, l, change, pct))
    
    # Sort by absolute change
    changes.sort(key=lambda x: abs(x[3]), reverse=True)
    
    # Show improvements (fixed issues)
    improvements = [(h, f, l, c, p) for h, f, l, c, p in changes if c < 0]
    if improvements:
        print("🎯 Improvements (Fixed Issues):")
        print("-" * 80)
        print(f"{'Linter':<20} {'First':<8} {'Latest':<8} {'Fixed':<10} {'% Change'}")
        print("-" * 80)
        for h, f, l, change, pct in improvements[:10]:
            print(f"{h:<20} {f:<8} {l:<8} {-change:<4d} 📉     {pct:+.1f}%")
        print()
    
    # Show regressions (new issues)
    regressions = [(h, f, l, c, p) for h, f, l, c, p in changes if c > 0]
    if regressions:
        print("⚠️  New/Increased Issues:")
        print("-" * 80)
        print(f"{'Linter':<20} {'First':<8} {'Latest':<8} {'Added':<10} {'% Change'}")
        print("-" * 80)
        for h, f, l, change, pct in regressions[:10]:
            print(f"{h:<20} {f:<8} {l:<8} {+change:<4d} 📈     {pct:+.1f}%")
        print()
    
    # Show unchanged
    unchanged = [(h, f, l, c, p) for h, f, l, c, p in changes if c == 0 and l > 0]
    if unchanged:
        print(f"➡️  Unchanged ({len(unchanged)} linters with persistent issues)")
        print("-" * 80)
        for h, f, l, change, pct in unchanged[:5]:
            print(f"{h:<20} {f:<8} (no change)")
        if len(unchanged) > 5:
            print(f"... and {len(unchanged) - 5} more")
        print()
    
    # Summary stats
    total_first = sum(first)
    total_latest = sum(latest)
    total_change = total_latest - total_first
    total_pct = ((total_latest - total_first) / total_first * 100) if total_first > 0 else 0
    
    print("=" * 80)
    print("📈 Summary:")
    print("-" * 80)
    print(f"Total issues (first snapshot):  {total_first:>6}")
    print(f"Total issues (latest snapshot): {total_latest:>6}")
    print(f"Net change:                     {total_change:+6} ({total_pct:+.1f}%)")
    print(f"Linters improved:               {len(improvements):>6}")
    print(f"Linters regressed:              {len(regressions):>6}")
    print("=" * 80)
    
    # Show snapshot history
    print()
    print("📅 Snapshot History:")
    print("-" * 80)
    for i, row in enumerate(rows[1:], 1):
        total = sum(int(x) for x in row)
        print(f"Snapshot {i:>2}: {total:>5} total issues")
    print("=" * 80)


if __name__ == "__main__":
    main()
