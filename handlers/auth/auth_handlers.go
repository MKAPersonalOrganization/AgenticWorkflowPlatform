package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"

	authcomp "school-with-ai/components/auth"
	"school-with-ai/domain"
	"school-with-ai/foundation/i18n"
	"school-with-ai/foundation/logger"
	"school-with-ai/infra/googleauth"
)

// AuthHandlers contains handlers for authentication routes.
type AuthHandlers struct {
	log          *logger.Logger
	googleClient *googleauth.Client
	sessions     *googleauth.SessionStore
	appURL       string
}

// NewAuthHandlers creates new authentication handlers.
func NewAuthHandlers(log *logger.Logger, googleClient *googleauth.Client, sessions *googleauth.SessionStore, appURL string) *AuthHandlers {
	return &AuthHandlers{
		log:          log,
		googleClient: googleClient,
		sessions:     sessions,
		appURL:       appURL,
	}
}

// LoginPage handles GET /auth/login — renders the Google login page.
func (h *AuthHandlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already authenticated, redirect to home
	if user := domain.GetUserFromContext(r.Context()); user != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Get optional error message from query
	errorMsg := r.URL.Query().Get("error")
	var messages []authcomp.UIMessage
	if errorMsg != "" {
		messages = append(messages, authcomp.UIMessage{
			Text: errorMsg,
			Type: "error",
		})
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

	googleURL := h.googleClient.AuthCodeURL(state)

	props := authcomp.LoginPageProps{
		GoogleAuthURL: googleURL,
		Messages:      messages,
	}

	component := authcomp.LoginPage(props)
	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render login page", "error", err)
	}
}

// CallbackHandler handles GET /auth/callback — the Google OAuth2 callback.
func (h *AuthHandlers) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify CSRF state
	session, err := h.sessions.Get(r)
	if err != nil {
		h.log.Error(ctx, "failed to get session", "error", err)
		http.Redirect(w, r, "/auth/login?error="+url.QueryEscape("Session error"), http.StatusSeeOther)
		return
	}

	expectedState, _ := session.Values["oauth_state"].(string)
	actualState := r.URL.Query().Get("state")
	if expectedState == "" || actualState != expectedState {
		h.log.Error(ctx, "OAuth state mismatch", "expected", expectedState, "actual", actualState)
		http.Redirect(w, r, "/auth/login?error="+url.QueryEscape("Invalid OAuth state"), http.StatusSeeOther)
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
		http.Redirect(w, r, "/auth/login?error="+url.QueryEscape(errDesc), http.StatusSeeOther)
		return
	}

	// Exchange code for token
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/auth/login?error="+url.QueryEscape("Missing authorization code"), http.StatusSeeOther)
		return
	}

	token, err := h.googleClient.Exchange(ctx, code)
	if err != nil {
		h.log.Error(ctx, "failed to exchange code for token", "error", err)
		http.Redirect(w, r, "/auth/login?error="+url.QueryEscape("Failed to authenticate with Google"), http.StatusSeeOther)
		return
	}

	// Fetch Google user info
	info, err := h.googleClient.FetchUserInfo(ctx, token)
	if err != nil {
		h.log.Error(ctx, "failed to fetch Google user info", "error", err)
		http.Redirect(w, r, "/auth/login?error="+url.QueryEscape("Failed to retrieve your profile"), http.StatusSeeOther)
		return
	}

	if !info.Verified {
		http.Redirect(w, r, "/auth/login?error="+url.QueryEscape("Email address is not verified"), http.StatusSeeOther)
		return
	}

	// Upsert user in database
	user, err := h.googleClient.UpsertUser(ctx, info)
	if err != nil {
		h.log.Error(ctx, "failed to upsert user", "error", err)
		http.Redirect(w, r, "/auth/login?error="+url.QueryEscape("Failed to create user account"), http.StatusSeeOther)
		return
	}

	// Store user ID in session
	if err := h.sessions.SetUserID(w, r, user.ID); err != nil {
		h.log.Error(ctx, "failed to save session", "error", err)
		http.Redirect(w, r, "/auth/login?error="+url.QueryEscape("Failed to create session"), http.StatusSeeOther)
		return
	}

	h.log.Info(ctx, "user authenticated via Google", "email", user.Email, "user_id", user.ID)

	// Redirect to return_to or home (sanitized to prevent open redirect)
	redirectURL := sanitizeReturnTo(returnTo)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// LogoutHandler handles GET /auth/logout — clears the session and redirects.
func (h *AuthHandlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.Clear(w, r); err != nil {
		h.log.Error(r.Context(), "failed to clear session", "error", err)
	}

	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

// SettingsPage handles GET /auth/settings — account management page.
func (h *AuthHandlers) SettingsPage(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/auth/login?return_to=/auth/settings", http.StatusSeeOther)
		return
	}

	props := authcomp.SettingsPageProps{
		User: user,
	}

	component := authcomp.SettingsPage(props)
	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render settings page", "error", err)
	}
}

// ErrorPage handles GET /auth/error — displays authentication errors.
func (h *AuthHandlers) ErrorPage(w http.ResponseWriter, r *http.Request) {
	errorParam := r.URL.Query().Get("error")
	errorMessage := errorParam
	if errorMessage == "" {
		errorMessage = i18n.T(r.Context(), "An unexpected error occurred")
	}

	props := authcomp.ErrorPageProps{
		ErrorMessage: errorMessage,
		LoginURL:     h.appURL + "/auth/login",
		HomeURL:      h.appURL + "/",
		User:         domain.GetUserFromContext(r.Context()),
	}

	component := authcomp.ErrorPage(props)
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, i18n.T(r.Context(), "Internal server error"), http.StatusInternalServerError)
	}
}

// sanitizeReturnTo ensures the return_to URL is a safe relative path.
// Rejects absolute URLs, protocol-relative URLs, and external domains.
func sanitizeReturnTo(returnTo string) string {
	if returnTo == "" {
		return "/"
	}

	// Parse the URL
	parsed, err := url.Parse(returnTo)
	if err != nil {
		return "/"
	}

	// Reject absolute URLs (has scheme or host) and protocol-relative URLs
	if parsed.Scheme != "" || parsed.Host != "" || len(returnTo) > 1 && returnTo[0:2] == "//" {
		return "/"
	}

	// Must start with /
	if len(returnTo) == 0 || returnTo[0] != '/' {
		return "/"
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
