#!/usr/bin/env python3
"""
go_hotspots.py — Code-as-a-Crime-Scene hotspot analysis for Go.

Ranks the parts of a Go codebase that are both most-changed (git commits) and
most-complex (effective lines of Go code). Hotspots are where refactoring pays
off most — Tornhill, *Your Code as a Crime Scene*, ch. 3.

Output surfaces:
  PNG       — a circle-pack enclosure diagram (for humans)
  HTML      — an interactive Plotly report with hover tooltips (for humans)
  JSON      — a versioned, structured report (for CI gates and AI consumers)
  CSV       — per-leaf table (for spreadsheets / ad-hoc scripts)
  stdout    — ranked top-20 table (suppress with --quiet)
  exit code — 0 clean, 1 threshold exceeded, 2 script error

Software is fractal, so the tool is recursive. Point it at:
  - a directory tree           → leaves are Go packages
  - a single-package directory → leaves are top-level funcs/methods
  - a single .go file          → leaves are top-level funcs/methods

JSON report schema: go-hotspots/1.

This is a production-grade rewrite: custom Go parser, custom circle-pack
algorithm and custom date parser have been replaced with battle-tested
open-source libraries:
  - tree-sitter + tree-sitter-go for Go AST parsing
  - circlify for nested circle packing (d3-hierarchy port)
  - dateparser for flexible --since/--until input
  - GitPython for git history access
  - matplotlib for PNG, plotly for interactive HTML
"""
from __future__ import annotations

import argparse
import csv
import json
import logging
import math
import os
import re
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Iterable

log = logging.getLogger("go_hotspots")


# --------------------------------------------------------------------------
# Dependency imports — fail with actionable messages if anything is missing.
# The tool has five hard dependencies; they are all pip-installable and
# pinned in scripts/architecture/requirements.txt.
# --------------------------------------------------------------------------

def _missing_dep(name: str, pip_name: str) -> "None":
    sys.stderr.write(
        f"error: go_hotspots requires the '{name}' package.\n"
        f"  install with:  pip install {pip_name}\n"
        f"  or install all deps: pip install -r scripts/architecture/requirements.txt\n"
    )
    sys.exit(2)


try:
    import numpy as np  # type: ignore
    import matplotlib.pyplot as plt  # type: ignore
    from matplotlib.patches import Rectangle  # type: ignore
    from matplotlib.colors import LinearSegmentedColormap  # type: ignore
    from matplotlib.lines import Line2D  # type: ignore
except ImportError:
    _missing_dep("matplotlib", "matplotlib")

try:
    import circlify  # type: ignore
except ImportError:
    _missing_dep("circlify", "circlify")

try:
    import plotly.graph_objects as pgo  # type: ignore
except ImportError:
    _missing_dep("plotly", "plotly")

try:
    import dateparser  # type: ignore
    from dateparser.search import search_dates as _dateparser_search  # type: ignore
except ImportError:
    _missing_dep("dateparser", "dateparser")

try:
    from git import Repo, InvalidGitRepositoryError, GitCommandError  # type: ignore
except ImportError:
    _missing_dep("GitPython", "GitPython")

try:
    import tree_sitter_go  # type: ignore
    from tree_sitter import Language, Parser  # type: ignore
except ImportError:
    _missing_dep("tree-sitter + tree-sitter-go", "tree_sitter tree_sitter_go")


# --------------------------------------------------------------------------
# Constants
# --------------------------------------------------------------------------

SCHEMA_VERSION = "go-hotspots/1"
TOOL_VERSION = "2.0"

EXIT_OK = 0
EXIT_THRESHOLD = 1
EXIT_ERROR = 2

SKIP_DIRS = frozenset({"vendor", "testdata", ".git", "node_modules"})

DEFAULT_EXCLUDES: tuple[str, ...] = (
    r"_templ\.go$",
    r"_mock\.go$",
    r"_mocks\.go$",
    r"\.pb\.go$",
    r"\.pb\.gw\.go$",
    r"_gen\.go$",
    r"(^|/)zz_generated",
    r"bindata\.go$",
)

BUG_KEYWORDS = re.compile(
    r"\b(fix|bug|issue|error|crash|panic|defect|regression|hotfix|patch|"
    r"broken|fail|fault|fixup|revert)\b",
    re.IGNORECASE,
)


class HotspotError(Exception):
    """Raised for any recoverable tool error; main() converts to exit 2."""


# --------------------------------------------------------------------------
# Date parsing — delegated to dateparser so any natural-language expression
# that works for humans also works here.
# --------------------------------------------------------------------------

_DATEPARSER_SETTINGS = {
    "RETURN_AS_TIMEZONE_AWARE": False,
    "PREFER_DATES_FROM": "past",
}


def normalize_date(flag: str, value: str) -> str:
    """Validate a --since/--until expression and return a form git accepts.

    dateparser handles both ISO dates and natural-language ("3 weeks ago",
    "last monday", "yesterday"). We convert to ISO date string so git's
    interpretation is unambiguous — git silently ignores unparseable input.
    """
    s = (value or "").strip()
    if not s:
        raise HotspotError(f"{flag} cannot be empty")
    parsed = dateparser.parse(s, settings=_DATEPARSER_SETTINGS)
    if parsed is None:
        # `last monday`, `last tuesday` etc. need dateparser.search
        try:
            found = _dateparser_search(s, languages=["en"])
        except Exception:
            found = None
        if found:
            parsed = found[0][1]
    if parsed is None:
        raise HotspotError(
            f"{flag} value {value!r} is not a recognised date.\n"
            "  Try an ISO date (2024-01-01), a relative phrase ('3 weeks ago'),\n"
            "  or a named day ('yesterday', 'last monday')."
        )
    return parsed.strftime("%Y-%m-%d %H:%M:%S")


def compile_excludes(patterns: list[str] | None) -> re.Pattern | None:
    if not patterns:
        return None
    combined = "|".join(f"(?:{p})" for p in patterns)
    try:
        return re.compile(combined)
    except re.error as e:
        raise HotspotError(f"invalid --exclude pattern: {e}") from e


# --------------------------------------------------------------------------
# LOC counter — language-neutral "effective lines" (non-blank, non-comment).
# Small and correct; not worth a library dependency.
# --------------------------------------------------------------------------

_BLOCK_COMMENT = re.compile(r"/\*.*?\*/", re.DOTALL)


def count_effective_loc(path: Path) -> int:
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError as e:
        log.warning("unreadable file %s: %s", path, e)
        return 0
    return _count_loc_text(text)


def _count_loc_text(text: str) -> int:
    text = _BLOCK_COMMENT.sub("", text)
    return sum(
        1
        for line in text.splitlines()
        if (s := line.strip()) and not s.startswith("//")
    )


# --------------------------------------------------------------------------
# Go parser — tree-sitter + tree-sitter-go.
# Replaces the previous regex+brace-counting implementation. Handles
# generics (including generic receivers), multi-line signatures, nested
# closures (ignored) and invalid code (tree-sitter ERROR nodes are skipped).
# --------------------------------------------------------------------------

_GO_LANGUAGE = Language(tree_sitter_go.language())
_GO_PARSER = Parser(_GO_LANGUAGE)


def _receiver_type_name(recv_node) -> str | None:
    """Return the receiver type name from a method_declaration's receiver.

    Walks the subtree looking for the first type_identifier — robust
    against pointer receivers, generic receivers, and generic pointer
    receivers (`*Foo[T]`, `Bar[K, V]`).
    """
    if recv_node is None:
        return None
    stack = [recv_node]
    while stack:
        node = stack.pop()
        if node.type == "type_identifier":
            return node.text.decode("utf-8", errors="replace")
        stack.extend(reversed(node.children))
    return None


def parse_go_functions(path: Path) -> list[dict]:
    """Return all top-level funcs/methods in *path* with precise line ranges."""
    try:
        source = path.read_bytes()
    except OSError as e:
        log.warning("unreadable file %s: %s", path, e)
        return []
    tree = _GO_PARSER.parse(source)
    funcs: list[dict] = []
    for node in tree.root_node.children:
        if node.type not in ("function_declaration", "method_declaration"):
            continue
        name_node = node.child_by_field_name("name")
        if name_node is None:
            continue
        name = name_node.text.decode("utf-8", errors="replace")
        if node.type == "method_declaration":
            recv = _receiver_type_name(node.child_by_field_name("receiver"))
            display = f"{recv}.{name}" if recv else name
        else:
            display = name
        start_line = node.start_point[0] + 1
        end_line = node.end_point[0] + 1
        body = source[node.start_byte:node.end_byte].decode(
            "utf-8", errors="replace")
        funcs.append({
            "name": display,
            "start_line": start_line,
            "end_line": end_line,
            "loc": _count_loc_text(body),
        })
    return funcs


# --------------------------------------------------------------------------
# Filesystem helpers
# --------------------------------------------------------------------------

def collect_go_files(root: Path) -> Iterable[Path]:
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames
                       if d not in SKIP_DIRS and not d.startswith(".")]
        for name in filenames:
            if name.endswith(".go"):
                yield Path(dirpath) / name


def _dir_has_go(d: Path) -> bool:
    try:
        for _dirpath, dirnames, filenames in os.walk(d):
            dirnames[:] = [x for x in dirnames
                           if x not in SKIP_DIRS and not x.startswith(".")]
            if any(f.endswith(".go") for f in filenames):
                return True
    except OSError:
        pass
    return False


def find_git_root(path: Path) -> Path:
    """Locate the enclosing git working tree via GitPython."""
    try:
        repo = Repo(str(path), search_parent_directories=True)
    except InvalidGitRepositoryError as e:
        raise HotspotError(f"{path} is not inside a git repository") from e
    wt = repo.working_tree_dir
    if not wt:
        raise HotspotError(f"{path} is inside a bare git repository")
    return Path(wt)


def find_go_module(scope: Path) -> tuple[Path, str] | None:
    """Walk up from *scope* looking for go.mod; return (dir, module_path)."""
    cur = scope.resolve()
    while True:
        gomod = cur / "go.mod"
        if gomod.is_file():
            try:
                for raw in gomod.read_text(errors="replace").splitlines():
                    s = raw.strip()
                    if s.startswith("module "):
                        val = s[len("module "):].strip()
                        if "//" in val:
                            val = val[:val.index("//")].strip()
                        if val.startswith('"') and val.endswith('"'):
                            val = val[1:-1]
                        if val:
                            return cur, val
            except OSError as e:
                log.debug("couldn't read %s: %s", gomod, e)
        if cur.parent == cur:
            return None
        cur = cur.parent


# --------------------------------------------------------------------------
# Git history — GitPython wraps subprocess cleanly.
# Package mode: single `git log --name-only` call, batch-parsed.
# Function mode: one `git log -L` per function.
# --------------------------------------------------------------------------

_LOG_PRETTY = "__C__%H%x09%an%x09%ad%x09%s"


def _parse_commit_line(prefixed: str) -> tuple[str, dict] | None:
    if not prefixed.startswith("__C__"):
        return None
    parts = prefixed[5:].split("\t", 3)
    if len(parts) < 4:
        parts += [""] * (4 - len(parts))
    h, author, date, subject = parts
    return h, {"author": author, "date": date, "subject": subject}


def _run_git_log(repo: Repo, *args: str, **kwargs: str) -> str:
    try:
        return repo.git.log(*args, **kwargs)
    except GitCommandError as e:
        stderr = (e.stderr or "").strip() or str(e)
        raise HotspotError(f"git log failed — {stderr}") from e


def git_commit_history(
    git_root: Path, scope: Path,
    since: str | None, until: str | None,
    exclude_regex: re.Pattern | None = None,
    live_files: set[str] | None = None,
) -> tuple[dict[str, set[str]], dict[str, dict]]:
    """Distinct-commit history for every .go path under *scope*.

    Returns (commits_per_file, commit_meta) from a single `git log` call.
    """
    repo = Repo(str(git_root))
    rel = scope.resolve().relative_to(git_root)
    path_arg = "." if str(rel) == "." else str(rel)

    kwargs: dict[str, str] = {
        "pretty": f"format:{_LOG_PRETTY}",
        "date": "short",
        "name_only": True,
    }
    if since:
        kwargs["since"] = since
    if until:
        kwargs["until"] = until

    log.debug("git log for %s (since=%s until=%s)", path_arg, since, until)
    out = _run_git_log(repo, "--", path_arg, **kwargs)

    commits_per_file: dict[str, set[str]] = defaultdict(set)
    commit_meta: dict[str, dict] = {}
    current: str | None = None
    for line in out.splitlines():
        parsed = _parse_commit_line(line)
        if parsed is not None:
            h, meta = parsed
            current = h
            commit_meta[h] = meta
            continue
        if not current or not line.endswith(".go"):
            continue
        path = line.strip()
        if live_files is not None and path not in live_files:
            continue
        if exclude_regex is not None and exclude_regex.search(path):
            continue
        commits_per_file[path].add(current)
    log.info("package-mode git log: %d commits, %d files touched",
             len(commit_meta), len(commits_per_file))
    return commits_per_file, commit_meta


def git_function_history(
    git_root: Path, rel_path: str, start: int, end: int,
    since: str | None, until: str | None,
) -> tuple[set[str], dict[str, dict]]:
    """Per-function history via `git log -L <start>,<end>:<file>`."""
    repo = Repo(str(git_root))
    kwargs: dict[str, str] = {
        "pretty": f"format:{_LOG_PRETTY}",
        "date": "short",
    }
    if since:
        kwargs["since"] = since
    if until:
        kwargs["until"] = until
    try:
        out = repo.git.log(f"-L{start},{end}:{rel_path}", "-s", **kwargs)
    except GitCommandError as e:
        log.debug("git log -L failed for %s:%d-%d: %s",
                  rel_path, start, end, (e.stderr or "").strip())
        return set(), {}
    hashes: set[str] = set()
    meta: dict[str, dict] = {}
    for line in out.splitlines():
        parsed = _parse_commit_line(line)
        if parsed is None:
            continue
        h, m = parsed
        if h:
            hashes.add(h)
            meta[h] = m
    return hashes, meta


# --------------------------------------------------------------------------
# Package aggregation
# --------------------------------------------------------------------------

def collect_leaf_packages(
    scope: Path,
    git_root: Path,
    commits_per_file: dict[str, set[str]],
    include_tests: bool,
    module_info: tuple[Path, str] | None,
    exclude_regex: re.Pattern | None = None,
) -> list[dict]:
    buckets: dict[Path, dict] = {}
    for go_file in collect_go_files(scope):
        if not include_tests and go_file.name.endswith("_test.go"):
            continue
        rel = str(go_file.resolve().relative_to(git_root))
        if exclude_regex is not None and exclude_regex.search(rel):
            continue
        loc = count_effective_loc(go_file)
        if loc == 0:
            continue
        dir_path = go_file.parent.resolve()
        entry = buckets.setdefault(dir_path, {
            "loc": 0, "files": 0, "commits": set(), "file_paths": [],
        })
        entry["loc"] += loc
        entry["files"] += 1
        entry["commits"] |= commits_per_file.get(rel, set())
        entry["file_paths"].append(rel)

    leaves: list[dict] = []
    scope_resolved = scope.resolve()
    mod_dir, mod_path = (module_info if module_info else (None, None))
    for dir_path, d in buckets.items():
        rel_to_scope = dir_path.relative_to(scope_resolved)
        parts = [p for p in rel_to_scope.parts if p not in ("", ".")]
        name = parts[-1] if parts else scope.name
        rel_str = "/".join(parts) if parts else "."
        if mod_dir is not None:
            try:
                mod_rel = dir_path.relative_to(mod_dir)
                mod_parts = [p for p in mod_rel.parts if p and p != "."]
                import_path = (
                    mod_path + "/" + "/".join(mod_parts) if mod_parts
                    else mod_path
                )
            except ValueError:
                import_path = rel_str
        else:
            import_path = rel_str
        revisions = len(d["commits"])
        leaves.append({
            "kind": "package",
            "path": str(dir_path.relative_to(git_root)),
            "rel_to_scope": rel_str,
            "import_path": import_path,
            "segments": parts,
            "name": name,
            "loc": d["loc"],
            "files": d["files"],
            "revisions": revisions,
            "score": revisions * d["loc"],
            "commit_hashes": d["commits"],
            "file_paths": sorted(d["file_paths"]),
        })
    return leaves


# --------------------------------------------------------------------------
# Package hierarchy (tree built from directory segments)
# --------------------------------------------------------------------------

class Node:
    __slots__ = ("name", "children", "pkg", "x", "y", "r")

    def __init__(self, name: str):
        self.name = name
        self.children: list[Node] = []
        self.pkg: dict | None = None
        self.x = 0.0
        self.y = 0.0
        self.r = 0.0

    def is_leaf(self) -> bool:
        return self.pkg is not None and not self.children


def build_tree(leaves: list[dict], root_name: str) -> Node:
    root = Node(root_name)
    for pkg in leaves:
        if not pkg["segments"]:
            root.pkg = pkg
            continue
        node = root
        for seg in pkg["segments"]:
            child = next((c for c in node.children if c.name == seg), None)
            if child is None:
                child = Node(seg)
                node.children.append(child)
            node = child
        node.pkg = pkg

    def normalize(n: Node) -> None:
        for c in n.children:
            normalize(c)
        if n.pkg is not None and n.children:
            synth = Node(n.name)
            synth.pkg = n.pkg
            n.pkg = None
            n.children.insert(0, synth)

    def compress(n: Node, is_root: bool) -> None:
        for c in n.children:
            compress(c, is_root=False)
        if is_root or n.pkg is not None:
            return
        if len(n.children) == 1 and n.children[0].pkg is None:
            child = n.children[0]
            n.name = f"{n.name}/{child.name}" if n.name else child.name
            n.children = child.children

    normalize(root)
    compress(root, is_root=True)
    return root


# --------------------------------------------------------------------------
# Function-level collection
# --------------------------------------------------------------------------

def _leaf_dict_for_function(fn: dict, rel_path: str, display_root: str,
                            hashes: set[str]) -> dict:
    path_id = f"{rel_path}:{fn['start_line']}-{fn['end_line']}"
    qualified = f"{display_root}::{fn['name']}"
    return {
        "kind": "function",
        "name": fn["name"],
        "path": path_id,
        "file_path": rel_path,
        "rel_to_scope": fn["name"],
        "import_path": qualified,
        "segments": [fn["name"]],
        "loc": fn["loc"],
        "files": 1,
        "revisions": len(hashes),
        "score": len(hashes) * fn["loc"],
        "start_line": fn["start_line"],
        "end_line": fn["end_line"],
        "commit_hashes": hashes,
    }


def collect_functions_for_file(
    file_path: Path, git_root: Path,
    since: str | None, until: str | None,
    display_root: str,
    meta_sink: dict[str, dict],
) -> list[dict]:
    rel = str(file_path.resolve().relative_to(git_root))
    fns = parse_go_functions(file_path)
    out = []
    for fn in fns:
        if fn["loc"] == 0:
            continue
        hashes, meta = git_function_history(
            git_root, rel, fn["start_line"], fn["end_line"], since, until)
        meta_sink.update(meta)
        out.append(_leaf_dict_for_function(fn, rel, display_root, hashes))
    return out


def build_function_tree(
    scope: Path, git_root: Path, include_tests: bool,
    since: str | None, until: str | None,
    exclude_regex: re.Pattern | None,
    root_display: str,
) -> tuple[Node, list[dict], dict[str, dict]]:
    leaves: list[dict] = []
    commit_meta: dict[str, dict] = {}

    if scope.is_file():
        if scope.suffix != ".go":
            raise HotspotError(f"{scope} is not a .go file")
        root = Node(root_display)
        funcs = collect_functions_for_file(
            scope, git_root, since, until, root_display, commit_meta)
        for f in funcs:
            leaf = Node(f["name"])
            leaf.pkg = f
            root.children.append(leaf)
        leaves.extend(funcs)
        return root, leaves, commit_meta

    root = Node(root_display)
    go_files = sorted(
        f for f in scope.iterdir()
        if f.is_file() and f.suffix == ".go"
    )
    if not go_files:
        raise HotspotError(
            f"{scope} has no .go files directly (function mode analyses one "
            "package at a time; point me at the single package dir, or pass "
            "--mode package for the nested view)"
        )
    for gf in go_files:
        if not include_tests and gf.name.endswith("_test.go"):
            continue
        rel = str(gf.resolve().relative_to(git_root))
        if exclude_regex is not None and exclude_regex.search(rel):
            continue
        file_display = f"{root_display}/{gf.name}"
        fns = collect_functions_for_file(
            gf, git_root, since, until, file_display, commit_meta)
        if not fns:
            continue
        file_container = Node(gf.name)
        for f in fns:
            leaf = Node(f["name"])
            leaf.pkg = f
            file_container.children.append(leaf)
        root.children.append(file_container)
        leaves.extend(fns)
    return root, leaves, commit_meta


def detect_mode(scope: Path, explicit: str) -> str:
    if explicit != "auto":
        return explicit
    if scope.is_file() and scope.suffix == ".go":
        return "function"
    if scope.is_dir():
        has_own = False
        has_sub_go = False
        for entry in scope.iterdir():
            if entry.is_file() and entry.suffix == ".go":
                has_own = True
            elif entry.is_dir() and entry.name not in SKIP_DIRS and \
                    not entry.name.startswith("."):
                if _dir_has_go(entry):
                    has_sub_go = True
                    break
        if has_own and not has_sub_go:
            return "function"
    return "package"


# --------------------------------------------------------------------------
# Enrichment — signals, recent commits, author distribution, suggestions.
# Pure domain logic; unchanged from the original.
# --------------------------------------------------------------------------

def enrich_leaves(leaves: list[dict], commit_meta: dict[str, dict]) -> None:
    for leaf in leaves:
        _enrich_leaf(leaf, commit_meta)


def _enrich_leaf(leaf: dict, commit_meta: dict[str, dict]) -> None:
    hashes = leaf.get("commit_hashes", set())
    known = [(h, commit_meta[h]) for h in hashes if h in commit_meta]
    known.sort(key=lambda hm: hm[1].get("date", ""), reverse=True)

    recent = [
        {
            "hash": h[:12],
            "date": m.get("date", ""),
            "author": m.get("author", ""),
            "subject": m.get("subject", ""),
            "looks_like_fix": bool(BUG_KEYWORDS.search(m.get("subject", ""))),
        }
        for h, m in known[:5]
    ]

    authors = Counter(m.get("author", "") for _, m in known)
    total = sum(authors.values())
    author_rows = [
        {
            "name": name or "(unknown)",
            "commits": n,
            "pct": round(100 * n / total, 1) if total else 0.0,
        }
        for name, n in authors.most_common(5)
    ]

    bug_fix_count = sum(
        1 for _, m in known if BUG_KEYWORDS.search(m.get("subject", "")))
    bug_fix_pct = round(100 * bug_fix_count / total, 1) if total else 0.0

    leaf["recent_commits"] = recent
    leaf["authors"] = author_rows
    leaf["author_count"] = len(authors)
    leaf["bug_fix_count"] = bug_fix_count
    leaf["bug_fix_pct"] = bug_fix_pct
    leaf["signals"] = _compute_signals(leaf, authors, bug_fix_count, total)
    leaf["suggested_actions"] = _compute_actions(leaf)


def _compute_signals(
    leaf: dict, authors: Counter, bug_fix_count: int, total: int,
) -> list[dict]:
    signals = []
    revs = leaf["revisions"]
    loc = leaf["loc"]

    if revs >= 20:
        signals.append({"kind": "high_churn", "severity": "high",
                        "detail": f"{revs} commits in window"})
    elif revs >= 10:
        signals.append({"kind": "high_churn", "severity": "medium",
                        "detail": f"{revs} commits in window"})

    if leaf.get("kind") == "package":
        if loc >= 3000:
            signals.append({"kind": "large", "severity": "high",
                            "detail": f"{loc} LOC"})
        elif loc >= 1500:
            signals.append({"kind": "large", "severity": "medium",
                            "detail": f"{loc} LOC"})
    else:
        if loc >= 150:
            signals.append({"kind": "large", "severity": "high",
                            "detail": f"{loc} LOC"})
        elif loc >= 60:
            signals.append({"kind": "large", "severity": "medium",
                            "detail": f"{loc} LOC"})

    if authors:
        top_name, top_n = authors.most_common(1)[0]
        if total >= 5:
            pct = 100 * top_n / total
            if pct >= 80:
                signals.append({
                    "kind": "knowledge_concentration", "severity": "high",
                    "detail": f"{top_name} wrote {pct:.0f}% of commits "
                              f"({top_n}/{total})",
                })
            elif pct >= 60 and total >= 10:
                signals.append({
                    "kind": "knowledge_concentration", "severity": "medium",
                    "detail": f"{top_name} wrote {pct:.0f}% of commits "
                              f"({top_n}/{total})",
                })

    if len(authors) == 1 and total >= 3:
        signals.append({
            "kind": "bus_factor_one", "severity": "high",
            "detail": "only one author has touched this in the window",
        })

    if bug_fix_count >= 3 and total > 0 and bug_fix_count / total >= 0.3:
        signals.append({
            "kind": "bug_hotspot", "severity": "high",
            "detail": (f"{bug_fix_count}/{total} commits look like bug fixes "
                       f"— repeated fault site"),
        })

    return signals


def _compute_actions(leaf: dict) -> list[str]:
    kinds = {s["kind"] for s in leaf.get("signals", [])}
    is_function = leaf.get("kind") == "function"
    actions: list[str] = []

    if "bug_hotspot" in kinds:
        actions.append(
            "Repeated bug-fix churn — investigate the invariant that the "
            "fixes keep patching and rewrite to enforce it, rather than "
            "continuing to firefight."
        )
    if "knowledge_concentration" in kinds or "bus_factor_one" in kinds:
        actions.append(
            "Knowledge silo — pair with the primary author before "
            "refactoring; undocumented invariants likely live only in "
            "their head."
        )
    if "large" in kinds and "high_churn" in kinds:
        if is_function:
            actions.append(
                "Large and hot function — extract sub-functions along the "
                "axes that vary together vs independently. Hot functions "
                "that keep growing usually glue 2–3 orthogonal concerns."
            )
        else:
            actions.append(
                "Large and hot package — split along the axes that change "
                "together vs independently. A hotspot package that is also "
                "large usually hides 2–3 glued concerns."
            )
    elif "high_churn" in kinds and is_function:
        actions.append(
            "Hot function — extract a strategy / helper; high per-commit "
            "surface suggests it accumulates new cases over time."
        )
    if "large" in kinds and "high_churn" not in kinds:
        actions.append(
            "Large but stable — leave alone unless a feature requires "
            "touching it; big-and-quiet is working code."
        )
    if not actions:
        actions.append(
            "Candidate for review. Read recent commits to confirm the "
            "churn is domain-driven rather than accidental."
        )
    return actions


# --------------------------------------------------------------------------
# Circle packing — delegated to circlify (d3-hierarchy port).
# --------------------------------------------------------------------------

def pack_tree(node: Node, scale: float = 100.0) -> None:
    """Lay out *node* and its children as a nested circle-pack.

    circlify returns absolute coordinates inside a unit enclosing circle;
    our renderer wants parent-relative coordinates, so we walk the result
    once and convert to offsets from each parent's centre.
    """
    node.x = 0.0
    node.y = 0.0

    if node.pkg is not None and not node.children:
        loc = max(1.0, float(node.pkg["loc"]))
        node.r = math.sqrt(loc / math.pi) * 2.0
        return
    if not node.children:
        node.r = 1.0
        return

    id_by_node: dict[int, str] = {}
    node_by_id: dict[str, Node] = {}
    counter = [0]

    def _entry(n: Node) -> dict:
        counter[0] += 1
        nid = f"n{counter[0]}"
        id_by_node[id(n)] = nid
        node_by_id[nid] = n
        if n.pkg is not None and not n.children:
            return {"id": nid, "datum": max(1.0, float(n.pkg["loc"]))}
        children_entries = [_entry(c) for c in n.children]
        total = sum(e.get("datum", 0) for e in children_entries) or 1.0
        return {"id": nid, "datum": total, "children": children_entries}

    top = [_entry(c) for c in node.children]
    circles = circlify.circlify(top, show_enclosure=False)

    abs_pos: dict[str, tuple[float, float]] = {}
    max_extent = 0.0
    for c in circles:
        nid = c.ex["id"] if isinstance(c.ex, dict) else None
        if nid is None or nid not in node_by_id:
            continue
        n = node_by_id[nid]
        ax_, ay = c.x * scale, c.y * scale
        abs_pos[nid] = (ax_, ay)
        n.r = c.r * scale
        extent = math.hypot(ax_, ay) + n.r
        if extent > max_extent:
            max_extent = extent

    node.r = max_extent or scale

    def _set_relative(n: Node, pax: float, pay: float) -> None:
        nid = id_by_node.get(id(n))
        if n is node or nid is None:
            ax_, ay = 0.0, 0.0
            if n is node:
                n.x = 0.0
                n.y = 0.0
        else:
            ax_, ay = abs_pos.get(nid, (pax, pay))
            n.x = ax_ - pax
            n.y = ay - pay
        for c in n.children:
            _set_relative(c, ax_, ay)

    _set_relative(node, 0.0, 0.0)


# --------------------------------------------------------------------------
# PNG rendering (matplotlib)
# --------------------------------------------------------------------------

LEAF_EDGE = "#1a1a1a"
LEAF_EDGE_HOT = "#000000"
LEAF_LW = 0.7
LEAF_LW_HOT = 2.6
GROUP_EDGE = "#a6a6a6"
GROUP_FILL = "#fafafa"
ROOT_EDGE = "#b8b8b8"


def _hotspot_colormap() -> LinearSegmentedColormap:
    """Truncated YlOrRd — starts at a visible light-yellow, not near-white.

    Plain YlOrRd at low heat blends into the background, which hides
    low-churn-but-existing packages. We skip the first ~18% of the ramp so
    every leaf circle has a perceptible fill even at the cold end.
    """
    base = plt.get_cmap("YlOrRd")
    colors = base(np.linspace(0.18, 1.0, 256))
    return LinearSegmentedColormap.from_list("hotspots_shifted", colors)


def _display_date(s: str | None) -> str | None:
    """Strip a trailing HH:MM:SS from a normalized date for display.

    dateparser yields 'YYYY-MM-DD HH:MM:SS'. Day precision is plenty for
    the title; the full timestamp just adds noise.
    """
    if not s:
        return s
    return s.split(" ", 1)[0]


def _render_leaf_circle(ax, node: Node, cx: float, cy: float,
                        cmap, norm, hot_map: dict[str, int],
                        depth: int) -> None:
    heat = norm(node.pkg["revisions"])
    color = cmap(heat)
    is_hot = node.pkg["path"] in hot_map
    ax.add_patch(plt.Circle(
        (cx, cy), node.r, facecolor=color,
        edgecolor=LEAF_EDGE_HOT if is_hot else LEAF_EDGE,
        linewidth=LEAF_LW_HOT if is_hot else LEAF_LW,
        zorder=10 + depth,
    ))
    text_color = "white" if heat > 0.55 else "#111"
    r = node.r
    if r > 3.0:
        fs = min(11.5, max(6.5, r * 0.42))
        label = node.name
        if fs * 0.58 * len(label) < 2 * r * 0.92:
            ax.text(cx, cy, label, ha="center", va="center",
                    fontsize=fs, fontweight="bold",
                    color=text_color, family="monospace",
                    zorder=12 + depth)
    if is_hot:
        num = str(hot_map[node.pkg["path"]])
        br = max(0.55, min(r * 0.22, 1.8))
        angle = math.radians(55)
        bx = cx + r * math.cos(angle)
        by = cy + r * math.sin(angle)
        ax.add_patch(plt.Circle(
            (bx, by), br, facecolor="#111", edgecolor="white",
            linewidth=0.9, zorder=14 + depth))
        ax.text(bx, by, num, ha="center", va="center",
                fontsize=min(11, br * 1.1), fontweight="bold",
                color="white", zorder=15 + depth)


def _render_container_circle(ax, node: Node, cx: float, cy: float,
                             depth: int) -> None:
    if depth == 0:
        ax.add_patch(plt.Circle(
            (cx, cy), node.r, facecolor="white",
            edgecolor=ROOT_EDGE, linewidth=1.3, zorder=1))
    else:
        ax.add_patch(plt.Circle(
            (cx, cy), node.r, facecolor=GROUP_FILL,
            edgecolor=GROUP_EDGE,
            linewidth=max(0.6, 1.2 - 0.18 * depth),
            alpha=0.92, zorder=1 + depth))
    if node.r > 3.2 and node.name:
        fs = min(10.5, max(7.0, node.r * 0.30))
        ax.text(cx, cy + node.r * 0.86, node.name,
                ha="center", va="center",
                fontsize=fs, fontweight="bold",
                color="#222", family="monospace",
                bbox=dict(boxstyle="round,pad=0.2",
                          fc="white", ec="#aaa", lw=0.5, alpha=0.95),
                zorder=13 + depth)


def _render_circle(ax, node: Node, cx: float, cy: float,
                   cmap, norm, hot_map: dict[str, int], depth: int) -> None:
    if node.pkg is not None and not node.children:
        _render_leaf_circle(ax, node, cx, cy, cmap, norm, hot_map, depth)
        return
    _render_container_circle(ax, node, cx, cy, depth)
    for child in node.children:
        _render_circle(ax, child, cx + child.x, cy + child.y,
                       cmap, norm, hot_map, depth + 1)


def plot_hotspots(
    leaves: list[dict], tree: Node,
    out_path: Path, title: str, subtitle: str,
) -> None:
    if not leaves:
        return
    TOP_N = 12
    max_revs = max((p["revisions"] for p in leaves), default=1) or 1
    cmap = _hotspot_colormap()
    norm = plt.Normalize(vmin=0, vmax=max_revs)
    top_hot = sorted(leaves, key=lambda p: p["score"], reverse=True)[:TOP_N]
    hot_map = {p["path"]: i + 1 for i, p in enumerate(top_hot)}
    pack_tree(tree)

    fig = plt.figure(figsize=(20, 13), facecolor="white")
    gs = fig.add_gridspec(
        3, 2, height_ratios=[0.14, 1.0, 0.035], width_ratios=[4, 1],
        left=0.03, right=0.98, top=0.97, bottom=0.04,
        hspace=0.08, wspace=0.04)
    ax_header = fig.add_subplot(gs[0, :])
    ax = fig.add_subplot(gs[1, 0])
    ax_side = fig.add_subplot(gs[1, 1])
    ax_cbar = fig.add_subplot(gs[2, 0])
    for a in (ax_header, ax, ax_side):
        a.axis("off")

    ax_header.text(0, 0.92, title, fontsize=19, fontweight="bold",
                   va="top", ha="left", family="sans-serif", color="#111")
    if subtitle:
        ax_header.text(0, 0.52, subtitle, fontsize=11,
                       va="top", ha="left", color="#333",
                       linespacing=1.45)
    is_fn = bool(leaves) and leaves[0].get("kind") == "function"
    if is_fn:
        caption = (
            "Enclosure diagram — function-level zoom. Filled circle = one "
            "top-level Go func/method; outlined circle = the file housing "
            "it. Radius ∝ √LOC · fill = commits touching the function's "
            "line range (git log -L) · numbered badge = top hotspot rank.")
        cbar_label = ("Change frequency — distinct commits touching the "
                      "function — cold → hot")
        sidebar_heading = "TOP HOTSPOT FUNCTIONS"
    else:
        caption = (
            "Enclosure diagram — filled circle = one Go package; outlined "
            "circle = grouping directory. Radius ∝ √(Σ LOC) · fill = "
            "commits touching the package · numbered badge = top hotspot "
            "rank. Big red circles = refactor first.")
        cbar_label = ("Change frequency — distinct commits touching the "
                      "package — cold → hot")
        sidebar_heading = "TOP HOTSPOT PACKAGES"
    ax_header.text(0, 0.05, caption, fontsize=9.5, color="#555",
                   va="top", ha="left")

    _render_circle(ax, tree, 0.0, 0.0, cmap, norm, hot_map, depth=0)
    margin = tree.r * 0.04
    ax.set_xlim(-tree.r - margin, tree.r + margin)
    ax.set_ylim(-tree.r - margin, tree.r + margin)
    ax.set_aspect("equal")

    ax_side.text(0.0, 1.0, sidebar_heading, fontsize=12.5, fontweight="bold",
                 color="#111", va="top", ha="left",
                 transform=ax_side.transAxes)
    ax_side.text(0.0, 0.973, "review these first — high churn × high size",
                 fontsize=8.5, color="#666", style="italic",
                 va="top", ha="left", transform=ax_side.transAxes)
    ax_side.add_line(Line2D(
        [0.0, 1.0], [0.945, 0.945], color="#ddd", linewidth=0.6,
        transform=ax_side.transAxes, clip_on=False))

    line_h = 0.935 / TOP_N  # vertical span / number of rows
    for i, p in enumerate(top_hot, 1):
        y_top = 0.935 - (i - 1) * line_h
        y_mid = y_top - line_h / 2
        heat = norm(p["revisions"])
        color = cmap(heat)
        chip_h = line_h * 0.62
        ax_side.add_patch(Rectangle(
            (0.0, y_mid - chip_h / 2), 0.09, chip_h,
            transform=ax_side.transAxes, facecolor=color,
            edgecolor="#111", linewidth=0.8, clip_on=False))
        ax_side.text(0.045, y_mid, str(i), ha="center", va="center",
                     fontsize=11.5, fontweight="bold",
                     color="white" if heat > 0.55 else "#111",
                     transform=ax_side.transAxes)
        ax_side.text(0.115, y_mid + chip_h * 0.18,
                     p.get("import_path") or p["rel_to_scope"],
                     fontsize=9.5, va="center", family="monospace",
                     color="#111", fontweight="bold",
                     transform=ax_side.transAxes)
        bits = f'{p["revisions"]} revs · {p["loc"]} LOC · score {p["score"]:,}'
        if p.get("files") and p.get("kind") == "package":
            bits = (f'{p["revisions"]} revs · {p["loc"]} LOC · '
                    f'{p["files"]} files · score {p["score"]:,}')
        ax_side.text(0.115, y_mid - chip_h * 0.45, bits,
                     fontsize=8.5, va="center", color="#555",
                     transform=ax_side.transAxes)
        if i < TOP_N:  # divider between rows
            ax_side.add_line(Line2D(
                [0.0, 1.0], [y_top - line_h, y_top - line_h],
                color="#eee", linewidth=0.5,
                transform=ax_side.transAxes, clip_on=False))

    sm = plt.cm.ScalarMappable(cmap=cmap, norm=norm)
    sm.set_array([])
    cbar = fig.colorbar(sm, cax=ax_cbar, orientation="horizontal")
    cbar.set_label(cbar_label, fontsize=9.5, labelpad=6, color="#333")
    cbar.ax.tick_params(labelsize=8, colors="#333")
    cbar.outline.set_edgecolor("#333")
    cbar.outline.set_linewidth(0.5)
    fig.savefig(out_path, dpi=150, bbox_inches="tight",
                facecolor=fig.get_facecolor())
    plt.close(fig)


# --------------------------------------------------------------------------
# Interactive HTML (plotly)
# --------------------------------------------------------------------------

def _rgba_css(rgba) -> str:
    r, g, b, a = rgba
    return f"rgba({int(r * 255)},{int(g * 255)},{int(b * 255)},{a:.3f})"


def _escape_html(s: str) -> str:
    return (s.replace("&", "&amp;").replace("<", "&lt;")
             .replace(">", "&gt;").replace('"', "&quot;"))


def _hover_html(leaf: dict, rank: int | None) -> str:
    lines: list[str] = []
    header = leaf.get("import_path") or leaf.get("name") or leaf.get("path")
    lines.append(f"<b>{_escape_html(str(header))}</b>")
    metric_bits = [f"{leaf['revisions']} revs",
                   f"{leaf['loc']} LOC",
                   f"score {leaf['score']:,}"]
    if leaf.get("kind") == "package" and leaf.get("files"):
        metric_bits.insert(2, f"{leaf['files']} files")
    if leaf.get("kind") == "function" and leaf.get("file_path"):
        metric_bits.append(
            f"L{leaf.get('start_line','?')}-{leaf.get('end_line','?')}")
    lines.append("<br>".join(metric_bits))
    if rank is not None:
        lines.append(f"<i>rank #{rank} hotspot</i>")

    signals = leaf.get("signals") or []
    if signals:
        lines.append("<br><b>Signals</b>")
        for s in signals:
            lines.append(
                f"• {_escape_html(s['kind'])} ({s.get('severity','?')})"
                f" — {_escape_html(s.get('detail',''))}")

    authors = leaf.get("authors") or []
    if authors:
        lines.append("<br><b>Authors</b>")
        for a in authors[:3]:
            lines.append(f"• {_escape_html(a['name'])} "
                         f"({a['commits']} · {a['pct']}%)")

    recent = leaf.get("recent_commits") or []
    if recent:
        lines.append("<br><b>Recent commits</b>")
        for rc in recent[:4]:
            subj = _escape_html(rc.get("subject", ""))[:70]
            tag = "🔧 " if rc.get("looks_like_fix") else ""
            lines.append(
                f"• {rc.get('date','')} · {_escape_html(rc.get('author',''))}"
                f" — {tag}{subj}")

    actions = leaf.get("suggested_actions") or []
    if actions:
        lines.append("<br><b>Suggested</b>")
        for a in actions[:3]:
            lines.append(f"• {_escape_html(a)[:180]}")
    return "<br>".join(lines)


def emit_html(
    leaves: list[dict], tree: Node, out_path: Path,
    title: str, subtitle: str,
) -> None:
    if not leaves:
        return

    TOP_N = 12
    max_revs = max((p["revisions"] for p in leaves), default=1) or 1
    cmap = _hotspot_colormap()
    norm = plt.Normalize(vmin=0, vmax=max_revs)
    top_hot = sorted(leaves, key=lambda p: p["score"], reverse=True)[:TOP_N]
    hot_map = {p["path"]: i + 1 for i, p in enumerate(top_hot)}

    pack_tree(tree)

    fig = pgo.Figure()
    hover_x: list[float] = []
    hover_y: list[float] = []
    hover_text: list[str] = []
    annotations: list[dict] = []

    def walk(n: Node, cx: float, cy: float, depth: int) -> None:
        if n.pkg is not None and not n.children:
            heat = norm(n.pkg["revisions"])
            fill = _rgba_css(cmap(heat))
            is_hot = n.pkg["path"] in hot_map
            fig.add_shape(
                type="circle", xref="x", yref="y",
                x0=cx - n.r, y0=cy - n.r,
                x1=cx + n.r, y1=cy + n.r,
                fillcolor=fill,
                line=dict(color="#000" if is_hot else "#1a1a1a",
                          width=2.2 if is_hot else 0.7),
                layer="below",
            )
            if n.r > 6 and n.r * 2 > len(n.name) * 0.8:
                text_color = "white" if heat > 0.55 else "#111"
                annotations.append(dict(
                    x=cx, y=cy, text=_escape_html(n.name),
                    showarrow=False,
                    font=dict(family="monospace",
                              size=min(14, max(8, n.r * 0.35)),
                              color=text_color),
                ))
            if is_hot:
                rank = hot_map[n.pkg["path"]]
                br = max(0.6, min(n.r * 0.22, 2.0))
                angle = math.radians(55)
                bx = cx + n.r * math.cos(angle)
                by = cy + n.r * math.sin(angle)
                fig.add_shape(
                    type="circle", xref="x", yref="y",
                    x0=bx - br, y0=by - br, x1=bx + br, y1=by + br,
                    fillcolor="#111",
                    line=dict(color="white", width=1.0),
                    layer="above",
                )
                annotations.append(dict(
                    x=bx, y=by, text=str(rank), showarrow=False,
                    font=dict(family="Arial Black",
                              size=min(13, max(8, br * 1.2)),
                              color="white"),
                ))
            hover_x.append(cx)
            hover_y.append(cy)
            hover_text.append(_hover_html(n.pkg, hot_map.get(n.pkg["path"])))
            return

        fig.add_shape(
            type="circle", xref="x", yref="y",
            x0=cx - n.r, y0=cy - n.r,
            x1=cx + n.r, y1=cy + n.r,
            fillcolor="rgba(252,252,252,0.75)",
            line=dict(color="#aaa",
                      width=max(0.6, 1.2 - 0.18 * depth)),
            layer="below",
        )
        if n.name and n.r > 6:
            annotations.append(dict(
                x=cx, y=cy + n.r * 0.88, text=_escape_html(n.name),
                showarrow=False, bgcolor="rgba(255,255,255,0.9)",
                bordercolor="#aaa", borderwidth=0.5, borderpad=2,
                font=dict(family="monospace",
                          size=min(13, max(8, n.r * 0.30)),
                          color="#222"),
            ))
        for child in n.children:
            walk(child, cx + child.x, cy + child.y, depth + 1)

    walk(tree, 0.0, 0.0, 0)

    fig.add_trace(pgo.Scatter(
        x=hover_x, y=hover_y, mode="markers",
        marker=dict(size=10, color="rgba(0,0,0,0)"),
        hovertext=hover_text, hoverinfo="text",
        showlegend=False, name="hotspots",
    ))

    header_html = (
        f"<b>{_escape_html(title)}</b>"
        + (f"<br><span style='font-size:12px;color:#555'>"
           f"{_escape_html(subtitle)}</span>" if subtitle else "")
    )

    margin = tree.r * 0.05 if tree.r else 1.0
    fig.update_layout(
        title=dict(text=header_html, x=0.01, y=0.98,
                   xanchor="left", yanchor="top"),
        xaxis=dict(visible=False, range=[-tree.r - margin,
                                          tree.r + margin],
                   scaleanchor="y", scaleratio=1),
        yaxis=dict(visible=False, range=[-tree.r - margin,
                                          tree.r + margin]),
        plot_bgcolor="white", paper_bgcolor="white",
        hoverlabel=dict(font=dict(family="monospace", size=11),
                        bgcolor="white", bordercolor="#888"),
        margin=dict(l=20, r=20, t=90, b=40),
        width=1400, height=900,
        annotations=annotations,
    )
    fig.add_annotation(
        xref="paper", yref="paper", x=0.01, y=-0.02,
        xanchor="left", yanchor="top", showarrow=False,
        font=dict(size=10, color="#888", family="monospace"),
        text=("Hover any circle for signals · authors · recent commits · "
              "suggested actions. Colour = commits, radius ∝ √LOC, "
              "numbered = top hotspots."),
    )

    fig.write_html(str(out_path), include_plotlyjs="cdn",
                   full_html=True, auto_open=False)


# --------------------------------------------------------------------------
# Report building, baseline compare, threshold gating
# --------------------------------------------------------------------------

def build_report(
    leaves: list[dict], args: argparse.Namespace,
    scope: Path, mode: str,
    module_info: tuple[Path, str] | None,
    thresholds: dict[str, Any],
    total_files: int, total_commits: int,
    top_n: int,
) -> dict[str, Any]:
    top_n_effective = top_n if top_n > 0 else len(leaves)
    hotspots = []
    for i, leaf in enumerate(leaves[:top_n_effective], 1):
        h = {
            "rank": i,
            "kind": leaf.get("kind", "package"),
            "name": leaf.get("name"),
            "import_path": leaf.get("import_path"),
            "path": leaf.get("path"),
            "file_path": leaf.get("file_path"),
            "start_line": leaf.get("start_line"),
            "end_line": leaf.get("end_line"),
            "metrics": {
                "loc": leaf["loc"],
                "files": leaf.get("files"),
                "revisions": leaf["revisions"],
                "score": leaf["score"],
            },
            "recent_commits": leaf.get("recent_commits", []),
            "authors": leaf.get("authors", []),
            "author_count": leaf.get("author_count", 0),
            "bug_fix_count": leaf.get("bug_fix_count", 0),
            "bug_fix_pct": leaf.get("bug_fix_pct", 0.0),
            "signals": leaf.get("signals", []),
            "suggested_actions": leaf.get("suggested_actions", []),
        }
        if leaf.get("kind") == "package":
            h["sample_files"] = leaf.get("file_paths", [])[:8]
        hotspots.append(h)

    return {
        "schema": SCHEMA_VERSION,
        "tool": {"name": "go_hotspots", "version": TOOL_VERSION},
        "generated_at": datetime.now(timezone.utc).isoformat(
            timespec="seconds"),
        "scope": {
            "arg": str(args.directory),
            "resolved": str(scope),
            "mode": mode,
            "module_path": module_info[1] if module_info else None,
            "module_dir": (str(module_info[0]) if module_info else None),
        },
        "window": {"since": args.since, "until": args.until},
        "filters": {
            "include_tests": bool(args.include_tests),
            "ignore_dead_files": bool(args.ignore_dead_files),
            "exclude": args.exclude or [],
        },
        "stats": {
            "leaves": len(leaves),
            "total_files": total_files,
            "total_commits": total_commits,
            "top_score": leaves[0]["score"] if leaves else 0,
        },
        "thresholds": thresholds,
        "hotspots": hotspots,
    }


def load_baseline(path: Path) -> dict:
    try:
        return json.loads(path.read_text())
    except FileNotFoundError as e:
        raise HotspotError(f"baseline {path} does not exist") from e
    except json.JSONDecodeError as e:
        raise HotspotError(f"baseline {path} is not valid JSON — {e}") from e


def compare_to_baseline(current: dict, baseline: dict) -> dict:
    b_by_path = {h["path"]: h for h in baseline.get("hotspots", [])}
    c_by_path = {h["path"]: h for h in current["hotspots"]}

    new_list = []
    for p, h in c_by_path.items():
        if p not in b_by_path:
            new_list.append({
                "path": p, "name": h.get("name"),
                "import_path": h.get("import_path"),
                "score": h["metrics"]["score"], "rank": h["rank"],
            })
    gone = []
    for p, h in b_by_path.items():
        if p not in c_by_path:
            gone.append({
                "path": p, "name": h.get("name"),
                "import_path": h.get("import_path"),
                "score_was": h["metrics"]["score"],
            })

    hotter, cooler = [], []
    for p in set(b_by_path) & set(c_by_path):
        bs = b_by_path[p]["metrics"]["score"]
        cs = c_by_path[p]["metrics"]["score"]
        delta = cs - bs
        if bs == 0:
            pct = 100.0 if cs > 0 else 0.0
        else:
            pct = 100.0 * delta / bs
        entry = {
            "path": p,
            "name": c_by_path[p].get("name"),
            "import_path": c_by_path[p].get("import_path"),
            "score_from": bs, "score_to": cs,
            "delta": delta, "delta_pct": round(pct, 1),
        }
        if delta > 0:
            hotter.append(entry)
        elif delta < 0:
            cooler.append(entry)

    hotter.sort(key=lambda e: e["delta"], reverse=True)
    cooler.sort(key=lambda e: e["delta"])
    return {
        "baseline_generated_at": baseline.get("generated_at"),
        "baseline_schema": baseline.get("schema"),
        "new": sorted(new_list, key=lambda e: e["score"], reverse=True),
        "gone": gone,
        "hotter": hotter,
        "cooler": cooler,
        "summary": {
            "new_count": len(new_list),
            "gone_count": len(gone),
            "hotter_count": len(hotter),
            "cooler_count": len(cooler),
        },
    }


def apply_thresholds(
    report: dict, args: argparse.Namespace,
) -> tuple[int, list[str]]:
    exit_code = 0
    msgs: list[str] = []
    if args.fail_on_score is not None:
        over = [h for h in report["hotspots"]
                if h["metrics"]["score"] >= args.fail_on_score]
        if over:
            names = ", ".join(h["import_path"] or h["path"] for h in over[:3])
            msgs.append(
                f"FAIL: {len(over)} hotspot(s) at or above "
                f"--fail-on-score={args.fail_on_score}: {names}")
            exit_code = 1

    delta = report.get("baseline_delta")
    if delta is None:
        return exit_code, msgs

    if args.fail_on_new_hotspot and delta["new"]:
        names = ", ".join(
            n.get("import_path") or n["path"] for n in delta["new"][:3])
        msgs.append(
            f"FAIL: {len(delta['new'])} new hotspot(s) vs baseline: {names}")
        exit_code = 1

    if args.fail_on_worsening_pct is not None:
        limit = args.fail_on_worsening_pct
        worse = [e for e in delta["hotter"] if e["delta_pct"] >= limit]
        if worse:
            names = ", ".join(
                f"{e.get('import_path') or e['path']} +{e['delta_pct']}%"
                for e in worse[:3])
            msgs.append(
                f"FAIL: {len(worse)} hotspot(s) worsened by ≥{limit}% "
                f"vs baseline: {names}")
            exit_code = 1

    return exit_code, msgs


# --------------------------------------------------------------------------
# Tabular outputs (stdout, CSV)
# --------------------------------------------------------------------------

def print_text_report(leaves: list[dict], mode: str) -> None:
    if mode == "function":
        print(f"{'score':>8} {'revs':>5} {'loc':>6}  function")
        print("-" * 78)
        for p in leaves[:20]:
            rng = (f" (L{p.get('start_line','?')}-"
                   f"{p.get('end_line','?')})")
            print(f"{p['score']:>8} {p['revisions']:>5} {p['loc']:>6}  "
                  f"{p['import_path']}{rng}")
    else:
        print(f"{'score':>8} {'revs':>5} {'loc':>6} {'files':>5}  "
              f"import path")
        print("-" * 78)
        for p in leaves[:20]:
            print(f"{p['score']:>8} {p['revisions']:>5} {p['loc']:>6} "
                  f"{p['files']:>5}  "
                  f"{p.get('import_path') or p['rel_to_scope']}")


def write_csv(leaves: list[dict], mode: str, path: Path) -> None:
    with path.open("w", newline="") as fh:
        if mode == "function":
            fields = ["import_path", "function", "path",
                      "start_line", "end_line",
                      "loc", "revisions", "score",
                      "signals", "author_count", "bug_fix_count"]
            w = csv.DictWriter(fh, fieldnames=fields)
            w.writeheader()
            for p in leaves:
                w.writerow({
                    "import_path": p["import_path"],
                    "function": p["name"],
                    "path": p["path"],
                    "start_line": p.get("start_line", ""),
                    "end_line": p.get("end_line", ""),
                    "loc": p["loc"],
                    "revisions": p["revisions"],
                    "score": p["score"],
                    "signals": ";".join(
                        s["kind"] for s in p.get("signals", [])),
                    "author_count": p.get("author_count", 0),
                    "bug_fix_count": p.get("bug_fix_count", 0),
                })
        else:
            fields = ["import_path", "package", "path",
                      "files", "loc", "revisions", "score",
                      "signals", "author_count", "bug_fix_count"]
            w = csv.DictWriter(fh, fieldnames=fields)
            w.writeheader()
            for p in leaves:
                w.writerow({
                    "import_path": p.get("import_path") or "",
                    "package": p["rel_to_scope"],
                    "path": p["path"],
                    "files": p["files"],
                    "loc": p["loc"],
                    "revisions": p["revisions"],
                    "score": p["score"],
                    "signals": ";".join(
                        s["kind"] for s in p.get("signals", [])),
                    "author_count": p.get("author_count", 0),
                    "bug_fix_count": p.get("bug_fix_count", 0),
                })


# --------------------------------------------------------------------------
# CLI
# --------------------------------------------------------------------------

def _build_argparser() -> argparse.ArgumentParser:
    ap = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("directory", type=Path, metavar="PATH",
                    help="directory or .go file to analyse")
    ap.add_argument("--since", default=None,
                    help="git --since (2024-01-01, '3 weeks ago', ...)")
    ap.add_argument("--until", default=None, help="git --until")
    ap.add_argument("--mode", choices=("auto", "package", "function"),
                    default="auto")
    ap.add_argument("--out", type=Path, default=Path("go_hotspots.png"),
                    help="output PNG path")
    ap.add_argument("--no-image", action="store_true",
                    help="skip writing the PNG (CI / headless use)")
    ap.add_argument("--html", type=Path, default=None,
                    help="interactive Plotly HTML report path. Auto-written "
                         "next to --out unless --no-html is set.")
    ap.add_argument("--no-html", action="store_true",
                    help="skip the interactive HTML")
    ap.add_argument("--csv", type=Path, default=None,
                    help="write per-leaf CSV to this path")
    ap.add_argument("--json", type=Path, default=None,
                    help="write the structured JSON report to this path "
                         "(schema go-hotspots/1). Intended for CI gates and "
                         "AI refactoring consumers.")
    ap.add_argument("--baseline", type=Path, default=None,
                    help="compare against a prior --json report; adds a "
                         "baseline_delta section (new, gone, hotter, cooler)")
    ap.add_argument("--top-n", type=int, default=20,
                    help="top-N hotspots to include in JSON (0 = all)")
    ap.add_argument("--fail-on-score", type=float, default=None,
                    help="exit 1 if any hotspot's score ≥ this value")
    ap.add_argument("--fail-on-new-hotspot", action="store_true",
                    help="exit 1 if any new hotspot appears vs baseline")
    ap.add_argument("--fail-on-worsening-pct", type=float, default=None,
                    help="exit 1 if any shared hotspot's score grows by "
                         "≥ this percent vs baseline (e.g. 20)")
    ap.add_argument("--quiet", action="store_true",
                    help="suppress the top-20 stdout table")
    ap.add_argument("--verbose", "-v", action="count", default=0,
                    help="-v for INFO, -vv for DEBUG")
    ap.add_argument("--include-tests", action="store_true")
    ap.add_argument("--ignore-dead-files", action="store_true",
                    help="[deprecated; on by default] no-op")
    ap.add_argument("--keep-dead-files", action="store_true",
                    help="keep commits whose only affected files no longer "
                         "exist on disk")
    ap.add_argument("--exclude", action="append", default=None,
                    metavar="REGEX",
                    help="exclude .go files matching this regex (repeatable). "
                         "Added on top of the auto-applied defaults.")
    ap.add_argument("--no-default-excludes", action="store_true",
                    help=f"don't auto-exclude common generated Go files. "
                         f"Defaults: {', '.join(DEFAULT_EXCLUDES)}")
    return ap


def _setup_logging(verbose: int) -> None:
    level = logging.WARNING
    if verbose == 1:
        level = logging.INFO
    elif verbose >= 2:
        level = logging.DEBUG
    logging.basicConfig(
        level=level,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
        datefmt="%H:%M:%S",
        stream=sys.stderr,
    )


def _resolve_outputs(args: argparse.Namespace) -> None:
    if (args.html is None and not args.no_html and not args.no_image):
        args.html = args.out.with_suffix(".html")


def _build_title_subtitle(
    leaves: list[dict], mode: str, root_display: str, args: argparse.Namespace,
    total_files: int, total_commits: int,
) -> tuple[str, str]:
    window_bits = []
    if args.since:
        window_bits.append(f"since {_display_date(args.since)}")
    if args.until:
        window_bits.append(f"until {_display_date(args.until)}")
    title_since = (" · " + " · ".join(window_bits)) if window_bits else ""
    top = leaves[0]
    if mode == "function":
        counts_line = f"{len(leaves)} functions · Σ {total_commits} commits"
        top_line = (f"top hotspot: {top['name']} "
                    f"({top['revisions']} revs × {top['loc']} LOC)")
        subtitle = f"{counts_line}\n{top_line}"
        title = f"Function hotspots in {root_display}{title_since}"
    else:
        counts_line = (f"{len(leaves)} packages · {total_files} files · "
                       f"{total_commits} commits")
        top_line = (
            f"top hotspot: {top.get('import_path') or top['rel_to_scope']} "
            f"({top['revisions']} revs × {top['loc']} LOC)")
        subtitle = f"{counts_line}\n{top_line}"
        title = f"Go hotspots in {root_display}{title_since}"
    return title, subtitle


def run(args: argparse.Namespace) -> int:
    scope = args.directory.resolve()
    if not scope.exists():
        raise HotspotError(f"{scope} does not exist")
    if not scope.is_dir() and not (scope.is_file() and scope.suffix == ".go"):
        raise HotspotError(f"{scope} must be a directory or a .go file")

    if args.since:
        args.since = normalize_date("--since", args.since)
    if args.until:
        args.until = normalize_date("--until", args.until)
    if args.fail_on_new_hotspot and args.baseline is None:
        raise HotspotError("--fail-on-new-hotspot requires --baseline")
    if args.fail_on_worsening_pct is not None and args.baseline is None:
        raise HotspotError("--fail-on-worsening-pct requires --baseline")

    effective_excludes: list[str] = []
    if not args.no_default_excludes:
        effective_excludes.extend(DEFAULT_EXCLUDES)
    if args.exclude:
        effective_excludes.extend(args.exclude)
    args.exclude = effective_excludes
    exclude_regex = compile_excludes(effective_excludes)

    ignore_dead_files = not args.keep_dead_files
    args.ignore_dead_files = ignore_dead_files

    _resolve_outputs(args)

    base = scope if scope.is_dir() else scope.parent
    git_root = find_git_root(base)
    module_info = find_go_module(base)
    mode = detect_mode(scope, args.mode)
    log.info("git_root=%s module=%s mode=%s",
             git_root, module_info[1] if module_info else None, mode)

    if mode == "function":
        if module_info is not None:
            mod_dir, mod_path = module_info
            try:
                rel = base.resolve().relative_to(mod_dir)
                rel_parts = [p for p in rel.parts if p and p != "."]
                pkg_label = (mod_path + "/" + "/".join(rel_parts)
                             if rel_parts else mod_path)
            except ValueError:
                pkg_label = base.name
        else:
            pkg_label = base.name
        root_display = (f"{pkg_label}/{scope.name}"
                        if scope.is_file() else pkg_label)

        tree, leaves, commit_meta = build_function_tree(
            scope, git_root, args.include_tests,
            args.since, args.until, exclude_regex, root_display,
        )
        if not leaves:
            raise HotspotError("no Go functions found in the given scope")
        total_files = 1 if scope.is_file() else len({
            p["file_path"] for p in leaves if p.get("file_path")
        })
        total_commits = len(commit_meta)
    else:
        if not scope.is_dir():
            raise HotspotError(f"{scope} is a file; --mode package needs "
                               "a directory")
        live_files: set[str] | None = None
        if ignore_dead_files:
            live_files = {
                str(f.resolve().relative_to(git_root))
                for f in collect_go_files(scope)
            }
        commits_per_file, commit_meta = git_commit_history(
            git_root, scope, args.since, args.until,
            exclude_regex=exclude_regex, live_files=live_files)
        if (args.since or args.until) and not commits_per_file:
            bits = []
            if args.since:
                bits.append(f"since {args.since!r}")
            if args.until:
                bits.append(f"until {args.until!r}")
            raise HotspotError(
                f"no commits touched any .go file in {scope} "
                f"({' / '.join(bits)}). Widen the date range or check "
                "the dates.")
        leaves = collect_leaf_packages(
            scope, git_root, commits_per_file, args.include_tests,
            module_info, exclude_regex=exclude_regex)
        if not leaves:
            raise HotspotError("no Go packages found under the given directory")
        total_files = sum(p["files"] for p in leaves)
        all_commits: set[str] = set()
        for commits in commits_per_file.values():
            all_commits |= commits
        total_commits = len(all_commits)

        if module_info is not None:
            mod_dir, mod_path = module_info
            try:
                scope_rel = scope.resolve().relative_to(mod_dir)
                rel_parts = [p for p in scope_rel.parts if p and p != "."]
                root_display = (mod_path + "/" + "/".join(rel_parts)
                                if rel_parts else mod_path)
            except ValueError:
                root_display = scope.name
        else:
            root_display = scope.name
        tree = build_tree(leaves, root_name=root_display)

    leaves.sort(key=lambda p: p["score"], reverse=True)
    enrich_leaves(leaves, commit_meta)

    if not args.quiet:
        print_text_report(leaves, mode)

    if args.csv:
        write_csv(leaves, mode, args.csv)
        if not args.quiet:
            print(f"wrote {args.csv}")

    thresholds_spec = {
        "fail_on_score": args.fail_on_score,
        "fail_on_new_hotspot": bool(args.fail_on_new_hotspot),
        "fail_on_worsening_pct": args.fail_on_worsening_pct,
    }
    report = build_report(
        leaves, args, scope, mode, module_info,
        thresholds_spec, total_files, total_commits, args.top_n,
    )

    if args.baseline is not None:
        baseline = load_baseline(args.baseline)
        report["baseline_delta"] = compare_to_baseline(report, baseline)

    if args.json is not None:
        args.json.write_text(json.dumps(report, indent=2) + "\n")
        if not args.quiet:
            print(f"wrote {args.json}")

    title, subtitle = _build_title_subtitle(
        leaves, mode, root_display, args, total_files, total_commits)

    if not args.no_image:
        plot_hotspots(leaves, tree, args.out, title, subtitle)
        if not args.quiet:
            print(f"wrote {args.out}")

    if args.html is not None:
        emit_html(leaves, tree, args.html, title, subtitle)
        if not args.quiet:
            print(f"wrote {args.html}")

    exit_code, msgs = apply_thresholds(report, args)
    for m in msgs:
        print(m, file=sys.stderr)
    return exit_code


def main() -> None:
    args = _build_argparser().parse_args()
    _setup_logging(args.verbose)
    try:
        sys.exit(run(args))
    except HotspotError as e:
        log.error("%s", e)
        sys.exit(EXIT_ERROR)
    except KeyboardInterrupt:
        sys.stderr.write("\ninterrupted\n")
        sys.exit(130)


if __name__ == "__main__":
    main()
