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
	"sync"
	"time"
)

const (
	// maxMetadataResponseBytes limits the size of OAuth metadata discovery
	// responses to prevent unbounded memory consumption from malicious servers.
	maxMetadataResponseBytes = 1 << 20 // 1 MiB

	// maxTokenResponseBytes limits the size of token endpoint responses.
	maxTokenResponseBytes = 256 << 10 // 256 KiB

	// oauthHTTPTimeout is the default timeout for all OAuth HTTP requests.
	oauthHTTPTimeout = 30 * time.Second

	// metadataCacheTTL is the TTL for cached OAuth metadata discovery results.
	// Matches agentstack's @alru_cache(ttl=600) for metadata endpoints.
	metadataCacheTTL = 10 * time.Minute

	// registrationCacheTTL is the TTL for cached dynamic client registrations.
	// Matches agentstack's @alru_cache(ttl=86400) for client registration.
	registrationCacheTTL = 24 * time.Hour
)

// OAuthService handles the OAuth 2.1 Authorization Code + PKCE flow for MCP servers.
type OAuthService struct {
	callbackURL string // e.g. https://app.example.com/v1/mcp/oauth/callback
	httpClient  *http.Client

	// In-memory TTL caches to avoid redundant network round-trips when
	// multiple connectors share the same auth server (agentstack pattern).
	resourceMetaCache *ttlCache[*ProtectedResourceMetadata]
	authMetaCache     *ttlCache[*OAuthMetadata]
	registrationCache *ttlCache[clientRegistration]
}

// clientRegistration holds the result of a dynamic client registration.
type clientRegistration struct {
	ClientID     string
	ClientSecret string
}

// NewOAuthService creates a new OAuth service.
func NewOAuthService(callbackURL string) *OAuthService {
	return &OAuthService{
		callbackURL: callbackURL,
		httpClient: &http.Client{
			Timeout: oauthHTTPTimeout,
		},
		resourceMetaCache: newTTLCache[*ProtectedResourceMetadata](metadataCacheTTL),
		authMetaCache:     newTTLCache[*OAuthMetadata](metadataCacheTTL),
		registrationCache: newTTLCache[clientRegistration](registrationCacheTTL),
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

// DiscoverProtectedResourceMetadata fetches RFC 9728 metadata from the MCP server.
// Results are cached for 10 minutes (agentstack pattern) to avoid redundant
// round-trips when multiple connectors point to the same server.
func (o *OAuthService) DiscoverProtectedResourceMetadata(ctx context.Context, serverURL string) (*ProtectedResourceMetadata, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parsing server URL: %w", err)
	}

	cacheKey := parsed.Scheme + "://" + parsed.Host
	if cached, ok := o.resourceMetaCache.Get(cacheKey); ok {
		return cached, nil
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
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMetadataResponseBytes)).Decode(&meta); err != nil {
		return nil, fmt.Errorf("decoding protected resource metadata: %w", err)
	}
	o.resourceMetaCache.Set(cacheKey, &meta)
	return &meta, nil
}

// DiscoverAuthServerMetadata fetches OAuth Authorization Server metadata (RFC 8414).
// Results are cached for 10 minutes (agentstack pattern).
func (o *OAuthService) DiscoverAuthServerMetadata(ctx context.Context, authServerURL string) (*OAuthMetadata, error) {
	parsed, err := url.Parse(authServerURL)
	if err != nil {
		return nil, fmt.Errorf("parsing auth server URL: %w", err)
	}

	cacheKey := parsed.Scheme + "://" + parsed.Host
	if cached, ok := o.authMetaCache.Get(cacheKey); ok {
		return cached, nil
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
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMetadataResponseBytes)).Decode(&meta); err != nil {
		return nil, fmt.Errorf("decoding auth server metadata: %w", err)
	}
	o.authMetaCache.Set(cacheKey, &meta)
	return &meta, nil
}

// RegisterClient performs Dynamic Client Registration (RFC 7591).
// RegisterClient performs Dynamic Client Registration (RFC 7591).
// Results are cached for 24 hours keyed by registration endpoint (agentstack pattern).
func (o *OAuthService) RegisterClient(ctx context.Context, registrationEndpoint, serverName string) (clientID, clientSecret string, err error) {
	if cached, ok := o.registrationCache.Get(registrationEndpoint); ok {
		return cached.ClientID, cached.ClientSecret, nil
	}

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
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes))
		return "", "", fmt.Errorf("client registration failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxTokenResponseBytes)).Decode(&result); err != nil {
		return "", "", fmt.Errorf("decoding registration response: %w", err)
	}
	o.registrationCache.Set(registrationEndpoint, clientRegistration{
		ClientID:     result.ClientID,
		ClientSecret: result.ClientSecret,
	})
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes))
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxTokenResponseBytes)).Decode(&tokenResp); err != nil {
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes))
		return nil, fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxTokenResponseBytes)).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding refresh response: %w", err)
	}
	return &tokenResp, nil
}

// RevokeToken revokes an access token at the authorization server's revocation
// endpoint (RFC 7009). Errors are returned but callers should treat revocation
// as best-effort — the token is cleared locally regardless.
func (o *OAuthService) RevokeToken(ctx context.Context, meta *OAuthMetadata, accessToken string) error {
	if meta.RevocationEndpoint == "" {
		return fmt.Errorf("authorization server does not support token revocation")
	}

	data := url.Values{
		"token":           {accessToken},
		"token_type_hint": {"access_token"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.RevocationEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("creating revocation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoking token: %w", err)
	}
	defer resp.Body.Close()

	// RFC 7009: the server returns 200 even if the token was already invalid.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes))
		return fmt.Errorf("token revocation failed (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
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

// TokenRefreshCallback is invoked after a successful token refresh so the
// caller can persist the new tokens (e.g. update the DB row).
type TokenRefreshCallback func(newToken *OAuthTokenResponse) error

// AutoRefreshTransport is an http.RoundTripper that automatically refreshes
// the OAuth token on 401 responses. Modelled after agentstack's
// auto_refresh_client pattern.
type AutoRefreshTransport struct {
	mu           sync.Mutex
	oauth        *OAuthService
	meta         *OAuthMetadata
	clientID     string
	clientSecret string
	token        OAuthTokenResponse
	expiresAt    time.Time // computed from token.ExpiresIn at creation
	onRefresh    TokenRefreshCallback
	base         http.RoundTripper
}

// NewAutoRefreshTransport creates a transport that injects the Bearer token and
// transparently refreshes it when the server responds with 401.
func NewAutoRefreshTransport(
	oauth *OAuthService,
	meta *OAuthMetadata,
	clientID, clientSecret string,
	token OAuthTokenResponse,
	onRefresh TokenRefreshCallback,
) *AutoRefreshTransport {
	var expiresAt time.Time
	if token.ExpiresIn > 0 {
		// Subtract a small buffer to refresh proactively.
		expiresAt = time.Now().Add(time.Duration(token.ExpiresIn)*time.Second - 30*time.Second)
	}
	return &AutoRefreshTransport{
		oauth:        oauth,
		meta:         meta,
		clientID:     clientID,
		clientSecret: clientSecret,
		token:        token,
		expiresAt:    expiresAt,
		onRefresh:    onRefresh,
		base:         http.DefaultTransport,
	}
}

// RoundTrip implements http.RoundTripper with automatic token refresh.
func (t *AutoRefreshTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Proactively refresh if the token is about to expire.
	if !t.expiresAt.IsZero() && time.Now().After(t.expiresAt) && t.token.RefreshToken != "" {
		_ = t.refreshToken(req.Context())
	}

	// Inject the current token.
	req2 := req.Clone(req.Context())
	t.mu.Lock()
	req2.Header.Set("Authorization", "Bearer "+t.token.AccessToken)
	t.mu.Unlock()

	resp, err := t.base.RoundTrip(req2)
	if err != nil {
		return nil, err
	}

	// If 401 and we have a refresh token, try once.
	if resp.StatusCode == http.StatusUnauthorized && t.token.RefreshToken != "" {
		resp.Body.Close()
		if refreshErr := t.refreshToken(req.Context()); refreshErr != nil {
			// Return the original 401 — caller will see it as an auth error.
			return t.base.RoundTrip(req.Clone(req.Context()))
		}
		// Retry with the new token.
		retry := req.Clone(req.Context())
		t.mu.Lock()
		retry.Header.Set("Authorization", "Bearer "+t.token.AccessToken)
		t.mu.Unlock()
		return t.base.RoundTrip(retry)
	}

	return resp, nil
}

// refreshToken calls the OAuth token endpoint with the refresh_token grant.
func (t *AutoRefreshTransport) refreshToken(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	newToken, err := t.oauth.RefreshToken(ctx, t.meta, t.clientID, t.clientSecret, t.token.RefreshToken)
	if err != nil {
		return err
	}

	// Preserve the refresh token if the server didn't issue a new one.
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = t.token.RefreshToken
	}

	t.token = *newToken
	if newToken.ExpiresIn > 0 {
		t.expiresAt = time.Now().Add(time.Duration(newToken.ExpiresIn)*time.Second - 30*time.Second)
	}

	// Notify the caller so it can persist the new token.
	if t.onRefresh != nil {
		_ = t.onRefresh(newToken)
	}
	return nil
}
