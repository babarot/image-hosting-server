package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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

	body, contentType := createMultipartFile(t, "fake.png", []byte("this is not a png"))
	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
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

func TestServeFile(t *testing.T) {
	srv := newTestServer(t)

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
