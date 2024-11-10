package main

import (
	"cmp"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/nasermirzaei89/blog/service"
	"html/template"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	tpl         *template.Template
	username    string
	password    string
	cookieStore *sessions.CookieStore
	postRepo    service.PostRepository
	settingRepo service.SettingRepository
}

func (h *Handler) NotFoundPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		settings, err := h.settingRepo.Load(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to load settings: %w", err))
		}

		pageData := struct {
			Settings service.Settings
		}{
			Settings: *settings,
		}

		err = h.tpl.ExecuteTemplate(w, "404-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render login page template: %w", err))
		}
	}
}

func (h *Handler) HomePageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			h.NotFoundPageHandler()(w, r)

			return
		}

		publishedPostList, err := h.postRepo.ListPublished(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to list published posts: %w", err))
		}

		settings, err := h.settingRepo.Load(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to load settings: %w", err))
		}

		pageData := struct {
			Settings service.Settings
			Posts    []service.Post
		}{
			Settings: *settings,
			Posts:    publishedPostList,
		}

		err = h.tpl.ExecuteTemplate(w, "home-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render home page template: %w", err))
		}
	})
}

func (h *Handler) SinglePostPageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postSlug := r.PathValue("postSlug")

		post, err := h.postRepo.GetBySlug(r.Context(), postSlug)
		if err != nil {
			var postBySlugNotFoundError service.PostBySlugNotFoundError
			if errors.As(err, &postBySlugNotFoundError) {
				h.NotFoundPageHandler().ServeHTTP(w, r)

				return
			}

			panic(fmt.Errorf("failed to get post: %w", err))
		}

		if post.Status != service.PostStatusPublished && !h.isAuthenticated(r) {
			h.NotFoundPageHandler()(w, r)

			return
		}

		settings, err := h.settingRepo.Load(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to load settings: %w", err))
		}

		pageData := struct {
			Settings        service.Settings
			Post            service.Post
			IsAuthenticated bool
		}{
			Settings:        *settings,
			Post:            *post,
			IsAuthenticated: h.isAuthenticated(r),
		}

		err = h.tpl.ExecuteTemplate(w, "single-post-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render single post page template: %w", err))
		}
	})
}

func (h *Handler) LoginPageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := h.settingRepo.Load(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to load settings: %w", err))
		}

		pageData := struct {
			Settings service.Settings
		}{
			Settings: *settings,
		}

		err = h.tpl.ExecuteTemplate(w, "login-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render login page template: %w", err))
		}
	})
}

const sessionNameAuth = "auth"

func (h *Handler) LoginHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := strings.TrimSpace(r.FormValue("username"))
		password := strings.TrimSpace(r.FormValue("password"))

		if username == h.username && password == h.password {
			session, err := h.cookieStore.Get(r, sessionNameAuth)
			if err != nil {
				panic(fmt.Errorf("failed to get session: %w", err))
			}

			session.Values["authenticated"] = true
			err = sessions.Save(r, w)
			if err != nil {
				panic(fmt.Errorf("failed to save session: %w", err))
			}

			redirect := cmp.Or(r.URL.Query().Get("redirect"), "/admin")
			http.Redirect(w, r, redirect, http.StatusFound)
			return
		}

		// TODO: show unauthorized message
		w.WriteHeader(http.StatusUnauthorized)
	})
}

func (h *Handler) LogoutHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := h.cookieStore.Get(r, sessionNameAuth)
		if err != nil {
			panic(fmt.Errorf("failed to get session: %w", err))
		}

		session.Values["authenticated"] = false
		err = sessions.Save(r, w)
		if err != nil {
			panic(fmt.Errorf("failed to save session: %w", err))
		}

		http.Redirect(w, r, "/login", http.StatusFound)
	})
}

func (h *Handler) isAuthenticated(r *http.Request) bool {
	session, err := h.cookieStore.Get(r, sessionNameAuth)
	if err != nil {
		panic(fmt.Errorf("failed to get session: %w", err))
	}

	auth, ok := session.Values["authenticated"].(bool)

	return ok && auth
}

func (h *Handler) AuthenticatedMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) AdminPageHandler() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/posts", http.StatusTemporaryRedirect)
	})

	return h.AuthenticatedMiddleware(hf)
}

func (h *Handler) AdminPostsPageHandler() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allPostList, err := h.postRepo.ListAll(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to list all posts: %w", err))
		}

		settings, err := h.settingRepo.Load(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to load settings: %w", err))
		}

		pageData := struct {
			Settings service.Settings
			Posts    []service.Post
		}{
			Settings: *settings,
			Posts:    allPostList,
		}

		err = h.tpl.ExecuteTemplate(w, "admin-posts-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render admin posts page template: %w", err))
		}
	})

	return h.AuthenticatedMiddleware(hf)
}

func (h *Handler) AdminNewPostPageHandler() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := h.settingRepo.Load(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to load settings: %w", err))
		}

		pageData := struct {
			Settings service.Settings
		}{
			Settings: *settings,
		}

		err = h.tpl.ExecuteTemplate(w, "admin-new-post-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render admin new post page template: %w", err))
		}
	})

	return h.AuthenticatedMiddleware(hf)
}

func (h *Handler) AdminCreateNewPostHandler() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		title := strings.TrimSpace(r.FormValue("title"))
		// TODO: sanitize slug
		// TODO: check slug is unique
		slug := strings.TrimSpace(r.FormValue("slug"))
		status := strings.TrimSpace(r.FormValue("status"))

		var publishedAt *time.Time
		if status == service.PostStatusPublished {
			publishedAtStr := strings.TrimSpace(r.FormValue("publishedAt"))
			v, err := time.Parse(DateTimeLocalFormat, publishedAtStr)
			if err != nil {
				// TODO: show invalid published at field
				panic(fmt.Errorf("failed to parse published at: %w", err))
			}

			publishedAt = &v
		}

		excerpt := strings.TrimSpace(r.FormValue("excerpt"))
		content := strings.TrimSpace(r.FormValue("content"))

		post := service.Post{
			UUID:        uuid.New(),
			Title:       title,
			Slug:        slug,
			Status:      service.PostStatus(status),
			PublishedAt: publishedAt,
			Excerpt:     excerpt,
			Content:     template.HTML(content),
		}

		err := h.postRepo.Insert(r.Context(), &post)
		if err != nil {
			panic(fmt.Errorf("failed to insert post: %w", err))
		}

		http.Redirect(w, r, "/admin/posts/"+post.UUID.String()+"/edit", http.StatusFound)
	})

	return h.AuthenticatedMiddleware(hf)
}

func (h *Handler) AdminEditPostPageHandler() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postUUIDStr := strings.TrimSpace(r.PathValue("postUuid"))

		postUUID, err := uuid.Parse(postUUIDStr)
		if err != nil {
			// TODO: show error
			panic(fmt.Errorf("failed to parse post uuid: %w", err))
		}

		post, err := h.postRepo.Get(r.Context(), postUUID)
		if err != nil {
			var postByUUIDNotFoundError service.PostByUUIDNotFoundError
			if errors.As(err, &postByUUIDNotFoundError) {
				h.NotFoundPageHandler().ServeHTTP(w, r)

				return
			}

			panic(fmt.Errorf("failed to get post: %w", err))
		}

		settings, err := h.settingRepo.Load(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to load settings: %w", err))
		}

		pageData := struct {
			Settings service.Settings
			Post     service.Post
		}{
			Settings: *settings,
			Post:     *post,
		}

		err = h.tpl.ExecuteTemplate(w, "admin-edit-post-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render admin edit post page template: %w", err))
		}
	})

	return h.AuthenticatedMiddleware(hf)
}

func (h *Handler) AdminUpdatePostHandler() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postUUIDStr := strings.TrimSpace(r.PathValue("postUuid"))

		postUUID, err := uuid.Parse(postUUIDStr)
		if err != nil {
			// TODO: show error
			panic(fmt.Errorf("failed to parse post uuid: %w", err))
		}

		post, err := h.postRepo.Get(r.Context(), postUUID)
		if err != nil {
			var postByUUIDNotFoundError service.PostByUUIDNotFoundError
			if errors.As(err, &postByUUIDNotFoundError) {
				h.NotFoundPageHandler().ServeHTTP(w, r)

				return
			}

			panic(fmt.Errorf("failed to get post: %w", err))
		}

		post.Title = strings.TrimSpace(r.FormValue("title"))
		// TODO: sanitize slug
		// TODO: check slug is unique
		post.Slug = strings.TrimSpace(r.FormValue("slug"))
		post.Status = service.PostStatus(strings.TrimSpace(r.FormValue("status")))

		var publishedAt *time.Time
		if post.Status == service.PostStatusPublished {
			publishedAtStr := strings.TrimSpace(r.FormValue("publishedAt"))
			v, err := time.Parse(DateTimeLocalFormat, publishedAtStr)
			if err != nil {
				// TODO: show error
				panic(fmt.Errorf("failed to parse published at: %w", err))
			}

			publishedAt = &v
		}

		post.PublishedAt = publishedAt

		post.Excerpt = strings.TrimSpace(r.FormValue("excerpt"))
		post.Content = template.HTML(strings.TrimSpace(r.FormValue("content")))

		err = h.postRepo.Replace(r.Context(), postUUID, post)
		if err != nil {
			panic(fmt.Errorf("failed to update post: %w", err))
		}

		http.Redirect(w, r, "/admin/posts/"+post.UUID.String()+"/edit", http.StatusFound)
	})

	return h.AuthenticatedMiddleware(hf)
}

func (h *Handler) AdminDeletePostHandler() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postUUIDStr := strings.TrimSpace(r.PathValue("postUuid"))

		postUUID, err := uuid.Parse(postUUIDStr)
		if err != nil {
			// TODO: show error
			panic(fmt.Errorf("failed to parse post uuid: %w", err))
		}

		_, err = h.postRepo.Get(r.Context(), postUUID)
		if err != nil {
			var postByUUIDNotFoundError service.PostByUUIDNotFoundError
			if errors.As(err, &postByUUIDNotFoundError) {
				h.NotFoundPageHandler().ServeHTTP(w, r)

				return
			}

			panic(fmt.Errorf("failed to get post: %w", err))
		}

		err = h.postRepo.Delete(r.Context(), postUUID)
		if err != nil {
			panic(fmt.Errorf("failed to update post: %w", err))
		}

		http.Redirect(w, r, "/admin/posts", http.StatusFound)
	})

	return h.AuthenticatedMiddleware(hf)
}

func (h *Handler) AdminSettingsPageHandler() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := h.settingRepo.Load(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to load settings: %w", err))
		}

		pageData := struct {
			Settings           service.Settings
			AvailableTimeZones map[string][]string
		}{
			Settings:           *settings,
			AvailableTimeZones: AvailableTimeZones,
		}

		err = h.tpl.ExecuteTemplate(w, "admin-settings-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render admin settings page template: %w", err))
		}
	})

	return h.AuthenticatedMiddleware(hf)
}

func (h *Handler) AdminUpdateSettingsHandler() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		title := strings.TrimSpace(r.FormValue("title"))
		tagline := strings.TrimSpace(r.FormValue("tagline"))
		timeZoneName := strings.TrimSpace(r.FormValue("timeZone"))
		timeZone, err := time.LoadLocation(timeZoneName)
		if err != nil {
			// TODO: show error
			panic(fmt.Errorf("failed to load timezone name: %w", err))
		}

		settings, err := h.settingRepo.Load(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to load settings: %w", err))
		}

		settings.Title = title
		settings.Tagline = tagline
		settings.TimeZone = timeZone

		err = h.settingRepo.Save(r.Context(), settings)
		if err != nil {
			panic(fmt.Errorf("failed to update settings: %w", err))
		}

		h.AdminSettingsPageHandler().ServeHTTP(w, r)
	})

	return h.AuthenticatedMiddleware(hf)
}
