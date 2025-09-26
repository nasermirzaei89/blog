package web

import (
	"context"
	"time"
)

type Post struct {
	ID        string
	Title     string
	Slug      string
	Excerpt   string
	Content   string
	AuthorID  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ListPostsParams struct {
	Limit  int
	Offset int
}

type PostRepository interface {
	List(ctx context.Context, params ListPostsParams) (posts []*Post, err error)
	Count(ctx context.Context) (count int, err error)
	GetBySlug(ctx context.Context, slug string) (post *Post, err error)
	GetByID(ctx context.Context, id string) (post *Post, err error)
	SlugExists(ctx context.Context, slug string) (exists bool, err error)
	Create(ctx context.Context, post *Post) (err error)
	Update(ctx context.Context, post *Post) (err error)
	Delete(ctx context.Context, id string) (err error)
}
