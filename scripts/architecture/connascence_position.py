#!/usr/bin/env python3
"""
connascence_position.py — Connascence of Position detector for Go.

Implements the strongest static-connascence level from Khononov,
*Balancing Coupling in Software Design* (Ch. 6, "Connascence"), drawing
on Page-Jones's original 1996 taxonomy. The book says:

    "Connascence of position is the highest level of static
     connascence… callers must know how the method extracts values
     from the ``data`` array; this knowledge is shared between the
     method itself and its callers. Such a method signature makes it
     quite easy to commit an error and trigger invalid system
     behavior by inadvertently passing values in the wrong order."

The book lists three concrete shapes (Listings 6.6 / 6.7 / 6.8):

  1. Method accepting an ordered array where each slot has a meaning
     (``SendEmail(string[] data)`` with ``data[0]`` = from, ``data[1]``
     = to, …). In Go this is rare but appears as ``func f(args ...T)``
     patterns or functional-slice APIs.
  2. A signature with several positional arguments of the same type:
     ``void SendEmail(string from, string to, string subject, string
     body)`` — the names are there but the types give the compiler no
     protection against reordering.
  3. Methods returning unnamed tuples: ``(DateTime, DateTime)
     GetCurrentDateTime()`` — callers memorise which value is local
     and which is UTC.

For Go, we flag every exported top-level function/method whose
signature exhibits any of:

  - **positional_same_type**: two or more consecutive parameters
    share an identical type expression — "the signature with several
    positional arguments of the same type" from Listing 6.7. This is
    the book's primary concrete example and the most refactor-worthy.
  - **positional_wide**: four or more positional params total. The
    book doesn't give a threshold; Uncle Bob and connascence writers
    commonly cite ≥ 4 as the point where humans start mis-ordering
    arguments. Variadic tails (``args ...T``) count once, not per
    possible value.
  - **return_same_type**: a multi-value return with two or more
    adjacent same-type results — Listing 6.8's ``(DateTime,
    DateTime)`` pattern.
  - **return_wide**: three or more return values *including error*
    is a grey zone; three-plus *excluding error* is a strong signal
    the caller is memorising positions. We flag ≥ 3 non-error returns.

Not flagged:
  - Variadic parameters alone (``func f(xs ...int)``) — the caller
    splats with a slice and positional error is impossible.
  - Methods with no parameters, unexported functions (they're part of
    the package's internal surface; only exported signatures create
    cross-module connascence — see Ch. 6's discussion of modularity).
  - ``context.Context`` as the first parameter (standard Go idiom,
    adds no positional risk).

Refactor recipes (book Ch. 6 + Khononov's Ch. 11 remediation table):
  - positional_same_type / positional_wide → introduce an **options
    struct** (``SendEmailRequest`` with named fields) or a **builder**.
    Lifts the signature from Connascence of Position to Connascence of
    Name — the strongest static-connascence reduction available.
  - return_same_type / return_wide → return a named struct instead of
    an unnamed tuple. Same reduction.

Verdict tiers (per file):
  HIGH    ≥ 3 flagged signatures
  MEDIUM  2 flagged signatures
  LOW     1 flagged signature
  FINE    no positional-connascence signature

Schema: ``connascence-position/1``.

Usage:
    scripts/architecture/connascence_position.py --json out/cop.json
    scripts/architecture/connascence_position.py --paths ./modules/... --top 30
    scripts/architecture/connascence_position.py --wide-threshold 5 --json out.json
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path


try:
    import tree_sitter_go
    from tree_sitter import Language, Parser
except ImportError:
    print("connascence_position.py requires tree-sitter and tree-sitter-go.\n"
          "  pip install tree-sitter tree-sitter-go",
          file=sys.stderr)
    sys.exit(1)


SCHEMA_VERSION = "connascence-position/1"

_GO_LANGUAGE = Language(tree_sitter_go.language())
_PARSER = Parser(_GO_LANGUAGE)

# Excludes mirror other architecture scripts.
_EXCLUDE_PATTERNS = re.compile(
    r"(^|/)(vendor|node_modules|testdata|_tmp|books|\.hotspots|\.READ|scripts/out)/"
    r"|_test\.go$|_templ\.go$|\.pb\.go$|_mock\.go$|_mocks\.go$"
    r"|(?:^|/)zz_generated|bindata\.go$"
)

# First-parameter types we treat as idiomatic plumbing, not signals.
# context.Context leads almost every Go function; counting it as a
# positional param would produce noise on every exported method.
_IDIOMATIC_FIRST_PARAM_TYPES = {"context.Context", "Context"}


def _node_text(node, src: bytes) -> str:
    return src[node.start_byte:node.end_byte].decode("utf-8", errors="replace")


def _iter_descendants(node):
    yield node
    for child in node.children:
        yield from _iter_descendants(child)


def _type_text_of(param_node, src: bytes) -> str:
    """Return the parameter's type expression as a stripped string.

    Parameters in tree-sitter-go can be:
      - ``parameter_declaration`` (one or more names sharing a type)
      - ``variadic_parameter_declaration`` (``args ...T``)
    We pull the ``type`` field and normalise whitespace so the
    same-type-adjacency check compares like-for-like.
    """
    type_node = param_node.child_by_field_name("type")
    if type_node is None:
        return ""
    text = _node_text(type_node, src)
    return re.sub(r"\s+", " ", text).strip()


def _param_count_of(param_node) -> int:
    """Number of names bound in a single parameter_declaration.

    ``func f(a, b, c int)`` is a single parameter_declaration with
    three names sharing one type. For Connascence of Position each
    name is a distinct positional slot.
    """
    names = param_node.children_by_field_name("name")
    return max(1, len(names))


# -- Signature walkers -----------------------------------------------------

def _walk_parameter_list(plist_node, src: bytes):
    """Yield (param_count, type_text, is_variadic) per parameter group.

    We flatten ``a, b int`` into ``[(2, "int", False)]`` — callers
    that care about positional risk treat them as two slots of the
    same type.
    """
    if plist_node is None:
        return
    for child in plist_node.children:
        if child.type == "parameter_declaration":
            yield _param_count_of(child), _type_text_of(child, src), False
        elif child.type == "variadic_parameter_declaration":
            yield 1, _type_text_of(child, src), True


def _analyze_signature(func_node, src: bytes, is_method: bool,
                       wide_threshold: int,
                       return_wide_threshold: int) -> dict | None:
    """Return a finding dict when this signature trips any rule, else None.

    ``is_method`` is True for ``method_declaration`` nodes; we include
    them but skip their receiver from positional counting — the
    receiver is bound syntactically and carries no ordering risk.
    """
    name_node = func_node.child_by_field_name("name")
    if name_node is None:
        return None
    name = _node_text(name_node, src)
    if not name or not name[0].isupper():
        # Unexported — book Ch. 6 scope is cross-module connascence;
        # package-private functions stay in the same compilation unit.
        return None

    params_node = func_node.child_by_field_name("parameters")
    result_node = func_node.child_by_field_name("result")

    # Flatten parameters into an ordered [(type, is_variadic), ...]
    # list. Variadics contribute one slot (splat-with-slice can't be
    # reordered).
    params: list[tuple[str, bool]] = []
    for count, type_text, is_var in _walk_parameter_list(params_node, src):
        for _ in range(count):
            params.append((type_text, is_var))

    # Drop the idiomatic leading context.Context so it doesn't inflate
    # positional counts on every handler / service method.
    if params and params[0][0] in _IDIOMATIC_FIRST_PARAM_TYPES:
        params = params[1:]

    total_positional = len(params)
    variadic_tail = params and params[-1][1]
    if variadic_tail:
        # The last slot is a variadic; caller can't reorder *into* it,
        # only the non-variadic prefix has positional risk.
        positional_for_count = total_positional - 1
    else:
        positional_for_count = total_positional

    # Same-type adjacency test: ≥ 2 consecutive params share one type.
    max_run, run_type = _longest_same_type_run(params)
    positional_same_type = max_run >= 2

    # Wide-signature test: threshold on the non-variadic positional count.
    positional_wide = positional_for_count >= wide_threshold

    # Return-value analysis.
    returns = list(_walk_parameter_list(result_node, src)) if result_node is not None else []
    flat_returns: list[tuple[str, bool]] = []
    for count, type_text, is_var in returns:
        for _ in range(count):
            flat_returns.append((type_text, is_var))
    # For Go, a result clause like ``DateTime`` (single, no parens) is
    # represented differently — it may appear as a type node, not as a
    # parameter_list. Accept anything with len ≤ 1 as "single return".
    non_error_returns = [r for r in flat_returns if r[0] != "error"]
    return_same_type_run, return_run_type = _longest_same_type_run(flat_returns)
    return_same_type = return_same_type_run >= 2
    return_wide = len(non_error_returns) >= return_wide_threshold

    triggers: list[str] = []
    if positional_same_type:
        triggers.append("positional_same_type")
    if positional_wide:
        triggers.append("positional_wide")
    if return_same_type:
        triggers.append("return_same_type")
    if return_wide:
        triggers.append("return_wide")
    if not triggers:
        return None

    return {
        "name": name,
        "is_method": is_method,
        "line": func_node.start_point[0] + 1,
        "positional_count": positional_for_count,
        "has_variadic_tail": bool(variadic_tail),
        "param_types": [p[0] for p in params],
        "return_count": len(flat_returns),
        "return_types": [r[0] for r in flat_returns],
        "longest_same_type_run": max_run,
        "same_type_run_type": run_type if max_run >= 2 else None,
        "longest_return_run": return_same_type_run,
        "return_run_type": return_run_type if return_same_type_run >= 2 else None,
        "triggers": triggers,
    }


def _longest_same_type_run(items: list[tuple[str, bool]]) -> tuple[int, str]:
    """Return (longest_run_length, run_type) over adjacent matching types.

    Empty-string types (parse failures) don't contribute — treating
    them as equal would inflate counts on malformed signatures.
    """
    best = 0
    best_type = ""
    cur = 0
    cur_type = ""
    for t, _ in items:
        if t and t == cur_type:
            cur += 1
        else:
            cur = 1
            cur_type = t
        if t and cur > best:
            best = cur
            best_type = t
    return best, best_type


def analyze_file(path: Path, wide_threshold: int,
                 return_wide_threshold: int) -> dict:
    """Return {path, findings, verdict} for one Go file."""
    src = path.read_bytes()
    tree = _PARSER.parse(src)
    findings: list[dict] = []
    for node in _iter_descendants(tree.root_node):
        if node.type == "function_declaration":
            f = _analyze_signature(node, src, False, wide_threshold, return_wide_threshold)
        elif node.type == "method_declaration":
            f = _analyze_signature(node, src, True, wide_threshold, return_wide_threshold)
        else:
            continue
        if f is not None:
            findings.append(f)
    # File verdict tier.
    n = len(findings)
    if n >= 3:
        verdict = "HIGH"
    elif n == 2:
        verdict = "MEDIUM"
    elif n == 1:
        verdict = "LOW"
    else:
        verdict = "FINE"
    return {
        "path": str(path),
        "findings": findings,
        "finding_count": n,
        "verdict": verdict,
    }


# -- Walking + reporting ---------------------------------------------------

def list_go_files(root: Path) -> list[Path]:
    out: list[Path] = []
    for base, dirs, files in os.walk(root):
        rel = Path(base).relative_to(root).as_posix()
        rel = "" if rel == "." else rel + "/"
        dirs[:] = [d for d in dirs
                   if not _EXCLUDE_PATTERNS.search(rel + d + "/")]
        for name in files:
            if not name.endswith(".go"):
                continue
            p = rel + name
            if _EXCLUDE_PATTERNS.search(p):
                continue
            out.append(root / p)
    return out


def print_stdout(records: list[dict], top: int) -> None:
    flagged = [r for r in records if r["finding_count"] > 0]
    if not flagged:
        print("No Connascence-of-Position signatures found.")
        return

    total_findings = sum(r["finding_count"] for r in flagged)
    trigger_hist: Counter = Counter()
    for r in flagged:
        for f in r["findings"]:
            for t in f["triggers"]:
                trigger_hist[t] += 1
    verdict_hist = Counter(r["verdict"] for r in flagged)
    print(f"Flagged files: {len(flagged)}  ·  total findings: {total_findings}")
    print("Verdict distribution: " +
          "  ".join(f"{v}={verdict_hist[v]}" for v in ("HIGH", "MEDIUM", "LOW") if verdict_hist[v]))
    print("Trigger distribution: " +
          "  ".join(f"{k}={v}" for k, v in trigger_hist.most_common()))
    print()

    flagged.sort(key=lambda r: (-r["finding_count"], r["path"]))
    print(f"── Top {min(top, len(flagged))} files (most findings first) ──")
    for r in flagged[:top]:
        print(f"  {r['verdict']:<7}  {r['finding_count']:>2} finding(s)  {r['path']}")
        for f in r["findings"][:3]:
            triggers = ",".join(f["triggers"])
            print(f"      L{f['line']:>4} {f['name']}   params={f['positional_count']}  "
                  f"returns={f['return_count']}  triggers=[{triggers}]")


# -- Driver ----------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=__doc__.split("Usage:")[0].strip(),
    )
    ap.add_argument("--paths", nargs="*", default=None,
                    help="restrict scan to one or more directories (default: repo root)")
    ap.add_argument("--wide-threshold", type=int, default=4,
                    help="positional-param count at/above which a signature is flagged (default 4)")
    ap.add_argument("--return-wide-threshold", type=int, default=3,
                    help="non-error return-value count at/above which a signature is flagged (default 3)")
    ap.add_argument("--top", type=int, default=25,
                    help="how many files to print on stdout (default 25)")
    ap.add_argument("-o", "--output", "--json", dest="output",
                    help="write JSON report to this path")
    args = ap.parse_args()

    repo_root = Path.cwd()
    roots = [Path(p) for p in args.paths] if args.paths else [repo_root]
    all_files: list[Path] = []
    for root in roots:
        all_files.extend(list_go_files(root))

    records = [analyze_file(p, args.wide_threshold, args.return_wide_threshold)
               for p in all_files]
    records = [r for r in records if r["finding_count"] > 0]
    # Normalise paths to repo-relative for JSON consumers.
    for r in records:
        try:
            r["path"] = str(Path(r["path"]).resolve().relative_to(repo_root.resolve()))
        except ValueError:
            pass

    print_stdout(records, args.top)

    if args.output:
        trigger_hist: Counter = Counter()
        for r in records:
            for f in r["findings"]:
                for t in f["triggers"]:
                    trigger_hist[t] += 1
        report = {
            "schema": SCHEMA_VERSION,
            "tool": "connascence_position",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "filters": {
                "wide_threshold": args.wide_threshold,
                "return_wide_threshold": args.return_wide_threshold,
                "excludes": [_EXCLUDE_PATTERNS.pattern],
                "idiomatic_first_param_skips": sorted(_IDIOMATIC_FIRST_PARAM_TYPES),
            },
            "tiers": {
                "HIGH": "≥ 3 flagged signatures",
                "MEDIUM": "2 flagged signatures",
                "LOW": "1 flagged signature",
                "FINE": "no positional-connascence signature",
            },
            "stats": {
                "files_flagged": len(records),
                "total_findings": sum(r["finding_count"] for r in records),
                "verdict_counts": {v: sum(1 for r in records if r["verdict"] == v)
                                   for v in ("HIGH", "MEDIUM", "LOW")},
                "trigger_counts": dict(trigger_hist),
            },
            "all": records,
            "ranked": sorted(records, key=lambda r: (-r["finding_count"], r["path"])),
        }
        Path(args.output).write_text(json.dumps(report, indent=2))
        print(f"\nJSON report: {args.output}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
