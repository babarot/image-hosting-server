package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		APIKey:       "test-key",
		UploadDir:    dir,
		BaseURL:      "http://localhost:8080",
		AuthDisabled: true,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(cfg, logger)
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", resp["status"])
	}
}

func createMultipartFile(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(content)
	writer.Close()
	return body, writer.FormDataContentType()
}

func createTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	for y := range 50 {
		for x := range 100 {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUploadSuccess(t *testing.T) {
	srv := newTestServer(t)
	pngData := createTestPNG(t)

	body, contentType := createMultipartFile(t, "test.png", pngData)
	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["url"] == nil || resp["url"].(string) == "" {
		t.Fatal("expected url in response")
	}
	if resp["filename"] == nil || resp["filename"].(string) == "" {
		t.Fatal("expected filename in response")
	}
	if resp["width"] == nil || resp["width"].(float64) != 100 {
		t.Fatalf("expected width 100, got %v", resp["width"])
	}
	if resp["height"] == nil || resp["height"].(float64) != 50 {
		t.Fatalf("expected height 50, got %v", resp["height"])
	}
}

func TestUploadRequiresAuth(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		APIKey:       "test-key",
		UploadDir:    dir,
		BaseURL:      "http://localhost:8080",
		AuthDisabled: false,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(cfg, logger)

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
	dir := t.TempDir()
	cfg := Config{
		APIKey:       "test-key",
		UploadDir:    dir,
		BaseURL:      "http://localhost:8080",
		AuthDisabled: false,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(cfg, logger)

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

func TestUploadDisallowedExtension(t *testing.T) {
	srv := newTestServer(t)

	body, contentType := createMultipartFile(t, "malware.exe", []byte("not an image"))
	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUploadContentMismatch(t *testing.T) {
	srv := newTestServer(t)

	// .png extension but not PNG content
	body, contentType := createMultipartFile(t, "fake.png", []byte("this is not a png"))
	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServeFile(t *testing.T) {
	srv := newTestServer(t)

	// Write a file to the upload dir
	subdir := filepath.Join(srv.config.UploadDir, "2026", "03")
	os.MkdirAll(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "test.png"), createTestPNG(t), 0644)

	req := httptest.NewRequest("GET", "/files/2026/03/test.png", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected image/png, got %q", ct)
	}
}

func TestServeFileNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/files/nonexistent.png", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteFile(t *testing.T) {
	srv := newTestServer(t)

	// Create a file
	subdir := filepath.Join(srv.config.UploadDir, "2026", "03")
	os.MkdirAll(subdir, 0755)
	filePath := filepath.Join(subdir, "delete-me.png")
	os.WriteFile(filePath, createTestPNG(t), 0644)

	req := httptest.NewRequest("DELETE", "/api/delete/2026/03/delete-me.png", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatal("file should have been deleted")
	}
}

func TestDeleteFileNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("DELETE", "/api/delete/nonexistent.png", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUploadPathTraversal(t *testing.T) {
	srv := newTestServer(t)

	pngData := createTestPNG(t)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.png")
	part.Write(pngData)
	writer.WriteField("path", "../../etc")
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Session / Auth tests ---

func newTestServerWithAuth(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		APIKey:             "test-key",
		UploadDir:          dir,
		BaseURL:            "http://localhost:8080",
		AuthDisabled:       false,
		GitHubClientID:     "fake-id",
		GitHubClientSecret: "fake-secret",
		GitHubAllowedUsers: []string{"alice", "bob"},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(cfg, logger)
}

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

func TestLogout(t *testing.T) {
	srv := newTestServerWithAuth(t)
	sessionID := srv.sessions.Create("alice", "", 1*time.Hour)

	req := httptest.NewRequest("GET", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", w.Code)
	}
	// Session should be deleted
	if sess := srv.sessions.Get(sessionID); sess != nil {
		t.Fatal("session should have been deleted")
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

// --- Middleware tests ---

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

// --- UI tests ---

func TestLoginPageRedirectsWhenAuthDisabled(t *testing.T) {
	srv := newTestServer(t) // AuthDisabled=true
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
