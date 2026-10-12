package main

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed static/login.html
var loginHTML embed.FS

//go:embed static/upload.html
var uploadHTML embed.FS

var (
	loginTmpl  = template.Must(template.ParseFS(loginHTML, "static/login.html"))
	uploadTmpl = template.Must(template.ParseFS(uploadHTML, "static/upload.html"))
)

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// If auth disabled, go straight to upload
	if s.config.AuthDisabled {
		http.Redirect(w, r, "/ui", http.StatusTemporaryRedirect)
		return
	}

	// If already logged in, redirect to upload page
	if cookie, err := r.Cookie("session_id"); err == nil {
		if s.sessions.Get(cookie.Value) != nil {
			http.Redirect(w, r, "/ui", http.StatusTemporaryRedirect)
			return
		}
	}

	s.renderTemplate(w, loginTmpl, "login.html", nil)
}

type uploadPageData struct {
	Username  string
	AvatarURL string
}

func (s *Server) handleUploadPage(w http.ResponseWriter, r *http.Request) {
	// Auth disabled: use placeholder user info
	if s.config.AuthDisabled {
		s.renderTemplate(w, uploadTmpl, "upload.html", uploadPageData{
			Username:  "local",
			AvatarURL: "",
		})
		return
	}

	cookie, _ := r.Cookie("session_id")
	sess := s.sessions.Get(cookie.Value)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	s.renderTemplate(w, uploadTmpl, "upload.html", uploadPageData{
		Username:  sess.Username,
		AvatarURL: sess.AvatarURL,
	})
}

func (s *Server) renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		s.logger.Error("render template failed", "template", name, "error", err)
	}
}
