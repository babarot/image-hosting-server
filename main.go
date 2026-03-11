package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	allowedUsers := []string{}
	if v := os.Getenv("ALLOWED_USERS"); v != "" {
		for _, u := range strings.Split(v, ",") {
			if trimmed := strings.TrimSpace(u); trimmed != "" {
				allowedUsers = append(allowedUsers, trimmed)
			}
		}
	}

	cfg := Config{
		APIKey:             os.Getenv("API_KEY"),
		UploadDir:          getEnv("UPLOAD_DIR", "/data/images"),
		BaseURL:            strings.TrimRight(strings.TrimSuffix(getEnv("BASE_URL", "http://localhost:8080"), "/files"), "/"),
		ListenAddr:         getEnv("LISTEN_ADDR", ":8080"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		AllowedUsers:       allowedUsers,
	}

	if cfg.APIKey == "" {
		logger.Error("API_KEY environment variable is required")
		os.Exit(1)
	}

	if cfg.GitHubClientID != "" {
		logger.Info("web UI enabled", "allowed_users", cfg.AllowedUsers)
	}

	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		logger.Error("failed to ensure upload directory", "error", err, "path", cfg.UploadDir)
		os.Exit(1)
	}

	srv := NewServer(cfg, logger)

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("shutting down server")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			logger.Error("server shutdown error", "error", err)
		}
	}()

	logger.Info("starting server", "addr", cfg.ListenAddr, "upload_dir", cfg.UploadDir)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
