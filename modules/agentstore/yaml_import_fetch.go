package agentstore

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxImportYAMLBytes = 1 << 20 // 1 MiB

// fetchYAMLFromURL performs a GET with basic SSRF protections (scheme allowlist,
// block loopback/private/link-local resolved addresses, size cap).
func fetchYAMLFromURL(ctx context.Context, rawURL string) ([]byte, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is empty")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("only http and https URLs are allowed")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("invalid host")
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasSuffix(host, ".local") {
		return nil, fmt.Errorf("this host is not allowed for URL import")
	}
	if err := rejectPrivateURLHost(ctx, host); err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SchoolWithAI-AgentDesigner/1.0")
	req.Header.Set("Accept", "application/yaml, text/yaml, text/plain, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching url: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImportYAMLBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImportYAMLBytes {
		return nil, fmt.Errorf("response body exceeds maximum size (%d bytes)", maxImportYAMLBytes)
	}
	return data, nil
}

func rejectPrivateURLHost(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsInterfaceLocalMulticast() {
			return fmt.Errorf("url resolves to a disallowed address")
		}
		return nil
	}
	r := &net.Resolver{}
	ips, err := r.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolving host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("host resolved to no addresses")
	}
	for _, ipa := range ips {
		ip := ipa.IP
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return fmt.Errorf("url resolves to a disallowed address")
		}
	}
	return nil
}
