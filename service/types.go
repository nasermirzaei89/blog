package service

import (
	"time"
)

type Settings struct {
	Title    string
	Tagline  string
	TimeZone *time.Location
}

var InMemorySettings = Settings{
	Title:    "AwesomePress",
	Tagline:  "My blog is awesome",
	TimeZone: time.Local,
}

type PostList []Post

func (items PostList) WithStatus(status PostStatus) PostList {
	res := make([]Post, 0, len(items))
	for _, post := range items {
		if post.Status == status {
			res = append(res, post)
		}
	}

	return res
}
