package main

import (
	"database/sql"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"
)

type Handler struct {
	static      fs.FS
	cookieStore *sessions.CookieStore
	sessionName string
	tmpl        *template.Template
	db          *sql.DB
}

func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == "/" {
		h.HandleHomePage(w, r)
		return
	}

	http.FileServer(http.FS(h.static)).ServeHTTP(w, r)
}

func (h *Handler) HandleHomePage(w http.ResponseWriter, r *http.Request) {
	posts, err := ListPosts(r.Context(), h.db)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list posts", "error", err)
		http.Error(w, "failed to list posts", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Posts": posts,
	}

	err = h.tmpl.ExecuteTemplate(w, "index-page.gohtml", data)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
		http.Error(w, "failed to execute template", http.StatusInternalServerError)
	}
}

func (h *Handler) HandleViewPostPage(w http.ResponseWriter, r *http.Request) {
	postSlug := r.PathValue("postSlug")
	post, err := GetPostBySlug(r.Context(), h.db, postSlug)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list posts", "error", err)
		http.Error(w, "failed to list posts", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Post": post,
	}

	err = h.tmpl.ExecuteTemplate(w, "single-post-page.gohtml", data)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
		http.Error(w, "failed to execute template", http.StatusInternalServerError)
	}
}
