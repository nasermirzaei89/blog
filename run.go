package blog

import (
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

	"github.com/gorilla/sessions"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/mattn/go-sqlite3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nasermirzaei89/blog/mailer"
	"github.com/nasermirzaei89/env"
)

//go:embed templates/* static/*
var embeddedFS embed.FS

const (
	HTTPServerTimeOut = 60 * time.Second
)

func Run(ctx context.Context) error {
	rootCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
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
	userRepo := &UserRepo{DB: db}
	postRepo := &PostRepo{DB: db}
	commentRepo := &CommentRepo{DB: db}
	passwordResetTokenRepo := &PasswordResetTokenRepo{DB: db}

	//
	static, err := fs.Sub(embeddedFS, "static")
	if err != nil {
		return fmt.Errorf("failed to get static folder as sub: %w", err)
	}

	//
	tmpl, err := template.New("").
		Funcs(Funcs).
		ParseFS(embeddedFS, "templates/*.gohtml", "templates/icons/*.svg")
	if err != nil {
		return fmt.Errorf("error on parse templates: %w", err)
	}

	//
	cookieStore := sessions.NewCookieStore([]byte(env.MustGetString("SESSION_KEY")))
	sessionName := env.GetString("SESSION_NAME", "blog")

	// Mailer
	smtpHost := env.MustGetString("SMTP_HOST")
	smtpPort := env.MustGetString("SMTP_PORT")
	smtpFrom := env.MustGetString("SMTP_FROM")
	smtpMailer := &mailer.SMTPMailer{
		Host:     smtpHost,
		Port:     smtpPort,
		Username: env.GetString("SMTP_USERNAME", ""),
		Password: env.GetString("SMTP_PASSWORD", ""),
		From:     smtpFrom,
	}

	// HTTP Handler
	handler := &Handler{
		Static:                 static,
		CookieStore:            cookieStore,
		SessionName:            sessionName,
		Template:               tmpl,
		UserRepo:               userRepo,
		PostRepo:               postRepo,
		CommentRepo:            commentRepo,
		PasswordResetTokenRepo: passwordResetTokenRepo,
		Mailer:                 smtpMailer,
		HTMLPolicy:             bluemonday.UGCPolicy(),
		TextPolicy:             bluemonday.StrictPolicy(),
		CSRFAuthKeys:           []byte(env.MustGetString("CSRF_AUTH_KEY")),
		CSRFTrustedOrigins: env.GetStringSlice(
			"CSRF_TRUSTED_ORIGINS",
			[]string{"localhost:8080"},
		),
	}

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
		handler.isShuttingDown.Store(true)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = server.Shutdown(shutdownCtx) //nolint:contextcheck
		if err != nil {
			return fmt.Errorf("error shutting down server: %w", err)
		}
	}

	return nil
}
