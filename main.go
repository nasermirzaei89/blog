package main

import (
	"cmp"
	"embed"
	"fmt"
	"github.com/gorilla/sessions"
	"html/template"
	"log"
	"net/http"
	"os"
)

//go:embed templates
var templates embed.FS

func main() {
	fs, err := template.New("templates").
		Funcs(funcMap).
		ParseFS(templates, "templates/*")
	if err != nil {
		panic(fmt.Errorf("failed to parse templates: %w", err))
	}

	mux := http.NewServeMux()

	username := cmp.Or(os.Getenv("ADMIN_USERNAME"), "admin")
	password := cmp.Or(os.Getenv("ADMIN_PASSWORD"), "admin")

	key := cmp.Or(os.Getenv("STORE_KEY"), "super-secret-key")
	cookieStore := sessions.NewCookieStore([]byte(key))
	cookieStore.Options = &sessions.Options{
		Path:        "",
		Domain:      "",
		MaxAge:      0,
		Secure:      false,
		HttpOnly:    false,
		Partitioned: false,
		SameSite:    0,
	}

	h := Handler{
		tpl:         fs,
		username:    username,
		password:    password,
		cookieStore: cookieStore,
	}

	mux.Handle("GET /", h.HomePageHandler())
	mux.Handle("GET /posts/{postSlug}", h.SinglePostPageHandler())
	mux.Handle("GET /login", h.LoginPageHandler())
	mux.Handle("POST /login", h.LoginHandler())
	mux.Handle("GET /logout", h.LogoutHandler())
	mux.Handle("GET /admin", h.AdminPageHandler())
	mux.Handle("GET /admin/posts", h.AdminPostsPageHandler())
	mux.Handle("GET /admin/posts/new", h.AdminNewPostPageHandler())
	mux.Handle("POST /admin/posts/new", h.AdminCreateNewPostHandler())
	mux.Handle("GET /admin/posts/{postUuid}/edit", h.AdminEditPostPageHandler())
	mux.Handle("POST /admin/posts/{postUuid}/edit", h.AdminUpdatePostHandler())
	mux.Handle("GET /admin/posts/{postUuid}/delete", h.AdminDeletePostHandler())
	mux.Handle("GET /admin/settings", h.AdminSettingsPageHandler())
	mux.Handle("POST /admin/settings", h.AdminUpdateSettingsHandler())

	host := cmp.Or(os.Getenv("HOST"), "0.0.0.0")
	port := cmp.Or(os.Getenv("PORT"), "8888")

	addr := fmt.Sprintf("%s:%s", host, port)

	log.Printf("Listening on http://%s", addr)

	err = http.ListenAndServe(addr, mux)
	if err != nil {
		panic(fmt.Errorf("failed to start server: %w", err))
	}
}
