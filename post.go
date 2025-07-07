package main

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"time"

	"github.com/Masterminds/squirrel"
)

type Post struct {
	ID        string
	Title     string
	Slug      string
	Excerpt   string
	Content   template.HTML
	AuthorID  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PostRepository struct {
	db *sql.DB
}

func (repo *PostRepository) List(ctx context.Context) ([]*Post, error) {
	q := squirrel.Select("*").From("posts").OrderBy("created_at DESC")

	q = q.RunWith(repo.db)

	rows, err := q.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error on query db: %w", err)
	}

	defer func() {
		err := rows.Close()
		if err != nil {
			slog.ErrorContext(ctx, "error on close rows", "error", err)
		}
	}()

	var posts []*Post

	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("error on scan post: %w", err)
		}

		posts = append(posts, post)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error on rows: %w", err)
	}

	return posts, nil
}

func (repo *PostRepository) GetBySlug(ctx context.Context, slug string) (*Post, error) {
	q := squirrel.Select("*").From("posts").Where(squirrel.Eq{"slug": slug})

	q = q.RunWith(repo.db)

	post, err := scanPost(q.QueryRowContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("error on scan post: %w", err)
	}

	return post, nil
}

func (repo *PostRepository) GetByID(ctx context.Context, id string) (*Post, error) {
	q := squirrel.Select("*").From("posts").Where(squirrel.Eq{"id": id})

	q = q.RunWith(repo.db)

	post, err := scanPost(q.QueryRowContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("error on scan post: %w", err)
	}

	return post, nil
}

func scanPost(rs squirrel.RowScanner) (*Post, error) {
	var post Post

	var postCreatedAt, postUpdatedAt string

	err := rs.Scan(&post.ID, &post.Title, &post.Slug, &post.Excerpt, &post.Content, &post.AuthorID, &postCreatedAt, &postUpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("error on scan row: %w", err)
	}

	post.CreatedAt, err = time.Parse(time.RFC3339, postCreatedAt)
	if err != nil {
		return nil, fmt.Errorf("error on parse post created at field: %w", err)
	}

	post.UpdatedAt, err = time.Parse(time.RFC3339, postUpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("error on parse post updated at field: %w", err)
	}

	return &post, nil
}
