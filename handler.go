package main

import (
	"bytes"
	"cmp"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/nasermirzaei89/blog/service"
	"html/template"
	"net/http"
	"strconv"
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

		page := uint64(1)
		if s := r.URL.Query().Get("page"); s != "" {
			// TODO: remove if page=1
			v, _ := strconv.Atoi(s)
			if v > 1 {
				page = uint64(v)
			}
		}

		perPage := uint64(defaultPerPage)
		//TODO: enable with SEO considerations
		//if s := r.URL.Query().Get("perPage"); s != "" {
		//	v, _ := strconv.Atoi(s)
		//	if v > 1 {
		//		perPage = uint64(v)
		//	}
		//}

		offset := (page - 1) * perPage

		req := service.ListPublishedRequest{Pagination: service.Pagination{
			Offset: offset,
			Limit:  perPage,
		}}

		publishedPostList, err := h.postRepo.ListPublished(r.Context(), req)
		if err != nil {
			panic(fmt.Errorf("failed to list published posts: %w", err))
		}

		totalPublishedPosts, err := h.postRepo.CountPublished(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to count published posts: %w", err))
		}

		settings, err := h.settingRepo.Load(r.Context())
		if err != nil {
			panic(fmt.Errorf("failed to load settings: %w", err))
		}

		var previousPageURL *string
		if page > 1 {
			q := r.URL.Query()

			if page == 2 {
				q.Del("page")
			} else {
				q.Set("page", fmt.Sprintf("%d", page-1))
			}

			if perPage == defaultPerPage {
				q.Del("perPage")
			}

			u := r.URL
			u.RawQuery = q.Encode()

			us := u.String()
			previousPageURL = &us
		}

		var nextPageURL *string
		if totalPublishedPosts > offset+perPage {
			q := r.URL.Query()

			q.Set("page", fmt.Sprintf("%d", page+1))

			if perPage == defaultPerPage {
				q.Del("perPage")
			}

			u := r.URL
			u.RawQuery = q.Encode()

			us := u.String()
			nextPageURL = &us
		}

		pagination := Pagination{
			CurrentPage:     page,
			PreviousPageURL: previousPageURL,
			NextPageURL:     nextPageURL,
			PerPage:         perPage,
		}

		pageData := struct {
			Settings        service.Settings
			Posts           []service.Post
			Pagination      Pagination
			IsAuthenticated bool
		}{
			Settings:        *settings,
			Posts:           publishedPostList,
			Pagination:      pagination,
			IsAuthenticated: h.isAuthenticated(r),
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

const (
	sessionNameAuth          = "auth"
	sessionNameAdminMessages = "adminMessages"
)

func (h *Handler) readMessages(r *http.Request, w http.ResponseWriter) []Message {
	session, err := h.cookieStore.Get(r, sessionNameAdminMessages)
	if err != nil {
		panic(fmt.Errorf("failed to get session: %w", err))
	}

	iMessages, ok := session.Values["messages"]
	if !ok {
		return []Message{}
	}

	messagesStr, ok := iMessages.(string)
	if !ok {
		panic(fmt.Errorf("failed to convert session.Values[messages] to string: %w", err))
	}

	if messagesStr == "" {
		return []Message{}
	}

	reader := strings.NewReader(messagesStr)
	var messages []Message

	err = gob.NewDecoder(reader).Decode(&messages)
	if err != nil {
		panic(fmt.Errorf("failed to decode message: %w", err))
	}

	if len(messages) == 0 {
		return []Message{}
	}

	delete(session.Values, "messages")

	err = session.Save(r, w)
	if err != nil {
		panic(fmt.Errorf("failed to save session: %w", err))
	}

	return messages
}

func (h *Handler) raiseMessage(r *http.Request, w http.ResponseWriter, message Message) {
	messages := h.readMessages(r, w)

	session, err := h.cookieStore.Get(r, sessionNameAdminMessages)
	if err != nil {
		panic(fmt.Errorf("failed to get session: %w", err))
	}

	messages = append(messages, message)

	var buf bytes.Buffer
	err = gob.NewEncoder(&buf).Encode(messages)
	if err != nil {
		panic(fmt.Errorf("failed to encode message: %w", err))
	}

	session.Values["messages"] = buf.String()

	err = session.Save(r, w)
	if err != nil {
		panic(fmt.Errorf("failed to save session: %w", err))
	}
}

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

			err = session.Save(r, w)
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

		err = session.Save(r, w)
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
			Messages []Message
		}{
			Settings: *settings,
			Posts:    allPostList,
			Messages: h.readMessages(r, w),
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
			Messages []Message
		}{
			Settings: *settings,
			Messages: h.readMessages(r, w),
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
			settings, err := h.settingRepo.Load(r.Context())
			if err != nil {
				panic(fmt.Errorf("failed to load settings: %w", err))
			}

			publishedAtStr := strings.TrimSpace(r.FormValue("publishedAt"))
			v, err := time.ParseInLocation(DateTimeLocalFormat, publishedAtStr, settings.TimeZone)
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

		h.raiseMessage(r, w, Message{
			Type:    MessageTypeSuccess,
			Content: "Post has been created successfully.",
		})

		http.Redirect(w, r, "/admin/posts/"+post.UUID.String()+"/edit", http.StatusFound)
	})

	return h.AuthenticatedMiddleware(hf)
}

func (h *Handler) AdminEditPostPageHandler() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postUUIDStr := strings.TrimSpace(r.PathValue("postUuid"))

		postUUID, err := uuid.Parse(postUUIDStr)
		if err != nil {
			h.NotFoundPageHandler().ServeHTTP(w, r)

			return
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
			Messages []Message
		}{
			Settings: *settings,
			Post:     *post,
			Messages: h.readMessages(r, w),
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
			h.NotFoundPageHandler().ServeHTTP(w, r)

			return
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
			settings, err := h.settingRepo.Load(r.Context())
			if err != nil {
				panic(fmt.Errorf("failed to load settings: %w", err))
			}

			publishedAtStr := strings.TrimSpace(r.FormValue("publishedAt"))

			v, err := time.ParseInLocation(DateTimeLocalFormat, publishedAtStr, settings.TimeZone)
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

		h.raiseMessage(r, w, Message{
			Type:    MessageTypeSuccess,
			Content: "Post has been updated successfully.",
		})

		http.Redirect(w, r, "/admin/posts/"+post.UUID.String()+"/edit", http.StatusFound)
	})

	return h.AuthenticatedMiddleware(hf)
}

func (h *Handler) AdminDeletePostHandler() http.Handler {
	hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postUUIDStr := strings.TrimSpace(r.PathValue("postUuid"))

		postUUID, err := uuid.Parse(postUUIDStr)
		if err != nil {
			h.NotFoundPageHandler().ServeHTTP(w, r)

			return
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

		h.raiseMessage(r, w, Message{
			Type:    MessageTypeSuccess,
			Content: "Post has been deleted successfully.",
		})

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
			Messages           []Message
		}{
			Settings:           *settings,
			AvailableTimeZones: AvailableTimeZones,
			Messages:           h.readMessages(r, w),
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

		h.raiseMessage(r, w, Message{
			Type:    MessageTypeSuccess,
			Content: "Settings has been updated successfully.",
		})

		h.AdminSettingsPageHandler().ServeHTTP(w, r)
	})

	return h.AuthenticatedMiddleware(hf)
}
