package googleauth

import (
	"crypto/sha256"
	"encoding/gob"
	"net/http"
	"school-with-ai/shared"

	"github.com/gorilla/sessions"
)

const (
	// SessionName is the cookie name for auth sessions.
	SessionName = "school_session"
	// SessionUserID is the key for storing user ID in the session.
	SessionUserID = "user_id"
)

func init() {
	// Register shared.User so gob can serialize it.
	gob.Register(&shared.User{})
}

// SessionStore wraps gorilla/sessions for cookie-based session management.
type SessionStore struct {
	store *sessions.CookieStore
}

// NewSessionStore creates a new session store with the given secret key.
// It derives separate 32-byte authentication and encryption keys from the secret
// so that session cookies are both signed (HMAC) and AES-encrypted.
func NewSessionStore(secret string) *SessionStore {
	// Derive authentication key (HMAC signing)
	authHash := sha256.Sum256([]byte("auth:" + secret))
	authKey := authHash[:]

	// Derive encryption key (AES-256)
	encHash := sha256.Sum256([]byte("enc:" + secret))
	encKey := encHash[:]

	store := sessions.NewCookieStore(authKey, encKey)
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30, // 30 days
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}

	return &SessionStore{store: store}
}

// Get returns the session for the request.
func (s *SessionStore) Get(r *http.Request) (*sessions.Session, error) {
	return s.store.Get(r, SessionName)
}

// SetUserID stores the user ID in the session.
func (s *SessionStore) SetUserID(w http.ResponseWriter, r *http.Request, userID string) error {
	session, err := s.store.Get(r, SessionName)
	if err != nil {
		return err
	}

	session.Values[SessionUserID] = userID
	return session.Save(r, w)
}

// GetUserID retrieves the user ID from the session.
func (s *SessionStore) GetUserID(r *http.Request) (string, bool) {
	session, err := s.store.Get(r, SessionName)
	if err != nil {
		return "", false
	}

	userID, ok := session.Values[SessionUserID].(string)
	if !ok || userID == "" {
		return "", false
	}

	return userID, true
}

// Clear destroys the session (logout).
func (s *SessionStore) Clear(w http.ResponseWriter, r *http.Request) error {
	session, err := s.store.Get(r, SessionName)
	if err != nil {
		return err
	}

	session.Options.MaxAge = -1
	return session.Save(r, w)
}
