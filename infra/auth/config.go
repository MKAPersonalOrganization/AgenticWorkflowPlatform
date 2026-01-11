package auth

// Config represents the authentication configuration.
type Config struct {
	AppURL         string
	KratosPublic   string
	KratosAdmin    string
	KratosBrowser  string
	AdminAPIToken  string
}
