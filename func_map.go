package blog

import (
	"cmp"
	"html/template"
	"time"
)

var Funcs = template.FuncMap{
	"formatTime": func(t time.Time, layout string) string {
		return t.Format(layout)
	},
	"or": func(s1, s2 string) string {
		return cmp.Or(s1, s2)
	},
	"html": func(s string) template.HTML {
		return template.HTML(s)
	},
	"sub": func(a, b int) int {
		return a - b
	},
	"add": func(a, b int) int {
		return a + b
	},
}
