package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"path"
	"strings"
	"time"
)

// GitHubVFS is a read-only VFS implementation backed by the GitHub REST API.
// It uses the Contents API (https://docs.github.com/en/rest/repos/contents) to
// list directories and read files without cloning.
type GitHubVFS struct {
	owner  string
	repo   string
	branch string
	token  string
	client *http.Client
}

// NewGitHubVFS creates a new read-only VFS for a GitHub repository.
func NewGitHubVFS(cfg GitHubConfig) *GitHubVFS {
	return &GitHubVFS{
		owner:  cfg.Owner,
		repo:   cfg.Repo,
		branch: cfg.Branch,
		token:  cfg.Token,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// ── GitHub API types ────────────────────────────────────────────────────────

type ghContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	Size        int    `json:"size"`
	DownloadURL string `json:"download_url"`
}

// ── helpers ─────────────────────────────────────────────────────────────────

func (v *GitHubVFS) apiURL(p string) string {
	p = strings.TrimPrefix(path.Clean(p), "/")
	base := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", v.owner, v.repo, p)
	if v.branch != "" {
		base += "?ref=" + v.branch
	}
	return base
}

// gitHubAPIHost is the fixed origin doGet will dial. Any request whose URL
// resolves to a different host is rejected so user-controlled paths cannot
// be turned into requests against arbitrary internal hosts (SSRF guard).
const gitHubAPIHost = "api.github.com"

func (v *GitHubVFS) doGet(ctx context.Context, rawURL string) (*http.Response, error) {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid github url: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host != gitHubAPIHost {
		return nil, fmt.Errorf("github url not allowed")
	}
	// Reconstruct the request URL from constants + validated components so
	// the URL object passed to the HTTP client is rooted in the literal
	// "https://api.github.com" origin rather than the caller-supplied string.
	safeURL := &neturl.URL{
		Scheme:   "https",
		Host:     gitHubAPIHost,
		Path:     parsed.Path,
		RawQuery: parsed.RawQuery,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, safeURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if v.token != "" {
		req.Header.Set("Authorization", "Bearer "+v.token)
	}
	return v.client.Do(req)
}

// ── VFS implementation (read-only) ──────────────────────────────────────────

func (v *GitHubVFS) Mkdir(_ context.Context, _ string) error {
	return ErrReadOnly
}

func (v *GitHubVFS) Rmdir(_ context.Context, _ string) error {
	return ErrReadOnly
}

func (v *GitHubVFS) ReadDir(ctx context.Context, p string) ([]DirEntry, error) {
	resp, err := v.doGet(ctx, v.apiURL(p))
	if err != nil {
		return nil, fmt.Errorf("githubvfs: readdir: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("githubvfs: readdir: status %d", resp.StatusCode)
	}

	var items []ghContent
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("githubvfs: readdir: decode: %w", err)
	}

	entries := make([]DirEntry, len(items))
	for i, item := range items {
		entries[i] = DirEntry{
			Name:  item.Name,
			IsDir: item.Type == "dir",
			Size:  int64(item.Size),
		}
	}
	return entries, nil
}

func (v *GitHubVFS) CreateFile(_ context.Context, _ string, _ io.Reader, _ string, _ int64) (*FileStat, error) {
	return nil, ErrReadOnly
}

func (v *GitHubVFS) ReadFile(ctx context.Context, p string) (io.ReadCloser, *FileStat, error) {
	resp, err := v.doGet(ctx, v.apiURL(p))
	if err != nil {
		return nil, nil, fmt.Errorf("githubvfs: readfile meta: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("githubvfs: readfile meta: status %d", resp.StatusCode)
	}

	var item ghContent
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, nil, fmt.Errorf("githubvfs: readfile meta: decode: %w", err)
	}

	if item.Type != "file" || item.DownloadURL == "" {
		return nil, nil, ErrNotFile
	}

	// Download raw content.
	dlResp, err := v.doGet(ctx, item.DownloadURL)
	if err != nil {
		return nil, nil, fmt.Errorf("githubvfs: readfile download: %w", err)
	}
	if dlResp.StatusCode != http.StatusOK {
		_ = dlResp.Body.Close()
		return nil, nil, fmt.Errorf("githubvfs: readfile download: status %d", dlResp.StatusCode)
	}

	stat := &FileStat{
		Name: item.Name,
		Size: int64(item.Size),
	}
	return dlResp.Body, stat, nil
}

func (v *GitHubVFS) Remove(_ context.Context, _ string) error {
	return ErrReadOnly
}

func (v *GitHubVFS) Stat(ctx context.Context, p string) (*FileStat, error) {
	p = normalizePath(p)
	if p == "/" {
		return &FileStat{Name: "/", IsDir: true}, nil
	}

	resp, err := v.doGet(ctx, v.apiURL(p))
	if err != nil {
		return nil, fmt.Errorf("githubvfs: stat: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	var item ghContent
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		// If decode fails, it's likely a directory (GitHub returns an array for dirs).
		return &FileStat{Name: path.Base(p), IsDir: true}, nil
	}

	return &FileStat{
		Name:  item.Name,
		IsDir: item.Type == "dir",
		Size:  int64(item.Size),
	}, nil
}

func (v *GitHubVFS) Rename(_ context.Context, _, _ string) error {
	return ErrReadOnly
}

// compile-time check
var _ VFS = (*GitHubVFS)(nil)
