package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginPageRedirectsWhenAuthDisabled(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/ui" {
		t.Fatalf("expected redirect to /ui, got %q", loc)
	}
}

func TestUploadPageWhenAuthDisabled(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/ui", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html, got %q", ct)
	}
}

func TestLoginPageShowsFormWhenAuthEnabled(t *testing.T) {
	srv := newTestServerWithAuth(t)
	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestLoginPageRedirectsWhenLoggedIn(t *testing.T) {
	srv := newTestServerWithAuth(t)
	sessionID := srv.sessions.Create("alice", "", 1*time.Hour)

	req := httptest.NewRequest("GET", "/login", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", w.Code)
	}
}

func TestUploadPageWithSession(t *testing.T) {
	srv := newTestServerWithAuth(t)
	sessionID := srv.sessions.Create("alice", "https://example.com/a.png", 1*time.Hour)

	req := httptest.NewRequest("GET", "/ui", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
