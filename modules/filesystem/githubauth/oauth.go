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

var githubEndpoint = oauth2.Endpoint{
	AuthURL:  "https://github.com/login/oauth/authorize",
	TokenURL: "https://github.com/login/oauth/access_token",
}

type ghUser struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`
}

type ghRepo struct {
	FullName      string `json:"full_name"`
	Name          string `json:"name"`
	Owner         string `json:"owner_login"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	Description   string `json:"description"`
	HTMLURL       string `json:"html_url"`
}

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

type Client struct {
	oauthConfig *oauth2.Config
	httpClient  *http.Client
}

func NewClient(cfg GitHubOAuthConfig) *Client {
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

func (c *Client) AuthCodeURL(state string) string {
	return c.oauthConfig.AuthCodeURL(state)
}

func (c *Client) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return c.oauthConfig.Exchange(ctx, code)
}

func (c *Client) FetchUser(ctx context.Context, token *oauth2.Token) (*ghUser, error) {
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

	var user ghUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("githubauth: decode user: %w", err)
	}

	return &user, nil
}

func (c *Client) ListRepos(ctx context.Context, accessToken string) ([]ghRepo, error) {
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

	repos := make([]ghRepo, len(raw))
	for i, r := range raw {
		repos[i] = ghRepo{
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

func (c *Client) Configured() bool {
	return c.oauthConfig.ClientID != "" && c.oauthConfig.ClientSecret != ""
}
