package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type Config struct {
	APIKey     string
	UploadDir  string
	BaseURL    string
	ListenAddr string
}

type Server struct {
	config  Config
	logger  *slog.Logger
	limiter *IPRateLimiter
	mux     *http.ServeMux
}

func NewServer(cfg Config, logger *slog.Logger) *Server {
	s := &Server{
		config:  cfg,
		logger:  logger,
		limiter: NewIPRateLimiter(rate.Every(2*time.Second), 5), // 30 req/min, burst 5
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.withCORS(s.withLogging(s.mux)).ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Public endpoints
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /files/{path...}", s.handleFiles)

	// Protected endpoints (auth + rate limit)
	s.mux.Handle("POST /api/upload", s.withRateLimit(s.withAuth(http.HandlerFunc(s.handleUpload))))
	s.mux.Handle("DELETE /api/delete/{path...}", s.withRateLimit(s.withAuth(http.HandlerFunc(s.handleDelete))))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
