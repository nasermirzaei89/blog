package fullstackgo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/sessions"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/mattn/go-sqlite3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nasermirzaei89/env"
	"github.com/nasermirzaei89/fullstackgo/db/sqlite3"
	"github.com/nasermirzaei89/fullstackgo/mailer"
	"github.com/nasermirzaei89/fullstackgo/web"
)

const (
	HTTPServerTimeOut = 60 * time.Second
)

func Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
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

	err = sqlite3.RunMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("error on run migrations: %w", err)
	}

	// Repositories
	userRepo := &sqlite3.UserRepo{DB: db}
	postRepo := &sqlite3.PostRepo{DB: db}
	commentRepo := &sqlite3.CommentRepo{DB: db}
	passwordResetTokenRepo := &sqlite3.PasswordResetTokenRepo{DB: db}

	// Session
	cookieStore := sessions.NewCookieStore([]byte(env.MustGetString("SESSION_KEY")))
	sessionName := env.GetString("SESSION_NAME", "fullstackgo")

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
	handler := &web.Handler{
		CookieStore:            cookieStore,
		SessionName:            sessionName,
		UserRepo:               userRepo,
		PostRepo:               postRepo,
		CommentRepo:            commentRepo,
		PasswordResetTokenRepo: passwordResetTokenRepo,
		Mailer:                 smtpMailer,
		HTMLPolicy:             bluemonday.UGCPolicy(),
		TextPolicy:             bluemonday.StrictPolicy(),
		CSRFAuthKeys:           []byte(env.MustGetString("CSRF_AUTH_KEY")),
		CSRFTrustedOrigins:     env.GetStringSlice("CSRF_TRUSTED_ORIGINS", []string{}),
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
		slog.InfoContext(ctx, "starting server", "address", address)

		serverErr <- server.ListenAndServe()
	}()

	// Wait for interruption.
	select {
	case err = <-serverErr:
		return err
	case <-ctx.Done():
		slog.Info("interrupt signal received")
		slog.Info("shutting down server gracefully...")
		handler.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = server.Shutdown(ctx) //nolint:contextcheck
		if err != nil {
			return fmt.Errorf("error shutting down server: %w", err)
		}
	}

	return nil
}
