package main

import (
	"cmp"
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
	"github.com/microcosm-cc/bluemonday"
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
	_ = slog.SetLogLoggerLevel(slog.LevelDebug)

	// Database
	dbDSN := env.GetString("DB_DSN", ":memory:")
	db, err := sql.Open("sqlite3", dbDSN)
	if err != nil {
		return fmt.Errorf("error on open database: %w", err)
	}

	defer db.Close()

	err = RunMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("error on run migrations: %w", err)
	}

	// Repositories
	userRepo := &UserRepository{db: db}
	postRepo := &PostRepository{db: db}
	commentRepo := &CommentRepository{db: db}

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
		"or": func(s1, s2 string) string {
			return cmp.Or(s1, s2)
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
	sessionName := env.GetString("SESSION_NAME", "blog")

	// HTTP Handler
	h := &Handler{
		static:      subStatic,
		cookieStore: cookieStore,
		sessionName: sessionName,
		tmpl:        tmpl,
		userRepo:    userRepo,
		postRepo:    postRepo,
		commentRepo: commentRepo,
		htmlPolicy:  bluemonday.UGCPolicy(),
	}

	mux := http.NewServeMux()

	// Routes
	mux.HandleFunc("GET /posts/{postSlug}", h.HandleViewPostPage)
	mux.Handle("GET /login", h.HandleLoginPage())
	mux.Handle("POST /login", h.HandleLogin())
	mux.Handle("GET /register", h.HandleRegisterPage())
	mux.Handle("POST /register", h.HandleRegister())
	mux.Handle("GET /logout", h.HandleLogoutPage())
	mux.Handle("POST /logout", h.HandleLogout())
	mux.Handle("POST /comments", h.HandleSubmitComment())
	mux.Handle("GET /comments/{commentId}/edit", h.HandleEditCommentPage())
	mux.Handle("POST /comments/{commentId}/edit", h.HandleEditComment())
	mux.Handle("GET /comments/{commentId}/delete", h.HandleDeleteCommentPage())
	mux.Handle("POST /comments/{commentId}/delete", h.HandleDeleteComment())
	mux.HandleFunc("GET /", h.HandleIndex)

	// CSRF Middleware
	csrfMW := csrf.Protect([]byte(env.MustGetString("CSRF_AUTH_KEY")), csrf.TrustedOrigins([]string{"localhost:8080"}))

	// GZip Middleware
	gzipMW := gziphandler.GzipHandler

	// Auth middleware
	authMW := h.AuthMiddleware()

	handler := gzipMW(csrfMW(h.RecoverMW(authMW(mux))))

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
