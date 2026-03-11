package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Session represents an authenticated user session.
type Session struct {
	Username  string
	AvatarURL string
	ExpiresAt time.Time
}

// SessionStore manages in-memory sessions.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	s := &SessionStore{
		sessions: make(map[string]*Session),
	}
	go s.cleanup()
	return s
}

func (s *SessionStore) Create(username, avatarURL string, ttl time.Duration) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateRandomHex(32)
	s.sessions[id] = &Session{
		Username:  username,
		AvatarURL: avatarURL,
		ExpiresAt: time.Now().Add(ttl),
	}
	return id
}

func (s *SessionStore) Get(id string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[id]
	if !ok || time.Now().After(sess.ExpiresAt) {
		delete(s.sessions, id)
		return nil
	}
	return sess
}

func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func (s *SessionStore) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		s.mu.Lock()
		now := time.Now()
		for id, sess := range s.sessions {
			if now.After(sess.ExpiresAt) {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}

// OAuthStateStore manages CSRF state tokens for OAuth flow.
type OAuthStateStore struct {
	mu     sync.Mutex
	states map[string]time.Time
}

func NewOAuthStateStore() *OAuthStateStore {
	s := &OAuthStateStore{
		states: make(map[string]time.Time),
	}
	go s.cleanup()
	return s
}

func (s *OAuthStateStore) Generate() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := generateRandomHex(16)
	s.states[state] = time.Now().Add(10 * time.Minute)
	return state
}

func (s *OAuthStateStore) Validate(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	expiry, ok := s.states[state]
	if !ok || time.Now().After(expiry) {
		delete(s.states, state)
		return false
	}
	delete(s.states, state) // single use
	return true
}

func (s *OAuthStateStore) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		s.mu.Lock()
		now := time.Now()
		for state, expiry := range s.states {
			if now.After(expiry) {
				delete(s.states, state)
			}
		}
		s.mu.Unlock()
	}
}

const sessionTTL = 24 * time.Hour

func (s *Server) handleGitHubAuth(w http.ResponseWriter, r *http.Request) {
	state := s.oauthStates.Generate()
	redirectURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=read:user&state=%s",
		s.config.GitHubClientID,
		s.config.BaseURL+"/auth/callback",
		state,
	)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if !s.oauthStates.Validate(state) {
		s.logger.Warn("oauth: invalid state", "ip", clientIP(r))
		writeError(w, http.StatusBadRequest, "invalid or expired state")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code")
		return
	}

	token, err := s.exchangeCode(code)
	if err != nil {
		s.logger.Error("oauth: code exchange failed", "error", err)
		writeError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	user, err := s.fetchGitHubUser(token)
	if err != nil {
		s.logger.Error("oauth: fetch user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get user info")
		return
	}

	if !s.isAllowedUser(user.Login) {
		s.logger.Warn("oauth: user not allowed", "user", user.Login, "ip", clientIP(r))
		writeError(w, http.StatusForbidden, "user not authorized")
		return
	}

	sessionID := s.sessions.Create(user.Login, user.AvatarURL, sessionTTL)

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.HasPrefix(s.config.BaseURL, "https"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})

	s.logger.Info("oauth: login success", "user", user.Login, "ip", clientIP(r))
	http.Redirect(w, r, "/ui", http.StatusTemporaryRedirect)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session_id"); err == nil {
		s.sessions.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
}

type githubUser struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

func (s *Server) exchangeCode(code string) (string, error) {
	body := fmt.Sprintf(
		"client_id=%s&client_secret=%s&code=%s",
		s.config.GitHubClientID,
		s.config.GitHubClientSecret,
		code,
	)
	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("github oauth error: %s", result.Error)
	}
	return result.AccessToken, nil
}

func (s *Server) fetchGitHubUser(token string) (*githubUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error: %d %s", resp.StatusCode, string(body))
	}

	var user githubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Server) isAllowedUser(username string) bool {
	for _, u := range s.config.GitHubAllowedUsers {
		if strings.EqualFold(u, username) {
			return true
		}
	}
	return false
}

func generateRandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
