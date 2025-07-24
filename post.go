package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/Masterminds/squirrel"
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

func (repo *PostRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	q := squirrel.Select("COUNT(*)").From("posts").Where(squirrel.Eq{"slug": slug})

	q = q.RunWith(repo.db)

	var count int
	err := q.QueryRowContext(ctx).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("error on query slug existence: %w", err)
	}

	return count > 0, nil
}

func scanPost(rs squirrel.RowScanner) (*Post, error) {
	var post Post

	err := rs.Scan(&post.ID, &post.Title, &post.Slug, &post.Excerpt, &post.Content, &post.AuthorID, &post.CreatedAt, &post.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("error on scan row: %w", err)
	}

	return &post, nil
}

func (repo *PostRepository) Insert(ctx context.Context, post *Post) error {
	q := squirrel.Insert("posts").
		Columns("id", "title", "slug", "excerpt", "content", "author_id", "created_at", "updated_at").
		Values(post.ID, post.Title, post.Slug, post.Excerpt, post.Content, post.AuthorID, post.CreatedAt, post.UpdatedAt)

	q = q.RunWith(repo.db)

	result, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec insert: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error on get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no rows affected")
	}

	return nil
}

func (repo *PostRepository) Replace(ctx context.Context, id string, post *Post) error {
	q := squirrel.Update("posts").
		Set("title", post.Title).
		Set("slug", post.Slug).
		Set("excerpt", post.Excerpt).
		Set("content", post.Content).
		Set("author_id", post.AuthorID).
		Set("updated_at", post.UpdatedAt).
		Where(squirrel.Eq{"id": id})

	q = q.RunWith(repo.db)

	result, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec replace: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error on get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no rows affected")
	}

	return nil
}

func (repo *PostRepository) Delete(ctx context.Context, id string) error {
	q := squirrel.Delete("posts").Where(squirrel.Eq{"id": id})

	q = q.RunWith(repo.db)

	result, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec delete: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error on get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no rows affected")
	}

	return nil
}
