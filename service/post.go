package service

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"html/template"
	"time"
)

type PostStatus string

const (
	PostStatusDraft     = "draft"
	PostStatusPublished = "published"
)

type Post struct {
	UUID        uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Title       string
	Slug        string
	Status      PostStatus
	PublishedAt *time.Time
	Excerpt     string
	Content     template.HTML
}

type PostByUUIDNotFoundError struct {
	UUID uuid.UUID
}

func (err PostByUUIDNotFoundError) Error() string {
	return fmt.Sprintf("post with UUID %s not found", err.UUID)
}

type PostBySlugNotFoundError struct {
	Slug string
}

func (err PostBySlugNotFoundError) Error() string {
	return fmt.Sprintf("post with slug %s not found", err.Slug)
}

type Pagination struct {
	Offset uint64
	Limit  uint64
}

type ListPublishedRequest struct {
	Pagination Pagination
}

type PostRepository interface {
	Insert(ctx context.Context, post *Post) (err error)
	Get(ctx context.Context, postUUID uuid.UUID) (post *Post, err error)
	GetBySlug(ctx context.Context, slug string) (post *Post, err error)
	ListAll(ctx context.Context) (postList []Post, err error)
	ListPublished(ctx context.Context, req ListPublishedRequest) (postList []Post, err error)
	CountPublished(ctx context.Context) (count uint64, err error)
	Replace(ctx context.Context, postUUID uuid.UUID, post *Post) (err error)
	Delete(ctx context.Context, postUUID uuid.UUID) (err error)
}
