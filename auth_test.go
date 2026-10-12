package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestSessionCreateAndGet(t *testing.T) {
	store := NewSessionStore()
	id := store.Create("alice", "https://example.com/avatar.png", 1*time.Hour)

	sess := store.Get(id)
	if sess == nil {
		t.Fatal("expected session")
	}
	if sess.Username != "alice" {
		t.Fatalf("expected alice, got %q", sess.Username)
	}
	if sess.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("unexpected avatar URL %q", sess.AvatarURL)
	}
}

func TestSessionExpired(t *testing.T) {
	store := NewSessionStore()
	id := store.Create("alice", "", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	if sess := store.Get(id); sess != nil {
		t.Fatal("expected expired session to be nil")
	}
}

func TestSessionDelete(t *testing.T) {
	store := NewSessionStore()
	id := store.Create("alice", "", 1*time.Hour)
	store.Delete(id)

	if sess := store.Get(id); sess != nil {
		t.Fatal("expected deleted session to be nil")
	}
}

func TestOAuthStateValidate(t *testing.T) {
	store := NewOAuthStateStore()
	state := store.Generate()

	if !store.Validate(state) {
		t.Fatal("expected valid state")
	}
	// Single use — second validate should fail
	if store.Validate(state) {
		t.Fatal("expected state to be consumed")
	}
}

func TestOAuthStateExpired(t *testing.T) {
	store := &OAuthStateStore{states: make(map[string]time.Time)}
	store.states["expired"] = time.Now().Add(-1 * time.Minute)

	if store.Validate("expired") {
		t.Fatal("expected expired state to fail")
	}
}

func TestIsAllowedUser(t *testing.T) {
	srv := newTestServerWithAuth(t)

	if !srv.isAllowedUser("alice") {
		t.Fatal("alice should be allowed")
	}
	if !srv.isAllowedUser("Alice") {
		t.Fatal("case-insensitive match should work")
	}
	if srv.isAllowedUser("eve") {
		t.Fatal("eve should not be allowed")
	}
}

func TestLogout(t *testing.T) {
	srv := newTestServerWithAuth(t)
	sessionID := srv.sessions.Create("alice", "", 1*time.Hour)

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if sess := srv.sessions.Get(sessionID); sess != nil {
		t.Fatal("session should have been deleted")
	}
}

func TestLogoutRejectsGET(t *testing.T) {
	srv := newTestServerWithAuth(t)
	sessionID := srv.sessions.Create("alice", "", 1*time.Hour)

	req := httptest.NewRequest("GET", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
	if sess := srv.sessions.Get(sessionID); sess == nil {
		t.Fatal("session should not be deleted by GET")
	}
}

func TestGitHubAuthRedirectEscapesParams(t *testing.T) {
	srv := newTestServerWithAuth(t)
	srv.config.BaseURL = "https://example.com/sub?x=1&y=2"

	req := httptest.NewRequest("GET", "/auth/github", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", w.Code)
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("invalid Location: %v", err)
	}
	q := loc.Query()
	if got, want := q.Get("redirect_uri"), "https://example.com/sub?x=1&y=2/auth/callback"; got != want {
		t.Fatalf("redirect_uri = %q, want %q", got, want)
	}
	if q.Get("client_id") != "fake-id" || q.Get("scope") != "read:user" || q.Get("state") == "" {
		t.Fatalf("unexpected query: %v", q)
	}
}
