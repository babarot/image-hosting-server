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
	// If already logged in, redirect to upload page
	if cookie, err := r.Cookie("session_id"); err == nil {
		if s.sessions.Get(cookie.Value) != nil {
			http.Redirect(w, r, "/ui", http.StatusTemporaryRedirect)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	loginTmpl.ExecuteTemplate(w, "login.html", nil)
}

type uploadPageData struct {
	Username  string
	AvatarURL string
}

func (s *Server) handleUploadPage(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("session_id")
	sess := s.sessions.Get(cookie.Value)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	uploadTmpl.ExecuteTemplate(w, "upload.html", uploadPageData{
		Username:  sess.Username,
		AvatarURL: sess.AvatarURL,
	})
}