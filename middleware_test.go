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

func TestClientIP(t *testing.T) {
	tests := []struct {
		name          string
		trustedHeader string
		headers       map[string]string
		want          string
	}{
		{
			name: "remote addr",
			want: "10.0.0.1",
		},
		{
			name:    "untrusted headers are ignored",
			headers: map[string]string{"CF-Connecting-IP": "1.2.3.4", "X-Forwarded-For": "5.6.7.8"},
			want:    "10.0.0.1",
		},
		{
			name:          "trusted header",
			trustedHeader: "CF-Connecting-IP",
			headers:       map[string]string{"CF-Connecting-IP": "1.2.3.4"},
			want:          "1.2.3.4",
		},
		{
			name:          "trusted header with IPv6 is normalized",
			trustedHeader: "CF-Connecting-IP",
			headers:       map[string]string{"CF-Connecting-IP": "2001:DB8:0:0::1"},
			want:          "2001:db8::1",
		},
		{
			name:          "trusted header with invalid value falls back",
			trustedHeader: "X-Real-IP",
			headers:       map[string]string{"X-Real-IP": "1.2.3.4, 5.6.7.8"},
			want:          "10.0.0.1",
		},
		{
			name:          "trusted header missing falls back",
			trustedHeader: "CF-Connecting-IP",
			want:          "10.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t)
			srv.config.TrustedIPHeader = tt.trustedHeader
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "10.0.0.1:12345"
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if ip := srv.clientIP(req); ip != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, ip)
			}
		})
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
