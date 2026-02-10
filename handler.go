package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxUploadSize = 20 << 20 // 20MB

var allowedExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".avif": "image/avif",
	".ico":  "image/x-icon",
	".pdf":  "application/pdf",
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+(1<<20))
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		s.logger.Warn("upload: parse form error", "error", err, "ip", clientIP(r))
		writeError(w, http.StatusRequestEntityTooLarge, "file too large (max 20MB)")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	// Validate extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if _, ok := allowedExts[ext]; !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("file type %q is not allowed", ext))
		return
	}

	// Read file content
	data, err := io.ReadAll(file)
	if err != nil {
		s.logger.Error("upload: read file error", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	// Validate MIME type by content sniffing
	if !isAllowedContent(ext, data) {
		writeError(w, http.StatusBadRequest, "file content does not match its extension")
		return
	}

	// Generate random filename (8 bytes = 16 hex chars)
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		s.logger.Error("upload: random generation error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	filename := hex.EncodeToString(randBytes) + ext

	// Determine subdirectory
	subdir := r.FormValue("path")
	if subdir == "" {
		now := time.Now()
		subdir = fmt.Sprintf("%d/%02d", now.Year(), now.Month())
	}

	if strings.Contains(subdir, "..") {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// Create directory and save file
	dirPath := filepath.Join(s.config.UploadDir, subdir)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		s.logger.Error("upload: mkdir error", "error", err, "path", dirPath)
		writeError(w, http.StatusInternalServerError, "failed to create directory")
		return
	}

	filePath := filepath.Join(dirPath, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		s.logger.Error("upload: write file error", "error", err, "path", filePath)
		writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	relativePath := filepath.ToSlash(filepath.Join(subdir, filename))
	url := s.config.BaseURL + "/files/" + relativePath

	s.logger.Info("upload: success",
		"ip", clientIP(r),
		"path", relativePath,
		"size", len(data),
		"duration", time.Since(start).String(),
	)

	writeJSON(w, http.StatusCreated, map[string]any{
		"url":      url,
		"path":     relativePath,
		"size":     len(data),
		"filename": filename,
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	pathParam := r.PathValue("path")
	if pathParam == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	fullPath, err := s.safePath(pathParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	if err := os.Remove(fullPath); err != nil {
		s.logger.Error("delete: remove error", "error", err, "path", pathParam)
		writeError(w, http.StatusInternalServerError, "failed to delete file")
		return
	}

	s.logger.Info("delete: success", "ip", clientIP(r), "path", pathParam)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": pathParam})
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	pathParam := r.PathValue("path")
	if pathParam == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	fullPath, err := s.safePath(pathParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	http.ServeFile(w, r, fullPath)
}

func (s *Server) safePath(p string) (string, error) {
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("path contains '..'")
	}
	cleaned := filepath.Clean(p)
	full := filepath.Join(s.config.UploadDir, cleaned)
	if !strings.HasPrefix(full, filepath.Clean(s.config.UploadDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal detected")
	}
	return full, nil
}

func isAllowedContent(ext string, data []byte) bool {
	detected := http.DetectContentType(data)
	expected, ok := allowedExts[ext]
	if !ok {
		return false
	}

	if strings.HasPrefix(detected, expected) {
		return true
	}

	// SVG may be detected as text/xml, text/plain, or text/html
	if ext == ".svg" && strings.HasPrefix(detected, "text/") {
		return true
	}

	// Some formats (AVIF, ICO, WebP) may not be recognized by DetectContentType
	if detected == "application/octet-stream" {
		return true
	}

	return false
}
