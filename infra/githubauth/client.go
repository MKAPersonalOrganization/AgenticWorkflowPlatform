package githubauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

// GitHub OAuth2 endpoints.
var githubEndpoint = oauth2.Endpoint{
	AuthURL:  "https://github.com/login/oauth/authorize",
	TokenURL: "https://github.com/login/oauth/access_token",
}

// GitHubUser represents the authenticated GitHub user profile.
type GitHubUser struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`
}

// GitHubRepo represents a repository returned by the GitHub API.
type GitHubRepo struct {
	FullName      string `json:"full_name"`
	Name          string `json:"name"`
	Owner         string `json:"owner_login"` // populated manually
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	Description   string `json:"description"`
	HTMLURL       string `json:"html_url"`
}

// ghRepoRaw is the raw JSON shape returned by the GitHub repos API.
type ghRepoRaw struct {
	FullName      string `json:"full_name"`
	Name          string `json:"name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	Description   string `json:"description"`
	HTMLURL       string `json:"html_url"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// Client wraps GitHub OAuth2 functionality.
type Client struct {
	oauthConfig *oauth2.Config
	httpClient  *http.Client
}

// NewClient creates a new GitHub OAuth2 client.
func NewClient(cfg Config) *Client {
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       []string{"read:user", "repo"},
		Endpoint:     githubEndpoint,
	}

	return &Client{
		oauthConfig: oauthConfig,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// AuthCodeURL returns the URL for the GitHub OAuth2 consent page.
func (c *Client) AuthCodeURL(state string) string {
	return c.oauthConfig.AuthCodeURL(state)
}

// Exchange exchanges an authorization code for a token.
func (c *Client) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return c.oauthConfig.Exchange(ctx, code)
}

// FetchUser retrieves the authenticated GitHub user profile.
func (c *Client) FetchUser(ctx context.Context, token *oauth2.Token) (*GitHubUser, error) {
	client := c.oauthConfig.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("githubauth: fetch user: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("githubauth: read user response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("githubauth: user API returned status %d: %s", resp.StatusCode, string(body))
	}

	var user GitHubUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("githubauth: decode user: %w", err)
	}

	return &user, nil
}

// ListRepos lists repositories accessible by the authenticated user.
// Returns up to 100 repos sorted by most recently pushed.
func (c *Client) ListRepos(ctx context.Context, accessToken string) ([]GitHubRepo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/user/repos?per_page=100&sort=pushed&affiliation=owner,collaborator,organization_member", nil)
	if err != nil {
		return nil, fmt.Errorf("githubauth: list repos request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("githubauth: list repos: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("githubauth: repos API returned status %d: %s", resp.StatusCode, string(body))
	}

	var raw []ghRepoRaw
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("githubauth: decode repos: %w", err)
	}

	repos := make([]GitHubRepo, len(raw))
	for i, r := range raw {
		repos[i] = GitHubRepo{
			FullName:      r.FullName,
			Name:          r.Name,
			Owner:         r.Owner.Login,
			Private:       r.Private,
			DefaultBranch: r.DefaultBranch,
			Description:   r.Description,
			HTMLURL:       r.HTMLURL,
		}
	}

	return repos, nil
}

// Configured returns true when GitHub OAuth credentials are present.
func (c *Client) Configured() bool {
	return c.oauthConfig.ClientID != "" && c.oauthConfig.ClientSecret != ""
}
