package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nasermirzaei89/env"
)

//go:embed templates/*
var templates embed.FS

//go:embed static/*
var static embed.FS

func main() {
	ctx := context.Background()

	err := run(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to run application", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// Database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return fmt.Errorf("error on open database: %w", err)
	}

	defer db.Close()

	err = RunMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("error on run migrations: %w", err)
	}

	//
	subStatic, err := fs.Sub(static, "static")
	if err != nil {
		return fmt.Errorf("failed to get static folder as sub: %w", err)
	}

	//
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"formatTime": func(t time.Time, layout string) string {
			return t.Format(layout)
		},
		"_lang": func() string {
			return "en"
		},
		"_dir": func() string {
			return "ltr"
		},
	}).ParseFS(templates, "templates/*.gohtml", "templates/icons/*.svg")
	if err != nil {
		return fmt.Errorf("error on parse templates: %w", err)
	}

	//
	cookieStore := sessions.NewCookieStore([]byte(env.MustGetString("SESSION_KEY")))
	sessionName := env.GetString("SESSION_NAME", "applicaset")

	// HTTP Handler
	h := &Handler{
		static:      subStatic,
		cookieStore: cookieStore,
		sessionName: sessionName,
		tmpl:        tmpl,
		db:          db,
	}

	mux := http.NewServeMux()

	// Routes
	mux.HandleFunc("GET /posts/{postSlug}", h.HandleViewPostPage)
	mux.HandleFunc("GET /", h.HandleIndex)

	// CSRF Middleware
	csrfMW := csrf.Protect([]byte(env.MustGetString("CSRF_AUTH_KEY")), csrf.TrustedOrigins([]string{"localhost:8080"}))

	// GZip Middleware
	gzipMW := gziphandler.GzipHandler

	handler := gzipMW(csrfMW(mux))

	// HTTP Server
	host := env.GetString("HOST", "")
	port := env.GetString("PORT", "8080")
	address := host + ":" + port

	slog.InfoContext(ctx, "starting server", "address", address)

	err = http.ListenAndServe(address, handler)
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
