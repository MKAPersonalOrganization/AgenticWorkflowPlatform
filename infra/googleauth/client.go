package googleauth

import (
	"aicompass/foundation/auth"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleUserInfo represents the response from Google's userinfo API.
type GoogleUserInfo struct {
	Sub        string `json:"sub"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Picture    string `json:"picture"`
	Verified   bool   `json:"email_verified"`
}

// Client wraps Google OAuth2 functionality and user persistence.
type Client struct {
	oauthConfig *oauth2.Config
	db          *sqlx.DB
}

// NewClient creates a new Google OAuth2 client.
func NewClient(cfg Config, db *sqlx.DB) *Client {
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes: []string{
			"openid",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &Client{
		oauthConfig: oauthConfig,
		db:          db,
	}
}

// AuthCodeURL returns the URL for the Google OAuth2 consent page.
func (c *Client) AuthCodeURL(state string) string {
	return c.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// Exchange exchanges an authorization code for a token.
func (c *Client) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return c.oauthConfig.Exchange(ctx, code)
}

// FetchUserInfo retrieves the user's Google profile using the OAuth2 token.
func (c *Client) FetchUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error) {
	client := c.oauthConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read user info response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("google userinfo returned status %d: %s", resp.StatusCode, string(body))
	}

	var info GoogleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &info, nil
}

// UpsertUser creates or updates a user in the database from Google profile data.
// Returns the user (created or existing).
func (c *Client) UpsertUser(ctx context.Context, info *GoogleUserInfo) (*auth.User, error) {
	now := time.Now().UTC()

	query := `
		INSERT INTO users (id, email, google_sub, first_name, last_name, picture, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (email) DO UPDATE SET
			google_sub = EXCLUDED.google_sub,
			first_name = EXCLUDED.first_name,
			last_name  = EXCLUDED.last_name,
			picture    = EXCLUDED.picture,
			updated_at = EXCLUDED.updated_at
		RETURNING id, email, google_sub, first_name, last_name, picture, role, created_at, updated_at
	`

	var user auth.User
	err := c.db.QueryRowxContext(ctx, query,
		uuid.New().String(),
		info.Email,
		info.Sub,
		info.GivenName,
		info.FamilyName,
		info.Picture,
		"user",
		now,
		now,
	).StructScan(&user)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert user: %w", err)
	}

	return &user, nil
}

// UpsertDebugUser ensures a user with the given ID and email exists. It is
// used by the dev-login path (POST /auth/dev-login) so integration tests can
// obtain a real session cookie for a well-known user without going through
// Google OAuth. Unlike UpsertUser, the caller chooses the ID up-front —
// that's the whole point: tests want a stable, predictable debug user that
// survives cluster restarts and can be referenced by env vars.
//
// On a fresh DB this row is inserted. On subsequent calls the row is left
// intact except for updated_at, so test runs don't fight each other over
// ownership of the user record.
func (c *Client) UpsertDebugUser(ctx context.Context, userID, email string) (*auth.User, error) {
	now := time.Now().UTC()

	query := `
		INSERT INTO users (id, email, google_sub, first_name, last_name, picture, role, created_at, updated_at)
		VALUES ($1, $2, '', 'Debug', 'User', '', 'user', $3, $3)
		ON CONFLICT (id) DO UPDATE SET
			updated_at = EXCLUDED.updated_at
		RETURNING id, email, google_sub, first_name, last_name, picture, role, created_at, updated_at
	`

	var user auth.User
	err := c.db.QueryRowxContext(ctx, query, userID, email, now).StructScan(&user)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert debug user: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by their ID.
func (c *Client) GetUserByID(ctx context.Context, userID string) (*auth.User, error) {
	var user auth.User
	err := c.db.QueryRowxContext(ctx,
		`SELECT id, email, google_sub, first_name, last_name, picture, role, created_at, updated_at
		 FROM users WHERE id = $1`, userID).StructScan(&user)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by email.
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*auth.User, error) {
	var user auth.User
	err := c.db.QueryRowxContext(ctx,
		`SELECT id, email, google_sub, first_name, last_name, picture, role, created_at, updated_at
		 FROM users WHERE email = $1`, email).StructScan(&user)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}
