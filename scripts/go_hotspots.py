#!/usr/bin/env python3
"""
go_hotspots.py — Code-as-a-Crime-Scene hotspot analysis for Go.

This script ranks the parts of a Go codebase that are both most-changed (git
commits) and most-complex (effective lines of Go code). Hotspots are where
refactoring pays off most (Tornhill, *Your Code as a Crime Scene*, ch. 3).

Output surfaces:

  PNG       — a circle-pack enclosure diagram (for humans)
  JSON      — a versioned, structured report (for CI gates and AI consumers)
  CSV       — per-leaf table (for spreadsheets / ad-hoc scripts)
  stdout    — ranked top-20 table (suppress with --quiet)
  exit code — 0 clean, 1 threshold exceeded, 2 script error

Software is fractal, so the tool is recursive. Point it at:
  - a directory tree           → leaves are Go packages
  - a single-package directory → leaves are top-level funcs/methods
  - a single .go file          → leaves are top-level funcs/methods

JSON report schema: go-hotspots/1. Baseline comparison, signal annotations,
and templated refactoring suggestions make each hotspot actionable for both
humans and automated refactoring agents.
"""
from __future__ import annotations

import argparse
import csv
import json
import math
import os
import re
import subprocess
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Iterable

import matplotlib.pyplot as plt
from matplotlib.patches import Rectangle

# Optional: delegate nested circle packing to the `circlify` library when
# it's available (actively maintained, handles edge cases better than our
# inlined Wang/Bostock routine). Falls back to the built-in pack_siblings
# when the library isn't installed so the tool remains zero-dep beyond
# matplotlib.
try:
    import circlify as _circlify  # type: ignore
except ImportError:  # pragma: no cover
    _circlify = None  # type: ignore

# Optional: emit an interactive HTML report via plotly when requested. The
# PNG remains the primary artefact; plotly gives hover tooltips carrying
# signals, authors, recent commits, and suggested actions — a dramatic
# UX upgrade for exploration.
try:
    import plotly.graph_objects as _go  # type: ignore
except ImportError:  # pragma: no cover
    _go = None  # type: ignore

SCHEMA_VERSION = "go-hotspots/1"
TOOL_VERSION = "1.0"

# Exit code contract:
#   0 — clean run, no thresholds exceeded
#   1 — thresholds exceeded (finding-level failure, intended for CI gates)
#   2 — script could not run (bad args, missing input, git failure)
EXIT_OK = 0
EXIT_THRESHOLD = 1
EXIT_ERROR = 2


def die(msg: str, code: int = EXIT_ERROR) -> None:
    print(msg, file=sys.stderr)
    sys.exit(code)

SKIP_DIRS = {"vendor", "testdata", ".git", "node_modules"}
_BLOCK_COMMENT = re.compile(r"/\*.*?\*/", re.DOTALL)

# Default exclude patterns for common generated Go files. These are auto-
# applied so a typical invocation doesn't have to spell them out; the user
# can opt out with --no-default-excludes or add their own via --exclude.
DEFAULT_EXCLUDES: tuple[str, ...] = (
    r"_templ\.go$",        # a-h/templ
    r"_mock\.go$",         # gomock, mockery
    r"_mocks\.go$",        # mockery (plural)
    r"\.pb\.go$",          # protoc
    r"\.pb\.gw\.go$",      # grpc-gateway
    r"_gen\.go$",          # go generate conventions
    r"(^|/)zz_generated",  # kubebuilder-style
    r"bindata\.go$",       # go-bindata
)

BUG_KEYWORDS = re.compile(
    r"\b(fix|bug|issue|error|crash|panic|defect|regression|hotfix|patch|"
    r"broken|fail|fault|fixup|revert)\b",
    re.IGNORECASE,
)

# Git silently ignores unparseable --since/--until values and returns zero
# commits — indistinguishable from a genuinely empty window. We validate the
# value in Python first so bad input fails loudly.
_DATE_FORMATS = (
    "%Y-%m-%d",
    "%Y-%m-%dT%H:%M:%S",
    "%Y-%m-%dT%H:%M:%SZ",
    "%Y-%m-%d %H:%M:%S",
    "%Y/%m/%d",
)
_RELATIVE_DATE_RE = re.compile(
    r"^\s*(\d+)\s*(second|minute|hour|day|week|month|year)s?"
    r"(?:\s+ago)?\s*$",
    re.IGNORECASE,
)
_WEEKDAY_RE = re.compile(
    r"^\s*(last\s+|this\s+|next\s+)?("
    r"sunday|monday|tuesday|wednesday|thursday|friday|saturday|"
    r"week|month|year"
    r")\s*$",
    re.IGNORECASE,
)
_NAMED_DATES = {"yesterday", "today", "now", "midnight", "noon"}


def normalize_date(flag: str, value: str) -> str:
    s = value.strip()
    if not s:
        die(f"error: {flag} cannot be empty")
    for fmt in _DATE_FORMATS:
        try:
            datetime.strptime(s, fmt)
            return s
        except ValueError:
            pass
    m = _RELATIVE_DATE_RE.match(s)
    if m:
        return f"{m.group(1)} {m.group(2).lower()}s ago"
    if s.lower() in _NAMED_DATES or _WEEKDAY_RE.match(s):
        return s
    die(
        f"error: {flag} value {value!r} is not a recognised date.\n"
        "  Use an ISO date (YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS),\n"
        "  a relative phrase such as '3 weeks ago' or '2 days',\n"
        "  or 'yesterday' / 'last monday' / 'last week'."
    )


def compile_excludes(patterns: list[str] | None) -> re.Pattern | None:
    if not patterns:
        return None
    combined = "|".join(f"(?:{p})" for p in patterns)
    try:
        return re.compile(combined)
    except re.error as e:
        die(f"error: invalid --exclude pattern: {e}")


# --------------------------------------------------------------------------
# Git / filesystem helpers
# --------------------------------------------------------------------------

def find_git_root(path: Path) -> Path:
    try:
        out = subprocess.check_output(
            ["git", "-C", str(path), "rev-parse", "--show-toplevel"],
            stderr=subprocess.DEVNULL,
        )
    except subprocess.CalledProcessError:
        die(f"error: {path} is not inside a git repository")
    return Path(out.decode().strip())


def find_go_module(scope: Path) -> tuple[Path, str] | None:
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
            except OSError:
                pass
        if cur.parent == cur:
            return None
        cur = cur.parent


def collect_go_files(root: Path) -> Iterable[Path]:
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames
                       if d not in SKIP_DIRS and not d.startswith(".")]
        for name in filenames:
            if name.endswith(".go"):
                yield Path(dirpath) / name


def count_effective_loc(path: Path) -> int:
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return 0
    return _count_loc_text(text)


def _count_loc_text(text: str) -> int:
    text = _BLOCK_COMMENT.sub("", text)
    loc = 0
    for line in text.splitlines():
        s = line.strip()
        if s and not s.startswith("//"):
            loc += 1
    return loc


# --------------------------------------------------------------------------
# Go function parser
# --------------------------------------------------------------------------

_FUNC_START_RE = re.compile(
    r'^func\s+'
    r'(?:\(\s*(?:\w+\s+)?(?:\*\s*)?(?P<recv>\w+)'
    r'(?:\s*\[[^\]]*\])?\s*\)\s+)?'
    r'(?P<name>\w+)\b'
)


def _strip_go_noise(text: str) -> str:
    out: list[str] = []
    i = 0
    n = len(text)
    while i < n:
        c = text[i]
        if c == '/' and i + 1 < n and text[i + 1] == '/':
            j = text.find('\n', i)
            if j == -1:
                out.append(' ' * (n - i)); i = n
            else:
                out.append(' ' * (j - i)); i = j
        elif c == '/' and i + 1 < n and text[i + 1] == '*':
            j = text.find('*/', i + 2)
            end = n if j == -1 else j + 2
            block = text[i:end]
            out.append(''.join(ch if ch == '\n' else ' ' for ch in block))
            i = end
        elif c == '`':
            j = text.find('`', i + 1)
            end = n if j == -1 else j + 1
            block = text[i:end]
            out.append(''.join(ch if ch == '\n' else ' ' for ch in block))
            i = end
        elif c in ('"', "'"):
            quote = c
            j = i + 1
            while j < n:
                if text[j] == '\\' and j + 1 < n:
                    j += 2; continue
                if text[j] == quote or text[j] == '\n':
                    break
                j += 1
            j = min(j + 1, n)
            block = text[i:j]
            out.append(''.join(ch if ch == '\n' else ' ' for ch in block))
            i = j
        else:
            out.append(c); i += 1
    return ''.join(out)


def _find_func_end(stripped_lines: list[str], start_idx: int) -> int | None:
    depth = 0
    opened = False
    for i in range(start_idx, len(stripped_lines)):
        for ch in stripped_lines[i]:
            if ch == '{':
                depth += 1; opened = True
            elif ch == '}':
                depth -= 1
                if opened and depth == 0:
                    return i + 1
    return None


def parse_go_functions(path: Path) -> list[dict]:
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return []
    lines = text.splitlines()
    stripped_lines = _strip_go_noise(text).splitlines()
    while len(stripped_lines) < len(lines):
        stripped_lines.append("")

    functions: list[dict] = []
    i = 0
    n = len(lines)
    while i < n:
        m = _FUNC_START_RE.match(stripped_lines[i])
        if not m:
            i += 1; continue
        end = _find_func_end(stripped_lines, i)
        if end is None:
            i += 1; continue
        name = m.group("name")
        recv = m.group("recv")
        display = f"{recv}.{name}" if recv else name
        body = "\n".join(lines[i:end])
        functions.append({
            "name": display,
            "start_line": i + 1,
            "end_line": end,
            "loc": _count_loc_text(body),
        })
        i = end
    return functions


# --------------------------------------------------------------------------
# Git history — package-level (one log call) and function-level (per-func).
# Both return (hashes_per_leaf, commit_meta) so enrichment can look up any
# hash's author/date/subject anywhere downstream.
# --------------------------------------------------------------------------

_LOG_PRETTY = "__C__%H%x09%an%x09%ad%x09%s"


def _parse_commit_line(prefixed: str) -> tuple[str, dict] | None:
    if not prefixed.startswith("__C__"):
        return None
    parts = prefixed[5:].split("\t", 3)
    if len(parts) < 4:
        # Pad missing subject (unlikely but safe).
        parts += [""] * (4 - len(parts))
    h, author, date, subject = parts[0], parts[1], parts[2], parts[3]
    return h, {"author": author, "date": date, "subject": subject}


def git_commit_history(
    git_root: Path, scope: Path,
    since: str | None, until: str | None,
    exclude_regex: re.Pattern | None = None,
    live_files: set[str] | None = None,
) -> tuple[dict[str, set[str]], dict[str, dict]]:
    """Distinct-commit history for every .go path under `scope`.

    Returns (commits_per_file, commit_meta) from a single `git log` invocation
    — cheaper than running log once per leaf, and commit_meta is the authoritative
    source for enrichment downstream.
    """
    cmd = ["git", "-C", str(git_root), "log",
           f"--pretty=format:{_LOG_PRETTY}",
           "--date=short", "--name-only"]
    if since:
        cmd.append(f"--since={since}")
    if until:
        cmd.append(f"--until={until}")
    rel = scope.resolve().relative_to(git_root)
    cmd.append("--")
    cmd.append("." if str(rel) == "." else str(rel))
    try:
        out = subprocess.check_output(
            cmd, stderr=subprocess.PIPE).decode(errors="replace")
    except subprocess.CalledProcessError as e:
        err = e.stderr.decode(errors="replace").strip() or "(no stderr)"
        die(f"error: git log failed — {err}")

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
    return commits_per_file, commit_meta


def git_function_history(
    git_root: Path, rel_path: str, start: int, end: int,
    since: str | None, until: str | None,
) -> tuple[set[str], dict[str, dict]]:
    """Per-function history via `git log -L <start>,<end>:<file>`.

    Returns (set of commit hashes touching the range, meta for those hashes).
    """
    cmd = ["git", "-C", str(git_root), "log",
           f"-L{start},{end}:{rel_path}",
           f"--pretty=format:{_LOG_PRETTY}",
           "--date=short", "-s"]
    if since:
        cmd.append(f"--since={since}")
    if until:
        cmd.append(f"--until={until}")
    try:
        out = subprocess.check_output(
            cmd, stderr=subprocess.PIPE).decode(errors="replace")
    except subprocess.CalledProcessError:
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
            die(f"error: {scope} is not a .go file")
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
        die(
            f"error: {scope} has no .go files directly (function mode "
            "only analyses one package at a time; point me at the single "
            "package dir, or pass --mode package for the nested view)"
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


# --------------------------------------------------------------------------
# Enrichment — signals, recent commits, author distribution, suggestions.
# This is the part that makes each hotspot actionable for a reviewer or an
# AI refactoring agent: not just "this is hot", but WHY and WHAT TO DO.
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
            "looks_like_fix": bool(
                BUG_KEYWORDS.search(m.get("subject", ""))),
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
    else:  # function
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
# Circle packing (Wang / Bostock front-chain)
# --------------------------------------------------------------------------

class _PCircle:
    __slots__ = ("r", "x", "y", "data", "_prev", "_next")
    def __init__(self, r: float, data=None):
        self.r = r; self.x = 0.0; self.y = 0.0; self.data = data
        self._prev: _PCircle | None = None; self._next: _PCircle | None = None


def _place(a: _PCircle, b: _PCircle, c: _PCircle) -> None:
    dx = b.x - a.x; dy = b.y - a.y; d2 = dx * dx + dy * dy
    if d2 > 0:
        a2 = (c.r + a.r) ** 2; b2 = (c.r + b.r) ** 2
        if a2 > b2:
            x = (d2 + b2 - a2) / (2 * d2)
            y = math.sqrt(max(0.0, b2 / d2 - x * x))
            c.x = b.x - x * dx - y * dy
            c.y = b.y - x * dy + y * dx
        else:
            x = (d2 + a2 - b2) / (2 * d2)
            y = math.sqrt(max(0.0, a2 / d2 - x * x))
            c.x = a.x + x * dx - y * dy
            c.y = a.y + x * dy + y * dx
    else:
        c.x = a.x + c.r; c.y = a.y


def _intersects(a: _PCircle, b: _PCircle, tol: float = 1e-9) -> bool:
    dr = a.r + b.r - tol
    dx = b.x - a.x; dy = b.y - a.y
    return dr > 0 and dr * dr > dx * dx + dy * dy


def _score(a: _PCircle) -> float:
    b = a._next
    if b is None:
        return 0.0
    ab = a.r + b.r
    if ab == 0:
        return 0.0
    dx = (a.x * b.r + b.x * a.r) / ab
    dy = (a.y * b.r + b.y * a.r) / ab
    return dx * dx + dy * dy


def _enclose(circles: list[_PCircle]) -> tuple[float, float, float]:
    if not circles:
        return 0.0, 0.0, 0.0
    cx = sum(c.x for c in circles) / len(circles)
    cy = sum(c.y for c in circles) / len(circles)
    R = max(math.hypot(c.x - cx, c.y - cy) + c.r for c in circles)
    for _ in range(64):
        changed = False
        for c in circles:
            d = math.hypot(c.x - cx, c.y - cy)
            if d + c.r > R + 1e-9:
                new_R = (R + d + c.r) / 2
                if d > 1e-12:
                    t = (new_R - R) / d
                    cx += (c.x - cx) * t
                    cy += (c.y - cy) * t
                R = new_R; changed = True
        if not changed:
            break
    return cx, cy, R


def pack_siblings(circles: list[_PCircle]) -> float:
    n = len(circles)
    if n == 0:
        return 0.0
    a = circles[0]; a.x = a.y = 0.0
    if n < 2:
        return a.r
    b = circles[1]; a.x = -b.r; b.x = a.r; b.y = 0.0
    if n < 3:
        cx, cy, R = _enclose(circles)
        for c in circles:
            c.x -= cx; c.y -= cy
        return R
    c = circles[2]; _place(b, a, c)
    a._next = c; a._prev = b
    b._next = a; b._prev = c
    c._next = b; c._prev = a
    i = 3
    while i < n:
        c = circles[i]; _place(a, b, c)
        j = b._next; k = a._prev; sj = b.r; sk = a.r
        overlap = False
        while True:
            if sj <= sk:
                if _intersects(j, c):
                    b = j; a._next = b; b._prev = a
                    overlap = True; break
                sj += j.r; j = j._next
            else:
                if _intersects(k, c):
                    a = k; a._next = b; b._prev = a
                    overlap = True; break
                sk += k.r; k = k._prev
            if j is k._next:
                break
        if overlap:
            continue
        c._prev = a; c._next = b
        a._next = c; b._prev = c
        b = c
        best = _score(a); cur = c._next
        while cur is not b:
            s = _score(cur)
            if s < best:
                a = cur; best = s
            cur = cur._next
        b = a._next
        i += 1
    cx, cy, R = _enclose(circles)
    for c in circles:
        c.x -= cx; c.y -= cy
    return R


def pack_tree(node: Node, pad: float = 1.8) -> None:
    node.x = 0.0; node.y = 0.0
    if node.pkg is not None and not node.children:
        loc = max(1.0, float(node.pkg["loc"]))
        node.r = math.sqrt(loc / math.pi)
        return
    if not node.children:
        node.r = 1.0; return
    for c in node.children:
        pack_tree(c, pad)
    peers = [_PCircle(c.r + pad, c) for c in node.children]
    R = pack_siblings(peers)
    for peer, child in zip(peers, node.children):
        child.x = peer.x
        child.y = peer.y
    node.r = R


def pack_tree_circlify(node: Node, scale: float = 100.0) -> None:
    """Pack a Node tree using the `circlify` library (if installed).

    circlify returns absolute coordinates relative to a unit enclosing
    circle; our renderer works in parent-relative coordinates. We walk the
    result, stash absolute (x, y) per node, then convert to parent-relative
    offsets the renderer can recurse through unchanged.
    """
    assert _circlify is not None
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
    circles = _circlify.circlify(top, show_enclosure=False)

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
                n.x = 0.0; n.y = 0.0
        else:
            ax_, ay = abs_pos.get(nid, (pax, pay))
            n.x = ax_ - pax
            n.y = ay - pay
        for c in n.children:
            _set_relative(c, ax_, ay)

    _set_relative(node, 0.0, 0.0)


def pack_tree_dispatch(node: Node, pad: float = 1.8) -> str:
    """Pack `node`; return the name of the engine used ('circlify'|'native')."""
    if _circlify is not None:
        pack_tree_circlify(node)
        return "circlify"
    pack_tree(node, pad)
    return "native"


# --------------------------------------------------------------------------
# Rendering
# --------------------------------------------------------------------------

LEAF_EDGE = "#1a1a1a"
LEAF_EDGE_HOT = "#000000"
LEAF_LW = 0.7
LEAF_LW_HOT = 2.4
GROUP_EDGE = "#a6a6a6"
GROUP_FILL = "#fcfcfc"
ROOT_EDGE = "#b8b8b8"


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
        bx = cx + r * math.cos(angle); by = cy + r * math.sin(angle)
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
    cmap = plt.get_cmap("YlOrRd")
    norm = plt.Normalize(vmin=0, vmax=max_revs)
    top_hot = sorted(leaves, key=lambda p: p["score"], reverse=True)[:TOP_N]
    hot_map = {p["path"]: i + 1 for i, p in enumerate(top_hot)}
    pack_tree_dispatch(tree)

    fig = plt.figure(figsize=(20, 13), facecolor="white")
    gs = fig.add_gridspec(
        3, 2, height_ratios=[0.11, 1.0, 0.035], width_ratios=[4, 1],
        left=0.03, right=0.98, top=0.97, bottom=0.04,
        hspace=0.08, wspace=0.04)
    ax_header = fig.add_subplot(gs[0, :])
    ax = fig.add_subplot(gs[1, 0])
    ax_side = fig.add_subplot(gs[1, 1])
    ax_cbar = fig.add_subplot(gs[2, 0])
    for a in (ax_header, ax, ax_side):
        a.axis("off")

    ax_header.text(0, 0.85, title, fontsize=18, fontweight="bold",
                   va="top", ha="left", family="sans-serif", color="#111")
    if subtitle:
        ax_header.text(0, 0.45, subtitle, fontsize=10.5,
                       va="top", ha="left", color="#444")
    is_fn = bool(leaves) and leaves[0].get("kind") == "function"
    if is_fn:
        caption = (
            "Code-as-a-Crime-Scene enclosure diagram — function-level "
            "zoom. Every filled circle is a top-level Go func/method; "
            "outlined circles are the files housing them. Radius ∝ √(LOC) "
            "· fill = distinct commits touching the function's line range "
            "(git log -L) · numbered = top hotspots.")
        cbar_label = ("Change frequency — distinct commits touching the "
                      "function — cold → hot")
        sidebar_heading = "TOP HOTSPOT FUNCTIONS"
    else:
        caption = (
            "Code-as-a-Crime-Scene enclosure diagram. Every filled circle "
            "is one Go package (one directory of .go files); outlined "
            "circles are grouping directories. Radius ∝ √(Σ LOC) · fill = "
            "distinct commits touching the package · numbered = top "
            "hotspots.")
        cbar_label = ("Change frequency — distinct commits touching the "
                      "package — cold → hot")
        sidebar_heading = "TOP HOTSPOT PACKAGES"
    ax_header.text(0, 0.08, caption, fontsize=9, color="#888",
                   va="top", ha="left", style="italic")

    _render_circle(ax, tree, 0.0, 0.0, cmap, norm, hot_map, depth=0)
    margin = tree.r * 0.04
    ax.set_xlim(-tree.r - margin, tree.r + margin)
    ax.set_ylim(-tree.r - margin, tree.r + margin)
    ax.set_aspect("equal")

    ax_side.text(0.0, 1.0, sidebar_heading, fontsize=12, fontweight="bold",
                 color="#111", va="top", ha="left",
                 transform=ax_side.transAxes)
    ax_side.text(0.0, 0.975, "review these first — high churn × high size",
                 fontsize=8.5, color="#666", style="italic",
                 va="top", ha="left", transform=ax_side.transAxes)
    line_h = 1.0 / (TOP_N + 1.5)
    for i, p in enumerate(top_hot, 1):
        y = 0.94 - (i - 0.5) * line_h
        heat = norm(p["revisions"]); color = cmap(heat)
        chip_h = line_h * 0.55
        ax_side.add_patch(Rectangle(
            (0.0, y - chip_h / 2), 0.085, chip_h,
            transform=ax_side.transAxes, facecolor=color,
            edgecolor="#111", linewidth=0.7, clip_on=False))
        ax_side.text(0.0425, y, str(i), ha="center", va="center",
                     fontsize=10.5, fontweight="bold",
                     color="white" if heat > 0.55 else "#111",
                     transform=ax_side.transAxes)
        ax_side.text(0.11, y + chip_h * 0.15,
                     p.get("import_path") or p["rel_to_scope"],
                     fontsize=9, va="center", family="monospace",
                     color="#111", transform=ax_side.transAxes)
        bits = f'{p["revisions"]} revs · {p["loc"]} LOC · score {p["score"]:,}'
        if p.get("files") and p.get("kind") == "package":
            bits = (f'{p["revisions"]} revs · {p["loc"]} LOC · '
                    f'{p["files"]} files · score {p["score"]:,}')
        ax_side.text(0.11, y - chip_h * 0.45, bits,
                     fontsize=8, va="center", color="#666",
                     transform=ax_side.transAxes)

    sm = plt.cm.ScalarMappable(cmap=cmap, norm=norm); sm.set_array([])
    cbar = fig.colorbar(sm, cax=ax_cbar, orientation="horizontal")
    cbar.set_label(cbar_label, fontsize=9.5, labelpad=6, color="#333")
    cbar.ax.tick_params(labelsize=8, colors="#333")
    cbar.outline.set_edgecolor("#333"); cbar.outline.set_linewidth(0.5)
    fig.savefig(out_path, dpi=150, bbox_inches="tight",
                facecolor=fig.get_facecolor())


# --------------------------------------------------------------------------
# Interactive HTML (plotly) — optional alternative to the static PNG.
# Same layout (circle-pack), but hover tooltips carry full signal/author/
# commit/action context so a reader can triage without opening the JSON.
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
    """Write an interactive plotly HTML report of the packed tree."""
    if _go is None:
        die("error: --html requires plotly (pip install plotly)")
    if not leaves:
        return

    TOP_N = 12
    max_revs = max((p["revisions"] for p in leaves), default=1) or 1
    cmap = plt.get_cmap("YlOrRd")
    norm = plt.Normalize(vmin=0, vmax=max_revs)
    top_hot = sorted(leaves, key=lambda p: p["score"], reverse=True)[:TOP_N]
    hot_map = {p["path"]: i + 1 for i, p in enumerate(top_hot)}

    pack_tree_dispatch(tree)

    fig = _go.Figure()
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
            # Optional centre label if the circle is big enough
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
            hover_x.append(cx); hover_y.append(cy)
            hover_text.append(_hover_html(n.pkg, hot_map.get(n.pkg["path"])))
            return

        # Container
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

    # Invisible hover markers at leaf centres
    fig.add_trace(_go.Scatter(
        x=hover_x, y=hover_y, mode="markers",
        marker=dict(size=10, color="rgba(0,0,0,0)"),
        hovertext=hover_text, hoverinfo="text",
        showlegend=False, name="hotspots",
    ))

    # Title + subtitle stacked above the plot
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
    # A small footer caption explaining the legend
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
    except FileNotFoundError:
        die(f"error: baseline {path} does not exist")
    except json.JSONDecodeError as e:
        die(f"error: baseline {path} is not valid JSON — {e}")


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
# Shared printing
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
            w = csv.DictWriter(fh, fieldnames=fields); w.writeheader()
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
            w = csv.DictWriter(fh, fieldnames=fields); w.writeheader()
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
# Main
# --------------------------------------------------------------------------

def main() -> None:
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
                    help="interactive Plotly HTML report path. When plotly "
                         "is installed the HTML is auto-written next to "
                         "--out unless --no-html is set.")
    ap.add_argument("--no-html", action="store_true",
                    help="skip the interactive HTML even if plotly is "
                         "installed")
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
    ap.add_argument("--include-tests", action="store_true")
    ap.add_argument("--ignore-dead-files", action="store_true",
                    help="[deprecated; on by default] kept as a no-op for "
                         "backward compatibility")
    ap.add_argument("--keep-dead-files", action="store_true",
                    help="keep commits whose only affected files no longer "
                         "exist on disk (opt out of the accurate default)")
    ap.add_argument("--exclude", action="append", default=None,
                    metavar="REGEX",
                    help="exclude .go files matching this regex (repeatable). "
                         "Added on top of the auto-applied defaults for "
                         "generated files.")
    ap.add_argument("--no-default-excludes", action="store_true",
                    help=f"don't auto-exclude common generated Go files. "
                         f"Defaults: {', '.join(DEFAULT_EXCLUDES)}")
    args = ap.parse_args()

    scope = args.directory.resolve()
    if not scope.exists():
        die(f"error: {scope} does not exist")
    if not scope.is_dir() and not (scope.is_file() and scope.suffix == ".go"):
        die(f"error: {scope} must be a directory or a .go file")

    if args.since:
        args.since = normalize_date("--since", args.since)
    if args.until:
        args.until = normalize_date("--until", args.until)
    if args.fail_on_new_hotspot and args.baseline is None:
        die("error: --fail-on-new-hotspot requires --baseline")
    if args.fail_on_worsening_pct is not None and args.baseline is None:
        die("error: --fail-on-worsening-pct requires --baseline")

    # Excludes: defaults + user additions, unless opted out. We pass the
    # merged list to filters so the JSON report records exactly what ran.
    effective_excludes: list[str] = []
    if not args.no_default_excludes:
        effective_excludes.extend(DEFAULT_EXCLUDES)
    if args.exclude:
        effective_excludes.extend(args.exclude)
    args.exclude = effective_excludes  # report surface shows merged list
    exclude_regex = compile_excludes(effective_excludes)

    # Dead-file handling is on by default (accurate); keep flag for opt-out.
    ignore_dead_files = not args.keep_dead_files
    args.ignore_dead_files = ignore_dead_files  # for JSON report surface

    # HTML is auto-emitted next to the PNG when plotly is available and the
    # user didn't ask to skip it.
    if (args.html is None and _go is not None and not args.no_html
            and not args.no_image):
        args.html = args.out.with_suffix(".html")

    base = scope if scope.is_dir() else scope.parent
    git_root = find_git_root(base)
    module_info = find_go_module(base)
    mode = detect_mode(scope, args.mode)

    leaves: list[dict]
    tree: Node
    commit_meta: dict[str, dict]
    total_files: int
    total_commits: int
    root_display: str

    # ---- collect leaves + tree (mode-specific) ----
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
            args.since, args.until, exclude_regex,
            root_display,
        )
        if not leaves:
            die("error: no Go functions found in the given scope")
        total_files = 1 if scope.is_file() else len({
            p["file_path"] for p in leaves if p.get("file_path")
        })
        total_commits = len(commit_meta)
    else:
        if not scope.is_dir():
            die(f"error: {scope} is a file; --mode package needs "
                     "a directory.")
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
            die(
                f"error: no commits touched any .go file in {scope} "
                f"({' / '.join(bits)}).\n"
                "  The window is empty — widen the date range or check "
                "the dates.")
        leaves = collect_leaf_packages(
            scope, git_root, commits_per_file, args.include_tests,
            module_info, exclude_regex=exclude_regex)
        if not leaves:
            die("error: no Go packages found under the given directory")
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

    # ---- rank + enrich ----
    leaves.sort(key=lambda p: p["score"], reverse=True)
    enrich_leaves(leaves, commit_meta)

    # ---- text output ----
    if not args.quiet:
        print_text_report(leaves, mode)

    # ---- CSV ----
    if args.csv:
        write_csv(leaves, mode, args.csv)
        if not args.quiet:
            print(f"wrote {args.csv}")

    # ---- report ----
    thresholds_spec = {
        "fail_on_score": args.fail_on_score,
        "fail_on_new_hotspot": bool(args.fail_on_new_hotspot),
        "fail_on_worsening_pct": args.fail_on_worsening_pct,
    }
    report = build_report(
        leaves, args, scope, mode, module_info,
        thresholds_spec, total_files, total_commits, args.top_n,
    )

    # ---- baseline diff ----
    if args.baseline is not None:
        baseline = load_baseline(args.baseline)
        report["baseline_delta"] = compare_to_baseline(report, baseline)

    # ---- JSON ----
    if args.json is not None:
        args.json.write_text(json.dumps(report, indent=2) + "\n")
        if not args.quiet:
            print(f"wrote {args.json}")

    # ---- title/subtitle (shared by PNG and HTML) ----
    window_bits = []
    if args.since:
        window_bits.append(f"since {args.since}")
    if args.until:
        window_bits.append(f"until {args.until}")
    title_since = (" " + " / ".join(window_bits)) if window_bits else ""
    top = leaves[0]
    if mode == "function":
        subtitle = (
            f"{len(leaves)} functions · Σ {total_commits} commits     "
            f"top hotspot: {top['name']} "
            f"({top['revisions']} revs × {top['loc']} LOC)")
        title = f"Function hotspots in {root_display}{title_since}"
    else:
        subtitle = (
            f"{len(leaves)} packages · {total_files} files · "
            f"{total_commits} commits     "
            f"top hotspot: {top.get('import_path') or top['rel_to_scope']} "
            f"({top['revisions']} revs × {top['loc']} LOC)")
        title = f"Go hotspots in {root_display}{title_since}"

    if not args.no_image:
        plot_hotspots(leaves, tree, args.out, title, subtitle)
        if not args.quiet:
            print(f"wrote {args.out}")

    if args.html is not None:
        emit_html(leaves, tree, args.html, title, subtitle)
        if not args.quiet:
            print(f"wrote {args.html}")

    # ---- thresholds / exit code ----
    exit_code, msgs = apply_thresholds(report, args)
    for m in msgs:
        print(m, file=sys.stderr)
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
