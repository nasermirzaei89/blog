package main

import "time"

const DateTimeLocalFormat = "2006-01-02T15:04"

var funcMap = map[string]any{
	"formatDateTime": func(t time.Time, tz *time.Location) string {
		return t.In(tz).Format("Jan _2, 2006 15:04")
	},
	"formatDate": func(t time.Time, tz *time.Location) string {
		return t.In(tz).Format("Jan _2, 2006")
	},
	"formatDateTimeLocal": func(t time.Time, tz *time.Location) string {
		return t.In(tz).Format(DateTimeLocalFormat)
	},
}
