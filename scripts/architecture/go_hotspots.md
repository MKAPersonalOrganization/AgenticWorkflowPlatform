# `go_hotspots` — Architectural hotspot analysis for Go

One tool, three zoom levels, four output surfaces, and three exit codes.

- Ranks the parts of a Go codebase most worth refactoring using git history
  and source-code complexity — Tornhill's *Your Code as a Crime Scene*
  methodology applied recursively at repo, package, and file scale.
- Emits a human-friendly **PNG** circle-pack diagram, an interactive
  **HTML** diagram with hover tooltips, a versioned **JSON** report
  (schema `go-hotspots/1`), a **CSV** table, and a ranked **stdout**
  summary.
- Usable by humans (PNG / HTML), CI gates (JSON + exit codes), and
  automated refactoring agents (JSON with signals + suggested actions).

---

## Contents

1. [Cheatsheet — the five invocations you'll use most](#1-cheatsheet)
2. [The theory](#2-the-theory)
3. [The three fractal modes](#3-the-three-fractal-modes)
4. [Reading the diagram](#4-reading-the-diagram)
5. [The JSON report (for CI and AI consumers)](#5-the-json-report)
6. [Signals and suggested actions](#6-signals-and-suggested-actions)
7. [CLI reference](#7-cli-reference)
8. [Exit codes](#8-exit-codes)
9. [CSV output](#9-csv-output)
10. [Workflows and recipes](#10-workflows-and-recipes)
11. [CI integration examples](#11-ci-integration-examples)
12. [How it works — internals](#12-how-it-works--internals)
13. [Troubleshooting and FAQ](#13-troubleshooting-and-faq)
14. [Limitations — when NOT to trust it](#14-limitations)
15. [Glossary](#15-glossary)
16. [Further reading](#16-further-reading)

---

## 1. Cheatsheet

Five invocations cover ~95% of day-to-day use.

```bash
# A. Get the lay of the land (repo scale, last 2 months).
#    Common generated files (_templ.go, _mock.go, .pb.go, _gen.go,
#    zz_generated, bindata.go) are auto-excluded. Dead files are
#    auto-ignored. HTML is written alongside the PNG when plotly is
#    installed.
python3 scripts/go_hotspots.py . --since "2 months ago" \
  --exclude '^frontend/'

# B. Zoom into the hottest package (function-level view).
python3 scripts/go_hotspots.py modules/evaluation --since "2 months ago"

# C. Zoom into a single file (function-level view inside the file).
python3 scripts/go_hotspots.py modules/evaluation/service.go \
  --since "2 months ago"

# D. CI gate — emit JSON, compare to baseline, fail on regression.
python3 scripts/go_hotspots.py . --since "3 months ago" \
  --exclude '^frontend/' \
  --json hotspots.json --baseline .ci/hotspots-baseline.json \
  --fail-on-worsening-pct 25 --fail-on-new-hotspot \
  --no-image --quiet

# E. Feed an AI refactoring agent.
python3 scripts/go_hotspots.py . --since "1 month ago" \
  --exclude '^frontend/' \
  --json hotspots.json --top-n 5 --no-image --quiet
# Then point your agent at hotspots.json; each top hotspot carries
# recent_commits, authors, signals, suggested_actions, file paths,
# and (in function mode) precise start/end line ranges.
```

All five invocations lean on sensible defaults. Opt out with
`--no-default-excludes`, `--keep-dead-files`, or `--no-html` when you
need to.

---

## 2. The theory

This tool implements Chapter 3 of Adam Tornhill's *Your Code as a Crime
Scene*. The core insight:

> A version-control system records the steps every developer took. Combine
> that with a complexity estimate and you get a geographic profile of the
> code — a way to find where trouble is hiding before you spend a week
> refactoring the wrong thing.

Three things make hotspots work:

1. **Change frequency predicts defects.** Files that change a lot have
   more defects per line — every change is another chance to break
   something. One of the strongest empirical predictors in software
   engineering research (Hassan, Moser, Nagappan).
2. **Complexity alone is a bad target.** A 5000-line file that nobody
   touches is *stable*; rewriting it is expensive with zero payback.
   A 200-line file touched every day might already be fine.
3. **Change × complexity ranks parts of the code where improvement is
   *both* needed *and* will pay back.** Small quiet things drop out.
   So do big frozen ones. What's left is the inventory of things
   actually worth refactoring.

This replaces intuition ("this module feels messy") with evidence.
Intuition doesn't scale across a 100k-line codebase and is biased by
whatever you touched last week.

**The ranking formula:** `score = revisions × LOC`. Both inputs are
measured over a user-chosen window via `--since` / `--until`.

### Why "fractal"

The same ranking idea applies at every scale:

- **Repo scale** — which *packages* should we prioritise?
- **Package scale** — which *files* or *functions* carry the weight?
- **File scale** — which *function* in this file is the real hotspot?

Drilling down turns a vague "refactor `evaluation`" into a concrete
"split `service.createEvalRun`: 78 LOC, touched in 3 of the last 2
months' commits."

---

## 3. The three fractal modes

The tool auto-detects the right mode from the path you give it:

| You point it at | Mode | Leaves | Containers |
|---|---|---|---|
| Directory with sub-packages (`./modules`, `.`) | `package` | Go packages | grouping directories |
| Single-package directory (only `.go` files inside, no sub-packages) | `function` | top-level funcs/methods | files in the package |
| Single `.go` file | `function` | top-level funcs/methods | the file itself |

Force a specific mode with `--mode {auto,package,function}`.

**Package history** uses a single `git log --name-only` call: we
aggregate distinct commit hashes per directory. One commit touching
five files in the same package counts as one change, not five.

**Function history** uses `git log -L <start>,<end>:<file>` — git's own
line-history tracker, which follows moves *within* a file (renames
across files aren't tracked; that limitation is shared with CodeScene).

**Function boundaries** come from an inline Go parser (regex + brace
counting on a noise-stripped copy of the source: comments, strings, and
rune literals are replaced with spaces first, so braces inside literals
can't confuse the counter). It handles methods, generic receivers
(`(r *Type[T])`), and generic functions (`func Foo[T any](...)`).
Multi-line signatures with `{` inside type constraints may be
mis-parsed — those are rare.

---

## 4. Reading the diagram

The PNG is a **circle-pack enclosure diagram** — the same layout
CodeScene uses (Bostock's d3-hierarchy pack, implemented inline in pure
Python using the Wang et al. front-chain algorithm).

| Thing on screen | Package mode | Function mode |
|---|---|---|
| Filled coloured circle | one Go package | one top-level func or method |
| Circle radius | √(Σ LOC in the package) | √(LOC of the function body) |
| Fill colour (yellow → red) | distinct git commits touching the package | distinct commits touching the function's line range |
| Outlined circle | grouping directory | enclosing file |
| Thick black outline | top-12 hotspot | top-12 hotspot |
| Numbered edge badge (on boundary) | hotspot rank (1 = hottest) | hotspot rank |
| Sidebar entry | import path, revs · LOC · files · score | import path, revs · LOC · score |
| Caption / colorbar / heading | auto-adapts to the current mode | |

### How to read it in under a minute

1. Look for **big red circles** — those are the hotspots.
2. Read their names in the sidebar. That's your action list.
3. **Ignore tiny grey/yellow circles** — fresh or stable, not your problem.
4. **Ignore large yellow circles** — big but cold. Working code, leave alone.

Rule of thumb: the top 3–5 items in the sidebar contain ~70% of the
"pain" in most scopes, at every zoom level.

---

## 5. The JSON report

Invoke with `--json path.json`. The report is versioned
(`schema: "go-hotspots/1"`). Consumers should reject unknown schema
versions rather than guess.

### Top-level shape

```json
{
  "schema": "go-hotspots/1",
  "tool": {"name": "go_hotspots", "version": "1.0"},
  "generated_at": "2026-04-18T14:22:00+00:00",
  "scope": {
    "arg": "modules",
    "resolved": "/home/user/repo/modules",
    "mode": "package",
    "module_path": "aicompass",
    "module_dir": "/home/user/repo"
  },
  "window": {"since": "2 months ago", "until": null},
  "filters": {
    "include_tests": false,
    "ignore_dead_files": false,
    "exclude": ["_templ\\.go$"]
  },
  "stats": {
    "leaves": 76, "total_files": 222,
    "total_commits": 128, "top_score": 47656
  },
  "thresholds": {
    "fail_on_score": null,
    "fail_on_new_hotspot": false,
    "fail_on_worsening_pct": null
  },
  "hotspots": [ /* see below */ ],
  "baseline_delta": /* only present if --baseline was given */
}
```

### One hotspot entry

```json
{
  "rank": 1,
  "kind": "package",
  "name": "evaluation",
  "import_path": "aicompass/modules/evaluation",
  "path": "modules/evaluation",
  "file_path": null,
  "start_line": null,
  "end_line": null,
  "metrics": {
    "loc": 3404, "files": 10,
    "revisions": 14, "score": 47656
  },
  "recent_commits": [
    {
      "hash": "fe2f389178a9",
      "date": "2026-04-17",
      "author": "Alice Example",
      "subject": "refactor(auth): move canonical User",
      "looks_like_fix": false
    }
  ],
  "authors": [
    {"name": "Alice Example", "commits": 12, "pct": 85.7},
    {"name": "Bob Example",   "commits": 2,  "pct": 14.3}
  ],
  "author_count": 2,
  "bug_fix_count": 3,
  "bug_fix_pct": 21.4,
  "signals": [
    {"kind": "high_churn",   "severity": "medium", "detail": "14 commits in window"},
    {"kind": "large",        "severity": "high",   "detail": "3404 LOC"},
    {"kind": "knowledge_concentration", "severity": "high",
     "detail": "Alice Example wrote 86% of commits (12/14)"}
  ],
  "suggested_actions": [
    "Knowledge silo — pair with the primary author before refactoring...",
    "Large and hot package — split along the axes that change together..."
  ],
  "sample_files": [
    "modules/evaluation/handler.go",
    "modules/evaluation/service.go"
  ]
}
```

In **function mode** each entry also has `file_path`, `start_line`,
`end_line` — enough for an editor or an AI agent to fetch exactly the
right code without re-analysing the repo.

### `baseline_delta` (only with `--baseline`)

```json
{
  "baseline_generated_at": "2026-03-18T10:00:00+00:00",
  "baseline_schema": "go-hotspots/1",
  "summary": {
    "new_count": 1, "gone_count": 0,
    "hotter_count": 3, "cooler_count": 5
  },
  "new":    [{"path":"...", "name":"...", "score":12345, "rank":4}, ...],
  "gone":   [{"path":"...", "score_was":23456}, ...],
  "hotter": [{"path":"...", "score_from":1000, "score_to":2000,
              "delta":1000, "delta_pct":100.0}, ...],
  "cooler": [{"path":"...", "score_from":5000, "score_to":3000,
              "delta":-2000, "delta_pct":-40.0}, ...]
}
```

### Guarantees

- **Stable ordering.** Hotspots are ranked by `score` desc; ties broken
  by the leaf's internal ordering.
- **Deterministic output.** Same repo + same window + same flags ⇒
  bit-identical JSON (modulo `generated_at`).
- **Authored, not generated.** `suggested_actions` are templated — no
  LLM involved — so consumers can trust them to be consistent.
- **Exit code gates sit next to the data.** `thresholds` records what
  gates were configured; `baseline_delta` contains the evidence.

---

## 6. Signals and suggested actions

A hotspot is a candidate. The `signals` array explains *why* it's hot,
and drives the `suggested_actions`.

### Signals

| kind | When triggered | Severity |
|---|---|---|
| `high_churn` | ≥10 / ≥20 commits in window | medium / high |
| `large` | Package: ≥1500 / ≥3000 LOC. Function: ≥60 / ≥150 LOC. | medium / high |
| `knowledge_concentration` | One author wrote ≥60% / ≥80% of commits (min 5–10 commits) | medium / high |
| `bus_factor_one` | Exactly one author has touched this in the window (≥3 commits) | high |
| `bug_hotspot` | ≥30% of commit subjects match bug/fix keywords (and ≥3 absolute) | high |

Bug keywords (case-insensitive, word boundaries):
`fix | bug | issue | error | crash | panic | defect | regression | hotfix | patch | broken | fail | fault | fixup | revert`

### Suggested actions

Templated, deterministic, keyed off signal combinations:

| Signal mix | Suggested action (summarised) |
|---|---|
| `bug_hotspot` | Investigate the invariant the fixes keep patching; rewrite to enforce it instead of firefighting. |
| `knowledge_concentration` or `bus_factor_one` | Pair with the primary author before refactoring. |
| `large + high_churn` (package) | Split along axes that change together vs independently. |
| `large + high_churn` (function) | Extract sub-functions along the axes that vary together vs independently. |
| `high_churn` only (function) | Extract a strategy / helper; the function accumulates new cases over time. |
| `large` alone (no churn) | Large but stable — leave alone unless a feature requires touching it. |
| (no signals) | Candidate for review; read recent commits to confirm churn is domain-driven. |

Several actions can stack on one hotspot. They're sorted by usefulness.

---

### Prerequisites

| | |
|---|---|
| Python | 3.10 + |
| Required | `matplotlib` |
| Optional | `circlify` — replaces the inlined Wang/Bostock packer with the actively-maintained library. Auto-detected at runtime; no flag needed. |
| Optional | `plotly` — enables the `--html` interactive output. |
| Environment | The target must be inside a git checkout. |

Install everything at once:

```bash
pip install matplotlib circlify plotly
```

The tool is usable with just matplotlib; `circlify` and `plotly` are
soft dependencies.

---

## 7. CLI reference

```
python3 scripts/go_hotspots.py PATH [options]
```

`PATH` is a directory or a `.go` file. Must be inside a git checkout.

### Mode / scope

| Flag | Default | What it does |
|---|---|---|
| `--mode {auto,package,function}` | `auto` | Force the granularity. `auto` picks `function` for a `.go` file or a directory that contains `.go` files but no sub-packages; `package` otherwise. |
| `--include-tests` | off | Include `*_test.go` files in both LOC and commit aggregation (excluded by default). |

### Window

| Flag | Default | What it does |
|---|---|---|
| `--since EXPR` | (full history) | Lower bound on the commit window. Accepts ISO (`2024-01-01`, `2024-01-01T12:00:00`), relative (`3 weeks ago`, or just `3 weeks`), or keywords (`yesterday`, `last monday`, `last week`). Pre-validated before git runs; bad formats fail with a clear error. |
| `--until EXPR` | (present) | Upper bound. Same formats as `--since`. |

### Filtering

| Flag | Default | What it does |
|---|---|---|
| `--exclude REGEX` | (adds on top of defaults) | Regex tested with `re.search` against the repo-relative POSIX path. Repeatable. Anchor with `^` / `$` for tight matches. Invalid regex fails fast with a position. |
| `--no-default-excludes` | off | Don't auto-exclude common generated files. Defaults: `_templ\.go$`, `_mock\.go$`, `_mocks\.go$`, `\.pb\.go$`, `\.pb\.gw\.go$`, `_gen\.go$`, `(^\|/)zz_generated`, `bindata\.go$`. |
| `--keep-dead-files` | off | Keep commits whose only affected files no longer exist on disk (opt out of the accurate default). |
| `--ignore-dead-files` | *deprecated* | No-op, kept for backward compatibility. Dead files are now ignored by default. |

### Outputs

| Flag | Default | What it does |
|---|---|---|
| `--out PATH` | `go_hotspots.png` | PNG circle-pack output path. |
| `--no-image` | off | Skip the PNG entirely — for headless CI runs. |
| `--html PATH` | auto (alongside `--out` if plotly installed) | **Interactive HTML** report (Plotly). Hovering any circle shows signals, authors, recent commits, and suggested actions — the full §5 context without opening the JSON. Auto-emitted when plotly is installed; pass `--no-html` to skip. |
| `--no-html` | off | Suppress the auto-HTML even when plotly is installed. |
| `--csv PATH` | off | Per-leaf CSV (see §9). |
| `--json PATH` | off | Structured JSON report (see §5). |
| `--top-n N` | 20 | Hotspots included in JSON and CSV (`0` means all). |
| `--quiet` | off | Suppress the top-20 stdout table. |

### Gating (CI)

| Flag | Default | What it does |
|---|---|---|
| `--baseline PATH` | off | Compare against a prior `--json` report; adds `baseline_delta`. |
| `--fail-on-score N` | off | Exit 1 if any hotspot's score ≥ N. |
| `--fail-on-new-hotspot` | off | Exit 1 if a new hotspot appears in the top-N vs baseline. Requires `--baseline`. |
| `--fail-on-worsening-pct N` | off | Exit 1 if any shared hotspot's score grows by ≥N% vs baseline. Requires `--baseline`. |

---

## 8. Exit codes

A CI-friendly three-tier contract:

| Code | Meaning |
|---|---|
| `0` | Clean run, no thresholds exceeded. Pass the build. |
| `1` | Finding-level failure — thresholds exceeded (new hotspot, worsening, score over limit). Fail the build. |
| `2` | Script-level error — bad args, missing input, git failure. Mark the job errored, not flaky. |

Verified behaviour:

```
missing dir          → exit 2
bad --since format   → exit 2
missing --baseline   → exit 2
fail-on-score below  → exit 1
fail-on-score above  → exit 0
clean plain run      → exit 0
```

---

## 9. CSV output

**Package mode columns:**
`import_path, package, path, files, loc, revisions, score, signals, author_count, bug_fix_count`

**Function mode columns:**
`import_path, function, path, start_line, end_line, loc, revisions, score, signals, author_count, bug_fix_count`

`signals` is a semicolon-joined list of signal kinds, so you can filter
in a spreadsheet:

```
=IF(ISNUMBER(SEARCH("bug_hotspot", H2)), "YES", "")
```

---

## 10. Workflows and recipes

### A. Sprint planning (package → function drill)

```bash
# Top packages to look at
python3 scripts/go_hotspots.py . --since "since last sprint start" \
  --exclude '^frontend/' --exclude '_templ\.go$' \
  --json sprint-pkg.json --out sprint-pkg.png

# Pick the top hotspot (e.g. modules/evaluation) and zoom in
python3 scripts/go_hotspots.py modules/evaluation \
  --since "since last sprint start" \
  --json sprint-fn.json --out sprint-fn.png
```

Use the function-level report to file concrete refactoring cards:
"split `Service.createEvalRun` into `buildEvalSpec` + `executeEvalRun` +
`persistEvalResult`" — not "clean up `evaluation/`".

### B. PR review triage

Before reviewing, run the tool on the PR branch with `--since` set to
the base commit. Any file the PR touches that appears in the top-20
deserves extra scrutiny.

```bash
BASE=$(git merge-base HEAD main)
python3 scripts/go_hotspots.py . --since "$(git show -s --format=%ci "$BASE")" \
  --json pr-hotspots.json --no-image --quiet
```

### C. CI regression gate

```bash
# One-time: record the baseline after an agreed-good state
git checkout main
python3 scripts/go_hotspots.py . --since "3 months ago" \
  --exclude '^frontend/' \
  --json .ci/hotspots-baseline.json --no-image --quiet
git add .ci/hotspots-baseline.json
git commit -m "ci: record hotspot baseline"

# On every PR: diff against it
python3 scripts/go_hotspots.py . --since "3 months ago" \
  --exclude '^frontend/' \
  --json hotspots.json --baseline .ci/hotspots-baseline.json \
  --fail-on-worsening-pct 25 --fail-on-new-hotspot \
  --no-image --quiet
```

Refresh the baseline after a planned refactor cycle.

### D. AI refactoring agent

```bash
python3 scripts/go_hotspots.py . --since "1 month ago" \
  --exclude '^frontend/' \
  --json hotspots.json --top-n 5 --no-image --quiet
```

The agent reads `hotspots.json`, filters by `signals` (e.g. take entries
with `bug_hotspot` or `large + high_churn`), and uses `file_path` /
`start_line` / `end_line` to fetch code. `suggested_actions` scopes the
intervention. `recent_commits` (with `looks_like_fix`) lets the agent
distinguish feature churn from defect churn.

**Measure afterwards:** rerun with the same window and `--baseline`
pointing at the pre-refactor JSON. Confirm the hotspot cooled.

### E. "Did the refactor work?"

```bash
# Save before
python3 scripts/go_hotspots.py modules/evaluation --since "2 months ago" \
  --json before.json --out before.png

# ... refactor ships ...

# Compare after
python3 scripts/go_hotspots.py modules/evaluation --since "2 months ago" \
  --json after.json --baseline before.json --out after.png
# Check baseline_delta.cooler — the hotspot should be there.
```

### F. Onboarding

Day one: show the new hire the repo-wide PNG.
Day two: show function-mode output for their first area.

They learn what matters without reading the whole codebase.

---

## 11. CI integration examples

### GitHub Actions

```yaml
name: Hotspot gate
on: [pull_request]

jobs:
  hotspots:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0    # full history needed for git log

      - uses: actions/setup-python@v5
        with: {python-version: "3.11"}

      - run: pip install matplotlib

      - name: Hotspot regression check
        run: |
          python3 scripts/go_hotspots.py . \
            --since "3 months ago" \
            --exclude '^frontend/' \
            --json hotspots.json \
            --baseline .ci/hotspots-baseline.json \
            --fail-on-worsening-pct 25 --fail-on-new-hotspot \
            --no-image --quiet

      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: hotspots-report
          path: hotspots.json
```

### GitLab CI

```yaml
hotspot-gate:
  image: python:3.11
  script:
    - pip install matplotlib
    - python3 scripts/go_hotspots.py .
        --since "3 months ago"
        --exclude '^frontend/'
        --json hotspots.json
        --baseline .ci/hotspots-baseline.json
        --fail-on-worsening-pct 25
        --no-image --quiet
  artifacts:
    when: always
    paths: [hotspots.json]
```

### Pre-commit hook (local)

```bash
#!/usr/bin/env bash
# .git/hooks/pre-push
python3 scripts/go_hotspots.py . --since "1 month ago" \
  --exclude '^frontend/' \
  --json /tmp/hotspots.json \
  --baseline .ci/hotspots-baseline.json \
  --fail-on-worsening-pct 50 \
  --no-image --quiet || {
    echo "Your push would worsen a hotspot by >50%. Consider refactoring."
    echo "See /tmp/hotspots.json baseline_delta.hotter for details."
    exit 1
  }
```

---

## 12. How it works — internals

### Data pipeline

```
              ┌──────────────────────────────┐
              │  scope (dir / file / .)      │
              └──────────────┬───────────────┘
                             ▼
   ┌────────────────────────────────────────────────┐
   │  git rev-parse --show-toplevel → git_root      │
   │  walk up from scope → find go.mod → module_path│
   └──────────────────────────┬─────────────────────┘
                              ▼
                ┌──────────── mode ────────────┐
                │ is file / is single-pkg dir? │
                └───┬──────────────────────┬───┘
                    │ function             │ package
                    ▼                      ▼
    parse_go_functions (regex          git log --name-only
    + brace count on noise-            (one call, parsed into
    stripped source)                   commits_per_file + meta)
                    │                      │
          git log -L per function           │
                    │                      │
                    ▼                      ▼
                ┌──── leaves[] + commit_meta ────┐
                └──────────────┬─────────────────┘
                               ▼
        enrich_leaves (recent_commits, authors, bug_fix_pct,
                       signals, suggested_actions)
                               ▼
       ┌────────────┬──────────┬───────────┬──────────────┐
       ▼            ▼          ▼           ▼              ▼
    stdout        CSV        JSON        PNG         baseline diff
                                                  (compare_to_baseline)
                               │
                               ▼
                      apply_thresholds → exit code
```

### Key algorithms

- **Nested circle packing** uses the `circlify` library when installed
  (actively maintained, `pip install circlify`). Falls back to an
  inlined **Wang / Bostock front-chain** implementation when the
  library isn't available, so the tool stays usable with just
  matplotlib in the environment.
- **LOC counting** strips `/* */` and `//` comments, then counts
  non-blank lines. Language-neutral, robust, and (per Herraiz & Hassan)
  not worse than fancier complexity proxies.
- **Function parser** is intentionally *not* a full Go AST. Regex +
  brace counting on noise-stripped source gets the 99% case without
  depending on `go`/`gopls` being on PATH.
- **Bug keyword detection** is a fixed regex — simple, explainable, and
  trivially portable. Teams with different commit conventions may tune
  it, but the default captures common English bug-fix vocabulary.
- **Baseline compare** works on `path` as the stable key for leaves,
  which matches across runs even when ranks shuffle.

### Performance

- **Package mode**: single `git log` call, O(files × commits) scan. A
  repo with 1000 `.go` files and 5000 commits in window runs in a few
  seconds.
- **Function mode**: one `git log -L` per function. A file with 20
  functions takes ~20 git invocations, usually a few seconds total.
  Don't point function mode at a huge package without need.
- JSON + CSV serialisation is dominated by IO; negligible vs the log
  parsing.

---

## 13. Troubleshooting and FAQ

**"no commits touched any .go file" / the window is empty.**
The `--since` / `--until` window contains no commits, or all excluded
by `--exclude`, or all files are `_test.go` and you didn't pass
`--include-tests`. Widen the window or loosen filters.

**"not inside a git repository".**
The path you passed isn't in a git checkout. Run from the repo, or pass
a path inside it. This is a hard dependency — the tool has no history
to work with otherwise.

**The script thinks something should be function mode but I want
package mode** (or vice versa). Pass `--mode {package,function}`
explicitly.

**Counts don't match `git log --oneline -- <path>`.**
`git log -- modules/evaluation/` with trailing slash includes commits
touching *subpackage* files (e.g. `modules/evaluation/rubric/...`). The
tool attributes those to the subpackage, not to `modules/evaluation`.
This is Go-correct; Tornhill's package-level numbers agree.

**A new file shows up colder than expected.**
Fresh files have short commit histories by construction. Widen
`--since` if you want to include their pre-window history, or accept
that freshly-created code can't be a hotspot yet.

**A recently-renamed or moved package looks cold.**
Known limitation: git's rename tracking (`--follow`) only works per
file, so we can't apply it at package scale. Function mode handles
intra-file moves correctly via `git log -L`.

**Commit counts feel inflated.**
Check for mechanical sweeps (rename, reformat, license header) in
`recent_commits`. Use `--since` to exclude them, or tag a refactor
commit in a way that flags it if you do them often.

**CI is red but I think the change is fine.**
Review `baseline_delta.hotter` in the JSON. If the worsening is
intentional, either (a) refresh the baseline (`--json` with the new
score), or (b) raise `--fail-on-worsening-pct`. Baseline updates should
be a deliberate commit with a reviewable diff, not automatic.

**JSON schema version in the report.**
Always `"go-hotspots/1"`. If the tool's internal changes break the
schema, this bumps to `"go-hotspots/2"`. Consumers that see an unknown
version should hard-fail rather than guess.

**Tool is slow in function mode.**
One `git log -L` per function. For a file with 50 functions, that's
50 subprocesses — maybe 10–30 seconds. If you need many files, run
package mode first and only drill into the top hotspot.

**Generated files showing as hotspots.**
Always exclude them:
`--exclude '_templ\.go$' --exclude '\.pb\.go$' --exclude '_mock\.go$'`
plus whatever your generators use.

---

## 14. Limitations

1. **"Hot" is relative.** The tool ranks within the scope you gave. If
   the whole scope is quiet, the "hottest" thing might still be fine.
   Calibrate against the colorbar number.
2. **Renames across files aren't followed.** Freshly renamed/moved
   packages look colder than they are.
3. **Author intent is invisible.** Mechanical sweeps inflate counts.
4. **LOC is a weak complexity proxy.** 500 lines of CRUD is less risky
   than 300 lines of state-machine code. Still language-neutral.
5. **Generated code lies.** Always exclude.
6. **Tests are excluded by default.** Re-run with `--include-tests`
   when test churn is the signal you want.
7. **Merges are invisible** (`--name-only` doesn't emit files for
   ordinary merges). Counts reflect landed changes, not merge activity.
8. **Multi-module repos.** Reads the outermost `go.mod`; run once per
   module if you have nested ones.
9. **Function parser is not a full Go AST.** Regex + brace counting
   handles common cases.
10. **Function mode is slower.** Don't point at a huge package for
    no reason.
11. **`bug_fix` detection is keyword-based.** Different commit
    conventions → false positives/negatives. Signal, not proof.

---

## 15. Glossary

| Term | Meaning |
|---|---|
| **Hotspot** | A leaf (package or function) whose `score = revisions × LOC` is high. The top-12 get the thick border + numbered badge. |
| **Leaf** | An atomic unit of analysis — a Go package in package mode, a top-level func or method in function mode. |
| **Scope** | The `PATH` you pass on the command line; the root of the analysis. |
| **Window** | The `--since` / `--until` range over which commits are counted. |
| **Revisions** | Distinct commit hashes that touched the leaf in the window. Not a file-revision sum — a commit touching five files in one package counts once. |
| **LOC** | "Effective" lines of Go code: non-blank, non-comment. Comments and blank lines are stripped first. |
| **Score** | `revisions × LOC`. The primary ranking metric. |
| **Signal** | A heuristic flag describing *why* a hotspot is hot (`high_churn`, `large`, `knowledge_concentration`, `bus_factor_one`, `bug_hotspot`). |
| **Suggested action** | A deterministic, templated recommendation triggered by one or more signals. Authored, not LLM-generated. |
| **Baseline** | A prior `--json` report, used as the reference point for `baseline_delta` and the `--fail-on-*` gates. |
| **Container / leaf (tree)** | In the circle-pack diagram, containers are grouping directories (outlined); leaves are real packages or functions (filled). |
| **Import path** | The canonical Go identifier (`github.com/user/mod/chat/thread`), derived from `go.mod` + directory path. Shown in sidebar and JSON. |

---

## 16. Further reading

- Adam Tornhill, *Your Code as a Crime Scene* (2nd ed., Pragmatic
  Bookshelf) — chapters 2–4 are where the hotspot method comes from.
- Adam Tornhill, *Software Design X-Rays* — the follow-up with more
  patterns (change coupling, knowledge maps, complexity trends).
- Mike Bostock, [d3-hierarchy pack](https://github.com/d3/d3-hierarchy)
  — the circle-packing algorithm (Wang et al. front-chain) implemented
  in the script.
- Herraiz & Hassan's chapter in *Making Software* — why LOC isn't a
  worse complexity proxy than the fancier alternatives.
- `git log -L` docs — the function-history primitive the script uses
  in function mode.

---

*This tool is a guide, not an oracle. It narrows your search area — you
still do the thinking.*
