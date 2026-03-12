package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"testing"
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
