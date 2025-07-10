package main

import (
	"cmp"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/mattn/go-sqlite3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nasermirzaei89/env"
)

//go:embed templates/* static/*
var embeddedFS embed.FS

const (
	HTTPServerTimeOut = 60 * time.Second
)

func main() {
	slog.Info("starting app...")

	err := run()
	if err != nil {
		slog.Error("failed to run application", "error", err)
		os.Exit(1)
	}

	slog.Info("app ran successfully")
}

func run() error {
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Database
	dbDSN := env.GetString("DB_DSN", ":memory:")
	db, err := sql.Open("sqlite3", dbDSN)
	if err != nil {
		return fmt.Errorf("error on open database: %w", err)
	}

	defer func() {
		err = errors.Join(err, db.Close())
	}()

	err = RunMigrations(rootCtx, db)
	if err != nil {
		return fmt.Errorf("error on run migrations: %w", err)
	}

	// Repositories
	userRepo := &UserRepository{db: db}
	postRepo := &PostRepository{db: db}
	commentRepo := &CommentRepository{db: db}

	//
	static, err := fs.Sub(embeddedFS, "static")
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
	}).ParseFS(embeddedFS, "templates/*.gohtml", "templates/icons/*.svg")
	if err != nil {
		return fmt.Errorf("error on parse templates: %w", err)
	}

	//
	cookieStore := sessions.NewCookieStore([]byte(env.MustGetString("SESSION_KEY")))
	sessionName := env.GetString("SESSION_NAME", "blog")

	// HTTP Handler
	h := &Handler{
		static:      static,
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
	mux.HandleFunc("GET /healthz", h.HandleHealthz)
	mux.HandleFunc("GET /", h.HandleIndex)

	// CSRF Middleware
	csrfMW := csrf.Protect([]byte(env.MustGetString("CSRF_AUTH_KEY")), csrf.TrustedOrigins(env.GetStringSlice("CSRF_TRUSTED_ORIGINS", []string{"localhost:8080"})))

	// GZip Middleware
	gzipMW := gziphandler.GzipHandler

	// Auth middleware
	authMW := h.AuthMiddleware()

	handler := gzipMW(csrfMW(h.RecoverMW(authMW(mux))))

	// HTTP Server
	host := env.GetString("HOST", "")
	port := env.GetString("PORT", "8080")
	address := host + ":" + port

	server := http.Server{
		Addr:              address,
		Handler:           handler,
		ReadTimeout:       HTTPServerTimeOut,
		ReadHeaderTimeout: HTTPServerTimeOut,
		WriteTimeout:      HTTPServerTimeOut,
		IdleTimeout:       HTTPServerTimeOut,
	}

	serverErr := make(chan error, 1)

	go func() {
		slog.InfoContext(rootCtx, "starting server", "address", address)

		serverErr <- server.ListenAndServe()
	}()

	// Wait for interruption.
	select {
	case err = <-serverErr:
		return err
	case <-rootCtx.Done():
		slog.Info("interrupt signal received")
		slog.Info("shutting down server gracefully...")
		h.isShuttingDown.Store(true)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = server.Shutdown(shutdownCtx)
		if err != nil {
			return fmt.Errorf("error shutting down server: %w", err)
		}
	}

	return nil
}
