package googleauth

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/i18n"
	"aicompass/foundation/logger"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// AuthHandlers contains handlers for authentication routes.
type AuthHandlers struct {
	log      *logger.Logger
	client   *Client
	sessions *SessionStore
	appURL   string

	// Dev-login config is zero-valued unless EnableDevLogin was called.
	// DevLoginHandler checks devLoginToken == "" and 404s if so — the
	// route should not be mounted in that case either, but this is a
	// second line of defence.
	devLoginToken  string
	devLoginUserID string
	devLoginEmail  string
}

// NewAuthHandlers creates new authentication handlers.
func NewAuthHandlers(log *logger.Logger, client *Client, sessions *SessionStore, appURL string) *AuthHandlers {
	return &AuthHandlers{
		log:      log,
		client:   client,
		sessions: sessions,
		appURL:   appURL,
	}
}

// EnableDevLogin configures the dev-login handler with the token integration
// tests must present, plus the user id/email that bearer is mapped to.
//
// Call this exactly when the caller also mounts POST /auth/dev-login — the
// expected pattern is:
//
//	if authHandlers.EnableDevLogin(token, userID, email) {
//	    r.Post("/auth/dev-login", authHandlers.DevLoginHandler)
//	}
//
// Leaving the handler without this call, or with an empty token, makes
// DevLoginHandler refuse every request — defensive in case the route is
// accidentally mounted unconditionally. EnableDevLogin returns true when
// dev-login is now active (token non-empty), so callers can branch on the
// return value to decide whether to mount the route.
func (h *AuthHandlers) EnableDevLogin(token, userID, email string) bool {
	h.devLoginToken = token
	h.devLoginUserID = userID
	h.devLoginEmail = email
	return token != ""
}

// DevLoginEnabled reports whether EnableDevLogin was called with a
// non-empty token. Route wiring uses this to decide whether to mount
// POST /auth/dev-login.
func (h *AuthHandlers) DevLoginEnabled() bool {
	return h.devLoginToken != ""
}

// LoginPage handles GET /auth/login — redirects directly to Google OAuth.
func (h *AuthHandlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already authenticated, redirect to chat
	if user := auth.GetUserFromContext(r.Context()); user != nil {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}

	// Build the Google OAuth URL with CSRF state
	returnTo := r.URL.Query().Get("return_to")
	state := generateState()

	// Store state and return_to in the session
	session, _ := h.sessions.Get(r)
	session.Values["oauth_state"] = state
	if returnTo != "" {
		session.Values["return_to"] = returnTo
	}
	_ = session.Save(r, w)

	googleURL := h.client.AuthCodeURL(state)
	http.Redirect(w, r, googleURL, http.StatusSeeOther)
}

// CallbackHandler handles GET /auth/callback — the Google OAuth2 callback.
func (h *AuthHandlers) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify CSRF state
	session, err := h.sessions.Get(r)
	if err != nil {
		h.log.Error(ctx, "failed to get session", "error", err)
		http.Redirect(w, r, "/auth/error?error="+url.QueryEscape("Session error"), http.StatusSeeOther)
		return
	}

	expectedState, _ := session.Values["oauth_state"].(string)
	actualState := r.URL.Query().Get("state")
	if expectedState == "" || actualState != expectedState {
		h.log.Error(ctx, "OAuth state mismatch", "expected", expectedState, "actual", actualState)
		http.Redirect(w, r, "/auth/error?error="+url.QueryEscape("Invalid OAuth state"), http.StatusSeeOther)
		return
	}

	// Clean up state
	delete(session.Values, "oauth_state")
	returnTo, _ := session.Values["return_to"].(string)
	delete(session.Values, "return_to")
	_ = session.Save(r, w)

	// Check for errors from Google
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		if errDesc == "" {
			errDesc = errParam
		}
		http.Redirect(w, r, "/auth/error?error="+url.QueryEscape(errDesc), http.StatusSeeOther)
		return
	}

	// Exchange code for token
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/auth/error?error="+url.QueryEscape("Missing authorization code"), http.StatusSeeOther)
		return
	}

	token, err := h.client.Exchange(ctx, code)
	if err != nil {
		h.log.Error(ctx, "failed to exchange code for token", "error", err)
		http.Redirect(w, r, "/auth/error?error="+url.QueryEscape("Failed to authenticate with Google"), http.StatusSeeOther)
		return
	}

	// Fetch Google user info
	info, err := h.client.FetchUserInfo(ctx, token)
	if err != nil {
		h.log.Error(ctx, "failed to fetch Google user info", "error", err)
		http.Redirect(w, r, "/auth/error?error="+url.QueryEscape("Failed to retrieve your profile"), http.StatusSeeOther)
		return
	}

	if !info.Verified {
		http.Redirect(w, r, "/auth/error?error="+url.QueryEscape("Email address is not verified"), http.StatusSeeOther)
		return
	}

	// Upsert user in database
	user, err := h.client.UpsertUser(ctx, info)
	if err != nil {
		h.log.Error(ctx, "failed to upsert user", "error", err)
		http.Redirect(w, r, "/auth/error?error="+url.QueryEscape("Failed to create user account"), http.StatusSeeOther)
		return
	}

	// Store user ID in session
	if err := h.sessions.SetUserID(w, r, user.ID); err != nil {
		h.log.Error(ctx, "failed to save session", "error", err)
		http.Redirect(w, r, "/auth/error?error="+url.QueryEscape("Failed to create session"), http.StatusSeeOther)
		return
	}

	h.log.Info(ctx, "user authenticated via Google", "email", user.Email, "user_id", user.ID)

	// Redirect to return_to or home (sanitized to prevent open redirect)
	redirectURL := sanitizeReturnTo(returnTo)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// DevLoginHandler handles POST /auth/dev-login — the test-only login path.
//
// It exists so integration tests can hit the public REST API exactly the
// way the UI does (same middleware, same endpoints, same event stream)
// without running the full Google OAuth flow. The caller presents the
// token configured via APP_DEV_LOGIN_TOKEN in the X-Dev-Login-Token
// header; on success the response carries a signed school_session cookie
// for the fixed debug user (APP_DEV_LOGIN_USER_ID / APP_DEV_LOGIN_EMAIL).
//
// Safety: EnableDevLogin is only called when APP_DEV_LOGIN_TOKEN is set at
// startup, and main.go only mounts this route in the same branch. When the
// env var is unset the route is absent and this handler is unreachable —
// the defensive `h.devLoginToken == ""` check below exists purely to
// survive accidental unconditional mounting.
func (h *AuthHandlers) DevLoginHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.devLoginToken == "" {
		http.NotFound(w, r)
		return
	}

	supplied := r.Header.Get("X-Dev-Login-Token")
	if supplied == "" || supplied != h.devLoginToken {
		http.Error(w, `{"error":"invalid dev-login token"}`, http.StatusUnauthorized)
		return
	}

	user, err := h.client.UpsertDebugUser(ctx, h.devLoginUserID, h.devLoginEmail)
	if err != nil {
		h.log.Error(ctx, "dev-login: failed to upsert debug user", "error", err)
		http.Error(w, `{"error":"failed to upsert debug user"}`, http.StatusInternalServerError)
		return
	}

	if err := h.sessions.SetUserID(w, r, user.ID); err != nil {
		h.log.Error(ctx, "dev-login: failed to save session", "error", err)
		http.Error(w, `{"error":"failed to save session"}`, http.StatusInternalServerError)
		return
	}

	h.log.Info(ctx, "dev-login: session established", "user_id", user.ID, "email", user.Email)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"user_id": user.ID,
		"email":   user.Email,
	})
}

// LogoutHandler handles GET /auth/logout — clears the session and redirects.
func (h *AuthHandlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.Clear(w, r); err != nil {
		h.log.Error(r.Context(), "failed to clear session", "error", err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// MeHandler handles GET /api/me — returns the current user as JSON.
func (h *AuthHandlers) MeHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user); err != nil {
		h.log.Error(r.Context(), "failed to encode user JSON", "error", err)
	}
}

// AuthErrorData holds the data needed to render an authentication error page.
// The rendering itself is handled at the route-wiring layer (main.go) to keep
// the infrastructure package free of presentation-layer (components/) imports.
type AuthErrorData struct {
	ErrorMessage string
	LoginURL     string
	HomeURL      string
	User         *auth.User
}

// ErrorPageData extracts authentication error data from the request.
// The caller is responsible for rendering the result into a UI component.
func (h *AuthHandlers) ErrorPageData(r *http.Request) AuthErrorData {
	errorMessage := r.URL.Query().Get("error")
	if errorMessage == "" {
		errorMessage = i18n.T(r.Context(), "An unexpected error occurred")
	}

	return AuthErrorData{
		ErrorMessage: errorMessage,
		LoginURL:     h.appURL + "/auth/login",
		HomeURL:      h.appURL + "/",
		User:         auth.GetUserFromContext(r.Context()),
	}
}

// sanitizeReturnTo ensures the return_to URL is a safe relative path.
// Rejects absolute URLs, protocol-relative URLs, and external domains.
func sanitizeReturnTo(returnTo string) string {
	if returnTo == "" {
		return "/chat"
	}

	// Parse the URL
	parsed, err := url.Parse(returnTo)
	if err != nil {
		return "/chat"
	}

	// Reject absolute URLs (has scheme or host) and protocol-relative URLs
	if parsed.Scheme != "" || parsed.Host != "" || len(returnTo) > 1 && returnTo[0:2] == "//" {
		return "/chat"
	}

	// Must start with /
	if len(returnTo) == 0 || returnTo[0] != '/' {
		return "/chat"
	}

	return returnTo
}

// generateState creates a random CSRF state token.
func generateState() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", b)
	}
	return hex.EncodeToString(b)
}
