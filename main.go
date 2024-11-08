package main

import (
	"cmp"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed templates
var templates embed.FS

func main() {
	fs, err := template.New("templates").
		Funcs(map[string]any{
			"formatDateTime": func(t time.Time) string {
				return t.Format("Jan _2, 2006 15:04")
			},
			"formatDate": func(t time.Time) string {
				return t.Format("Jan _2, 2006")
			},
		}).
		ParseFS(templates, "templates/*")
	if err != nil {
		panic(fmt.Errorf("failed to parse templates: %w", err))
	}

	mux := http.NewServeMux()

	mux.Handle("GET /", HomePageHandler(fs))
	mux.Handle("GET /posts/{postSlug}", SinglePostPageHandler(fs))
	mux.Handle("GET /login", LoginPageHandler(fs))
	mux.Handle("POST /login", LoginHandler())

	host := cmp.Or(os.Getenv("HOST"), "0.0.0.0")
	port := cmp.Or(os.Getenv("PORT"), "8888")

	addr := fmt.Sprintf("%s:%s", host, port)

	log.Printf("Listening on http://%s", addr)

	err = http.ListenAndServe(addr, mux)
	if err != nil {
		panic(fmt.Errorf("failed to start server: %w", err))
	}
}

type Settings struct {
	Title   string
	Tagline string
}

type Post struct {
	ID          string
	Title       string
	Slug        string
	Status      string
	PublishedAt time.Time
	Excerpt     string
	Content     template.HTML
}

var posts = []Post{
	{
		Title:       "Yet another post",
		Slug:        "yet-another-post",
		Status:      "published",
		PublishedAt: time.Date(2024, 11, 7, 22, 51, 0, 0, time.Local),
		Excerpt:     "Interdum et malesuada fames ac ante ipsum primis in faucibus. Suspendisse a nisl lorem. Nunc vitae ullamcorper mauris, sit amet condimentum metus. Maecenas pretium ut odio id semper. Sed sagittis arcu mauris, feugiat maximus libero facilisis id. Fusce in lacus vel libero vulputate commodo quis biam.",
		Content: `
<h2>What is Lorem Ipsum?</h2>
<p>Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.</p>
<h2>Where does it come from?</h2>
<p>Contrary to popular belief, Lorem Ipsum is not simply random text. It has roots in a piece of classical Latin literature from 45 BC, making it over 2000 years old. Richard McClintock, a Latin professor at Hampden-Sydney College in Virginia, looked up one of the more obscure Latin words, consectetur, from a Lorem Ipsum passage, and going through the cites of the word in classical literature, discovered the undoubtable source. Lorem Ipsum comes from sections 1.10.32 and 1.10.33 of "de Finibus Bonorum et Malorum" (The Extremes of Good and Evil) by Cicero, written in 45 BC. This book is a treatise on the theory of ethics, very popular during the Renaissance. The first line of Lorem Ipsum, "Lorem ipsum dolor sit amet..", comes from a line in section 1.10.32.</p>
<p>The standard chunk of Lorem Ipsum used since the 1500s is reproduced below for those interested. Sections 1.10.32 and 1.10.33 from "de Finibus Bonorum et Malorum" by Cicero are also reproduced in their exact original form, accompanied by English versions from the 1914 translation by H. Rackham.</p>
<h2>Why do we use it?</h2>
<p>It is a long established fact that a reader will be distracted by the readable content of a page when looking at its layout. The point of using Lorem Ipsum is that it has a more-or-less normal distribution of letters, as opposed to using 'Content here, content here', making it look like readable English. Many desktop publishing packages and web page editors now use Lorem Ipsum as their default model text, and a search for 'lorem ipsum' will uncover many web sites still in their infancy. Various versions have evolved over the years, sometimes by accident, sometimes on purpose (injected humour and the like).</p>
<h2>Where can I get some?</h2>
<p>There are many variations of passages of Lorem Ipsum available, but the majority have suffered alteration in some form, by injected humour, or randomised words which don't look even slightly believable. If you are going to use a passage of Lorem Ipsum, you need to be sure there isn't anything embarrassing hidden in the middle of text. All the Lorem Ipsum generators on the Internet tend to repeat predefined chunks as necessary, making this the first true generator on the Internet. It uses a dictionary of over 200 Latin words, combined with a handful of model sentence structures, to generate Lorem Ipsum which looks reasonable. The generated Lorem Ipsum is therefore always free from repetition, injected humour, or non-characteristic words etc.</p2>
`,
	},
	{
		Title:       "Hello future",
		Slug:        "hello-future",
		Status:      "published",
		PublishedAt: time.Date(2024, 11, 7, 17, 24, 0, 0, time.Local),
		Excerpt:     "Praesent massa sem, maximus ut purus nec, lacinia interdum lorem. Integer nec finibus augue. Aenean blandit arcu id quam aliquet, ac porta lorem iaculis. Suspendisse potenti. Aenean non porta magna. Mauris eget est sit amet justo tincidunt scelerisque at non mi. Integer in elementum urna. Nullam ac.",
		Content: `
<h2>What is Lorem Ipsum?</h2>
<p>Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.</p>
<h2>Where does it come from?</h2>
<p>Contrary to popular belief, Lorem Ipsum is not simply random text. It has roots in a piece of classical Latin literature from 45 BC, making it over 2000 years old. Richard McClintock, a Latin professor at Hampden-Sydney College in Virginia, looked up one of the more obscure Latin words, consectetur, from a Lorem Ipsum passage, and going through the cites of the word in classical literature, discovered the undoubtable source. Lorem Ipsum comes from sections 1.10.32 and 1.10.33 of "de Finibus Bonorum et Malorum" (The Extremes of Good and Evil) by Cicero, written in 45 BC. This book is a treatise on the theory of ethics, very popular during the Renaissance. The first line of Lorem Ipsum, "Lorem ipsum dolor sit amet..", comes from a line in section 1.10.32.</p>
<p>The standard chunk of Lorem Ipsum used since the 1500s is reproduced below for those interested. Sections 1.10.32 and 1.10.33 from "de Finibus Bonorum et Malorum" by Cicero are also reproduced in their exact original form, accompanied by English versions from the 1914 translation by H. Rackham.</p>
<h2>Why do we use it?</h2>
<p>It is a long established fact that a reader will be distracted by the readable content of a page when looking at its layout. The point of using Lorem Ipsum is that it has a more-or-less normal distribution of letters, as opposed to using 'Content here, content here', making it look like readable English. Many desktop publishing packages and web page editors now use Lorem Ipsum as their default model text, and a search for 'lorem ipsum' will uncover many web sites still in their infancy. Various versions have evolved over the years, sometimes by accident, sometimes on purpose (injected humour and the like).</p>
<h2>Where can I get some?</h2>
<p>There are many variations of passages of Lorem Ipsum available, but the majority have suffered alteration in some form, by injected humour, or randomised words which don't look even slightly believable. If you are going to use a passage of Lorem Ipsum, you need to be sure there isn't anything embarrassing hidden in the middle of text. All the Lorem Ipsum generators on the Internet tend to repeat predefined chunks as necessary, making this the first true generator on the Internet. It uses a dictionary of over 200 Latin words, combined with a handful of model sentence structures, to generate Lorem Ipsum which looks reasonable. The generated Lorem Ipsum is therefore always free from repetition, injected humour, or non-characteristic words etc.</p2>
`,
	},
}

func NotFoundPageHandler(tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		pageData := struct {
			Settings Settings
		}{
			Settings: Settings{
				Title:   "AwesomePress",
				Tagline: "My blog is awesome",
			},
		}

		err := tpl.ExecuteTemplate(w, "404-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render login page template: %w", err))
		}
	}
}

func HomePageHandler(tpl *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			NotFoundPageHandler(tpl)(w, r)

			return
		}

		pageData := struct {
			Settings Settings
			Posts    []Post
		}{
			Settings: Settings{
				Title:   "AwesomePress",
				Tagline: "My blog is awesome",
			},
			Posts: posts,
		}
		err := tpl.ExecuteTemplate(w, "home-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render home page template: %w", err))
		}
	})
}

func findPostBySlug(postSlug string) (*Post, error) {
	for i := range posts {
		if posts[i].Slug == postSlug {
			return &posts[i], nil
		}
	}

	return nil, fmt.Errorf("post with slug %s not found", postSlug)
}

func SinglePostPageHandler(tpl *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		post, err := findPostBySlug(r.PathValue("postSlug"))
		if err != nil {
			NotFoundPageHandler(tpl)(w, r)

			return
		}

		pageData := struct {
			Settings Settings
			Post     Post
		}{
			Settings: Settings{
				Title:   "AwesomePress",
				Tagline: "My blog is awesome",
			},
			Post: *post,
		}

		err = tpl.ExecuteTemplate(w, "single-post-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render single post page template: %w", err))
		}
	})
}

func LoginPageHandler(tpl *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageData := struct {
			Settings Settings
		}{
			Settings: Settings{
				Title:   "AwesomePress",
				Tagline: "My blog is awesome",
			},
		}

		err := tpl.ExecuteTemplate(w, "login-page", pageData)
		if err != nil {
			panic(fmt.Errorf("failed to render login page template: %w", err))
		}
	})
}

func LoginHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Email Address: %s", strings.TrimSpace(r.FormValue("emailAddress")))
		log.Printf("Password: %s", strings.TrimSpace(r.FormValue("password")))

		w.WriteHeader(http.StatusNotImplemented)
	})
}
