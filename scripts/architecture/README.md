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
| **Coupling (temporal)** | `file_coupling.py`    | Which files co-change at unexpected rates?                       |
| **Coupling (structural)** | `package_deps.py`   | Which Go packages concentrate fan-in/fan-out/cut-through ([goda](https://github.com/loov/goda))? |
| **Bug density**     | `bugfix_density.py`       | Which files absorb a disproportionate share of fix commits?      |
| **Test coverage**   | `coverage.py`             | Which Go files / packages are actually exercised by the existing test suite? Pairs with bugfix_density — high-fix + low-coverage is the clearest risk combo. |
| **Commit kinds**    | `commit_kinds.py`         | Conventional-commit prefix distribution (feat/fix/refactor/chore/…) per file. Tells you WHY a file churns — feat-dominant is active growth, fix-dominant is a bug magnet, refactor-dominant is pile-on risk. Complements bugfix_density's keyword matcher. |
| **Dead code**       | `dead_code.py`            | Unreachable functions (via `golang.org/x/tools/cmd/deadcode` call-graph traversal) + unused fields/consts/methods (via `staticcheck -checks U1000`). Surfaces delete-targets distinct from "untested" code. |
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

## External binaries

Several signals shell out to Go tools installed via `go install`:

- `cyclomatic.py` — uses [lizard](https://github.com/terryyin/lizard) (Python dep, in `requirements.txt`).
- `package_deps.py` — requires the `goda` binary on `$PATH`:
  `go install github.com/loov/goda@latest`.
- `coverage.py` — requires `go` on `$PATH` (the module's own toolchain).
- `dead_code.py` — requires `deadcode` + `staticcheck` on `$PATH` or `$GOPATH/bin`:
  ```
  go install golang.org/x/tools/cmd/deadcode@latest
  go install honnef.co/go/tools/cmd/staticcheck@latest
  ```

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
