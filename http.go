package main

import (
	"context"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	static         fs.FS
	cookieStore    *sessions.CookieStore
	sessionName    string
	tmpl           *template.Template
	userRepo       *UserRepository
	postRepo       *PostRepository
	commentRepo    *CommentRepository
	htmlPolicy     *bluemonday.Policy
	isShuttingDown atomic.Bool
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
	if r.RequestURI == "/" {
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
	posts, err := h.postRepo.List(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list posts", "error", err)
		http.Error(w, "failed to list posts", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"CurrentUser": userFromContext(r.Context()),
		"CurrentPath": r.URL.Path,
		"Posts":       posts,
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

		err = h.userRepo.Insert(r.Context(), user)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on insert user", "error", err)
			http.Error(w, "error on insert user", http.StatusInternalServerError)

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

func (h *Handler) HandleViewPostPage(w http.ResponseWriter, r *http.Request) {
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

		err = h.commentRepo.Insert(r.Context(), comment)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on insert comment", "error", err)
			http.Error(w, "error on insert comment", http.StatusInternalServerError)

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

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"CurrentUser":    userFromContext(r.Context()),
			"CurrentPath":    r.URL.Path,
			"Comment":        comment,
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

		err = h.commentRepo.Replace(r.Context(), commentID, comment)
		if err != nil {
			slog.ErrorContext(r.Context(), "error on replace comment", "error", err)
			http.Error(w, "error on replace comment", http.StatusInternalServerError)

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

		data := map[string]any{
			csrf.TemplateTag: csrf.TemplateField(r),
			"CurrentUser":    userFromContext(r.Context()),
			"CurrentPath":    r.URL.Path,
			"Comment":        comment,
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
