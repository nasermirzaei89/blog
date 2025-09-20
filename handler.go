package blog

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	slugify "github.com/gosimple/slug"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nasermirzaei89/blog/mailer"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	handler                http.Handler
	Static                 fs.FS
	CookieStore            *sessions.CookieStore
	SessionName            string
	Template               *template.Template
	UserRepo               *UserRepository
	PostRepo               *PostRepository
	CommentRepo            *CommentRepository
	PasswordResetTokenRepo *PasswordResetTokenRepository
	Mailer                 mailer.Mailer
	HTMLPolicy             *bluemonday.Policy
	TextPolicy             *bluemonday.Policy
	CSRFAuthKeys           []byte
	CSRFTrustedOrigins     []string
	isShuttingDown         atomic.Bool
}

type contextKeyUserType struct{}

var contextKeyUser = contextKeyUserType{}

type NotificationType string

const (
	NotificationTypeInfo    NotificationType = "info"
	NotificationTypeWarning NotificationType = "warning"
	NotificationTypeError   NotificationType = "error"
	NotificationTypeSuccess NotificationType = "success"
)

type Notification struct {
	Type    NotificationType
	Message string
}

func init() {
	// Register type for gob encoding
	gob.Register(Notification{})
	gob.Register(map[string]any{})
}

var _ http.Handler = (*Handler)(nil)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.handler == nil {
		mux := http.NewServeMux()

		// Routes
		mux.HandleFunc("GET /healthz", h.HandleHealthz)

		mux.Handle("GET /login", h.HandleLoginPage())
		mux.Handle("POST /login", h.HandleLogin())
		mux.Handle("GET /register", h.HandleRegisterPage())
		mux.Handle("POST /register", h.HandleRegister())
		mux.Handle("GET /logout", h.HandleLogoutPage())
		mux.Handle("POST /logout", h.HandleLogout())
		mux.Handle("GET /forgot-password", h.HandleForgotPasswordPage())
		mux.Handle("POST /forgot-password", h.HandleForgotPassword())
		mux.Handle("GET /reset-password", h.HandleResetPasswordPage())
		mux.Handle("POST /reset-password", h.HandleResetPassword())

		mux.Handle("GET /profile", h.HandleProfilePage())
		mux.Handle("POST /profile", h.HandleProfileUpdate())
		mux.Handle("POST /profile/password", h.HandleProfilePasswordUpdate())

		mux.Handle("GET /posts/{postSlug}", h.HandleViewPostPage())
		mux.Handle("GET /posts/new", h.HandleNewPostPage())
		mux.Handle("POST /posts", h.HandleCreatePost())
		mux.Handle("GET /posts/{postSlug}/edit", h.HandleEditPostPage())
		mux.Handle("POST /posts/{postSlug}/edit", h.HandleEditPost())
		mux.Handle("GET /posts/{postSlug}/delete", h.HandleDeletePostPage())
		mux.Handle("POST /posts/{postSlug}/delete", h.HandleDeletePost())

		mux.Handle("POST /comments", h.HandleSubmitComment())
		mux.Handle("GET /comments/{commentId}/edit", h.HandleEditCommentPage())
		mux.Handle("POST /comments/{commentId}/edit", h.HandleEditComment())
		mux.Handle("GET /comments/{commentId}/delete", h.HandleDeleteCommentPage())
		mux.Handle("POST /comments/{commentId}/delete", h.HandleDeleteComment())

		mux.HandleFunc("GET /", h.HandleIndex)

		// CSRF Middleware
		csrfMW := csrf.Protect(h.CSRFAuthKeys, csrf.TrustedOrigins(h.CSRFTrustedOrigins))

		// GZip Middleware
		gzipMW := gziphandler.GzipHandler

		// Auth middleware
		authMW := h.AuthMiddleware()

		h.handler = gzipMW(csrfMW(h.RecoverMW(authMW(mux))))
	}

	h.handler.ServeHTTP(w, r)
}

type SessionValueNotFoundError struct {
	Key string
}

func (err SessionValueNotFoundError) Error() string {
	return fmt.Sprintf("session value not found for key: %s", err.Key)
}

func (h *Handler) getSessionValue(r *http.Request, key string) (any, error) {
	session, err := h.CookieStore.Get(r, h.SessionName)
	if err != nil {
		return nil, fmt.Errorf("error getting session: %w", err)
	}

	value, ok := session.Values[key]
	if !ok {
		return nil, SessionValueNotFoundError{Key: key}
	}

	return value, nil
}

func (h *Handler) setSessionValue(w http.ResponseWriter, r *http.Request, key string, value any) error {
	session, err := h.CookieStore.Get(r, h.SessionName)
	if err != nil {
		return fmt.Errorf("error getting session: %w", err)
	}

	session.Values[key] = value
	err = session.Save(r, w)
	if err != nil {
		return fmt.Errorf("error saving session: %w", err)
	}

	return nil
}

func (h *Handler) deleteSessionValue(w http.ResponseWriter, r *http.Request, key string) error {
	session, err := h.CookieStore.Get(r, h.SessionName)
	if err != nil {
		return fmt.Errorf("error getting session: %w", err)
	}

	delete(session.Values, key)
	err = session.Save(r, w)
	if err != nil {
		return fmt.Errorf("error saving session: %w", err)
	}

	return nil
}

func (h *Handler) getSessionFlash(w http.ResponseWriter, r *http.Request, key string) ([]any, error) {
	session, err := h.CookieStore.Get(r, h.SessionName)
	if err != nil {
		return nil, fmt.Errorf("error getting session: %w", err)
	}

	res := session.Flashes(key)

	err = session.Save(r, w)
	if err != nil {
		return nil, fmt.Errorf("error saving session: %w", err)
	}

	return res, nil
}

func (h *Handler) addSessionFlash(w http.ResponseWriter, r *http.Request, key string, value any) error {
	session, err := h.CookieStore.Get(r, h.SessionName)
	if err != nil {
		return fmt.Errorf("error getting session: %w", err)
	}

	session.AddFlash(value, key)
	err = session.Save(r, w)
	if err != nil {
		return fmt.Errorf("error saving session: %w", err)
	}

	return nil
}

func (h *Handler) RecoverMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.ErrorContext(r.Context(), "recovered from panic", "error", err, "stack", debug.Stack())
				http.Error(w, "internal error occured", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) AuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, err := h.getSessionValue(r, "username")
			if err != nil && !errors.As(err, &SessionValueNotFoundError{}) {
				slog.ErrorContext(r.Context(), "error on getting session value", "key", "username", "error", err)
				http.Error(w, "error on getting session value", http.StatusInternalServerError)
				return
			}

			if username != nil && username.(string) != "" {
				user, err := h.UserRepo.GetByUsername(r.Context(), username.(string))
				if err != nil {
					if errors.As(err, &UserByUsernameNotFoundError{}) {
						err = h.deleteSessionValue(w, r, "username")
						if err != nil {
							slog.ErrorContext(r.Context(), "error on deleting session value", "key", "username", "error", err)
							http.Error(w, "error on deleting session value", http.StatusInternalServerError)
							return
						}

						next.ServeHTTP(w, r)
						return
					}

					slog.ErrorContext(r.Context(), "error retrieving user", "error", err)
					http.Error(w, "error on retrieving user", http.StatusInternalServerError)

					return
				}

				r = r.WithContext(context.WithValue(r.Context(), contextKeyUser, user))
			}

			next.ServeHTTP(w, r)
		})
	}
}

func userFromContext(ctx context.Context) *User {
	user, ok := ctx.Value(contextKeyUser).(*User)
	if !ok {
		return nil
	}

	return user
}

func (h *Handler) notificationsFromSession(w http.ResponseWriter, r *http.Request) []Notification {
	values, err := h.getSessionFlash(w, r, "notifications")
	if err != nil {
		return nil
	}

	notifications := make([]Notification, 0, len(values))
	for _, v := range values {
		if n, ok := v.(Notification); ok {
			notifications = append(notifications, n)
		}
	}

	return notifications
}

func (h *Handler) addNotificationToSession(w http.ResponseWriter, r *http.Request, n Notification) {
	err := h.addSessionFlash(w, r, "notifications", n)
	if err != nil {
		slog.ErrorContext(r.Context(), "error adding notification to session", "error", err)
	}
}

func (h *Handler) addErrorMessage(w http.ResponseWriter, r *http.Request, message string) {
	h.addNotificationToSession(w, r, Notification{
		Message: message,
		Type:    NotificationTypeError,
	})
}

func (h *Handler) addSuccessMessage(w http.ResponseWriter, r *http.Request, message string) {
	h.addNotificationToSession(w, r, Notification{
		Message: message,
		Type:    NotificationTypeSuccess,
	})
}

func (h *Handler) addInfoMessage(w http.ResponseWriter, r *http.Request, message string) {
	h.addNotificationToSession(w, r, Notification{
		Message: message,
		Type:    NotificationTypeInfo,
	})
}

func (h *Handler) addWarningMessage(w http.ResponseWriter, r *http.Request, message string) {
	h.addNotificationToSession(w, r, Notification{
		Message: message,
		Type:    NotificationTypeWarning,
	})
}

func (h *Handler) formErrorsFromSession(w http.ResponseWriter, r *http.Request) map[string]any {
	slog.DebugContext(r.Context(), "retrieving form errors from session")
	values, err := h.getSessionFlash(w, r, "formErrors")
	if err != nil {
		slog.ErrorContext(r.Context(), "error getting form errors from session", "error", err)
		return nil
	}

	slog.DebugContext(r.Context(), "form errors from session", "values", values)

	errors := make(map[string]any)
	for _, iv := range values {
		if v, ok := iv.(map[string]any); ok {
			maps.Copy(errors, v)
		}
	}

	return errors
}

func (h *Handler) addFormErrorsToSession(w http.ResponseWriter, r *http.Request, formID string, val any) error {
	slog.DebugContext(r.Context(), "adding form errors to session", "formID", formID, "val", val)
	err := h.addSessionFlash(w, r, "formErrors", map[string]any{formID: val})
	if err != nil {
		return fmt.Errorf("error adding form error to session: %w", err)
	}

	return nil
}

func (h *Handler) AuthenticatedOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userFromContext(r.Context()) == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) GuestOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userFromContext(r.Context()) != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) renderTemplate(w http.ResponseWriter, r *http.Request, name string, extraData map[string]any) {
	data := map[string]any{
		"CurrentUser":   userFromContext(r.Context()),
		"CurrentPath":   r.URL.Path,
		"Notifications": h.notificationsFromSession(w, r),
		"FormErrors":    h.formErrorsFromSession(w, r),
		"Lang":          "en",
		"Dir":           "ltr",
	}

	maps.Copy(data, extraData)

	err := h.Template.ExecuteTemplate(w, name, data)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
		http.Error(w, "failed to execute template", http.StatusInternalServerError)
	}
}

func (h *Handler) HandleStatic(w http.ResponseWriter, r *http.Request) {
	// w.Header().Set("Cache-Control", "public, max-age=3600")
	http.FileServer(http.FS(h.Static)).ServeHTTP(w, r)
}

func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		h.HandleHomePage(w, r)
		return
	}

	h.HandleStatic(w, r)
}

func (h *Handler) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	if h.isShuttingDown.Load() {
		http.Error(w, "Shutting down", http.StatusServiceUnavailable)
		return
	}

	w.Write([]byte("OK"))
}

func (h *Handler) HandleHomePage(w http.ResponseWriter, r *http.Request) {
	listPostsParams := ListPostsParams{
		Limit:  10,
		Offset: 0,
	}

	pageNum := 1

	page := r.URL.Query().Get("page")
	if page != "" {
		var err error

		pageNum, err = strconv.Atoi(page)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse page number", "error", err, "page", page)
			http.Error(w, "invalid page number", http.StatusBadRequest)
			return
		}

		if pageNum < 1 {
			slog.ErrorContext(r.Context(), "invalid page number", "page", pageNum)
			http.Error(w, "invalid page number", http.StatusBadRequest)
			return
		}

		listPostsParams.Offset = (pageNum - 1) * listPostsParams.Limit
	}

	posts, err := h.PostRepo.List(r.Context(), listPostsParams)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list posts", "error", err)
		http.Error(w, "failed to list posts", http.StatusInternalServerError)
		return
	}

	totalPosts, err := h.PostRepo.Count(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to count posts", "error", err)
		http.Error(w, "failed to count posts", http.StatusInternalServerError)
		return
	}

	totalPages := (totalPosts + listPostsParams.Limit - 1) / listPostsParams.Limit

	data := map[string]any{
		"Posts":       posts,
		"CurrentPage": pageNum,
		"TotalPages":  totalPages,
	}

	h.renderTemplate(w, r, "home-page.gohtml", data)
}

func (h *Handler) HandleLoginPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
		}

		h.renderTemplate(w, r, "login-page.gohtml", data)
	})

	return h.GuestOnly(hf)
}

func (h *Handler) HandleLogin() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse form", "error", err)
			http.Error(w, "error on parse form", http.StatusInternalServerError)

			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		user, err := h.UserRepo.GetByUsername(r.Context(), username)
		if err != nil {
			if errors.As(err, &UserByUsernameNotFoundError{}) {
				h.addErrorMessage(w, r, "Invalid username or password.")
				w.WriteHeader(http.StatusUnauthorized)
				h.HandleLoginPage().ServeHTTP(w, r)

				return
			}

			slog.ErrorContext(r.Context(), "error retrieving user", "error", err)
			http.Error(w, "error on retrieving user", http.StatusInternalServerError)

			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
		if err != nil {
			h.addErrorMessage(w, r, "Invalid username or password.")
			w.WriteHeader(http.StatusUnauthorized)
			h.HandleLoginPage().ServeHTTP(w, r)

			return
		}

		err = h.setSessionValue(w, r, "username", user.Username)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on setting session value", "key", "username", "error", err)
			http.Error(w, "error on setting session value", http.StatusInternalServerError)
			return
		}

		h.addSuccessMessage(w, r, "Logged in successfully.")

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return h.GuestOnly(hf)
}

func (h *Handler) HandleRegisterPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
		}

		h.renderTemplate(w, r, "register-page.gohtml", data)
	})

	return h.GuestOnly(hf)
}

func (h *Handler) HandleRegister() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse form", "error", err)
			http.Error(w, "error on parse form", http.StatusInternalServerError)

			return
		}

		username := r.FormValue("username")
		emailAddress := r.FormValue("emailAddress")
		password := r.FormValue("password")
		passwordConfirmation := r.FormValue("passwordConfirmation")

		if password != passwordConfirmation {
			http.Error(w, "password and confirmation doesn't match", http.StatusBadRequest)

			return
		}

		// FIXME: what to do on security?
		usernameExists, err := h.UserRepo.ExistsByUsername(r.Context(), username)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on checking user by username", "error", err, "username", username)
			http.Error(w, "error on checking user", http.StatusInternalServerError)

			return
		}

		if usernameExists {
			http.Error(w, "username already taken", http.StatusBadRequest)

			return
		}

		// FIXME: what to do on security?
		emailAddressExists, err := h.UserRepo.ExistsByEmailAddress(r.Context(), emailAddress)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on checking user by email address", "error", err, "emailAddress", emailAddress)
			http.Error(w, "error on checking user", http.StatusInternalServerError)

			return
		}

		if emailAddressExists {
			http.Error(w, "email address already exists", http.StatusBadRequest)

			return
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on hash password", "error", err)
			http.Error(w, "error on hash password", http.StatusInternalServerError)

			return
		}

		timeNow := time.Now()

		user := &User{
			ID:           uuid.NewString(),
			Username:     username,
			EmailAddress: emailAddress,
			PasswordHash: string(passwordHash),
			Name:         username,
			AvatarURL:    "",
			CreatedAt:    timeNow,
			UpdatedAt:    timeNow,
		}

		err = h.UserRepo.Create(r.Context(), user)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on create user", "error", err)
			http.Error(w, "error on create user", http.StatusInternalServerError)

			return
		}

		err = h.setSessionValue(w, r, "username", user.Username)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on setting session value", "key", "username", "error", err)
			http.Error(w, "error on setting session value", http.StatusInternalServerError)
			return
		}

		h.addSuccessMessage(w, r, "User has been registered successfully.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return h.GuestOnly(hf)
}

func (h *Handler) HandleLogoutPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
		}

		h.renderTemplate(w, r, "logout-page.gohtml", data)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleLogout() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := h.deleteSessionValue(w, r, "username")
		if err != nil {
			slog.ErrorContext(r.Context(), "error on deleting session value", "key", "username", "error", err)
			http.Error(w, "error on deleting session value", http.StatusInternalServerError)
			return
		}

		h.addSuccessMessage(w, r, "Logged out successfully.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleForgotPasswordPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
		}

		h.renderTemplate(w, r, "forgot-password-page.gohtml", data)
	})

	return h.GuestOnly(hf)
}

func (h *Handler) HandleForgotPassword() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse form", "error", err)
			http.Error(w, "error on parse form", http.StatusInternalServerError)
			return
		}

		emailAddress := r.FormValue("emailAddress")
		if emailAddress == "" {
			http.Error(w, "email address cannot be empty", http.StatusBadRequest)
			return
		}

		user, err := h.UserRepo.GetByEmailAddress(r.Context(), emailAddress)
		if err != nil {
			if errors.As(err, &UserByEmailNotFoundError{}) {
				// Do not reveal if email exists for security
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}

			slog.ErrorContext(r.Context(), "error retrieving user by email", "error", err)
			http.Error(w, "error retrieving user", http.StatusInternalServerError)
			return
		}

		resetToken := uuid.NewString()
		resetTokenExpiry := time.Now().Add(1 * time.Hour)

		reset := &PasswordResetToken{
			ID:        uuid.NewString(),
			UserID:    user.ID,
			Token:     resetToken,
			ExpiresAt: resetTokenExpiry,
		}
		if err := h.PasswordResetTokenRepo.Create(r.Context(), reset); err != nil {
			slog.ErrorContext(r.Context(), "error saving reset token", "error", err)
			http.Error(w, "error saving reset token", http.StatusInternalServerError)
			return
		}

		// Send reset email
		resetLink := fmt.Sprintf("%s/reset-password?token=%s", getHostURL(r), resetToken)
		subject := "Password Reset Request"
		body := fmt.Sprintf("To reset your password, click the following link:\n\n%s\n\nIf you did not request a password reset, you can ignore this email.", resetLink)
		if err := h.Mailer.SendEmail(r.Context(), emailAddress, subject, body); err != nil {
			slog.ErrorContext(r.Context(), "failed to send reset email", "error", err)
			// Do not reveal error to user
		}

		h.addSuccessMessage(w, r, "Reset password link has been sent successfully.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return h.GuestOnly(hf)
}

func getHostURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if r.Header.Get("X-Forwarded-Proto") != "" {
		scheme = r.Header.Get("X-Forwarded-Proto")
	}

	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

func (h *Handler) HandleResetPasswordPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "token is required", http.StatusBadRequest)
			return
		}

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"Token":          token,
		}

		h.renderTemplate(w, r, "reset-password-page.gohtml", data)
	})

	return h.GuestOnly(hf)
}

func (h *Handler) HandleResetPassword() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse form", "error", err)
			http.Error(w, "error on parse form", http.StatusInternalServerError)
			return
		}

		token := r.FormValue("token")
		if token == "" {
			http.Error(w, "reset token is required", http.StatusBadRequest)
			return
		}

		resetToken, err := h.PasswordResetTokenRepo.GetByToken(r.Context(), token)
		if err != nil {
			slog.ErrorContext(r.Context(), "invalid or expired reset token", "error", err)
			http.Error(w, "invalid or expired reset token", http.StatusBadRequest)
			return
		}

		if time.Now().After(resetToken.ExpiresAt) {
			http.Error(w, "reset token has expired", http.StatusBadRequest)
			return
		}

		user, err := h.UserRepo.GetByID(r.Context(), resetToken.UserID)
		if err != nil {
			slog.ErrorContext(r.Context(), "user not found for reset token", "error", err)
			http.Error(w, "user not found", http.StatusInternalServerError)
			return
		}

		newPassword := r.FormValue("newPassword")
		newPasswordConfirmation := r.FormValue("newPasswordConfirmation")
		if newPassword == "" || newPasswordConfirmation == "" {
			http.Error(w, "password and confirmation cannot be empty", http.StatusBadRequest)
			return
		}
		if newPassword != newPasswordConfirmation {
			http.Error(w, "password and confirmation do not match", http.StatusBadRequest)
			return
		}

		newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			slog.ErrorContext(r.Context(), "error hashing new password", "error", err)
			http.Error(w, "error hashing new password", http.StatusInternalServerError)
			return
		}

		user.PasswordHash = string(newPasswordHash)
		user.UpdatedAt = time.Now()
		if err := h.UserRepo.Update(r.Context(), user); err != nil {
			slog.ErrorContext(r.Context(), "error updating user password", "error", err)
			http.Error(w, "error updating password", http.StatusInternalServerError)
			return
		}

		if err := h.PasswordResetTokenRepo.Delete(r.Context(), resetToken.ID); err != nil {
			slog.ErrorContext(r.Context(), "error deleting used reset token", "error", err)
		}

		h.addSuccessMessage(w, r, "Password has been reset successfully.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return h.GuestOnly(hf)
}

func (h *Handler) HandleProfilePage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
		}

		h.renderTemplate(w, r, "profile-page.gohtml", data)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleProfileUpdate() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse form", "error", err)
			http.Error(w, "error on parse form", http.StatusInternalServerError)
			return
		}

		user := userFromContext(r.Context())

		name := r.FormValue("name")
		emailAddress := r.FormValue("emailAddress")
		avatarURL := r.FormValue("avatarUrl")

		user.Name = name
		user.EmailAddress = emailAddress
		user.AvatarURL = avatarURL

		err = h.UserRepo.Update(r.Context(), user)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on update user", "error", err)
			http.Error(w, "error on update user", http.StatusInternalServerError)
			return
		}

		h.addSuccessMessage(w, r, "Profile has been updated successfully.")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleProfilePasswordUpdate() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse form", "error", err)
			h.addErrorMessage(w, r, "Error on parse form.")
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		user := userFromContext(r.Context())

		currentPassword := r.FormValue("currentPassword")
		newPassword := r.FormValue("newPassword")
		newPasswordConfirmation := r.FormValue("newPasswordConfirmation")

		formErrors := map[string]any{}

		if newPassword != newPasswordConfirmation {
			formErrors["NewPasswordConfirmation"] = "New password and confirmation do not match"
		}

		if currentPassword == "" || newPassword == "" {
			http.Error(w, "current password and new password cannot be empty", http.StatusBadRequest)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
			formErrors["CurrentPassword"] = "Current password is incorrect"
		}

		if len(formErrors) > 0 {
			h.addErrorMessage(w, r, "Invalid form submission.")
			// TODO: add form values to session
			err := h.addFormErrorsToSession(w, r, "ProfilePasswordForm", formErrors)
			if err != nil {
				slog.ErrorContext(r.Context(), "error adding form errors to session", "error", err)
				h.addErrorMessage(w, r, "Error adding form errors.")
			}
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on hash new password", "error", err)
			h.addErrorMessage(w, r, "Error on hash new password.")
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		user.PasswordHash = string(newPasswordHash)

		err = h.UserRepo.Update(r.Context(), user)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on update user", "error", err)
			h.addErrorMessage(w, r, "Error on update user.")
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		h.addSuccessMessage(w, r, "Password has been updated successfully.")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleViewPostPage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postSlug := r.PathValue("postSlug")
		post, err := h.PostRepo.GetBySlug(r.Context(), postSlug)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to list posts", "error", err)
			http.Error(w, "failed to list posts", http.StatusInternalServerError)
			return
		}

		comments, err := h.CommentRepo.List(r.Context(), ListCommentsParams{PostID: post.ID})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to post comments", "error", err)
			http.Error(w, "failed to list post comments", http.StatusInternalServerError)
			return
		}

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"Post":           post,
			"PostComments":   comments,
		}

		h.renderTemplate(w, r, "single-post-page.gohtml", data)
	})
}

func (h *Handler) HandleNewPostPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
		}

		h.renderTemplate(w, r, "new-post-page.gohtml", data)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleCreatePost() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse form", "error", err)
			http.Error(w, "error on parse form", http.StatusInternalServerError)

			return
		}

		title := r.FormValue("title")
		slug := r.FormValue("slug")
		excerpt := r.FormValue("excerpt")
		content := r.FormValue("content")

		if slug == "" {
			slug = title
		}
		slug = slugify.Make(slug)

		uniqueSlug, err := h.generateUniqueSlug(r.Context(), slug)
		if err != nil {
			slog.ErrorContext(r.Context(), "error generating unique slug", "error", err)
			http.Error(w, "error generating unique slug", http.StatusInternalServerError)
			return
		}

		if excerpt == "" {
			excerpt = h.TextPolicy.Sanitize(content)
		}

		excerpt = h.generateExcerpt(excerpt, 160)

		content = h.HTMLPolicy.Sanitize(content)

		user := userFromContext(r.Context())

		timeNow := time.Now()

		post := &Post{
			ID:        uuid.NewString(),
			Title:     title,
			Slug:      uniqueSlug,
			Excerpt:   excerpt,
			Content:   content,
			AuthorID:  user.ID,
			CreatedAt: timeNow,
			UpdatedAt: timeNow,
		}

		err = h.PostRepo.Create(r.Context(), post)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on create post", "error", err)
			http.Error(w, "error on create post", http.StatusInternalServerError)

			return
		}

		h.addSuccessMessage(w, r, "Post has been created successfully.")
		http.Redirect(w, r, "/posts/"+post.Slug, http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleEditPostPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postSlug := r.PathValue("postSlug")
		post, err := h.PostRepo.GetBySlug(r.Context(), postSlug)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get post by slug", "error", err, "postSlug", postSlug)
			http.Error(w, "error on get post by slug", http.StatusInternalServerError)

			return
		}

		user := userFromContext(r.Context())

		if post.AuthorID != user.ID {
			http.Error(w, "cannot edit post", http.StatusForbidden)

			return
		}

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"Post":           post,
		}

		h.renderTemplate(w, r, "edit-post-page.gohtml", data)
	})
	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleEditPost() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postSlug := r.PathValue("postSlug")
		post, err := h.PostRepo.GetBySlug(r.Context(), postSlug)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get post by slug", "error", err, "postSlug", postSlug)
			http.Error(w, "error on get post by slug", http.StatusInternalServerError)
			return
		}

		user := userFromContext(r.Context())
		if post.AuthorID != user.ID {
			http.Error(w, "cannot edit post", http.StatusForbidden)
			return
		}

		err = r.ParseForm()
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse form", "error", err)
			http.Error(w, "error on parse form", http.StatusInternalServerError)

			return
		}

		title := r.FormValue("title")
		slug := r.FormValue("slug")
		excerpt := r.FormValue("excerpt")
		content := r.FormValue("content")

		if slug == "" {
			slug = title
		}
		slug = slugify.Make(slug)

		uniqueSlug := slug

		if uniqueSlug != post.Slug {
			var err error
			uniqueSlug, err = h.generateUniqueSlug(r.Context(), slug)
			if err != nil {
				slog.ErrorContext(r.Context(), "error generating unique slug", "error", err)
				http.Error(w, "error generating unique slug", http.StatusInternalServerError)
				return
			}
		}

		if excerpt == "" {
			excerpt = h.TextPolicy.Sanitize(content)
		}

		excerpt = h.generateExcerpt(excerpt, 160)

		content = h.HTMLPolicy.Sanitize(content)

		post.Title = title
		post.Slug = uniqueSlug
		post.Excerpt = excerpt
		post.Content = content
		post.UpdatedAt = time.Now()

		err = h.PostRepo.Update(r.Context(), post)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on update post", "error", err)
			http.Error(w, "error on update post", http.StatusInternalServerError)

			return
		}

		h.addSuccessMessage(w, r, "Post has been updated successfully.")
		http.Redirect(w, r, "/posts/"+post.Slug, http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleDeletePostPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postSlug := r.PathValue("postSlug")
		post, err := h.PostRepo.GetBySlug(r.Context(), postSlug)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get post by slug", "error", err, "postSlug", postSlug)
			http.Error(w, "error on get post by slug", http.StatusInternalServerError)

			return
		}

		user := userFromContext(r.Context())
		if post.AuthorID != user.ID {
			http.Error(w, "cannot delete post", http.StatusForbidden)

			return
		}

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"Post":           post,
		}

		h.renderTemplate(w, r, "delete-post-page.gohtml", data)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleDeletePost() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postSlug := r.PathValue("postSlug")
		post, err := h.PostRepo.GetBySlug(r.Context(), postSlug)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get post by slug", "error", err, "postSlug", postSlug)
			http.Error(w, "error on get post by slug", http.StatusInternalServerError)

			return
		}

		user := userFromContext(r.Context())
		if post.AuthorID != user.ID {
			http.Error(w, "cannot delete post", http.StatusForbidden)

			return
		}

		err = h.PostRepo.Delete(r.Context(), post.ID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on delete post", "error", err)
			http.Error(w, "error on delete post", http.StatusInternalServerError)

			return
		}

		h.addSuccessMessage(w, r, "Post has been deleted successfully.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleSubmitComment() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse form", "error", err)
			http.Error(w, "error on parse form", http.StatusInternalServerError)

			return
		}

		postID := r.FormValue("postId")
		content := r.FormValue("content")

		content = h.HTMLPolicy.Sanitize(content)

		user := userFromContext(r.Context())

		post, err := h.PostRepo.GetByID(r.Context(), postID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get post by id", "error", err, "postId", postID)
			http.Error(w, "error on get post by id", http.StatusInternalServerError)

			return
		}

		timeNow := time.Now()

		comment := &Comment{
			ID:        uuid.NewString(),
			PostID:    postID,
			UserID:    user.ID,
			Content:   content,
			CreatedAt: timeNow,
			UpdatedAt: timeNow,
		}

		err = h.CommentRepo.Create(r.Context(), comment)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on create comment", "error", err)
			http.Error(w, "error on create comment", http.StatusInternalServerError)

			return
		}

		h.addSuccessMessage(w, r, "Comment has been created successfully.")
		http.Redirect(w, r, "/posts/"+post.Slug, http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleEditCommentPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commentID := r.PathValue("commentId")
		comment, err := h.CommentRepo.GetByID(r.Context(), commentID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get comment by id", "error", err, "commentId", commentID)
			http.Error(w, "error on get comment by id", http.StatusInternalServerError)

			return
		}

		user := userFromContext(r.Context())

		if comment.UserID != user.ID {
			http.Error(w, "cannot edit comment", http.StatusForbidden)

			return
		}

		post, err := h.PostRepo.GetByID(r.Context(), comment.PostID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get post by id", "error", err, "postId", comment.PostID)
			http.Error(w, "error on get post by id", http.StatusInternalServerError)

			return
		}

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"Comment":        comment,
			"Post":           post,
		}

		h.renderTemplate(w, r, "edit-comment-page.gohtml", data)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleEditComment() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commentID := r.PathValue("commentId")

		err := r.ParseForm()
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse form", "error", err)
			http.Error(w, "error on parse form", http.StatusInternalServerError)

			return
		}

		content := r.FormValue("content")

		content = h.HTMLPolicy.Sanitize(content)

		comment, err := h.CommentRepo.GetByID(r.Context(), commentID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get comment by id", "error", err, "commentId", commentID)
			http.Error(w, "error on get comment by id", http.StatusInternalServerError)

			return
		}

		user := userFromContext(r.Context())

		if comment.UserID != user.ID {
			http.Error(w, "cannot edit comment", http.StatusForbidden)

			return
		}

		comment.Content = content
		comment.UpdatedAt = time.Now()

		err = h.CommentRepo.Update(r.Context(), comment)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on update comment", "error", err)
			http.Error(w, "error on update comment", http.StatusInternalServerError)

			return
		}

		h.addSuccessMessage(w, r, "Comment has been updated successfully.")

		post, err := h.PostRepo.GetByID(r.Context(), comment.PostID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get post by id", "error", err, "postId", comment.PostID)
			http.Error(w, "error on get post by id", http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/posts/"+post.Slug, http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleDeleteCommentPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commentID := r.PathValue("commentId")
		comment, err := h.CommentRepo.GetByID(r.Context(), commentID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get comment by id", "error", err, "commentId", commentID)
			http.Error(w, "error on get comment by id", http.StatusInternalServerError)

			return
		}

		user := userFromContext(r.Context())

		if comment.UserID != user.ID {
			http.Error(w, "cannot delete comment", http.StatusForbidden)

			return
		}

		post, err := h.PostRepo.GetByID(r.Context(), comment.PostID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get post by id", "error", err, "postId", comment.PostID)
			http.Error(w, "error on get post by id", http.StatusInternalServerError)

			return
		}

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"Comment":        comment,
			"Post":           post,
		}

		h.renderTemplate(w, r, "delete-comment-page.gohtml", data)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleDeleteComment() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commentID := r.PathValue("commentId")

		comment, err := h.CommentRepo.GetByID(r.Context(), commentID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get comment by id", "error", err, "commentId", commentID)
			http.Error(w, "error on get comment by id", http.StatusInternalServerError)

			return
		}

		user := userFromContext(r.Context())

		if comment.UserID != user.ID {
			http.Error(w, "cannot delete comment", http.StatusForbidden)

			return
		}

		err = h.CommentRepo.Delete(r.Context(), commentID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on delete comment", "error", err)
			http.Error(w, "error on delete comment", http.StatusInternalServerError)

			return
		}

		h.addSuccessMessage(w, r, "Comment has been deleted successfully.")

		post, err := h.PostRepo.GetByID(r.Context(), comment.PostID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get post by id", "error", err, "postId", comment.PostID)
			http.Error(w, "error on get post by id", http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/posts/"+post.Slug, http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

// generateExcerpt creates a properly sized excerpt from content
func (h *Handler) generateExcerpt(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}

	// Find the last complete word within the limit
	excerpt := content[:maxLength]
	lastSpace := strings.LastIndex(excerpt, " ")

	if lastSpace > 0 {
		excerpt = excerpt[:lastSpace]
	}

	return excerpt + "..."
}

func (h *Handler) generateUniqueSlug(ctx context.Context, baseSlug string) (string, error) {
	exists, err := h.PostRepo.SlugExists(ctx, baseSlug)
	if err != nil {
		return "", fmt.Errorf("error checking slug existence: %w", err)
	}

	if !exists {
		return baseSlug, nil
	}

	basePart := baseSlug
	counter := 2

	parts := strings.Split(baseSlug, "-")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		if num, err := strconv.Atoi(lastPart); err == nil {
			basePart = strings.Join(parts[:len(parts)-1], "-")
			counter = num + 1
		}
	}

	for {
		candidateSlug := basePart + "-" + fmt.Sprintf("%d", counter)
		exists, err := h.PostRepo.SlugExists(ctx, candidateSlug)
		if err != nil {
			return "", err
		}

		if !exists {
			return candidateSlug, nil
		}

		counter++

		if counter > 1000 {
			return "", fmt.Errorf("unable to generate unique slug after 1000 attempts")
		}
	}
}
