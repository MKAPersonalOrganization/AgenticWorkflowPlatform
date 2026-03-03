// Package googleauth provides Google OAuth2 authentication.
package googleauth

// Config holds configuration for Google OAuth2 authentication.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // e.g., "https://localhost:8090/auth/callback"
	SessionKey   string // Secret key for gorilla/sessions cookie encryption
}
