#!/usr/bin/env python3
import os
import re
import sys
import json
import math
import time
import fnmatch
import argparse
import subprocess
import numpy as np
import networkx as nx
from itertools import combinations

# Default walk-exclude set. DO NOT add 'assets' here — the frontend
# source lives under assets/js and participates in the strongest
# intra-module coupling relationships in the repo (chat Alpine stores,
# locale files, templ-adjacent JS). Excluding assets blinded the
# earlier coupling reports entirely; keep it in scope.
_DEFAULT_WALK_EXCLUDES = {
    '.git', 'node_modules', 'vendor', 'bin', '.venv',
    '__pycache__', 'dist', 'build',
}


def build_graph(root_path, exclude_dirs=None):
    if exclude_dirs is None:
        exclude_dirs = _DEFAULT_WALK_EXCLUDES
    
    abs_root = os.path.abspath(root_path)
    
    G = nx.Graph()
    G.add_node(abs_root)
    
    def add_node(parent, path):
        G.add_node(path)
        if parent != path:
            G.add_edge(parent, path)
        
        try:
            entries = sorted(os.listdir(path))
        except PermissionError:
            return
        
        for entry in entries:
            if entry.startswith('.') or entry in exclude_dirs:
                continue
            
            full_path = os.path.join(path, entry)
            
            if os.path.isdir(full_path):
                add_node(path, full_path)
            else:
                G.add_node(full_path)
                G.add_edge(path, full_path)
    
    add_node(abs_root, abs_root)
    return G, abs_root

def get_current_files(root_path, exclude_dirs=None):
    if exclude_dirs is None:
        exclude_dirs = _DEFAULT_WALK_EXCLUDES
    
    files = []
    for root, dirs, filenames in os.walk(root_path):
        dirs[:] = [d for d in dirs if d not in exclude_dirs and not d.startswith('.')]
        for f in filenames:
            if not f.startswith('.'):
                rel_path = os.path.relpath(os.path.join(root, f), root_path)
                files.append(rel_path)
    return set(files)

def _find_git_root(abs_root):
    if os.path.exists(os.path.join(abs_root, ".git")):
        return abs_root, ""
    parent = os.path.dirname(abs_root)
    while parent and not os.path.exists(os.path.join(parent, ".git")):
        old_parent = parent
        parent = os.path.dirname(parent)
        if parent == old_parent:
            return None, None
    subdir = os.path.relpath(abs_root, parent)
    return parent, "" if subdir == "." else subdir

def _strip_subdir(path, subdir):
    if path.startswith("./"):
        path = path[2:]
    if not subdir:
        return path
    prefix = subdir + "/"
    return path[len(prefix):] if path.startswith(prefix) else None

def _resolve_rename(name, rename_map):
    seen = set()
    while name in rename_map and name not in seen:
        seen.add(name)
        name = rename_map[name]
    return name

def _matches_any(path, patterns):
    return any(fnmatch.fnmatch(path, p) for p in patterns)

def get_commits(root_path, since=None, max_commit_files=50, exclude_patterns=None):
    """Returns [(timestamp, [canonical_files])]. Skips merges, detects renames,
       drops oversized commits, applies file-pattern exclusions."""
    exclude_patterns = exclude_patterns or []
    abs_root = os.path.abspath(root_path)
    git_root, subdir = _find_git_root(abs_root)
    if git_root is None:
        print("Error: No git repository found")
        return [], abs_root

    cmd = ["git", "-C", git_root, "log", "--no-merges", "-M",
           "--pretty=format:__COMMIT__ %ct", "--name-status"]
    if since:
        cmd.insert(-1, f"--since={since}")

    result = subprocess.run(cmd, capture_output=True, text=True)

    commits = []
    rename_map = {}
    cur_ts = None
    cur_files = []

    def flush():
        if cur_ts is None or not cur_files:
            return
        unique = list({f for f in cur_files})
        if max_commit_files and len(unique) > max_commit_files:
            return
        commits.append((cur_ts, unique))

    for line in result.stdout.split("\n"):
        if not line:
            continue
        if line.startswith("__COMMIT__ "):
            flush()
            cur_ts = int(line[len("__COMMIT__ "):].strip())
            cur_files = []
            continue
        parts = line.split("\t")
        if len(parts) < 2:
            continue
        status = parts[0]
        if status.startswith("R") and len(parts) >= 3:
            old, new = parts[1], parts[2]
            old_s = _strip_subdir(old, subdir)
            new_s = _strip_subdir(new, subdir)
            if old_s is None or new_s is None:
                continue
            canonical = _resolve_rename(new_s, rename_map)
            rename_map[old_s] = canonical
            if not _matches_any(canonical, exclude_patterns):
                cur_files.append(canonical)
        elif status[:1] in ("A", "M", "D", "T", "C"):
            f = _strip_subdir(parts[-1], subdir)
            if f is None:
                continue
            canonical = _resolve_rename(f, rename_map)
            if not _matches_any(canonical, exclude_patterns):
                cur_files.append(canonical)
    flush()

    return commits, abs_root

def calculate_distance_matrix(G, files):
    file_paths = [f["path"] for f in files]

    n = len(files)
    matrix = [[0] * n for _ in range(n)]
    
    for i, p1 in enumerate(file_paths):
        for j, p2 in enumerate(file_paths):
            if i >= j:
                continue
            if p1 not in G or p2 not in G:
                continue
            try:
                dist = nx.shortest_path_length(G, p1, p2)
                matrix[i][j] = dist
                matrix[j][i] = dist
            except nx.NetworkXNoPath:
                pass
    
    return matrix

def get_package(filepath):
    return os.path.dirname(filepath) or "."

_IMPORT_BLOCK_RE = re.compile(r'import\s*\((.*?)\)', re.DOTALL)
_IMPORT_LINE_RE = re.compile(r'^\s*import\s+(?:[_.]\s+|[A-Za-z_]\w*\s+)?"([^"]+)"', re.MULTILINE)
_IMPORT_ENTRY_RE = re.compile(r'(?:[_.]\s+|[A-Za-z_]\w*\s+)?"([^"]+)"')

def _read_go_module(root_path):
    gomod = os.path.join(root_path, "go.mod")
    if not os.path.exists(gomod):
        return None
    with open(gomod, "r", encoding="utf-8", errors="ignore") as f:
        for line in f:
            line = line.strip()
            if line.startswith("module "):
                return line.split(None, 1)[1].strip()
    return None

def _parse_go_imports(filepath):
    try:
        with open(filepath, "r", encoding="utf-8", errors="ignore") as f:
            content = f.read()
    except OSError:
        return []
    imports = []
    for m in _IMPORT_BLOCK_RE.finditer(content):
        for entry in _IMPORT_ENTRY_RE.finditer(m.group(1)):
            imports.append(entry.group(1))
    for m in _IMPORT_LINE_RE.finditer(content):
        imports.append(m.group(1))
    return imports

def build_structural_graph(root_path, abs_root):
    """Returns dict pkg -> set of pkgs it imports, using go.mod module path."""
    module = _read_go_module(abs_root)
    graph = {}
    if not module:
        return graph, module
    prefix = module + "/"
    for dirpath, dirnames, filenames in os.walk(abs_root):
        dirnames[:] = [d for d in dirnames if d not in
                       {'.git', 'node_modules', 'vendor', 'bin', '.venv', '__pycache__', 'assets'}
                       and not d.startswith('.')]
        for fn in filenames:
            if not fn.endswith(".go"):
                continue
            src = os.path.join(dirpath, fn)
            src_pkg = os.path.relpath(dirpath, abs_root) or "."
            for imp in _parse_go_imports(src):
                if imp == module:
                    tgt_pkg = "."
                elif imp.startswith(prefix):
                    tgt_pkg = imp[len(prefix):]
                else:
                    continue
                graph.setdefault(src_pkg, set()).add(tgt_pkg)
    return graph, module

def structurally_coupled(pkg_a, pkg_b, struct_graph):
    return (pkg_b in struct_graph.get(pkg_a, set())
            or pkg_a in struct_graph.get(pkg_b, set()))

def _commit_weight(ts, now_ts, half_life_days):
    if not half_life_days or half_life_days <= 0:
        return 1.0
    age_days = max(0.0, (now_ts - ts) / 86400.0)
    return 0.5 ** (age_days / half_life_days)

def build_cooccurrence_matrix(commits, files, half_life_days=0, now_ts=None,
                              cross_package_only=False):
    """Pair-wise temporal coupling matrix.

    Historically this function hard-skipped intra-package pairs
    (``get_package(f1) == get_package(f2)``) on the theory that
    *cross*-package coupling is the "interesting" signal. That's true
    for a classical layered application, but on this repo it throws
    away the strongest co-change relationships: siblings inside an
    Alpine component directory, the adkgo runner/workflow pair, two
    locale JSON files, the types.go + repo.go pair inside any
    agentdefinition-style module.

    The filter is now **opt-in** via ``cross_package_only=True``.
    Default behaviour keeps intra-package pairs so the strongest
    real couplings actually show up.
    """
    file_to_idx = {f["path"]: i for i, f in enumerate(files)}
    n = len(files)
    matrix = np.zeros((n, n), dtype=float)
    now_ts = now_ts if now_ts is not None else int(time.time())

    for ts, commit_files in commits:
        unique = {f for f in commit_files if f in file_to_idx}
        if len(unique) < 2:
            continue
        w = _commit_weight(ts, now_ts, half_life_days)
        for f1, f2 in combinations(unique, 2):
            if cross_package_only and get_package(f1) == get_package(f2):
                continue
            i, j = file_to_idx[f1], file_to_idx[f2]
            matrix[i, j] += w
            matrix[j, i] += w

    return matrix

def count_file_revisions(commits, file_set, half_life_days=0, now_ts=None):
    revs = {f: 0.0 for f in file_set}
    now_ts = now_ts if now_ts is not None else int(time.time())
    for ts, commit_files in commits:
        w = _commit_weight(ts, now_ts, half_life_days)
        for f in set(commit_files):
            if f in revs:
                revs[f] += w
    return revs

def package_coupling_from_commits(commits, file_to_pkg, min_revs, min_cooc,
                                  half_life_days=0, now_ts=None):
    pkg_revs = {}
    pkg_cooc = {}
    now_ts = now_ts if now_ts is not None else int(time.time())
    for ts, commit_files in commits:
        pkgs = {file_to_pkg[f] for f in commit_files if f in file_to_pkg}
        w = _commit_weight(ts, now_ts, half_life_days)
        for p in pkgs:
            pkg_revs[p] = pkg_revs.get(p, 0.0) + w
        for p1, p2 in combinations(sorted(pkgs), 2):
            pkg_cooc[(p1, p2)] = pkg_cooc.get((p1, p2), 0.0) + w

    results = []
    for (p1, p2), cooc in pkg_cooc.items():
        r1, r2 = pkg_revs[p1], pkg_revs[p2]
        if cooc < min_cooc:
            continue
        mn = min(r1, r2)
        if mn < min_revs:
            continue
        pct_min = cooc * 100.0 / mn
        pct_a = cooc * 100.0 / r1
        pct_b = cooc * 100.0 / r2
        jaccard = cooc * 100.0 / (r1 + r2 - cooc) if (r1 + r2 - cooc) > 0 else 0.0
        results.append((p1, p2, cooc, r1, r2, pct_min, pct_a, pct_b, jaccard))
    results.sort(key=lambda x: -x[8])
    return results

def aggregate_by_package(pairs):
    scores = {}
    cooccur = {}
    counts = {}
    for f1, f2, _dist, cooc, score in pairs:
        p1, p2 = get_package(f1), get_package(f2)
        if p1 == p2:
            continue
        key = (p1, p2) if p1 < p2 else (p2, p1)
        scores[key] = scores.get(key, 0) + score
        cooccur[key] = cooccur.get(key, 0) + cooc
        counts[key] = counts.get(key, 0) + 1

    results = [(k[0], k[1], cooccur[k], counts[k], scores[k]) for k in scores]
    results.sort(key=lambda x: -x[4])
    return results

def combine_matrices(tree_matrix, git_matrix, tree_files, git_files):
    tree_idx = {f: i for i, f in enumerate(tree_files)}
    git_idx = {f: i for i, f in enumerate(git_files)}

    common_files = sorted(set(tree_files) & set(git_files))

    tree_order = [tree_idx[f] for f in common_files]
    git_order = [git_idx[f] for f in common_files]
    
    tree_aligned = np.array(tree_matrix)[np.ix_(tree_order, tree_order)]
    git_aligned = np.array(git_matrix)[np.ix_(git_order, git_order)]
    
    combined = tree_aligned * git_aligned
    
    return common_files, combined, tree_aligned, git_aligned

def render_toplevel_graph(toplevel_pairs, output_path, title_suffix=""):
    """Minimal circular diagram for top-level package coupling — 1 node per group."""
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    from matplotlib.lines import Line2D
    from matplotlib.patches import FancyArrowPatch

    if not toplevel_pairs:
        print(f"No top-level pairs to visualize; skipping {output_path}")
        return

    pkgs = set()
    revs = {}
    for p1, p2, _cooc, r1, r2, _pm, _pa, _pb, _jac, _st in toplevel_pairs:
        pkgs.update([p1, p2])
        revs[p1] = max(revs.get(p1, 0), r1)
        revs[p2] = max(revs.get(p2, 0), r2)

    pkgs = sorted(pkgs, key=lambda p: -revs.get(p, 0))
    n = len(pkgs)
    R = 1.0
    angles = {p: 2 * math.pi * i / n - math.pi / 2 for i, p in enumerate(pkgs)}
    pos = {p: (R * math.cos(angles[p]), R * math.sin(angles[p])) for p in pkgs}

    palette = ["#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd",
               "#8c564b", "#e377c2", "#17becf", "#bcbd22", "#7f7f7f"]
    color_for = {p: palette[i % len(palette)] for i, p in enumerate(pkgs)}

    fig, ax = plt.subplots(figsize=(14, 14), dpi=160)
    fig.patch.set_facecolor("white")
    ax.set_facecolor("#fcfcfc")

    max_jac = max(p[8] for p in toplevel_pairs)

    for p1, p2, _cooc, _r1, _r2, _pm, _pa, _pb, jac, struct in toplevel_pairs:
        color = "#3d6bd0" if struct else "#d8452e"
        alpha = 0.5 if struct else 0.88
        w = 1.2 + 8.0 * (jac / max_jac)
        arrow = FancyArrowPatch(
            pos[p1], pos[p2], arrowstyle="-",
            connectionstyle="arc3,rad=0.22",
            color=color, alpha=alpha, linewidth=w,
            capstyle="round", zorder=1 if struct else 2,
        )
        ax.add_patch(arrow)

    for p in pkgs:
        x, y = pos[p]
        r = max(revs.get(p, 1), 1)
        size = 800 + math.sqrt(r) * 90
        ax.scatter(x, y, s=size, color=color_for[p],
                   edgecolors="white", linewidths=3, zorder=3)

    for p in pkgs:
        a = angles[p]
        lx = 1.22 * math.cos(a)
        ly = 1.22 * math.sin(a)
        rot = math.degrees(a)
        if math.cos(a) < 0:
            rot += 180
            ha = "right"
        else:
            ha = "left"
        ax.text(lx, ly, p, fontsize=16, fontweight="bold",
                ha=ha, va="center",
                rotation=rot, rotation_mode="anchor",
                color=color_for[p], zorder=4)

    for p in pkgs:
        x, y = pos[p]
        ax.text(x, y, f"{int(revs.get(p, 0))}", fontsize=9,
                ha="center", va="center", color="white",
                fontweight="bold", zorder=4)

    edge_handles = [
        Line2D([], [], color="#3d6bd0", linewidth=4, label="structurally coupled (Go imports)"),
        Line2D([], [], color="#d8452e", linewidth=4, label="logical only (hidden coupling)"),
    ]
    ax.legend(handles=edge_handles, loc="lower center",
              bbox_to_anchor=(0.5, -0.04), fontsize=10,
              frameon=True, framealpha=0.95, ncol=2)

    ax.set_title(
        f"Project package coupling{title_suffix}\n"
        f"arc width ∝ Jaccard %   •   node size ∝ revisions   •   number inside = commits",
        fontsize=14, pad=20, color="#1a1a1a",
    )
    ax.set_xlim(-1.75, 1.75)
    ax.set_ylim(-1.75, 1.75)
    ax.set_aspect("equal")
    ax.set_xticks([])
    ax.set_yticks([])
    for s in ax.spines.values():
        s.set_visible(False)

    plt.tight_layout()
    plt.savefig(output_path, dpi=160, bbox_inches="tight", facecolor="white")
    plt.close(fig)
    print(f"Package coupling visualization saved to {output_path}")


def _build_hierarchy_tree(packages):
    """Build {parent_path: [child_paths]} for a set of package paths."""
    from collections import defaultdict
    tree = defaultdict(set)
    parent_of = {}
    for pkg in packages:
        parts = pkg.split("/")
        for i in range(len(parts)):
            child = "/".join(parts[:i + 1])
            parent = "/".join(parts[:i])
            tree[parent].add(child)
            parent_of[child] = parent
    return {k: sorted(v) for k, v in tree.items()}, parent_of

def _ordered_leaves(tree, root=""):
    """DFS traversal keeping siblings adjacent; returns ordered list of leaf packages."""
    leaves = []
    stack = [root]
    while stack:
        node = stack.pop()
        children = tree.get(node, [])
        if not children:
            if node:
                leaves.append(node)
        else:
            for child in reversed(children):
                stack.append(child)
    return leaves

def _heb_control_points(a, b, pos, parent_of, bundle=0.82):
    """Control points from a → LCA → b in the hierarchy tree, pulled toward a straight line by (1-bundle)."""
    def ancestors(x):
        chain = [x]
        while x in parent_of:
            x = parent_of[x]
            chain.append(x)
        return chain
    aa = ancestors(a)
    bb = ancestors(b)
    set_b = set(bb)
    lca = ""
    for node in aa:
        if node in set_b:
            lca = node
            break
    up = aa[:aa.index(lca) + 1]
    down = list(reversed(bb[:bb.index(lca)]))
    path = up + down
    pts = [pos[n] for n in path]
    if len(pts) < 2:
        return pts
    ax0, ay0 = pts[0]
    ax1, ay1 = pts[-1]
    bundled = [pts[0]]
    for i in range(1, len(pts) - 1):
        t = i / (len(pts) - 1)
        sx = ax0 + t * (ax1 - ax0)
        sy = ay0 + t * (ay1 - ay0)
        ox, oy = pts[i]
        bundled.append((ox * bundle + sx * (1 - bundle),
                        oy * bundle + sy * (1 - bundle)))
    bundled.append(pts[-1])
    return bundled

def _cluster_layout(G, top_level_fn, seed=42):
    """Two-level layout: top-level groups on a ring (ordered by cross-coupling),
       local spring layout inside each group, sized so clusters don't overlap."""
    from collections import defaultdict
    groups = defaultdict(list)
    for n in G.nodes:
        groups[top_level_fn(n)].append(n)

    top_levels = list(groups.keys())

    cross = defaultdict(float)
    for u, v, d in G.edges(data=True):
        tu, tv = top_level_fn(u), top_level_fn(v)
        if tu != tv:
            key = tuple(sorted((tu, tv)))
            cross[key] += d.get("jaccard", 1.0)

    if len(top_levels) > 1:
        remaining = set(top_levels)
        order = [max(remaining, key=lambda t: len(groups[t]))]
        remaining.remove(order[0])
        while remaining:
            last = order[-1]
            best = max(remaining,
                       key=lambda t: cross.get(tuple(sorted((last, t))), 0.0))
            order.append(best)
            remaining.remove(best)
    else:
        order = top_levels

    def inner_radius(n_nodes):
        return 0.45 + 0.22 * math.sqrt(max(n_nodes, 1))

    radii = {t: inner_radius(len(groups[t])) for t in order}
    max_r = max(radii.values()) if radii else 0.5
    n_groups = len(order)
    if n_groups <= 1:
        meta_radius = 0.0
    else:
        chord = 2 * max_r * 1.4
        meta_radius = max(chord / (2 * math.sin(math.pi / n_groups)), 2.0)

    meta_pos = {}
    for i, t in enumerate(order):
        if n_groups == 1:
            meta_pos[t] = (0.0, 0.0)
        else:
            a = -math.pi / 2 + 2 * math.pi * i / n_groups
            meta_pos[t] = (meta_radius * math.cos(a), meta_radius * math.sin(a))

    pos = {}
    for t, nodes in groups.items():
        cx, cy = meta_pos[t]
        if len(nodes) == 1:
            pos[nodes[0]] = (cx, cy)
            continue
        sub = G.subgraph(nodes)
        if sub.number_of_edges() > 0:
            sub_pos = nx.spring_layout(sub, seed=seed, k=1.3, iterations=300)
        else:
            sub_pos = nx.circular_layout(sub)

        sxs = [p[0] for p in sub_pos.values()]
        sys = [p[1] for p in sub_pos.values()]
        sub_span = max(max(sxs) - min(sxs), max(sys) - min(sys), 0.01)
        inner_scale = radii[t] / sub_span
        for n, (x, y) in sub_pos.items():
            pos[n] = (cx + x * inner_scale, cy + y * inner_scale)

    return pos, meta_pos

def render_package_graph(package_pairs, output_path, title_suffix=""):
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    import matplotlib.patches as mpatches
    from matplotlib.lines import Line2D

    if not package_pairs:
        print(f"No package pairs to visualize; skipping {output_path}")
        return

    packages = set()
    revs_for = {}
    for p1, p2, cooc, r1, r2, _pm, pa, pb, jac, struct in package_pairs:
        packages.update([p1, p2])
        revs_for[p1] = max(revs_for.get(p1, 0), r1)
        revs_for[p2] = max(revs_for.get(p2, 0), r2)

    tree, parent_of = _build_hierarchy_tree(packages)
    leaves = _ordered_leaves(tree)

    n = len(leaves)
    R = 1.0
    leaf_angle = {}
    leaf_pos = {}
    for i, pkg in enumerate(leaves):
        angle = 2 * math.pi * i / n - math.pi / 2
        leaf_angle[pkg] = angle
        leaf_pos[pkg] = (R * math.cos(angle), R * math.sin(angle))

    def descendants(node):
        if node not in tree:
            return [node] if node in leaf_pos else []
        result = []
        for child in tree[node]:
            result.extend(descendants(child))
        return result

    def depth_of(node):
        return node.count("/") + (0 if node == "" else 1)

    max_depth = max((depth_of(n) for n in parent_of.keys()), default=1)
    internal_pos = {"": (0.0, 0.0)}
    for node in parent_of.keys():
        if node in leaf_pos:
            continue
        ds = [d for d in descendants(node) if d in leaf_pos]
        if not ds:
            continue
        mx = sum(leaf_pos[d][0] for d in ds) / len(ds)
        my = sum(leaf_pos[d][1] for d in ds) / len(ds)
        pull = depth_of(node) / (max_depth + 1)
        internal_pos[node] = (mx * pull, my * pull)

    pos = {**internal_pos, **leaf_pos}

    def top_level(pkg):
        return pkg.split("/", 1)[0] if "/" in pkg else pkg

    top_levels = sorted({top_level(n) for n in leaves})
    palette = [
        "#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd",
        "#8c564b", "#e377c2", "#17becf", "#bcbd22", "#7f7f7f",
        "#393b79", "#637939", "#8c6d31", "#843c39", "#7b4173",
    ]
    color_for = {t: palette[i % len(palette)] for i, t in enumerate(top_levels)}

    fig, ax = plt.subplots(figsize=(18, 18), dpi=160)
    fig.patch.set_facecolor("white")
    ax.set_facecolor("#fafbfc")

    max_jac = max(p[8] for p in package_pairs) if package_pairs else 1.0

    struct_pairs = [p for p in package_pairs if p[9]]
    logical_pairs = [p for p in package_pairs if not p[9]]

    def _smooth_curve(points, n_samples=80):
        if len(points) < 3:
            xs = [p[0] for p in points]
            ys = [p[1] for p in points]
            return xs, ys
        try:
            import numpy as np
            from scipy.interpolate import splprep, splev
            xs = [p[0] for p in points]
            ys = [p[1] for p in points]
            k = min(3, len(points) - 1)
            tck, _ = splprep([xs, ys], s=0, k=k)
            u = np.linspace(0, 1, n_samples)
            sx, sy = splev(u, tck)
            return list(sx), list(sy)
        except Exception:
            return [p[0] for p in points], [p[1] for p in points]

    for group_pairs, color, base_alpha, z in [
        (struct_pairs, "#4a6fdc", 0.35, 1),
        (logical_pairs, "#d8452e", 0.85, 2),
    ]:
        for p in group_pairs:
            p1, p2, _cooc, _r1, _r2, _pm, _pa, _pb, jac, _st = p
            cps = _heb_control_points(p1, p2, pos, parent_of, bundle=0.82)
            sx, sy = _smooth_curve(cps)
            w = 0.5 + 3.4 * (jac / max_jac)
            ax.plot(sx, sy, color=color, alpha=base_alpha, linewidth=w,
                    solid_capstyle="round", zorder=z)

    for pkg in leaves:
        x, y = leaf_pos[pkg]
        revs = max(revs_for.get(pkg, 1), 1)
        size = 40 + math.sqrt(revs) * 12
        ax.scatter(x, y, s=size, color=color_for[top_level(pkg)],
                   edgecolors="white", linewidths=0.9, zorder=3)

    for pkg in leaves:
        a = leaf_angle[pkg]
        label_r = R * 1.04
        lx = label_r * math.cos(a)
        ly = label_r * math.sin(a)
        rot_deg = math.degrees(a)
        if math.cos(a) < 0:
            rot_deg += 180
            ha = "right"
        else:
            ha = "left"
        label = pkg
        if len(label) > 36:
            label = "…" + label[-35:]
        ax.text(lx, ly, label, fontsize=8,
                ha=ha, va="center",
                rotation=rot_deg, rotation_mode="anchor",
                color=color_for[top_level(pkg)], zorder=4)

    outer_label_r = R * 1.32
    from collections import defaultdict
    top_spans = defaultdict(list)
    for pkg in leaves:
        top_spans[top_level(pkg)].append(leaf_angle[pkg])
    for t, angles in top_spans.items():
        mid = sum(angles) / len(angles)
        lx = outer_label_r * math.cos(mid)
        ly = outer_label_r * math.sin(mid)
        ax.text(lx, ly, t, fontsize=14, fontweight="bold",
                ha="center", va="center",
                color=color_for[t], alpha=0.9, zorder=5)

    top_handles = [mpatches.Patch(color=color_for[t], label=t) for t in top_levels]
    edge_handles = [
        Line2D([], [], color="#4a6fdc", linewidth=3, label="structurally coupled"),
        Line2D([], [], color="#d8452e", linewidth=3, label="logical only (hidden)"),
    ]
    leg1 = ax.legend(handles=top_handles, loc="upper left",
                     bbox_to_anchor=(-0.02, 1.0), fontsize=9,
                     title="Top-level package", title_fontsize=10,
                     frameon=True, framealpha=0.95)
    ax.add_artist(leg1)
    ax.legend(handles=edge_handles, loc="upper right",
              bbox_to_anchor=(1.02, 1.0), fontsize=9,
              title="Edge type", title_fontsize=10,
              frameon=True, framealpha=0.95)

    ax.set_title(
        f"Package coupling{title_suffix} — hierarchical edge bundling\n"
        f"edge width ∝ Jaccard %   •   node size ∝ revisions",
        fontsize=13, pad=18, color="#1a1a1a",
    )
    ax.set_xlim(-1.5, 1.5)
    ax.set_ylim(-1.5, 1.5)
    ax.set_aspect("equal")
    ax.set_xticks([])
    ax.set_yticks([])
    for spine in ax.spines.values():
        spine.set_visible(False)

    plt.tight_layout()
    plt.savefig(output_path, dpi=160, bbox_inches="tight", facecolor="white")
    plt.close(fig)
    print(f"Package coupling visualization saved to {output_path}")


DEFAULT_EXCLUDES = [
    "go.sum", "**/go.sum",
    "*.pb.go", "**/*.pb.go",
    "*_templ.go", "**/*_templ.go",
    "*.gen.go", "**/*.gen.go",
    "package-lock.json", "**/package-lock.json",
    "yarn.lock", "**/yarn.lock",
    "pnpm-lock.yaml", "**/pnpm-lock.yaml",
]

def main():
    parser = argparse.ArgumentParser(description="Find top N coupled files in a git repository")
    parser.add_argument("directory", help="Git repository directory")
    parser.add_argument("-n", "--top", type=int, default=20, help="Number of top file pairs to return (default: 20)")
    parser.add_argument("-o", "--output", type=str, help="Output JSON file (optional)")
    parser.add_argument("--min-revs", type=int, default=5, help="Min(revs) floor for normalized ranking (default: 5)")
    parser.add_argument("--min-cooc", type=int, default=3, help="Absolute co-occurrence floor (default: 3)")
    parser.add_argument("--max-commit-files", type=int, default=50, help="Drop commits touching >N files (default: 50; 0 disables)")
    parser.add_argument("--since", type=str, default=None, help="git log --since (e.g. '6 months ago' or '2025-01-01')")
    parser.add_argument("--half-life-days", type=float, default=0.0, help="Recency decay half-life in days (0=no decay)")
    parser.add_argument("--exclude", action="append", default=[], help="Glob to exclude (repeatable)")
    parser.add_argument("--no-default-excludes", action="store_true", help="Disable built-in excludes (go.sum, *.pb.go, *_templ.go, ...)")
    parser.add_argument("--no-structural", action="store_true", help="Skip Go import structural overlay")
    parser.add_argument("--cross-package-only", action="store_true",
                        help="Drop intra-package co-change pairs. Off by "
                             "default — on this codebase the strongest "
                             "real coupling is intra-package (e.g. "
                             "chat-state.js ↔ chat-streaming.js). Set "
                             "only when you explicitly want a cross-"
                             "package architectural view.")
    parser.add_argument("--viz", type=str, default=None, help="Write package coupling graph PNG to this path")
    parser.add_argument("--viz-level", choices=["top", "leaf"], default="top",
                        help="top = one node per top-level group (clean, default); leaf = full hierarchical edge bundling")
    parser.add_argument("--viz-top", type=int, default=0, help="Limit edges in --viz to top N by Jaccard (0=all that pass filters)")
    parser.add_argument("-v", "--verbose", action="store_true", help="Print progress")
    
    args = parser.parse_args()

    root_path = args.directory
    exclude_patterns = list(args.exclude)
    if not args.no_default_excludes:
        exclude_patterns.extend(DEFAULT_EXCLUDES)

    if args.verbose:
        print("Building file tree...")
    G, abs_root = build_graph(root_path)

    if args.verbose:
        print("Getting current files...")
    current_files = get_current_files(root_path)
    tree_files = sorted(f for f in current_files if not _matches_any(f, exclude_patterns))

    if args.verbose:
        print(f"  {len(tree_files)} files after exclusions")

    if args.verbose:
        print("Fetching git history...")
    commits, _ = get_commits(root_path, since=args.since,
                             max_commit_files=args.max_commit_files,
                             exclude_patterns=exclude_patterns)
    if args.verbose:
        print(f"  {len(commits)} commits after filters (no-merges, size cap, --since)")

    now_ts = int(time.time())
    git_files = sorted(tree_files)

    tree_files_abs = [os.path.join(abs_root, f) for f in tree_files]
    tree_matrix = calculate_distance_matrix(G, [{"path": f} for f in tree_files_abs])

    if args.verbose:
        print("Building co-occurrence matrix...")
    git_matrix = build_cooccurrence_matrix(
        commits, [{"path": f} for f in git_files],
        half_life_days=args.half_life_days, now_ts=now_ts,
        cross_package_only=args.cross_package_only,
    )
    
    files, combined, tree_aligned, git_aligned = combine_matrices(
        tree_matrix, git_matrix, tree_files, git_files
    )

    struct_graph = {}
    if not args.no_structural:
        if args.verbose:
            print("Parsing Go imports for structural overlay...")
        struct_graph, module = build_structural_graph(root_path, abs_root)
        if args.verbose:
            print(f"  module={module}, packages with imports: {len(struct_graph)}")

    # File revisions are needed both to score pairs and to compute the
    # normalized view further down. Compute once, use twice.
    file_revs = count_file_revisions(commits, set(files),
                                     half_life_days=args.half_life_days, now_ts=now_ts)

    n = len(files)
    pairs = []
    for i in range(n):
        for j in range(i+1, n):
            cooc_w = float(git_aligned[i][j])
            if cooc_w <= 0:
                continue
            dist = int(tree_aligned[i][j])
            # Score prioritises *how often two files actually move
            # together* (cooc) and uses a Jaccard bonus as a tie-
            # breaker. The previous formula was ``cooc × tree_distance``
            # which rewarded pairs merely because they lived in
            # distant directories — on a monorepo that systematically
            # hides same-module coupling. Distance is preserved on
            # the tuple as a metadata column so older consumers that
            # read it still see the right value.
            r1 = file_revs.get(files[i], 0.0)
            r2 = file_revs.get(files[j], 0.0)
            union = r1 + r2 - cooc_w
            jaccard = (cooc_w / union) if union > 0 else 0.0
            score = cooc_w * (1.0 + jaccard)
            pairs.append((files[i], files[j], dist, cooc_w, score))

    pairs.sort(key=lambda x: -x[4])
    top_pairs = pairs[:args.top]

    package_pairs = aggregate_by_package(pairs)
    top_packages = package_pairs[:args.top]

    normalized_pairs = []
    for f1, f2, dist, cooc, _score in pairs:
        if cooc < args.min_cooc:
            continue
        r1, r2 = file_revs.get(f1, 0.0), file_revs.get(f2, 0.0)
        mn = min(r1, r2)
        if mn < args.min_revs:
            continue
        pct_min = cooc * 100.0 / mn
        pct_a = cooc * 100.0 / r1 if r1 else 0.0
        pct_b = cooc * 100.0 / r2 if r2 else 0.0
        jaccard = cooc * 100.0 / (r1 + r2 - cooc) if (r1 + r2 - cooc) > 0 else 0.0
        struct = structurally_coupled(get_package(f1), get_package(f2), struct_graph)
        normalized_pairs.append((f1, f2, dist, cooc, r1, r2, pct_min, pct_a, pct_b, jaccard, struct))
    normalized_pairs.sort(key=lambda x: -x[9])
    top_normalized = normalized_pairs[:args.top]

    file_to_pkg = {f: get_package(f) for f in files}
    normalized_package_pairs = package_coupling_from_commits(
        commits, file_to_pkg, args.min_revs, args.min_cooc,
        half_life_days=args.half_life_days, now_ts=now_ts,
    )
    normalized_package_pairs = [
        p + (structurally_coupled(p[0], p[1], struct_graph),)
        for p in normalized_package_pairs
    ]
    top_normalized_packages = normalized_package_pairs[:args.top]

    result = {
        "directory": root_path,
        "total_files": len(files),
        "total_commits": len(commits),
        "filters": {
            "min_revs": args.min_revs, "min_cooc": args.min_cooc,
            "max_commit_files": args.max_commit_files, "since": args.since,
            "half_life_days": args.half_life_days,
            "excludes": exclude_patterns,
        },
        "top_pairs": [{"file1": p[0], "file2": p[1], "distance": p[2], "cooccur": round(p[3], 2), "score": round(p[4], 2)} for p in top_pairs],
        "top_package_pairs": [{"pkg1": p[0], "pkg2": p[1], "cooccur": round(p[2], 2), "file_pairs": p[3], "score": round(p[4], 2)} for p in top_packages],
        "top_pairs_normalized": [
            {"file1": p[0], "file2": p[1], "distance": p[2], "cooccur": round(p[3], 2),
             "revs1": round(p[4], 2), "revs2": round(p[5], 2),
             "pct_min": round(p[6], 2), "pct_a_to_b": round(p[7], 2), "pct_b_to_a": round(p[8], 2),
             "jaccard": round(p[9], 2), "structural": p[10]}
            for p in top_normalized
        ],
        "top_package_pairs_normalized": [
            {"pkg1": p[0], "pkg2": p[1], "cooccur": round(p[2], 2),
             "revs1": round(p[3], 2), "revs2": round(p[4], 2),
             "pct_min": round(p[5], 2), "pct_a_to_b": round(p[6], 2), "pct_b_to_a": round(p[7], 2),
             "jaccard": round(p[8], 2), "structural": p[9]}
            for p in top_normalized_packages
        ],
    }

    print(f"\nTop {args.top} file pairs by (distance × co-occurrence):")
    print(f"{'Rank':<6} {'Dist':<6} {'CoOcc':<8} {'Score':<10} {'File 1':<28} {'File 2':<28}")
    print("-" * 100)
    for i, (f1, f2, dist, cooc, score) in enumerate(top_pairs, 1):
        print(f"{i:<6} {dist:<6} {cooc:<8.1f} {score:<10.1f} {f1[-28:]:<28} {f2[-28:]:<28}")

    print(f"\nTop {args.top} package pairs (sum of file-pair scores):")
    print(f"{'Rank':<6} {'Pairs':<8} {'CoOcc':<8} {'Score':<10} {'Package 1':<28} {'Package 2':<28}")
    print("-" * 100)
    for i, (p1, p2, cooc, npairs, score) in enumerate(top_packages, 1):
        print(f"{i:<6} {npairs:<8} {cooc:<8.1f} {score:<10.1f} {p1[-28:]:<28} {p2[-28:]:<28}")

    print(f"\nTop {args.top} file pairs (normalized; sorted by Jaccard; min_revs={args.min_revs}, min_cooc={args.min_cooc}):")
    print(f"{'Rank':<6} {'CoOcc':<7} {'RevA':<6} {'RevB':<6} {'Jacc':<7} {'A→B':<7} {'B→A':<7} {'S':<2} {'File 1':<24} {'File 2':<24}")
    print("-" * 120)
    for i, (f1, f2, _d, cooc, r1, r2, _pm, pa, pb, jac, struct) in enumerate(top_normalized, 1):
        s = "*" if struct else " "
        print(f"{i:<6} {cooc:<7.1f} {r1:<6.0f} {r2:<6.0f} {jac:<7.1f} {pa:<7.1f} {pb:<7.1f} {s:<2} {f1[-24:]:<24} {f2[-24:]:<24}")

    print(f"\nTop {args.top} package pairs (normalized; sorted by Jaccard; min_revs={args.min_revs}, min_cooc={args.min_cooc}):")
    print(f"{'Rank':<6} {'CoOcc':<7} {'RevA':<6} {'RevB':<6} {'Jacc':<7} {'A→B':<7} {'B→A':<7} {'S':<2} {'Package 1':<24} {'Package 2':<24}")
    print("-" * 120)
    for i, (p1, p2, cooc, r1, r2, _pm, pa, pb, jac, struct) in enumerate(top_normalized_packages, 1):
        s = "*" if struct else " "
        print(f"{i:<6} {cooc:<7.1f} {r1:<6.0f} {r2:<6.0f} {jac:<7.1f} {pa:<7.1f} {pb:<7.1f} {s:<2} {p1[-24:]:<24} {p2[-24:]:<24}")
    print("\n  S column: '*' = packages also structurally coupled via Go imports")

    if args.viz:
        def top_of(pkg):
            return pkg.split("/", 1)[0] if "/" in pkg else pkg

        if args.viz_level == "top":
            file_to_toplevel = {f: top_of(get_package(f)) for f in files}
            toplevel_pairs = package_coupling_from_commits(
                commits, file_to_toplevel, args.min_revs, args.min_cooc,
                half_life_days=args.half_life_days, now_ts=now_ts,
            )
            toplevel_struct = set()
            for src, dests in struct_graph.items():
                for dest in dests:
                    a, b = top_of(src), top_of(dest)
                    if a != b:
                        toplevel_struct.add(tuple(sorted((a, b))))
            toplevel_pairs = [
                p + ((tuple(sorted((p[0], p[1]))) in toplevel_struct),)
                for p in toplevel_pairs
            ]
            if args.viz_top and len(toplevel_pairs) > args.viz_top:
                toplevel_pairs = toplevel_pairs[:args.viz_top]
            render_toplevel_graph(toplevel_pairs, args.viz)
        else:
            viz_edges = normalized_package_pairs
            if args.viz_top and len(viz_edges) > args.viz_top:
                viz_edges = viz_edges[:args.viz_top]
            suffix = f" (top {args.viz_top} by Jaccard)" if args.viz_top else ""
            render_package_graph(viz_edges, args.viz, title_suffix=suffix)

    if args.output:
        with open(args.output, "w") as f:
            json.dump(result, f, indent=2)
        print(f"\nResults saved to {args.output}")
    
    return result

if __name__ == "__main__":
    main()