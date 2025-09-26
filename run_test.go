package fullstackgo_test

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nasermirzaei89/fullstackgo/db/sqlite3"
	"github.com/nasermirzaei89/fullstackgo/mailer"
	"github.com/nasermirzaei89/fullstackgo/web"
	"github.com/playwright-community/playwright-go"
)

func TestAll(t *testing.T) {
	server := runServer(t)
	defer server.Close()

	t.Run("App", runApp(server.URL))
}

func runApp(serverURL string) func(t *testing.T) {
	return func(t *testing.T) {
		err := playwright.Install()
		if err != nil {
			t.Fatalf("could not install Playwright: %v", err)
		}

		pw, err := playwright.Run()
		if err != nil {
			t.Fatalf("could not run Playwright: %v", err)
		}

		defer func() { _ = pw.Stop() }()

		browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(true),
		})
		if err != nil {
			t.Fatalf("could not launch browser: %v", err)
		}

		defer func() { _ = browser.Close() }()

		context, err := browser.NewContext()
		if err != nil {
			t.Fatalf("could not create context: %v", err)
		}

		defer func() { _ = context.Close() }()

		page, err := context.NewPage()
		if err != nil {
			t.Fatalf("could not create page: %v", err)
		}

		_, err = page.Goto(serverURL)
		if err != nil {
			t.Fatalf("could not go to page: %v", err)
		}

		title, err := page.Title()
		if err != nil {
			t.Fatalf("could not get title: %v", err)
		}

		expectedTitle := "My Awesome Blog"

		if title != expectedTitle {
			t.Errorf("expected title '%s', got '%s'", expectedTitle, title)
		}

		t.Run("Register", runRegister(page))
	}
}

func runRegister(page playwright.Page) func(t *testing.T) {
	return func(t *testing.T) {
		err := page.GetByText("Register").Click()
		if err != nil {
			t.Fatalf("could not click Register: %v", err)
		}

		username := "testuser"

		err = page.Locator("input[name=username]").Fill(username)
		if err != nil {
			t.Fatalf("could not fill username: %v", err)
		}

		emailAddress := "testuser@example.com"

		err = page.Locator("input[name=emailAddress]").Fill(emailAddress)
		if err != nil {
			t.Fatalf("could not fill email address: %v", err)
		}

		password := "password123"

		err = page.Locator("input[name=password]").Fill(password)
		if err != nil {
			t.Fatalf("could not fill password: %v", err)
		}

		err = page.Locator("input[name=passwordConfirmation]").Fill(password)
		if err != nil {
			t.Fatalf("could not fill password confirmation: %v", err)
		}

		err = page.GetByText("Sign Up").Click()
		if err != nil {
			t.Fatalf("could not click sign up button: %v", err)
		}

		logoutLocator := page.GetByText("Logout")

		err = logoutLocator.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(5000),
		})
		if err != nil {
			t.Fatalf("registration failed or user not logged in: %v", err)
		}

		t.Run("Logout", runLogout(page, username, password))
		t.Run("Update Profile", runUpdateProfile(page))
		t.Run("Change Password", runChangePassword(page, username))
	}
}

func runUpdateProfile(page playwright.Page) func(t *testing.T) {
	return func(t *testing.T) {
		err := page.GetByRole("link", playwright.PageGetByRoleOptions{Name: "Profile"}).Click()
		if err != nil {
			t.Fatalf("could not click Profile: %v", err)
		}

		newName := "Updated User"

		err = page.Locator("input[name=name]").Fill(newName)
		if err != nil {
			t.Fatalf("could not fill name: %v", err)
		}

		newEmail := "updateduser@example.com"

		err = page.Locator("input[name=emailAddress]").Fill(newEmail)
		if err != nil {
			t.Fatalf("could not fill email address: %v", err)
		}

		err = page.Locator("input[name=avatarUrl]").Fill("https://i.pravatar.cc/100?u=" + newEmail)
		if err != nil {
			t.Fatalf("could not fill avatar URL: %v", err)
		}

		err = page.GetByText("Update Profile").Click()
		if err != nil {
			t.Fatalf("could not click update profile: %v", err)
		}
	}
}

func runChangePassword(page playwright.Page, username string) func(t *testing.T) {
	return func(t *testing.T) {
		err := page.GetByRole("link", playwright.PageGetByRoleOptions{Name: "Profile"}).Click()
		if err != nil {
			t.Fatalf("could not click profile: %v", err)
		}

		currentPassword := "password123"

		err = page.Locator("input[name=currentPassword]").Fill(currentPassword)
		if err != nil {
			t.Fatalf("could not fill current password: %v", err)
		}

		newPassword := "newpassword456"

		err = page.Locator("input[name=newPassword]").Fill(newPassword)
		if err != nil {
			t.Fatalf("could not fill new password: %v", err)
		}

		err = page.Locator("input[name=newPasswordConfirmation]").Fill(newPassword)
		if err != nil {
			t.Fatalf("could not fill new password confirmation: %v", err)
		}

		err = page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Change Password"}).Click()
		if err != nil {
			t.Fatalf("could not click change password: %v", err)
		}

		t.Run("Logout", runLogout(page, username, newPassword))
	}
}

func runLogout(page playwright.Page, username, password string) func(t *testing.T) {
	return func(t *testing.T) {
		err := page.GetByText("Logout").Click()
		if err != nil {
			t.Fatalf("could not click logout: %v", err)
		}

		err = page.GetByText("Sign Out").Click()
		if err != nil {
			t.Fatalf("could not click sign out: %v", err)
		}

		loginLocator := page.GetByText("Login")

		err = loginLocator.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(5000),
		})
		if err != nil {
			t.Fatalf("registration failed or user not logged in: %v", err)
		}

		t.Run("Login", runLogin(page, username, password))
	}
}

func runLogin(page playwright.Page, username, password string) func(t *testing.T) {
	return func(t *testing.T) {
		err := page.GetByText("Login").Click()
		if err != nil {
			t.Fatalf("could not click Login: %v", err)
		}

		err = page.Locator("input[name=username]").Fill(username)
		if err != nil {
			t.Fatalf("could not fill username: %v", err)
		}

		err = page.Locator("input[name=password]").Fill(password)
		if err != nil {
			t.Fatalf("could not fill password: %v", err)
		}

		err = page.GetByText("Sign In").Click()
		if err != nil {
			t.Fatalf("could not click sign in: %v", err)
		}
	}
}

func runServer(t *testing.T) *httptest.Server {
	t.Helper()

	ctx := context.Background()

	cookieStore := sessions.NewCookieStore([]byte("test-session-key"))
	sessionName := "fullstackgo-test"

	// Database
	dbDSN := ":memory:"

	db, err := sql.Open("sqlite3", dbDSN)
	if err != nil {
		t.Fatalf("could not open database: %v", err)
	}

	// defer func() {
	// err = errors.Join(err, db.Close())
	// }()

	err = sqlite3.RunMigrations(ctx, db)
	if err != nil {
		t.Fatalf("could not run migrations: %v", err)
	}

	// Repositories
	userRepo := &sqlite3.UserRepo{DB: db}
	postRepo := &sqlite3.PostRepo{DB: db}
	commentRepo := &sqlite3.CommentRepo{DB: db}
	passwordResetTokenRepo := &sqlite3.PasswordResetTokenRepo{DB: db}

	mockMailer := &mailer.MockMailer{}

	handler := &web.Handler{
		CookieStore:            cookieStore,
		SessionName:            sessionName,
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

	handler.CSRFTrustedOrigins = []string{strings.TrimLeft(server.URL, "http://")}

	t.Logf("Server running at %s", server.URL)

	return server
}
