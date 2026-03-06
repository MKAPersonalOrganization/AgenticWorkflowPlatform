package mcp

import "strings"

// ConnectorPreset is a pre-configured template for common MCP servers.
// Matches agentstack's connector preset system — users can create connectors
// with just a URL and the platform auto-fills credentials and metadata.
type ConnectorPreset struct {
	URL          string `json:"url"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
}

// builtinPresets are the default connector presets shipped with the platform.
// Matches agentstack's built-in preset catalog (GitHub, Stripe, Box).
// Additional presets can be configured via environment variables at deploy time.
var builtinPresets = []ConnectorPreset{
	{
		URL:         "https://api.githubcopilot.com/mcp",
		Name:        "GitHub",
		Description: "Access and interact with your GitHub repositories and code intelligence",
	},
	{
		URL:         "https://mcp.stripe.com",
		Name:        "Stripe",
		Description: "Payment processing and financial infrastructure tools",
	},
	{
		URL:         "https://mcp.box.com",
		Name:        "Box",
		Description: "Search, access and get insights on your Box content",
	},
	{
		URL:         "https://mcp.linear.app/sse",
		Name:        "Linear",
		Description: "Project management and issue tracking tools",
	},
	{
		URL:         "https://mcp.notion.so/mcp",
		Name:        "Notion",
		Description: "Access and manage your Notion workspace, pages, and databases",
	},
}

// MatchPreset finds the best matching preset for a given server URL.
// Matching is done by URL prefix (scheme + host), following agentstack's
// match_preset=true default behavior.
func MatchPreset(serverURL string, presets []ConnectorPreset) *ConnectorPreset {
	serverURL = strings.TrimRight(serverURL, "/")
	for i, p := range presets {
		presetURL := strings.TrimRight(p.URL, "/")
		if strings.EqualFold(serverURL, presetURL) {
			return &presets[i]
		}
		// Also match by host prefix for variations (e.g. with/without path).
		if sameHost(serverURL, presetURL) {
			return &presets[i]
		}
	}
	return nil
}

// sameHost returns true if two URLs share the same scheme + host.
func sameHost(a, b string) bool {
	aHost := extractHost(a)
	bHost := extractHost(b)
	return aHost != "" && strings.EqualFold(aHost, bHost)
}

// extractHost extracts "scheme://host" from a URL.
func extractHost(rawURL string) string {
	// Find scheme separator.
	idx := strings.Index(rawURL, "://")
	if idx < 0 {
		return ""
	}
	rest := rawURL[idx+3:]
	// Find end of host (first slash after scheme).
	slashIdx := strings.Index(rest, "/")
	if slashIdx < 0 {
		return rawURL
	}
	return rawURL[:idx+3+slashIdx]
}
