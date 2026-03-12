package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCORSHeaders(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("OPTIONS", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing CORS header")
	}
}

func TestUploadRequiresAuth(t *testing.T) {
	srv := newTestServerWithAuth(t)

	pngData := createTestPNG(t)
	body, contentType := createMultipartFile(t, "test.png", pngData)
	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUploadWithAPIKey(t *testing.T) {
	srv := newTestServerWithAuth(t)

	pngData := createTestPNG(t)
	body, contentType := createMultipartFile(t, "test.png", pngData)
	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadWithSessionCookie(t *testing.T) {
	srv := newTestServerWithAuth(t)
	sessionID := srv.sessions.Create("alice", "", 1*time.Hour)

	pngData := createTestPNG(t)
	body, contentType := createMultipartFile(t, "test.png", pngData)
	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRequiresAuth(t *testing.T) {
	srv := newTestServerWithAuth(t)

	req := httptest.NewRequest("DELETE", "/api/delete/foo.png", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestSessionAuthRedirectsToLogin(t *testing.T) {
	srv := newTestServerWithAuth(t)

	req := httptest.NewRequest("GET", "/ui", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}
}

func TestClientIPFromCFHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("CF-Connecting-IP", "1.2.3.4")
	if ip := clientIP(req); ip != "1.2.3.4" {
		t.Fatalf("expected 1.2.3.4, got %q", ip)
	}
}

func TestClientIPFromXFF(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "5.6.7.8, 9.10.11.12")
	if ip := clientIP(req); ip != "5.6.7.8" {
		t.Fatalf("expected 5.6.7.8, got %q", ip)
	}
}

func TestClientIPFromRemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	if ip := clientIP(req); ip != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %q", ip)
	}
}

func TestRateLimiting(t *testing.T) {
	srv := newTestServer(t)
	pngData := createTestPNG(t)

	// Exhaust rate limit (burst=5)
	for i := 0; i < 6; i++ {
		body, contentType := createMultipartFile(t, "test.png", pngData)
		req := httptest.NewRequest("POST", "/api/upload", body)
		req.Header.Set("Content-Type", contentType)
		req.RemoteAddr = "10.0.0.99:1234"
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if i < 5 && w.Code != http.StatusCreated {
			t.Fatalf("request %d: expected 201, got %d", i, w.Code)
		}
		if i == 5 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d: expected 429, got %d", i, w.Code)
		}
	}
}
