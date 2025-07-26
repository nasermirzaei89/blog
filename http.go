package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	slugify "github.com/gosimple/slug"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nasermirzaei89/blog/mailer"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	static                 fs.FS
	cookieStore            *sessions.CookieStore
	sessionName            string
	tmpl                   *template.Template
	userRepo               *UserRepository
	postRepo               *PostRepository
	commentRepo            *CommentRepository
	passwordResetTokenRepo *PasswordResetTokenRepository
	mailer                 mailer.Mailer
	htmlPolicy             *bluemonday.Policy
	textPolicy             *bluemonday.Policy
	isShuttingDown         atomic.Bool
}

type contextKeyUserType struct{}

var contextKeyUser = contextKeyUserType{}

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
			session, err := h.cookieStore.Get(r, h.sessionName)
			if err != nil {
				slog.ErrorContext(r.Context(), "error on getting session", "error", err)
				http.Error(w, "error on getting session", http.StatusInternalServerError)
				return
			}

			if username := session.Values["username"]; username != nil && username.(string) != "" {
				user, err := h.userRepo.GetByUsername(r.Context(), username.(string))
				if err != nil {
					if errors.As(err, &UserByUsernameNotFoundError{}) {
						session.Values["username"] = nil

						err = session.Save(r, w)
						if err != nil {
							slog.ErrorContext(r.Context(), "error on saving session", "error", err)
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

func (h *Handler) HandleStatic(w http.ResponseWriter, r *http.Request) {
	// w.Header().Set("Cache-Control", "public, max-age=3600")
	http.FileServer(http.FS(h.static)).ServeHTTP(w, r)
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

	posts, err := h.postRepo.List(r.Context(), listPostsParams)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list posts", "error", err)
		http.Error(w, "failed to list posts", http.StatusInternalServerError)
		return
	}

	totalPosts, err := h.postRepo.Count(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to count posts", "error", err)
		http.Error(w, "failed to count posts", http.StatusInternalServerError)
		return
	}

	totalPages := (totalPosts + listPostsParams.Limit - 1) / listPostsParams.Limit

	data := map[string]any{
		"CurrentUser": userFromContext(r.Context()),
		"CurrentPath": r.URL.Path,
		"Posts":       posts,
		"CurrentPage": pageNum,
		"TotalPages":  totalPages,
	}

	err = h.tmpl.ExecuteTemplate(w, "home-page.gohtml", data)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
		http.Error(w, "failed to execute template", http.StatusInternalServerError)
	}
}

func (h *Handler) HandleLoginPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"CurrentUser":    userFromContext(r.Context()),
			"CurrentPath":    r.URL.Path,
		}

		err := h.tmpl.ExecuteTemplate(w, "login-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
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

		user, err := h.userRepo.GetByUsername(r.Context(), username)
		if err != nil {
			if errors.As(err, &UserByUsernameNotFoundError{}) {
				http.Error(w, "invalid username or password", http.StatusUnauthorized)
				return
			}

			slog.ErrorContext(r.Context(), "error retrieving user", "error", err)
			http.Error(w, "error on retrieving user", http.StatusInternalServerError)

			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
		if err != nil {
			http.Error(w, "invalid username or password", http.StatusUnauthorized)

			return
		}

		session, err := h.cookieStore.Get(r, h.sessionName)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on getting session", "error", err)
			http.Error(w, "error on getting session", http.StatusInternalServerError)

			return
		}

		session.Values["username"] = user.Username

		err = session.Save(r, w)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on saving session", "error", err)
			http.Error(w, "error on saving session", http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return h.GuestOnly(hf)
}

func (h *Handler) HandleRegisterPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"CurrentUser":    userFromContext(r.Context()),
			"CurrentPath":    r.URL.Path,
		}

		err := h.tmpl.ExecuteTemplate(w, "register-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
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
		usernameExists, err := h.userRepo.ExistsByUsername(r.Context(), username)
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
		emailAddressExists, err := h.userRepo.ExistsByEmailAddress(r.Context(), emailAddress)
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

		err = h.userRepo.Create(r.Context(), user)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on create user", "error", err)
			http.Error(w, "error on create user", http.StatusInternalServerError)

			return
		}

		session, err := h.cookieStore.Get(r, h.sessionName)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on getting session", "error", err)
			http.Error(w, "error on getting session", http.StatusInternalServerError)

			return
		}

		session.Values["username"] = user.Username

		err = session.Save(r, w)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on saving session", "error", err)
			http.Error(w, "error on saving session", http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return h.GuestOnly(hf)
}

func (h *Handler) HandleLogoutPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"CurrentUser":    userFromContext(r.Context()),
			"CurrentPath":    r.URL.Path,
		}

		err := h.tmpl.ExecuteTemplate(w, "logout-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleLogout() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := h.cookieStore.Get(r, h.sessionName)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on getting session", "error", err)
			http.Error(w, "error on getting session", http.StatusInternalServerError)

			return
		}

		session.Values["username"] = nil

		err = session.Save(r, w)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on saving session", "error", err)
			http.Error(w, "error on saving session", http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleForgotPasswordPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
		}

		err := h.tmpl.ExecuteTemplate(w, "forgot-password-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
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

		user, err := h.userRepo.GetByEmailAddress(r.Context(), emailAddress)
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
		if err := h.passwordResetTokenRepo.Create(r.Context(), reset); err != nil {
			slog.ErrorContext(r.Context(), "error saving reset token", "error", err)
			http.Error(w, "error saving reset token", http.StatusInternalServerError)
			return
		}

		// Send reset email
		resetLink := fmt.Sprintf("%s/reset-password?token=%s", getHostURL(r), resetToken)
		subject := "Password Reset Request"
		body := fmt.Sprintf("To reset your password, click the following link:\n\n%s\n\nIf you did not request a password reset, you can ignore this email.", resetLink)
		if err := h.mailer.SendEmail(r.Context(), emailAddress, subject, body); err != nil {
			slog.ErrorContext(r.Context(), "failed to send reset email", "error", err)
			// Do not reveal error to user
		}

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

		err := h.tmpl.ExecuteTemplate(w, "reset-password-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
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

		resetToken, err := h.passwordResetTokenRepo.GetByToken(r.Context(), token)
		if err != nil {
			slog.ErrorContext(r.Context(), "invalid or expired reset token", "error", err)
			http.Error(w, "invalid or expired reset token", http.StatusBadRequest)
			return
		}

		if time.Now().After(resetToken.ExpiresAt) {
			http.Error(w, "reset token has expired", http.StatusBadRequest)
			return
		}

		user, err := h.userRepo.GetByID(r.Context(), resetToken.UserID)
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
		if err := h.userRepo.Update(r.Context(), user); err != nil {
			slog.ErrorContext(r.Context(), "error updating user password", "error", err)
			http.Error(w, "error updating password", http.StatusInternalServerError)
			return
		}

		if err := h.passwordResetTokenRepo.Delete(r.Context(), resetToken.ID); err != nil {
			slog.ErrorContext(r.Context(), "error deleting used reset token", "error", err)
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return h.GuestOnly(hf)
}

func (h *Handler) HandleProfilePage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r.Context())

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"CurrentUser":    user,
			"CurrentPath":    r.URL.Path,
		}

		err := h.tmpl.ExecuteTemplate(w, "profile-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
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

		err = h.userRepo.Update(r.Context(), user)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on update user", "error", err)
			http.Error(w, "error on update user", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleProfilePasswordUpdate() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			slog.ErrorContext(r.Context(), "error on parse form", "error", err)
			http.Error(w, "error on parse form", http.StatusInternalServerError)
			return
		}

		user := userFromContext(r.Context())

		currentPassword := r.FormValue("currentPassword")
		newPassword := r.FormValue("newPassword")
		newPasswordConfirmation := r.FormValue("newPasswordConfirmation")

		if newPassword != newPasswordConfirmation {
			http.Error(w, "new password and confirmation do not match", http.StatusBadRequest)
			return
		}

		if currentPassword == "" || newPassword == "" {
			http.Error(w, "current password and new password cannot be empty", http.StatusBadRequest)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
			http.Error(w, "current password is incorrect", http.StatusUnauthorized)
			return
		}

		newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on hash new password", "error", err)
			http.Error(w, "error on hash new password", http.StatusInternalServerError)
			return
		}

		user.PasswordHash = string(newPasswordHash)

		err = h.userRepo.Update(r.Context(), user)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on update user", "error", err)
			http.Error(w, "error on update user", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleViewPostPage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postSlug := r.PathValue("postSlug")
		post, err := h.postRepo.GetBySlug(r.Context(), postSlug)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to list posts", "error", err)
			http.Error(w, "failed to list posts", http.StatusInternalServerError)
			return
		}

		comments, err := h.commentRepo.List(r.Context(), ListCommentsParams{PostID: post.ID})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to post comments", "error", err)
			http.Error(w, "failed to list post comments", http.StatusInternalServerError)
			return
		}

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"CurrentUser":    userFromContext(r.Context()),
			"CurrentPath":    r.URL.Path,
			"Post":           post,
			"PostComments":   comments,
		}

		err = h.tmpl.ExecuteTemplate(w, "single-post-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
	})
}

func (h *Handler) HandleNewPostPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"CurrentUser":    userFromContext(r.Context()),
			"CurrentPath":    r.URL.Path,
		}

		err := h.tmpl.ExecuteTemplate(w, "new-post-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
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
			excerpt = h.textPolicy.Sanitize(content)
		}

		excerpt = h.generateExcerpt(excerpt, 160)

		content = h.htmlPolicy.Sanitize(content)

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

		err = h.postRepo.Create(r.Context(), post)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on create post", "error", err)
			http.Error(w, "error on create post", http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/posts/"+post.Slug, http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleEditPostPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postSlug := r.PathValue("postSlug")
		post, err := h.postRepo.GetBySlug(r.Context(), postSlug)
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
			"CurrentUser":    user,
			"CurrentPath":    r.URL.Path,
			"Post":           post,
		}

		err = h.tmpl.ExecuteTemplate(w, "edit-post-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
	})
	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleEditPost() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postSlug := r.PathValue("postSlug")
		post, err := h.postRepo.GetBySlug(r.Context(), postSlug)
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
			excerpt = h.textPolicy.Sanitize(content)
		}

		excerpt = h.generateExcerpt(excerpt, 160)

		content = h.htmlPolicy.Sanitize(content)

		post.Title = title
		post.Slug = uniqueSlug
		post.Excerpt = excerpt
		post.Content = content
		post.UpdatedAt = time.Now()

		err = h.postRepo.Update(r.Context(), post)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on update post", "error", err)
			http.Error(w, "error on update post", http.StatusInternalServerError)

			return
		}

		http.Redirect(w, r, "/posts/"+post.Slug, http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleDeletePostPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postSlug := r.PathValue("postSlug")
		post, err := h.postRepo.GetBySlug(r.Context(), postSlug)
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
			"CurrentUser":    user,
			"CurrentPath":    r.URL.Path,
			"Post":           post,
		}

		err = h.tmpl.ExecuteTemplate(w, "delete-post-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleDeletePost() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postSlug := r.PathValue("postSlug")
		post, err := h.postRepo.GetBySlug(r.Context(), postSlug)
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

		err = h.postRepo.Delete(r.Context(), post.ID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on delete post", "error", err)
			http.Error(w, "error on delete post", http.StatusInternalServerError)

			return
		}

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

		content = h.htmlPolicy.Sanitize(content)

		user := userFromContext(r.Context())

		post, err := h.postRepo.GetByID(r.Context(), postID)
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

		err = h.commentRepo.Create(r.Context(), comment)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on create comment", "error", err)
			http.Error(w, "error on create comment", http.StatusInternalServerError)

			return
		}

		if r.Header.Get("X-Alpine-Request") == "true" {
			comments, err := h.commentRepo.List(r.Context(), ListCommentsParams{
				PostID: postID,
			})
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to list comments", "error", err)
				http.Error(w, "failed to list comments", http.StatusInternalServerError)
				return
			}

			data := map[string]any{
				"Post":         post,
				"PostComments": comments,
				"CurrentUser":  user,
				"csrfField":    csrf.TemplateField(r),
			}

			var buf bytes.Buffer

			if err := h.tmpl.ExecuteTemplate(&buf, "comments-list.gohtml", data); err != nil {
				slog.ErrorContext(r.Context(), "error on execute template", "error", err, "template", "comments-list.gohtml")
				http.Error(w, "error on execute template", http.StatusInternalServerError)
				return
			}

			if err := h.tmpl.ExecuteTemplate(&buf, "comment-form.gohtml", data); err != nil {
				slog.ErrorContext(r.Context(), "error on execute template", "error", err, "template", "comment-form.gohtml")
				http.Error(w, "error on execute template", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/html")
			w.Write(buf.Bytes())
			return
		}

		http.Redirect(w, r, "/posts/"+post.Slug, http.StatusSeeOther)
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleEditCommentPage() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commentID := r.PathValue("commentId")
		comment, err := h.commentRepo.GetByID(r.Context(), commentID)
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

		post, err := h.postRepo.GetByID(r.Context(), comment.PostID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get post by id", "error", err, "postId", comment.PostID)
			http.Error(w, "error on get post by id", http.StatusInternalServerError)

			return
		}

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"CurrentUser":    userFromContext(r.Context()),
			"CurrentPath":    r.URL.Path,
			"Comment":        comment,
			"Post":           post,
		}

		err = h.tmpl.ExecuteTemplate(w, "edit-comment-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
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

		content = h.htmlPolicy.Sanitize(content)

		comment, err := h.commentRepo.GetByID(r.Context(), commentID)
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

		err = h.commentRepo.Update(r.Context(), comment)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on update comment", "error", err)
			http.Error(w, "error on update comment", http.StatusInternalServerError)

			return
		}

		post, err := h.postRepo.GetByID(r.Context(), comment.PostID)
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
		comment, err := h.commentRepo.GetByID(r.Context(), commentID)
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

		post, err := h.postRepo.GetByID(r.Context(), comment.PostID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on get post by id", "error", err, "postId", comment.PostID)
			http.Error(w, "error on get post by id", http.StatusInternalServerError)

			return
		}

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"CurrentUser":    userFromContext(r.Context()),
			"CurrentPath":    r.URL.Path,
			"Comment":        comment,
			"Post":           post,
		}

		err = h.tmpl.ExecuteTemplate(w, "delete-comment-page.gohtml", data)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to execute template", "error", err)
			http.Error(w, "failed to execute template", http.StatusInternalServerError)
		}
	})

	return h.AuthenticatedOnly(hf)
}

func (h *Handler) HandleDeleteComment() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commentID := r.PathValue("commentId")

		comment, err := h.commentRepo.GetByID(r.Context(), commentID)
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

		err = h.commentRepo.Delete(r.Context(), commentID)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on delete comment", "error", err)
			http.Error(w, "error on delete comment", http.StatusInternalServerError)

			return
		}

		post, err := h.postRepo.GetByID(r.Context(), comment.PostID)
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
	exists, err := h.postRepo.SlugExists(ctx, baseSlug)
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
		exists, err := h.postRepo.SlugExists(ctx, candidateSlug)
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
