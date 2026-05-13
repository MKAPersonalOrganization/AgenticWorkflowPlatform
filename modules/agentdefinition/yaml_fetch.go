package agentdefinition

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
	u, err := validateImportURL(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	req, err := buildYAMLFetchRequest(ctx, u)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching url: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}
	return readCappedBody(resp.Body)
}

// validateImportURL parses the caller-supplied URL and runs every pre-
// fetch SSRF check: non-empty + scheme allowlist + hostname allowlist +
// DNS-resolved address check. Returns the parsed URL ready for the
// subsequent request build.
func validateImportURL(ctx context.Context, rawURL string) (*url.URL, error) {
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
	if isBlockedImportHost(host) {
		return nil, fmt.Errorf("this host is not allowed for URL import")
	}
	if err := rejectPrivateURLHost(ctx, host); err != nil {
		return nil, err
	}
	return u, nil
}

// isBlockedImportHost returns true for hostnames that bypass DNS and
// always point at the local machine or a local-network mDNS peer.
func isBlockedImportHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return strings.HasSuffix(host, ".local")
}

// buildYAMLFetchRequest constructs the GET with the UA + Accept headers
// every import request carries.
func buildYAMLFetchRequest(ctx context.Context, u *url.URL) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SchoolWithAI-AgentDesigner/1.0")
	req.Header.Set("Accept", "application/yaml, text/yaml, text/plain, */*")
	return req, nil
}

// readCappedBody reads up to maxImportYAMLBytes from body, returning an
// error when the response exceeds the cap (the +1 byte is the overflow
// sentinel — anything over means the source was too large).
func readCappedBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxImportYAMLBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImportYAMLBytes {
		return nil, fmt.Errorf("response body exceeds maximum size (%d bytes)", maxImportYAMLBytes)
	}
	return data, nil
}

// rejectPrivateURLHost blocks import URLs whose host either literally is
// a private/loopback IP or resolves to one via DNS. Any matching address
// in the resolved set fails the whole host — no "first-public-IP wins"
// semantics.
func rejectPrivateURLHost(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		if isDisallowedIP(ip) {
			return fmt.Errorf("url resolves to a disallowed address")
		}
		return nil
	}
	ips, err := (&net.Resolver{}).LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolving host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("host resolved to no addresses")
	}
	for _, ipa := range ips {
		if isDisallowedIP(ipa.IP) {
			return fmt.Errorf("url resolves to a disallowed address")
		}
	}
	return nil
}

// isDisallowedIP is the shared predicate for the literal-host path and
// the DNS-resolved path: loopback, RFC1918 private, or link-local.
// InterfaceLocalMulticast is included for the literal-IP arm only in the
// original code; we fold it in here for parity across both paths.
func isDisallowedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsInterfaceLocalMulticast()
}
