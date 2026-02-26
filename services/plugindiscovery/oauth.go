package plugindiscovery

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthService handles the OAuth 2.1 Authorization Code + PKCE flow for MCP servers.
type OAuthService struct {
	callbackURL string // e.g. https://app.example.com/v1/mcp/oauth/callback
	httpClient  *http.Client
}

// NewOAuthService creates a new OAuth service.
func NewOAuthService(callbackURL string) *OAuthService {
	return &OAuthService{
		callbackURL: callbackURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// OAuthMetadata holds the discovered OAuth server metadata (RFC 8414 / RFC 9728).
type OAuthMetadata struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint,omitempty"`
	RevocationEndpoint    string   `json:"revocation_endpoint,omitempty"`
	ScopesSupported       []string `json:"scopes_supported,omitempty"`
}

// ProtectedResourceMetadata is the RFC 9728 metadata from the MCP server itself.
type ProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers,omitempty"`
	ScopesSupported      []string `json:"scopes_supported,omitempty"`
}

// OAuthTokenResponse holds the token endpoint response.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// OAuthState holds transient state during the OAuth flow.
type OAuthState struct {
	ConnectorID  string `json:"connector_id"`
	State        string `json:"state"`
	CodeVerifier string `json:"code_verifier"`
}

// DiscoverProtectedResourceMetadata fetches RFC 9728 metadata from the MCP server.
func (o *OAuthService) DiscoverProtectedResourceMetadata(ctx context.Context, serverURL string) (*ProtectedResourceMetadata, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parsing server URL: %w", err)
	}

	metaURL := fmt.Sprintf("%s://%s/.well-known/oauth-protected-resource", parsed.Scheme, parsed.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating metadata request: %w", err)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching protected resource metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("protected resource metadata returned status %d", resp.StatusCode)
	}

	var meta ProtectedResourceMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("decoding protected resource metadata: %w", err)
	}
	return &meta, nil
}

// DiscoverAuthServerMetadata fetches OAuth Authorization Server metadata (RFC 8414).
func (o *OAuthService) DiscoverAuthServerMetadata(ctx context.Context, authServerURL string) (*OAuthMetadata, error) {
	parsed, err := url.Parse(authServerURL)
	if err != nil {
		return nil, fmt.Errorf("parsing auth server URL: %w", err)
	}

	metaURL := fmt.Sprintf("%s://%s/.well-known/oauth-authorization-server", parsed.Scheme, parsed.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating auth metadata request: %w", err)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching auth server metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth server metadata returned status %d", resp.StatusCode)
	}

	var meta OAuthMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("decoding auth server metadata: %w", err)
	}
	return &meta, nil
}

// RegisterClient performs Dynamic Client Registration (RFC 7591).
func (o *OAuthService) RegisterClient(ctx context.Context, registrationEndpoint, serverName string) (clientID, clientSecret string, err error) {
	body := map[string]any{
		"client_name":                "SchoolWithAI - " + serverName,
		"redirect_uris":              []string{o.callbackURL},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "client_secret_post",
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", "", fmt.Errorf("marshalling registration body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registrationEndpoint, strings.NewReader(string(jsonBody)))
	if err != nil {
		return "", "", fmt.Errorf("creating registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("performing client registration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("client registration failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("decoding registration response: %w", err)
	}
	return result.ClientID, result.ClientSecret, nil
}

// BuildAuthorizationURL constructs the authorization URL with PKCE.
func (o *OAuthService) BuildAuthorizationURL(meta *OAuthMetadata, clientID, state, codeChallenge string, scopes []string) string {
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {o.callbackURL},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	if len(scopes) > 0 {
		params.Set("scope", strings.Join(scopes, " "))
	}
	return meta.AuthorizationEndpoint + "?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens.
func (o *OAuthService) ExchangeCode(ctx context.Context, meta *OAuthMetadata, clientID, clientSecret, code, codeVerifier string) (*OAuthTokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {o.callbackURL},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchanging code for token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}
	return &tokenResp, nil
}

// RefreshToken refreshes an expired access token.
func (o *OAuthService) RefreshToken(ctx context.Context, meta *OAuthMetadata, clientID, clientSecret, refreshToken string) (*OAuthTokenResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	}
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding refresh response: %w", err)
	}
	return &tokenResp, nil
}

// GenerateState creates a cryptographically random state parameter.
func GenerateState() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random state: %s", err))
	}
	return hex.EncodeToString(b)
}

// GenerateCodeVerifier creates a PKCE code verifier.
func GenerateCodeVerifier() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate code verifier: %s", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// CodeChallenge computes the S256 code challenge from a verifier.
func CodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// BearerTokenTransport is an http.RoundTripper that injects a Bearer token.
type BearerTokenTransport struct {
	Token string
	Base  http.RoundTripper
}

// RoundTrip adds the Authorization header.
func (t *BearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+t.Token)
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req2)
}
