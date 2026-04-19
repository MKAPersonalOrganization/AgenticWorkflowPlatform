#!/usr/bin/env python3
"""
lcom4_trend.py — Chapter 6 of Your Code as a Crime Scene.
LCOM4 (Lack of Cohesion in Methods, v4) for Go files, over git history.

LCOM4 counts how many disjoint groups of methods a struct type has.
Two methods share a group if they reference the same receiver member
(field or another method). LCOM4 = 1 is perfectly cohesive; ≥ 2 means
the type is juggling multiple responsibilities and is a candidate for
Extract Class.

Edge rule (simplified): for each method M, collect the set of selectors
made on its receiver — e.g. `r.name` adds "name", `r.Save()` adds "Save".
Each method also implicitly references its own name. Two methods get an
edge iff their sets intersect. The self-reference trick turns "A calls B"
into a shared "B" token, so caller/callee connect without needing full
type resolution.

Requires:
    pip install tree-sitter tree-sitter-go matplotlib

Usage:
    lcom4_trend.py <path>...                       # trend over git history
    lcom4_trend.py --snapshot <path>...            # current state only
    lcom4_trend.py --discover                      # scan repo, rank worst
    lcom4_trend.py --discover --json out.json

Classifies evolution as:
    DETERIORATING — LCOM4 grew (type is fracturing)
    REFACTORED    — LCOM4 dropped from a peak (someone cleaned it up)
    GROWING       — methods added but LCOM4 held (healthy growth)
    STABLE        — little change
    INSUFFICIENT  — too few samples to judge
"""
from __future__ import annotations
import argparse
import json
import math
import subprocess
import sys
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path

# --- Tree-sitter setup ----------------------------------------------------
try:
    import tree_sitter_go
    from tree_sitter import Language, Parser
except ImportError:
    print("This script requires tree-sitter and tree-sitter-go.", file=sys.stderr)
    print("Install with: pip install tree-sitter tree-sitter-go", file=sys.stderr)
    sys.exit(1)

_GO_LANGUAGE = Language(tree_sitter_go.language())
_parser = Parser(_GO_LANGUAGE)

SCHEMA_VERSION = "lcom4-trend/2"

VERDICT_COLORS = {
    "DETERIORATING": "#d7263d",
    "GROWING":       "#e09f3e",
    "REFACTORED":    "#2a9d8f",
    "STABLE":        "#3a7ca5",
    "INSUFFICIENT":  "#808080",
    # Snapshot verdicts (when we don't have a trend):
    "SEVERE":        "#d7263d",
    "LOW_COHESION":  "#e09f3e",
    "COHESIVE":      "#2a9d8f",
}
VERDICT_BLURB = {
    "DETERIORATING": "LCOM4 growing, type fracturing",
    "GROWING":       "methods added, cohesion held",
    "REFACTORED":    "LCOM4 dropped from peak",
    "STABLE":        "little structural change",
    "INSUFFICIENT":  "not enough samples",
    "SEVERE":        "max LCOM4 ≥ 4",
    "LOW_COHESION":  "max LCOM4 in [2, 3]",
    "COHESIVE":      "max LCOM4 ≤ 1",
}

# --- AST analysis ---------------------------------------------------------

def _extract_type_name(type_node) -> str | None:
    """Extract the bare type name from a receiver type node.
    Handles: T, *T, T[X], *T[X,Y] — all normalize to "T"."""
    if type_node is None:
        return None
    t = type_node.type
    if t == "type_identifier":
        return type_node.text.decode()
    if t == "pointer_type":
        # pointer_type wraps the inner type; skip the leading '*' token
        for child in type_node.children:
            if child.type != "*":
                name = _extract_type_name(child)
                if name:
                    return name
    if t == "generic_type":
        # `T[X, Y]` — first child is the type_identifier
        for child in type_node.children:
            if child.type == "type_identifier":
                return child.text.decode()
    return None


def _collect_receiver_refs(body_node, recv_name: str) -> set[str]:
    """Walk body, collect every `recv_name.X` selector.
    Doesn't distinguish field access from method call — LCOM4 treats both
    as cohesion edges."""
    refs: set[str] = set()

    def visit(node):
        if node.type == "selector_expression":
            operand = node.child_by_field_name("operand")
            field = node.child_by_field_name("field")
            if (operand is not None and field is not None
                    and operand.type == "identifier"
                    and operand.text.decode() == recv_name):
                refs.add(field.text.decode())
        for child in node.children:
            visit(child)

    visit(body_node)
    return refs


# --- Trivial-method detection ---------------------------------------------
#
# Classic LCOM4 implementations over-flag Go code because every trivial
# accessor and every embedded-interface delegate looks like its own island
# of responsibility. Concretely:
#
#   func (a *dbAgent) ID()    agent.ID { return a.id }       // one field
#   func (a *dbAgent) Name()  string   { return a.displayName }   // another field
#   func (a *dbAgent) Icon()  string   { return a.icon }     // another field
#   func (a *dbAgent) Config() Config  { return a.Agent.(Configurable).Config() }  // delegate
#
# Each method touches exactly one field (or the embedded type) and nothing
# else, so the union-find sees 4 disjoint groups — LCOM4=4 — even though
# there's nothing to refactor. Every Go type that packs data + exposes it
# via getters would hit this.
#
# Treating these methods as cohesion-neutral (exempt from the graph) cleans
# out the noise. Trivial patterns we strip:
#
#   - empty body                               → pure marker / interface tag
#   - `return`                                 → no-op / defensive path
#   - `return r.field`                         → getter
#   - `return r.field.otherField`              → chained getter
#   - `return r.method(args...)`               → self-delegate
#   - `return r.embedded.method(args...)`      → embedded-interface delegate
#   - any of the above wrapped in a type assertion (e.g. `r.Agent.(X).F()`)
#
# Methods with > 1 statement, side-effecting expression statements, control
# flow, or literal/computed return values stay counted. Those are the ones
# that actually carry structural responsibility.

def _expr_grounds_in_receiver(node, recv_name: str) -> bool:
    """Walk a selector/call chain down to an identifier and check it's the
    receiver. Accepts r.X, r.X.Y, calls on any of those, and type
    assertions (r.(T) or r.Embedded.(T))."""
    if node is None:
        return False
    t = node.type
    if t == "identifier":
        return node.text.decode() == recv_name
    if t == "selector_expression":
        return _expr_grounds_in_receiver(
            node.child_by_field_name("operand"), recv_name)
    if t == "type_assertion_expression":
        # `r.Agent.(Configurable)` — the operand is the selector we care about.
        operand = node.child_by_field_name("operand")
        return _expr_grounds_in_receiver(operand, recv_name)
    if t == "parenthesized_expression":
        for child in node.children:
            if child.type not in ("(", ")"):
                return _expr_grounds_in_receiver(child, recv_name)
    return False


def _is_simple_accessor_expr(node, recv_name: str | None) -> bool:
    """True when `node` is a trivial getter/delegate expression anchored
    on the receiver: a selector chain, a call on such a chain, or a type
    assertion on one. Literals, operators, and non-receiver calls fail."""
    if recv_name is None or node is None:
        return False
    t = node.type
    if t == "selector_expression":
        return _expr_grounds_in_receiver(node, recv_name)
    if t == "call_expression":
        func = node.child_by_field_name("function")
        if func is None:
            return False
        # Only "r.X(...)" / "r.X.Y(...)" / "r.X.(T).Y(...)" count as delegates.
        if func.type in ("selector_expression", "type_assertion_expression"):
            return _expr_grounds_in_receiver(func, recv_name)
        return False
    if t == "type_assertion_expression":
        # `return r.field.(Concrete)` — still a receiver-grounded getter.
        return _expr_grounds_in_receiver(node, recv_name)
    if t == "parenthesized_expression":
        for child in node.children:
            if child.type not in ("(", ")"):
                return _is_simple_accessor_expr(child, recv_name)
    return False


def _method_statements(body) -> list:
    """Return the list of top-level statements inside a method body.

    tree-sitter-go wraps statements in a `statement_list` node (not
    directly in the `block`), so unwrap one level when present. Also
    filters out punctuation and comments so callers can reason about
    statement counts directly.
    """
    if body is None:
        return []
    # The block may be: { statement_list { stmt, stmt... } } OR (for empty
    # bodies) just { }. Unwrap the inner statement_list when present.
    inner = body
    for c in body.children:
        if c.type == "statement_list":
            inner = c
            break
    return [c for c in inner.children
            if c.type not in ("{", "}", ";", "comment", "statement_list")]


def _body_touches_receiver(body, recv_name: str | None) -> bool:
    """True iff the method body references `recv_name` anywhere.

    A method that never touches its receiver contributes nothing to
    cohesion — it can't share state with siblings because it doesn't
    touch any state. These methods are common as interface boilerplate
    (e.g. `Type() EventType { return EventTypeX }` returning a
    package-level constant) and should not count toward LCOM4.
    """
    if body is None or recv_name is None:
        return False

    def visit(node) -> bool:
        if node.type == "identifier" and node.text.decode() == recv_name:
            return True
        for child in node.children:
            if visit(child):
                return True
        return False

    return visit(body)


def _is_trivial_method(body, recv_name: str | None) -> bool:
    """A method is trivial if it's a simple accessor/delegate, an empty
    body, or interface boilerplate that doesn't reference the receiver.
    Trivial methods are exempt from LCOM4 — they don't represent
    structural responsibility, only data exposure or interface
    conformance.

    See the comment block above for the full list of patterns.
    """
    if body is None:
        # Declaration-only (rare in Go but possible via cgo). Treat as
        # trivial so it doesn't skew the graph.
        return True
    # Interface-boilerplate shortcut: a body that never mentions the
    # receiver has no cohesion-relevant content regardless of its
    # statement count. Catches `Type() X { return ConstantX }` and
    # similar stubs that otherwise look like "logic".
    if not _body_touches_receiver(body, recv_name):
        return True
    stmts = _method_statements(body)
    if len(stmts) == 0:
        # Empty `func (r *T) isFoo() {}` — classic marker method.
        return True
    if len(stmts) > 1:
        return False
    stmt = stmts[0]
    if stmt.type == "return_statement":
        # Strip the leading `return` keyword + any comments.
        values = [c for c in stmt.children
                  if c.type not in ("return", "comment")]
        if len(values) == 0:
            return True            # bare `return`
        # Go's `return a, b` parses as an expression_list with multiple
        # children; multiple tree-sitter values here means logic.
        if len(values) == 1 and values[0].type == "expression_list":
            exprs = [c for c in values[0].children if c.type != ","]
            if len(exprs) == 0:
                return True
            if len(exprs) > 1:
                return False
            return _is_simple_accessor_expr(exprs[0], recv_name)
        if len(values) > 1:
            return False
        return _is_simple_accessor_expr(values[0], recv_name)
    # Any other single statement (if/for/expression/assignment…) → not trivial.
    return False


def _process_method(node, by_type: dict[str, dict[str, dict]],
                    trivial_by_type: dict[str, set[str]] | None = None,
                    include_trivial: bool = False) -> None:
    """Pull receiver info + ref set out of a method_declaration node and
    record it under the appropriate type bucket.

    Trivial methods (simple getters/delegates/marker methods — see
    ``_is_trivial_method``) are skipped by default because they cause
    runaway LCOM4 inflation on data-carrier and embedded-interface
    types without indicating any real structural problem. Pass
    ``include_trivial=True`` to restore the classic behaviour (useful
    for sanity comparisons).

    ``trivial_by_type`` — when supplied — receives the names of every
    trivial method per type so the caller can surface the count
    alongside the LCOM4 finding ("type has 6 trivial accessors
    collapsed into the graph").
    """
    receiver = node.child_by_field_name("receiver")
    if receiver is None:
        return
    # receiver is a parameter_list containing one parameter_declaration
    param_decl = next((c for c in receiver.children
                       if c.type == "parameter_declaration"), None)
    if param_decl is None:
        return
    name_node = param_decl.child_by_field_name("name")
    type_node = param_decl.child_by_field_name("type")
    type_name = _extract_type_name(type_node)
    if type_name is None:
        return
    recv_name = name_node.text.decode() if name_node else None

    method_name_node = node.child_by_field_name("name")
    if method_name_node is None:
        return
    method_name = method_name_node.text.decode()

    body = node.child_by_field_name("body")

    if not include_trivial and _is_trivial_method(body, recv_name):
        if trivial_by_type is not None:
            trivial_by_type.setdefault(type_name, set()).add(method_name)
        return

    # Self-reference trick: including M's own name in its ref set turns
    # "A contains r.M()" into a shared "M" token between A and M.
    refs = {method_name}
    if body is not None and recv_name is not None:
        refs.update(_collect_receiver_refs(body, recv_name))

    by_type.setdefault(type_name, {})[method_name] = {
        "refs": refs,
        "line": node.start_point[0] + 1,
    }


def analyze_file(src: bytes, filename: str = "",
                 include_trivial: bool = False) -> list[dict]:
    """Return one result dict per struct type with ≥ 2 _non-trivial_
    methods defined in this file. Each entry includes LCOM4, method
    groups, and the count of trivial accessors that were filtered out
    (so the reader can see "this type has 8 trivial getters we ignored
    plus 3 real methods in 2 groups").

    Note: this is a FILE-level view. If methods for the same type live in
    multiple files of a package, each file's LCOM4 is computed independently.
    This matches the file-oriented style of complexity_trend.py and keeps
    the signal tied to the unit we're actually analyzing.
    """
    tree = _parser.parse(src)
    root = tree.root_node

    by_type: dict[str, dict[str, dict]] = {}
    type_lines: dict[str, int] = {}
    trivial_by_type: dict[str, set[str]] = {}

    # First pass: find struct type declarations so we can record first_line.
    def scan_types(node):
        if node.type == "type_spec":
            name_node = node.child_by_field_name("name")
            type_node = node.child_by_field_name("type")
            if (name_node is not None and type_node is not None
                    and type_node.type == "struct_type"):
                name = name_node.text.decode()
                by_type.setdefault(name, {})
                type_lines[name] = name_node.start_point[0] + 1
        for child in node.children:
            scan_types(child)
    scan_types(root)

    # Second pass: find method declarations and attach to their type.
    # Methods may appear for types not declared in this file (they're
    # declared elsewhere in the package); we still analyze what we see.
    def scan_methods(node):
        if node.type == "method_declaration":
            _process_method(node, by_type, trivial_by_type, include_trivial)
        for child in node.children:
            scan_methods(child)
    scan_methods(root)

    results = []
    # Surface types that have either real methods or a wall of trivial
    # accessors — every type mentioned anywhere gets considered.
    all_type_names = set(by_type) | set(trivial_by_type) | set(type_lines)
    for type_name in all_type_names:
        methods = by_type.get(type_name, {})
        trivial_count = len(trivial_by_type.get(type_name, ()))
        if len(methods) < 2:
            # Cohesion is undefined for 0- or 1-method types after the
            # trivial filter. Emit the record only if there's something
            # interesting to report (e.g. a lot of trivial getters) —
            # otherwise skip entirely so the JSON stays lean.
            continue
        lcom4, groups = compute_lcom4(methods)
        results.append({
            "type": type_name,
            "methods": len(methods),
            "trivial_methods": trivial_count,
            "lcom4": lcom4,
            "groups": groups,
            "first_line": type_lines.get(type_name, 0),
        })
    results.sort(key=lambda r: (-r["lcom4"], -r["methods"]))
    return results


# --- LCOM4 via union-find -------------------------------------------------

class UnionFind:
    def __init__(self, n: int):
        self.parent = list(range(n))
        self.rank = [0] * n

    def find(self, x: int) -> int:
        if self.parent[x] != x:
            self.parent[x] = self.find(self.parent[x])
        return self.parent[x]

    def union(self, x: int, y: int) -> None:
        rx, ry = self.find(x), self.find(y)
        if rx == ry:
            return
        if self.rank[rx] < self.rank[ry]:
            rx, ry = ry, rx
        self.parent[ry] = rx
        if self.rank[rx] == self.rank[ry]:
            self.rank[rx] += 1


def compute_lcom4(methods: dict[str, dict]) -> tuple[int, list[list[str]]]:
    """Return (group_count, groups). Groups are lists of method names,
    sorted by size (largest first) for stable output."""
    names = sorted(methods)  # deterministic iteration order
    uf = UnionFind(len(names))
    for i in range(len(names)):
        refs_i = methods[names[i]]["refs"]
        for j in range(i + 1, len(names)):
            if refs_i & methods[names[j]]["refs"]:
                uf.union(i, j)
    groups_by_root: dict[int, list[str]] = {}
    for i, name in enumerate(names):
        groups_by_root.setdefault(uf.find(i), []).append(name)
    groups = sorted(groups_by_root.values(), key=len, reverse=True)
    return len(groups), groups


# --- Git operations -------------------------------------------------------

def revisions_for(path: str, since: str) -> list[tuple[str, int]]:
    """(hash, unix_ts) pairs that touched `path` in period, oldest first.
    Uses git's own `--since` (approxidate) — the same "6 months ago" /
    "2 weeks ago" strings work on Linux, macOS, and Windows."""
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
    """Resolve current `path` to its name at `rev`. Returns None when we
    can't locate the file (caller skips the revision rather than reading
    an unrelated blob)."""
    # Fast path: file exists under the same name.
    probe = subprocess.run(
        ["git", "cat-file", "-e", f"{rev}:{path}"],
        capture_output=True,
    )
    if probe.returncode == 0:
        return path
    # Renamed. Walk follow-history for rename events (R), unwinding
    # new→old until we find a name that exists at `rev`.
    out = subprocess.run(
        ["git", "log", "--follow", "--name-status", "--diff-filter=R",
         "--pretty=format:", "--", path],
        capture_output=True, text=True,
    )
    current = path
    for line in out.stdout.splitlines():
        parts = line.split("\t")
        if len(parts) >= 3 and parts[0].startswith("R") and parts[2] == current:
            current = parts[1]
            probe = subprocess.run(
                ["git", "cat-file", "-e", f"{rev}:{current}"],
                capture_output=True,
            )
            if probe.returncode == 0:
                return current
    return None


def blob_at(path: str, rev: str) -> bytes | None:
    """Return the file's bytes at `rev`, or None if git can't find it.
    Returns bytes (not str) because tree-sitter parses bytes directly."""
    out = subprocess.run(
        ["git", "show", f"{rev}:{path}"],
        capture_output=True,
    )
    if out.returncode != 0:
        return None
    return out.stdout


# --- Sampling & classification --------------------------------------------

@dataclass
class Sample:
    rev: str
    ts: int  # unix timestamp
    max_lcom4: int
    total_methods: int
    types: list[dict] = field(default_factory=list)


def sample_history(path: str, revs: list[tuple[str, int]],
                   max_samples: int, include_trivial: bool = False) -> list[Sample]:
    """Sample up to max_samples evenly-spaced revisions, always including
    the most recent. Skip revisions we can't locate or parse."""
    if len(revs) > max_samples:
        step = len(revs) / max_samples
        idxs = sorted({int(i * step) for i in range(max_samples)}
                      | {len(revs) - 1})
        revs = [revs[i] for i in idxs if i < len(revs)]
    samples: list[Sample] = []
    for rev, ts in revs:
        rev_path = path_at_revision(path, rev)
        if rev_path is None:
            continue
        blob = blob_at(rev_path, rev)
        if blob is None:
            continue
        try:
            types = analyze_file(blob, rev_path, include_trivial=include_trivial)
        except Exception:
            # tree-sitter is forgiving, but a corrupt/empty blob might
            # still break something downstream. Skip the revision.
            continue
        if not types:
            # No struct types with 2+ methods; signal is undefined here.
            continue
        samples.append(Sample(
            rev=rev[:9], ts=ts,
            max_lcom4=max(t["lcom4"] for t in types),
            total_methods=sum(t["methods"] for t in types),
            types=types,
        ))
    return samples


def classify(samples: list[Sample]) -> tuple[str, dict]:
    if len(samples) < 2:
        return "INSUFFICIENT", {"samples": len(samples)}
    first, last = samples[0], samples[-1]
    peak = max(s.max_lcom4 for s in samples)
    methods_delta_pct = 0.0
    if first.total_methods:
        methods_delta_pct = (last.total_methods - first.total_methods) / first.total_methods * 100
    metrics = {
        "first_max_lcom4":  first.max_lcom4,
        "last_max_lcom4":   last.max_lcom4,
        "peak_max_lcom4":   peak,
        "lcom4_delta":      last.max_lcom4 - first.max_lcom4,
        "lcom4_peak_dip":   peak - last.max_lcom4,
        "first_methods":    first.total_methods,
        "last_methods":     last.total_methods,
        "methods_delta":    last.total_methods - first.total_methods,
        "methods_delta_pct": round(methods_delta_pct, 1),
        "samples":          len(samples),
    }
    # REFACTORED takes priority — a clear dip from peak is the strongest
    # signal, even if net delta happens to be 0 or slightly positive.
    if metrics["lcom4_peak_dip"] >= 2:
        return "REFACTORED", metrics
    if metrics["lcom4_delta"] >= 2:
        return "DETERIORATING", metrics
    if metrics["lcom4_delta"] == 1 and methods_delta_pct < 20:
        # Gained a group without commensurate method growth → active decay.
        return "DETERIORATING", metrics
    if metrics["lcom4_delta"] == 0 and methods_delta_pct > 20:
        return "GROWING", metrics
    return "STABLE", metrics


def deterioration_score(metrics: dict) -> float:
    """Rank files by how concerning their cohesion trend is.
    Weights growth in LCOM4 heaviest, tempered by absolute current level."""
    if not metrics or "last_max_lcom4" not in metrics:
        return 0.0
    score = 0.0
    if metrics.get("lcom4_delta", 0) > 0:
        score += 20.0 * metrics["lcom4_delta"]
    score += 8.0 * metrics["last_max_lcom4"]  # already-bad state matters
    if metrics.get("methods_delta_pct", 0) > 0:
        score += 0.2 * metrics["methods_delta_pct"]
    return score


def snapshot_verdict(max_lcom4: int) -> str:
    if max_lcom4 >= 4:
        return "SEVERE"
    if max_lcom4 >= 2:
        return "LOW_COHESION"
    return "COHESIVE"


# --- Discovery ------------------------------------------------------------

GENERATED_PATTERNS = (
    "_templ.go", ".pb.go", "_gen.go", "_mock.go", "_mocks.go",
    "zz_generated", "bindata.go", "/vendor/", "/testdata/",
)

def is_generated(path: str) -> bool:
    return any(pat in path for pat in GENERATED_PATTERNS)

def is_test_file(path: str) -> bool:
    return path.endswith("_test.go")


def discover_candidates(since: str, min_commits: int, min_loc: int,
                        max_candidates: int) -> list[tuple[str, int, int]]:
    """(path, commits, loc) for Go files with enough churn and size to
    be worth screening. Sorted by commit count, capped at max_candidates."""
    out = subprocess.run(
        ["git", "log", f"--since={since}", "--name-only", "--pretty=format:"],
        check=True, capture_output=True, text=True,
    )
    counts: dict[str, int] = {}
    for line in out.stdout.splitlines():
        p = line.strip()
        if not p or not p.endswith(".go") or is_generated(p):
            continue
        counts[p] = counts.get(p, 0) + 1
    candidates: list[tuple[str, int, int]] = []
    for path, n_commits in counts.items():
        if n_commits < min_commits:
            continue
        fp = Path(path)
        if not fp.is_file():
            continue
        try:
            loc = sum(1 for ln in fp.read_text(errors="ignore").splitlines()
                      if ln.strip())
        except OSError:
            continue
        if loc < min_loc:
            continue
        candidates.append((path, n_commits, loc))
    candidates.sort(key=lambda r: -r[1])
    return candidates[:max_candidates]


# --- Report building ------------------------------------------------------

def _build_entry(path: str, samples: list[Sample], verdict: str, metrics: dict,
                 commits: int | None = None, loc: int | None = None) -> dict:
    """Assemble a JSON-serializable entry describing one file's trend.
    `score` is computed here rather than mutated later so every entry has
    the same schema regardless of whether it makes the ranked set."""
    score = deterioration_score(metrics)
    entry: dict = {
        "path": path,
        "is_test": is_test_file(path),
        "verdict": verdict,
        "score": round(score, 1),
        "samples": len(samples),
        "metrics": metrics,
    }
    if commits is not None:
        entry["commits"] = commits
    if loc is not None:
        entry["loc"] = loc
    if samples:
        entry["first"] = _sample_summary(samples[0])
        entry["last"] = _sample_summary(samples[-1])
    return entry


def _sample_summary(s: Sample) -> dict:
    return {
        "rev": s.rev, "ts": s.ts,
        "max_lcom4": s.max_lcom4,
        "total_methods": s.total_methods,
        "types": s.types,
    }


def _build_snapshot_entry(path: str, types: list[dict]) -> dict:
    max_l = max((t["lcom4"] for t in types), default=0)
    return {
        "path": path,
        "is_test": is_test_file(path),
        "verdict": snapshot_verdict(max_l),
        "score": float(max_l * 10),
        "snapshot": {"max_lcom4": max_l, "types": types},
    }


# --- Runners --------------------------------------------------------------

def run_snapshot(path: str, include_trivial: bool = False) -> dict | None:
    try:
        src = Path(path).read_bytes()
    except OSError as e:
        print(f"{path}: cannot read: {e}", file=sys.stderr)
        return None
    try:
        types = analyze_file(src, path, include_trivial=include_trivial)
    except Exception as e:
        print(f"{path}: parse failed: {e}", file=sys.stderr)
        return None
    return _build_snapshot_entry(path, types)


def run_trend(path: str, since: str, max_samples: int,
              quiet: bool = False, include_trivial: bool = False
              ) -> tuple[list[Sample], str, dict] | None:
    revs = revisions_for(path, since)
    if not revs:
        if not quiet:
            print(f"{path}: no commits since {since}", file=sys.stderr)
        return None
    samples = sample_history(path, revs, max_samples,
                             include_trivial=include_trivial)
    if not samples:
        if not quiet:
            print(f"{path}: no parseable samples (file may lack struct types "
                  f"with ≥2 methods)", file=sys.stderr)
        return None
    verdict, metrics = classify(samples)
    if not quiet:
        _print_trend(path, samples, verdict, metrics)
    return samples, verdict, metrics


def _print_trend(path: str, samples: list[Sample],
                 verdict: str, metrics: dict) -> None:
    print(f"\n=== {path} ===")
    print(f"  revisions sampled: {len(samples)}")
    print(f"  FIRST ({samples[0].rev})  max LCOM4={samples[0].max_lcom4}  "
          f"methods={samples[0].total_methods}")
    print(f"  LAST  ({samples[-1].rev})  max LCOM4={samples[-1].max_lcom4}  "
          f"methods={samples[-1].total_methods}")
    if metrics:
        print(f"  ΔLCOM4: {metrics['lcom4_delta']:+d}   "
              f"Δmethods: {metrics['methods_delta']:+d} "
              f"({metrics['methods_delta_pct']:+.0f}%)   "
              f"peak-dip: {metrics['lcom4_peak_dip']}")
    print(f"  VERDICT: {verdict}")
    # Show the worst current offender(s) so the user has somewhere to look.
    # `trivial_methods` is the count of accessors/delegates we deliberately
    # left out of the graph; displaying it signals to the reader that a
    # type's LCOM4 is computed over its *non-trivial* surface.
    for t in samples[-1].types:
        if t["lcom4"] < 2:
            continue
        group_sizes = [len(g) for g in t["groups"]]
        trivia = t.get("trivial_methods", 0)
        trivia_tag = f"  (+{trivia} trivial)" if trivia else ""
        print(f"    {t['type']}  methods={t['methods']}  LCOM4={t['lcom4']}  "
              f"groups={group_sizes}{trivia_tag}")


def _print_snapshot(entry: dict) -> None:
    snap = entry["snapshot"]
    print(f"\n=== {entry['path']} ===")
    print(f"  verdict: {entry['verdict']}   max LCOM4: {snap['max_lcom4']}")
    for t in snap["types"]:
        if t["lcom4"] < 2:
            continue
        group_sizes = [len(g) for g in t["groups"]]
        trivia = t.get("trivial_methods", 0)
        trivia_tag = f"  (+{trivia} trivial)" if trivia else ""
        print(f"    {t['type']}  methods={t['methods']}  LCOM4={t['lcom4']}  "
              f"groups={group_sizes}{trivia_tag}")


# --- Plotting -------------------------------------------------------------

def _try_import_mpl():
    try:
        import matplotlib
        matplotlib.use("Agg")
        import matplotlib.pyplot as plt
        import matplotlib.dates as mdates
        from matplotlib.patches import Patch
        return plt, mdates, Patch
    except ImportError:
        return None, None, None


def _sanitize(path: str) -> str:
    import os
    return os.path.normpath(path).replace("/", "__").replace("\\", "__")


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
        "xtick.labelsize": 8,
        "ytick.labelsize": 8,
        "figure.facecolor": "white",
        "axes.facecolor": "#fafbfc",
        "legend.fontsize": 8,
    })


def plot_file(path: str, samples: list[Sample], verdict: str,
              metrics: dict, out_dir: Path) -> Path | None:
    plt, mdates, Patch = _try_import_mpl()
    if plt is None or len(samples) < 2:
        return None
    _apply_style(plt)

    dates = [datetime.fromtimestamp(s.ts, tz=timezone.utc) for s in samples]
    lcom4s = [s.max_lcom4 for s in samples]
    methods = [s.total_methods for s in samples]
    color = VERDICT_COLORS.get(verdict, "#444")

    fig, ax = plt.subplots(figsize=(11, 5.5))
    fig.subplots_adjust(top=0.85, bottom=0.13, left=0.07, right=0.93)

    # LCOM4 as a step plot — it's integer-valued, so steps are honest.
    ax.step(dates, lcom4s, color=color, linewidth=2.4,
            where="post", label="max LCOM4 (per file)")
    ax.scatter(dates, lcom4s, color=color, s=30, zorder=4)
    ax.set_ylabel("max LCOM4", color=color)
    ax.tick_params(axis="y", labelcolor=color)
    # Integer ticks — fractional LCOM4 is meaningless.
    import matplotlib.ticker as mtick
    ax.yaxis.set_major_locator(mtick.MaxNLocator(integer=True))
    ax.set_ylim(bottom=0)
    ax.axhline(1, color="#bbb", linewidth=0.7, linestyle=":", zorder=0)

    # Methods count on secondary axis.
    ax2 = ax.twinx()
    ax2.plot(dates, methods, color="#404040", linewidth=1.5,
             linestyle="--", marker="s", markersize=3, label="methods")
    ax2.set_ylabel("total methods", color="#404040")
    ax2.tick_params(axis="y", labelcolor="#404040")
    ax2.grid(False)

    # Merged legend.
    h1, l1 = ax.get_legend_handles_labels()
    h2, l2 = ax2.get_legend_handles_labels()
    ax.legend(h1 + h2, l1 + l2, loc="upper left")

    # Verdict pill.
    blurb = VERDICT_BLURB.get(verdict, "")
    label = f"{verdict} — {blurb}" if blurb else verdict
    ax.text(0.005, 1.04, label, transform=ax.transAxes,
            ha="left", va="bottom",
            fontsize=9, fontweight="bold", color="white",
            bbox=dict(boxstyle="round,pad=0.35",
                      facecolor=color, edgecolor="none"))
    if metrics:
        delta_txt = (f"ΔLCOM4 {metrics['lcom4_delta']:+d}   "
                     f"Δmethods {metrics['methods_delta']:+d} "
                     f"({metrics['methods_delta_pct']:+.0f}%)   "
                     f"peak-dip {metrics['lcom4_peak_dip']}")
        ax.text(0.995, 1.04, delta_txt, transform=ax.transAxes,
                ha="right", va="bottom", fontsize=8.5, color="#404040")

    locator = mdates.AutoDateLocator(minticks=4, maxticks=7)
    ax.xaxis.set_major_locator(locator)
    ax.xaxis.set_major_formatter(
        mdates.ConciseDateFormatter(locator, show_offset=False))

    fig.suptitle(path, fontsize=12, fontweight="bold", y=0.97, x=0.07, ha="left")
    footer = (f"{len(samples)} samples · "
              f"{dates[0].strftime('%b %d, %Y')} → "
              f"{dates[-1].strftime('%b %d, %Y')}")
    fig.text(0.07, 0.015, footer, fontsize=7.5, color="#707070", ha="left")

    out_dir.mkdir(parents=True, exist_ok=True)
    out_file = out_dir / f"{_sanitize(path)}.png"
    fig.savefig(out_file, dpi=150)
    plt.close(fig)
    return out_file


# --- Discover mode --------------------------------------------------------

def discover_mode(args) -> int:
    print(f"Discovering candidates touched since {args.since!r} "
          f"(≥{args.min_commits} commits, ≥{args.min_loc} LOC)...")
    candidates = discover_candidates(args.since, args.min_commits,
                                     args.min_loc, args.max_candidates)
    if not args.include_tests:
        before = len(candidates)
        candidates = [c for c in candidates if not is_test_file(c[0])]
        print(f"  → {len(candidates)} candidates to screen "
              f"({before - len(candidates)} test files excluded; "
              f"use --include-tests to keep).")
    else:
        print(f"  → {len(candidates)} candidates to screen.")
    if not candidates:
        print("  no candidates found.", file=sys.stderr)
        return 1

    print(f"Screening with {args.screen_samples} samples per file...")
    all_entries: list[dict] = []
    concerning: list[dict] = []
    final_samples: dict[str, tuple[list[Sample], str, dict]] = {}

    for i, (path, n_commits, loc) in enumerate(candidates, 1):
        try:
            r = run_trend(path, args.since, args.screen_samples, quiet=True,
                          include_trivial=args.include_trivial)
        except subprocess.CalledProcessError:
            continue
        if r is None:
            continue
        samples, verdict, metrics = r
        entry = _build_entry(path, samples, verdict, metrics, n_commits, loc)
        all_entries.append(entry)

        # A file is worth surfacing if it's currently low-cohesion
        # OR the trend says it's deteriorating. Either is actionable.
        current_bad = samples[-1].max_lcom4 >= 2
        if not (current_bad or verdict == "DETERIORATING"):
            continue
        concerning.append(entry)
        final_samples[path] = (samples, verdict, metrics)
        print(f"  [{i:3d}/{len(candidates)}] {path}  "
              f"verdict={verdict:<13} score={entry['score']:5.1f}  "
              f"LCOM4={metrics['last_max_lcom4']} "
              f"(Δ{metrics['lcom4_delta']:+d})  "
              f"methods={samples[-1].total_methods}")

    concerning.sort(key=lambda e: -e["score"])
    top = concerning[:args.top]

    if not top:
        print("\nNo cohesion problems found. 🎉")
    else:
        print(f"\nTop {len(top)} files with cohesion problems:")
        for rank, e in enumerate(top, 1):
            m = e["metrics"]
            print(f"  {rank:2d}. {e['path']:<60}  "
                  f"score={e['score']:6.1f}  "
                  f"LCOM4={m['last_max_lcom4']} "
                  f"(Δ{m['lcom4_delta']:+d})  "
                  f"methods={m['last_methods']}")

    if args.json:
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "lcom4_trend",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "since": args.since,
                "min_commits": args.min_commits,
                "min_loc": args.min_loc,
                "max_candidates": args.max_candidates,
                "screen_samples": args.screen_samples,
                "include_tests": args.include_tests,
                "include_trivial": args.include_trivial,
            },
            "stats": {
                "candidates_screened": len(all_entries),
                "concerning_found": len(concerning),
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
    for e in top:
        # Re-sample at the full sample count for nicer plots.
        r = run_trend(e["path"], args.since, args.samples, quiet=True,
                      include_trivial=args.include_trivial)
        if r is None:
            continue
        samples, verdict, metrics = r
        img = plot_file(e["path"], samples, verdict, metrics, out_dir)
        if img:
            print(f"  wrote {img}")
    return 0


# --- CLI ------------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(
        description="LCOM4 cohesion analysis for Go files, with git-history trends.",
    )
    ap.add_argument("paths", nargs="*",
                    help="Go files to analyze (omit when using --discover)")
    ap.add_argument("--since", default="6 months ago")
    ap.add_argument("--samples", type=int, default=20,
                    help="samples per file for trend plots (default 20)")
    ap.add_argument("--out", default="scripts/out/lcom4_trend",
                    help="output directory for PNGs")
    ap.add_argument("--no-plot", action="store_true",
                    help="skip PNG generation")
    ap.add_argument("--snapshot", action="store_true",
                    help="current state only (no git history)")
    # Discover mode:
    ap.add_argument("--discover", action="store_true",
                    help="auto-find and rank low-cohesion Go files")
    ap.add_argument("--top", type=int, default=10,
                    help="how many files to plot (default 10)")
    ap.add_argument("--min-commits", type=int, default=8)
    ap.add_argument("--min-loc", type=int, default=150)
    ap.add_argument("--max-candidates", type=int, default=120)
    ap.add_argument("--screen-samples", type=int, default=6,
                    help="samples per file during cheap screening")
    ap.add_argument("--include-tests", action="store_true",
                    help="keep _test.go files in --discover results")
    ap.add_argument("--include-trivial", action="store_true",
                    help="count trivial accessors (simple getters, "
                         "embedded-interface delegates, marker methods) "
                         "toward LCOM4. Off by default — these methods "
                         "otherwise dominate the graph on data-carrier "
                         "types and produce misleading LCOM4 scores.")
    ap.add_argument("--json", type=str,
                    help="write structured report to this path")
    args = ap.parse_args()

    if args.discover:
        return discover_mode(args)

    if not args.paths:
        ap.error("provide file paths or use --discover")

    out_dir = Path(args.out)
    entries: list[dict] = []
    rc = 0
    for p in args.paths:
        if args.snapshot:
            entry = run_snapshot(p, include_trivial=args.include_trivial)
            if entry is None:
                rc = 1
                continue
            _print_snapshot(entry)
            entries.append(entry)
            continue

        try:
            r = run_trend(p, args.since, args.samples,
                          include_trivial=args.include_trivial)
        except subprocess.CalledProcessError as e:
            print(f"{p}: git error: {e}", file=sys.stderr)
            rc = 1
            continue
        if r is None:
            rc = 1
            continue
        samples, verdict, metrics = r
        entries.append(_build_entry(p, samples, verdict, metrics))
        if not args.no_plot:
            img = plot_file(p, samples, verdict, metrics, out_dir)
            if img:
                print(f"  wrote {img}")

    if args.json:
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "lcom4_trend",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "mode": "snapshot" if args.snapshot else "trend",
            "all": entries,
        }
        Path(args.json).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.json}")
    return rc


if __name__ == "__main__":
    sys.exit(main())