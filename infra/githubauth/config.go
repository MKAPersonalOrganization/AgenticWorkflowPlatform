// Package githubauth provides GitHub OAuth2 authentication for connecting
// user GitHub accounts (not for login – login uses Google OAuth).
package githubauth

// Config holds configuration for GitHub OAuth2 connected accounts.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // e.g., "https://localhost:8090/api/github/callback"
}
