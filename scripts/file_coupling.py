#!/usr/bin/env python3
import os
import sys
import json
import argparse
import numpy as np
import networkx as nx
from itertools import combinations

def build_graph(root_path, exclude_dirs=None):
    if exclude_dirs is None:
        exclude_dirs = {'.git', 'node_modules', 'vendor', 'bin', '.venv', '__pycache__', 'assets'}
    
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
        exclude_dirs = {'.git', 'node_modules', 'vendor', 'bin', '.venv', '__pycache__', 'assets'}
    
    files = []
    for root, dirs, filenames in os.walk(root_path):
        dirs[:] = [d for d in dirs if d not in exclude_dirs and not d.startswith('.')]
        for f in filenames:
            if not f.startswith('.'):
                rel_path = os.path.relpath(os.path.join(root, f), root_path)
                files.append(rel_path)
    return set(files)

def get_git_files_by_commit(root_path):
    import subprocess
    
    abs_root = os.path.abspath(root_path)
    cwd = os.getcwd()
    
    if os.path.exists(os.path.join(abs_root, ".git")):
        git_root = abs_root
        subdir = ""
    else:
        parent = os.path.dirname(abs_root)
        while parent and not os.path.exists(os.path.join(parent, ".git")):
            old_parent = parent
            parent = os.path.dirname(parent)
            if parent == old_parent:
                print("Error: No git repository found")
                return [], ""
        
        if parent and os.path.exists(os.path.join(parent, ".git")):
            git_root = parent
            subdir = os.path.relpath(abs_root, git_root)
            if subdir == ".":
                subdir = ""
        else:
            print("Error: No git repository found")
            return [], ""
    
    if subdir == abs_root:
        subdir = ""
    
    os.chdir(git_root)
    
    result = subprocess.run(
        ["git", "log", "--pretty=format:%n", "--name-only"],
        capture_output=True,
        text=True
    )
    
    commits = []
    files = []
    
    for line in result.stdout.split('\n'):
        line = line.strip()
        if not line:
            if files:
                commits.append(files)
                files = []
        else:
            if line.startswith("./"):
                line = line[2:]
            if subdir and line.startswith(subdir + "/"):
                line = line[len(subdir) + 1:]
                files.append(line)
            elif not subdir:
                files.append(line)
    
    if files:
        commits.append(files)
    
    os.chdir(cwd)
    
    return commits, abs_root

def calculate_distance_matrix(G, files):
    file_paths = [f["path"] for f in files]
    valid_paths = [p for p in file_paths if p in G]
    
    valid_to_idx = {p: i for i, p in enumerate(file_paths)}
    
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
    parts = filepath.split('/')
    return parts[0] if parts else ""

def build_cooccurrence_matrix(commits, files):
    file_to_idx = {f["path"]: i for i, f in enumerate(files)}
    n = len(files)
    matrix = [[0] * n for _ in range(n)]
    
    for files_set in commits:
        unique_files = set(files_set)
        filtered = [f for f in unique_files if f in file_to_idx]
        
        if len(filtered) >= 2:
            for f1, f2 in combinations(filtered, 2):
                if get_package(f1) == get_package(f2):
                    continue
                i, j = file_to_idx[f1], file_to_idx[f2]
                matrix[i][j] += 1
                matrix[j][i] += 1
    
    return matrix

def combine_matrices(tree_matrix, git_matrix, tree_files, git_files):
    common = tree_files & git_files
    
    tree_idx = {f: i for i, f in enumerate(tree_files)}
    git_idx = {f: i for i, f in enumerate(git_files)}
    
    common_files = sorted(common)
    n = len(common_files)
    
    tree_order = [tree_idx[f] for f in common_files]
    git_order = [git_idx[f] for f in common_files]
    
    tree_aligned = np.array(tree_matrix)[np.ix_(tree_order, tree_order)]
    git_aligned = np.array(git_matrix)[np.ix_(git_order, git_order)]
    
    combined = tree_aligned * git_aligned
    
    return common_files, combined, tree_aligned, git_aligned

def main():
    parser = argparse.ArgumentParser(description="Find top N coupled files in a git repository")
    parser.add_argument("directory", help="Git repository directory")
    parser.add_argument("-n", "--top", type=int, default=20, help="Number of top file pairs to return (default: 20)")
    parser.add_argument("-o", "--output", type=str, help="Output JSON file (optional)")
    parser.add_argument("-v", "--verbose", action="store_true", help="Print progress")
    
    args = parser.parse_args()
    
    root_path = args.directory
    
    if args.verbose:
        print("Building file tree...")
    G, abs_root = build_graph(root_path)
    
    if args.verbose:
        print("Getting current files...")
    current_files = get_current_files(root_path)
    
    tree_files = sorted([os.path.relpath(f, os.getcwd()) for f in current_files])
    
    if args.verbose:
        print(f"  {len(tree_files)} files in tree")
        print(f"  Sample: {tree_files[:3]}")
    
    if args.verbose:
        print("Fetching git history...")
    commits, git_root = get_git_files_by_commit(root_path)
    
    if args.verbose:
        print(f"  {len(commits)} commits")
    
    git_files = sorted(tree_files)
    
    if args.verbose:
        print("Calculating distance matrix...")
        print(f"  Tree files sample: {tree_files[:3]}")
    
    tree_files_abs = [os.path.join(abs_root, f) for f in tree_files]
    if args.verbose:
        print(f"  tree_files_abs sample: {tree_files_abs[:3]}")
    
    tree_matrix = calculate_distance_matrix(G, [{"path": f} for f in tree_files_abs])
    
    if args.verbose:
        print("Building co-occurrence matrix...")
        print(f"  Git files: {git_files[:3]}")
    git_matrix = build_cooccurrence_matrix(commits, [{"path": f} for f in git_files])
    
    if args.verbose:
        print(f"  Git commits: {len(commits)}")
        print(f"  Git files: {len(git_files)}")
        print(f"  Non-zero in git matrix: {np.count_nonzero(np.array(git_matrix))}")
    
    if args.verbose:
        print("Combining matrices...")
    files, combined, tree_aligned, git_aligned = combine_matrices(tree_matrix, git_matrix, set(tree_files), set(git_files))
    
    if args.verbose:
        print(f"  {len(files)} common files")
    
    n = len(files)
    pairs = []
    for i in range(n):
        for j in range(i+1, n):
            if combined[i][j] > 0 and get_package(files[i]) != get_package(files[j]):
                pairs.append((files[i], files[j], int(tree_aligned[i][j]), int(git_aligned[i][j]), int(combined[i][j])))
    
    pairs.sort(key=lambda x: -x[4])
    top_pairs = pairs[:args.top]
    
    result = {
        "directory": root_path,
        "total_files": len(files),
        "total_commits": len(commits),
        "top_pairs": [{"file1": p[0], "file2": p[1], "distance": p[2], "cooccur": p[3], "score": p[4]} for p in top_pairs]
    }
    
    print(f"\nTop {args.top} file pairs by (distance × co-occurrence):")
    print(f"{'Rank':<6} {'Dist':<6} {'CoOcc':<8} {'Score':<8} {'File 1':<28} {'File 2':<28}")
    print("-" * 100)
    for i, (f1, f2, dist, cooc, score) in enumerate(top_pairs, 1):
        print(f"{i:<6} {dist:<6} {cooc:<8} {score:<8} {f1[-28:]:<28} {f2[-28:]:<28}")
    
    if args.output:
        with open(args.output, "w") as f:
            json.dump(result, f, indent=2)
        print(f"\nResults saved to {args.output}")
    
    return result

if __name__ == "__main__":
    main()