# Architectural analysis scripts

Static + historical metrics for the repo's Go / JS / templ tree, run from
local git history. Output is data (JSON + PNGs); judgment is for the
reader. Scripts here are standalone — no cross-imports.

## Signals (one tool per aspect)

| Signal              | Script                    | Answers                                                          |
|---                  |---                        |---                                                               |
| **Hotspots**        | `go_hotspots.py`          | Which Go packages/functions are both large and most-changed?     |
| **Complexity (trend)** | `complexity_trend.py`  | Is whitespace-indent complexity (proxy for nesting) deteriorating? |
| **Complexity (cyclomatic)** | `cyclomatic.py`   | Which functions branch too much (McCabe CCN via [lizard](https://github.com/terryyin/lizard))? Multi-language. |
| **Cohesion**        | `cohesion.py`             | Do Go types juggle disjoint responsibilities (LCOM4 ≥ 2)?         |
| **Coupling**        | `file_coupling.py`        | Which files co-change at unexpected rates (temporal coupling)?   |
| **Bug density**     | `bugfix_density.py`       | Which files absorb a disproportionate share of fix commits?      |
| **Code-Maat dashboard** | `maat_visualize.py`   | CSV-driven visualization of an external Code-Maat report set.    |

## Synthesis (cross-signal)

- `refactor_priority.py` — merges the JSON outputs from every signal
  above into one data report (markdown + JSON + cross-signal matrix
  and scatter PNGs). Does not rank, score, or recommend; the reader
  reads across rows to decide.

## Supporting files

- `requirements.txt` — Python deps for the signal scripts.
- `go_grouping.txt` — package-grouping rules used by some signals to
  roll sub-packages up to module-level views.
- `go_hotspots.md` — in-depth reference for `go_hotspots.py`.

## Running

```bash
python3 -m venv .venv && . .venv/bin/activate
pip install -r scripts/architecture/requirements.txt

# one signal at a time
python3 scripts/architecture/complexity_trend.py --discover --since '6 months ago' --json out/trends.json

# cross-signal report with plots
python3 scripts/architecture/refactor_priority.py \
  --hotspots out/hotspots-*.json \
  --trends out/trends.json \
  --coupling out/coupling.json \
  --bugfix out/bugfix.json \
  --lcom4 out/cohesion.json \
  --cyclomatic out/cyclomatic.json \
  --out-md out/refactor_priority.md \
  --out-plots out/plots
```

Outputs default to `scripts/out/…` (gitignored). `debug-agent-run.sh`
stays at `scripts/` root — it's unrelated to architecture analysis.
