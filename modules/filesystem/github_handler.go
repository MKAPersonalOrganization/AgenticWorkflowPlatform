package filesystem

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"

	"aicompass/infra/githubauth"
)

// githubHandler handles GitHub OAuth connection endpoints.
type githubHandler struct {
	ghClient *githubauth.Client
	ghStore  *githubauth.Store
	appURL   string
}

func newGitHubHandler(ghClient *githubauth.Client, ghStore *githubauth.Store, appURL string) *githubHandler {
	return &githubHandler{
		ghClient: ghClient,
		ghStore:  ghStore,
		appURL:   appURL,
	}
}

// ── GitHub OAuth flow ───────────────────────────────────────────────────────

// connect starts the GitHub OAuth flow by redirecting to GitHub.
// GET /api/github/connect
func (h *githubHandler) connect(w http.ResponseWriter, r *http.Request) {
	if _, ok := userFromRequest(r); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if !h.ghClient.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GitHub OAuth is not configured")
		return
	}

	state := generateOAuthState()

	// Store state in a short-lived cookie so we can verify on callback.
	http.SetCookie(w, &http.Cookie{
		Name:     "gh_oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	authURL := h.ghClient.AuthCodeURL(state)
	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

// callback handles the GitHub OAuth callback.
// GET /api/github/callback
func (h *githubHandler) callback(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		http.Redirect(w, r, h.appURL+"/?error=unauthorized", http.StatusSeeOther)
		return
	}

	// Verify state
	stateCookie, err := r.Cookie("gh_oauth_state")
	if err != nil || stateCookie.Value == "" {
		http.Redirect(w, r, h.appURL+"/?error="+url.QueryEscape("Invalid OAuth state"), http.StatusSeeOther)
		return
	}
	actualState := r.URL.Query().Get("state")
	if actualState != stateCookie.Value {
		http.Redirect(w, r, h.appURL+"/?error="+url.QueryEscape("OAuth state mismatch"), http.StatusSeeOther)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "gh_oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Check for errors from GitHub
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Redirect(w, r, h.appURL+"/?error="+url.QueryEscape(errParam), http.StatusSeeOther)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, h.appURL+"/?error="+url.QueryEscape("Missing authorization code"), http.StatusSeeOther)
		return
	}

	// Exchange code for token
	token, err := h.ghClient.Exchange(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, h.appURL+"/?error="+url.QueryEscape("Failed to authenticate with GitHub"), http.StatusSeeOther)
		return
	}

	// Fetch GitHub user profile
	ghUser, err := h.ghClient.FetchUser(r.Context(), token)
	if err != nil {
		http.Redirect(w, r, h.appURL+"/?error="+url.QueryEscape("Failed to retrieve GitHub profile"), http.StatusSeeOther)
		return
	}

	// Store connection
	ts := now()
	conn := githubauth.Connection{
		UserID:         user.ID,
		AccessToken:    token.AccessToken,
		GitHubUsername: ghUser.Login,
		GitHubAvatar:   ghUser.AvatarURL,
		ConnectedAt:    ts,
		UpdatedAt:      ts,
	}

	if err := h.ghStore.Upsert(r.Context(), conn); err != nil {
		http.Redirect(w, r, h.appURL+"/?error="+url.QueryEscape("Failed to save GitHub connection"), http.StatusSeeOther)
		return
	}

	// Redirect back to the app — the frontend will detect the connection
	// and close the popup / refresh the dialog.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html><html><body>
<script>
if (window.opener) {
	window.opener.postMessage({type: 'github-connected', username: '`+ghUser.Login+`'}, '*');
	window.close();
} else {
	window.location.href = '`+h.appURL+`';
}
</script>
</body></html>`)
}

// status returns the current GitHub connection status for the user.
// GET /api/github/status
func (h *githubHandler) status(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	conn, err := h.ghStore.Get(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check GitHub status")
		return
	}

	type statusResponse struct {
		Connected bool   `json:"connected"`
		Username  string `json:"username,omitempty"`
		Avatar    string `json:"avatar,omitempty"`
	}

	if conn == nil {
		writeJSON(w, http.StatusOK, statusResponse{Connected: false})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		Connected: true,
		Username:  conn.GitHubUsername,
		Avatar:    conn.GitHubAvatar,
	})
}

// repos lists accessible GitHub repositories for the connected user.
// GET /api/github/repos
func (h *githubHandler) repos(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	token, err := h.ghStore.GetToken(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve GitHub token")
		return
	}
	if token == "" {
		writeError(w, http.StatusBadRequest, "GitHub account not connected")
		return
	}

	repos, err := h.ghClient.ListRepos(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to list GitHub repositories")
		return
	}

	writeJSON(w, http.StatusOK, repos)
}

// disconnect removes the user's GitHub connection.
// DELETE /api/github/disconnect
func (h *githubHandler) disconnect(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.ghStore.Delete(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disconnect GitHub account")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

// ── helpers ─────────────────────────────────────────────────────────────────

func generateOAuthState() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", b)
	}
	return hex.EncodeToString(b)
}
