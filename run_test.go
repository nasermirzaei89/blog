package blog_test

import (
	"context"
	"database/sql"
	"html/template"
	"io/fs"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nasermirzaei89/blog"
	"github.com/nasermirzaei89/blog/mailer"
	"github.com/playwright-community/playwright-go"
)

func TestAll(t *testing.T) {
	ctx := context.Background()

	fsys := os.DirFS(".")

	static, err := fs.Sub(fsys, "static")
	if err != nil {
		t.Fatalf("could not get static files: %v", err)
	}

	cookieStore := sessions.NewCookieStore([]byte("test-session-key"))
	sessionName := "test-blog"

	tmpl, err := template.New("").Funcs(blog.Funcs).ParseFS(fsys, "templates/*.gohtml", "templates/icons/*.svg")
	if err != nil {
		t.Fatalf("could not parse templates: %v", err)
	}

	// Database
	dbDSN := ":memory:"
	db, err := sql.Open("sqlite3", dbDSN)
	if err != nil {
		t.Fatalf("could not open database: %v", err)
	}

	// defer func() {
	// err = errors.Join(err, db.Close())
	// }()

	err = blog.RunMigrations(ctx, db)
	if err != nil {
		t.Fatalf("could not run migrations: %v", err)
	}

	// Repositories
	userRepo := &blog.UserRepository{DB: db}
	postRepo := &blog.PostRepository{DB: db}
	commentRepo := &blog.CommentRepository{DB: db}
	passwordResetTokenRepo := &blog.PasswordResetTokenRepository{DB: db}

	mockMailer := &mailer.MockMailer{}

	handler := &blog.Handler{
		Static:                 static,
		CookieStore:            cookieStore,
		SessionName:            sessionName,
		Template:               tmpl,
		UserRepo:               userRepo,
		PostRepo:               postRepo,
		CommentRepo:            commentRepo,
		PasswordResetTokenRepo: passwordResetTokenRepo,
		Mailer:                 mockMailer,
		HTMLPolicy:             bluemonday.UGCPolicy(),
		TextPolicy:             bluemonday.StrictPolicy(),
		CSRFAuthKeys:           []byte("test-csrf-auth-key"),
		CSRFTrustedOrigins:     []string{},
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	handler.CSRFTrustedOrigins = []string{strings.TrimLeft(server.URL, "http://")}

	t.Run("App", func(t *testing.T) {
		err := playwright.Install()
		if err != nil {
			t.Fatalf("could not install Playwright: %v", err)
		}

		pw, err := playwright.Run()
		if err != nil {
			t.Fatalf("could not run Playwright: %v", err)
		}
		defer pw.Stop()

		browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(true),
		})
		if err != nil {
			t.Fatalf("could not launch browser: %v", err)
		}
		defer browser.Close()

		context, err := browser.NewContext()
		if err != nil {
			t.Fatalf("could not create context: %v", err)
		}
		defer context.Close()

		page, err := context.NewPage()
		if err != nil {
			t.Fatalf("could not create page: %v", err)
		}

		if _, err := page.Goto(server.URL); err != nil {
			t.Fatalf("could not go to page: %v", err)
		}

		title, err := page.Title()
		if err != nil {
			t.Fatalf("could not get title: %v", err)
		}

		if title != "Blog" {
			t.Errorf("expected title 'Blog', got '%s'", title)
		}

		t.Run("Register", func(t *testing.T) {
			if err := page.GetByText("Register").Click(); err != nil {
				t.Fatalf("could not click Register: %v", err)
			}

			username := "testuser"
			if err := page.Locator("input[name=username]").Fill(username); err != nil {
				t.Fatalf("could not fill username: %v", err)
			}

			emailAddress := "testuser@example.com"
			if err := page.Locator("input[name=emailAddress]").Fill(emailAddress); err != nil {
				t.Fatalf("could not fill email address: %v", err)
			}

			password := "password123"
			if err := page.Locator("input[name=password]").Fill(password); err != nil {
				t.Fatalf("could not fill password: %v", err)
			}

			if err := page.Locator("input[name=passwordConfirmation]").Fill(password); err != nil {
				t.Fatalf("could not fill password confirmation: %v", err)
			}

			if err := page.GetByText("Sign Up").Click(); err != nil {
				t.Fatalf("could not click sign up button: %v", err)
			}

			logoutLocator := page.GetByText("Logout")
			if err := logoutLocator.WaitFor(playwright.LocatorWaitForOptions{
				State:   playwright.WaitForSelectorStateVisible,
				Timeout: playwright.Float(5000),
			}); err != nil {
				t.Fatalf("registration failed or user not logged in: %v", err)
			}

			t.Run("Logout", func(t *testing.T) {
				if err := page.GetByText("Logout").Click(); err != nil {
					t.Fatalf("could not click logout: %v", err)
				}

				if err := page.GetByText("Sign Out").Click(); err != nil {
					t.Fatalf("could not click sign out: %v", err)
				}

				loginLocator := page.GetByText("Login")
				if err := loginLocator.WaitFor(playwright.LocatorWaitForOptions{
					State:   playwright.WaitForSelectorStateVisible,
					Timeout: playwright.Float(5000),
				}); err != nil {
					t.Fatalf("registration failed or user not logged in: %v", err)
				}

				t.Run("Login", func(t *testing.T) {
					if err := page.GetByText("Login").Click(); err != nil {
						t.Fatalf("could not click Login: %v", err)
					}

					if err := page.Locator("input[name=username]").Fill(username); err != nil {
						t.Fatalf("could not fill username: %v", err)
					}

					if err := page.Locator("input[name=password]").Fill(password); err != nil {
						t.Fatalf("could not fill password: %v", err)
					}

					if err := page.GetByText("Sign In").Click(); err != nil {
						t.Fatalf("could not click sign in: %v", err)
					}
				})
			})
		})
	})
}
