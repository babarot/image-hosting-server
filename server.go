package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type Config struct {
	APIKey             string
	UploadDir          string
	BaseURL            string
	ListenAddr         string
	GitHubClientID     string
	GitHubClientSecret string
	AllowedUsers       []string
}

type Server struct {
	config      Config
	logger      *slog.Logger
	limiter     *IPRateLimiter
	mux         *http.ServeMux
	sessions    *SessionStore
	oauthStates *OAuthStateStore
}

func NewServer(cfg Config, logger *slog.Logger) *Server {
	s := &Server{
		config:      cfg,
		logger:      logger,
		limiter:     NewIPRateLimiter(rate.Every(2*time.Second), 5), // 30 req/min, burst 5
		mux:         http.NewServeMux(),
		sessions:    NewSessionStore(),
		oauthStates: NewOAuthStateStore(),
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
	s.mux.Handle("POST /api/upload", s.withRateLimit(s.withUploadAuth(http.HandlerFunc(s.handleUpload))))
	s.mux.Handle("DELETE /api/delete/{path...}", s.withRateLimit(s.withAuth(http.HandlerFunc(s.handleDelete))))

	// Web UI (only when OAuth is configured)
	if s.config.GitHubClientID != "" {
		s.mux.HandleFunc("GET /login", s.handleLoginPage)
		s.mux.HandleFunc("GET /auth/github", s.handleGitHubAuth)
		s.mux.HandleFunc("GET /auth/callback", s.handleGitHubCallback)
		s.mux.HandleFunc("GET /auth/logout", s.handleLogout)
		s.mux.Handle("GET /ui", s.withSessionAuth(http.HandlerFunc(s.handleUploadPage)))
		s.mux.HandleFunc("GET /ui/preview", s.handleUploadPreview)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
