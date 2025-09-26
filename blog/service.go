package blog

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	slugify "github.com/gosimple/slug"
	"github.com/microcosm-cc/bluemonday"
)

type Service struct {
	PostRepo    PostRepository
	CommentRepo CommentRepository
	HTMLPolicy  *bluemonday.Policy
	TextPolicy  *bluemonday.Policy
}

func (svc *Service) GetPostBySlug(ctx context.Context, slug string) (*Post, error) {
	post, err := svc.PostRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get post by slug: %w", err)
	}

	return post, nil
}

func (svc *Service) PostBySlugExists(ctx context.Context, slug string) (bool, error) {
	exists, err := svc.PostRepo.SlugExists(ctx, slug)
	if err != nil {
		return false, fmt.Errorf("failed to check if post slug exists: %w", err)
	}

	return exists, nil
}

func (svc *Service) GetPostByID(ctx context.Context, id string) (*Post, error) {
	post, err := svc.PostRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get post by ID: %w", err)
	}

	return post, nil
}

func (svc *Service) ListPosts(ctx context.Context, params ListPostsParams) ([]*Post, error) {
	posts, err := svc.PostRepo.List(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list posts: %w", err)
	}

	return posts, nil
}

func (svc *Service) CountPosts(ctx context.Context) (int, error) {
	count, err := svc.PostRepo.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count posts: %w", err)
	}

	return count, nil
}

type CreatePostRequest struct {
	Title    string
	Slug     string
	Excerpt  string
	Content  string
	AuthorID string
}

func (svc *Service) CreatePost(ctx context.Context, req *CreatePostRequest) (*Post, error) {
	if req.Slug == "" {
		req.Slug = req.Title
	}

	req.Slug = slugify.Make(req.Slug)

	uniqueSlug, err := svc.generateUniqueSlug(ctx, req.Slug)
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique slug: %w", err)
	}

	if req.Excerpt == "" {
		req.Excerpt = svc.TextPolicy.Sanitize(req.Content)
	}

	req.Excerpt = svc.generateExcerpt(req.Excerpt, 160)

	req.Content = svc.HTMLPolicy.Sanitize(req.Content)

	timeNow := time.Now()

	post := &Post{
		ID:        uuid.NewString(),
		Title:     req.Title,
		Slug:      uniqueSlug,
		Excerpt:   req.Excerpt,
		Content:   req.Content,
		AuthorID:  req.AuthorID,
		CreatedAt: timeNow,
		UpdatedAt: timeNow,
	}

	err = svc.PostRepo.Create(ctx, post)
	if err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	return post, nil
}

type UpdatePostRequest struct {
	Title   string
	Slug    string
	Excerpt string
	Content string
}

func (svc *Service) UpdatePost(ctx context.Context, id string, req *UpdatePostRequest) (*Post, error) {
	if req.Slug == "" {
		req.Slug = req.Title
	}

	req.Slug = slugify.Make(req.Slug)

	uniqueSlug := req.Slug

	post, err := svc.GetPostByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	if uniqueSlug != post.Slug {
		var err error

		uniqueSlug, err = svc.generateUniqueSlug(ctx, req.Slug)
		if err != nil {
			return nil, fmt.Errorf("failed to generate unique slug: %w", err)
		}
	}

	if req.Excerpt == "" {
		req.Excerpt = svc.TextPolicy.Sanitize(req.Content)
	}

	req.Excerpt = svc.generateExcerpt(req.Excerpt, 160)

	req.Content = svc.HTMLPolicy.Sanitize(req.Content)

	post.Title = req.Title
	post.Slug = uniqueSlug
	post.Excerpt = req.Excerpt
	post.Content = req.Content
	post.UpdatedAt = time.Now()

	err = svc.PostRepo.Update(ctx, post)
	if err != nil {
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	return post, nil
}

func (svc *Service) generateExcerpt(content string, maxLength int) string {
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

var ErrUnableToGenerateUniqueSlug = errors.New("unable to generate unique slug after 1000 attempts")

func (svc *Service) generateUniqueSlug(ctx context.Context, baseSlug string) (string, error) {
	exists, err := svc.PostBySlugExists(ctx, baseSlug)
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

		num, err := strconv.Atoi(lastPart)
		if err == nil {
			basePart = strings.Join(parts[:len(parts)-1], "-")
			counter = num + 1
		}
	}

	for {
		candidateSlug := basePart + "-" + strconv.Itoa(counter)

		exists, err := svc.PostBySlugExists(ctx, candidateSlug)
		if err != nil {
			return "", fmt.Errorf("error checking slug existence: %w", err)
		}

		if !exists {
			return candidateSlug, nil
		}

		counter++

		if counter > 1000 {
			return "", ErrUnableToGenerateUniqueSlug
		}
	}
}

func (svc *Service) DeletePost(ctx context.Context, id string) error {
	err := svc.PostRepo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	return nil
}

func (svc *Service) ListComments(ctx context.Context, params ListCommentsParams) ([]*Comment, error) {
	comments, err := svc.CommentRepo.List(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	return comments, nil
}

type CreateCommentRequest struct {
	PostID  string
	UserID  string
	Content string
}

func (svc *Service) CreateComment(ctx context.Context, req *CreateCommentRequest) (*Comment, error) {
	timeNow := time.Now()

	req.Content = svc.HTMLPolicy.Sanitize(req.Content)

	comment := &Comment{
		ID:        uuid.NewString(),
		PostID:    req.PostID,
		UserID:    req.UserID,
		Content:   req.Content,
		CreatedAt: timeNow,
		UpdatedAt: timeNow,
	}

	err := svc.CommentRepo.Create(ctx, comment)
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return comment, nil
}

func (svc *Service) GetCommentByID(ctx context.Context, id string) (*Comment, error) {
	comment, err := svc.CommentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get comment by ID: %w", err)
	}

	return comment, nil
}

func (svc *Service) DeleteComment(ctx context.Context, id string) error {
	err := svc.CommentRepo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
}

type UpdateCommentRequest struct {
	Content string
}

func (svc *Service) UpdateComment(ctx context.Context, id string, req *UpdateCommentRequest) error {
	comment, err := svc.GetCommentByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get comment by ID: %w", err)
	}

	req.Content = svc.HTMLPolicy.Sanitize(req.Content)

	comment.Content = req.Content

	err = svc.CommentRepo.Update(ctx, comment)
	if err != nil {
		return fmt.Errorf("failed to update comment: %w", err)
	}

	return nil
}
