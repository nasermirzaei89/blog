package sqlite3

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Masterminds/squirrel"
	"github.com/nasermirzaei89/fullstackgo/blog"
)

type PostRepo struct {
	DB *sql.DB
}

func (repo *PostRepo) List(ctx context.Context, params blog.ListPostsParams) ([]*blog.Post, error) {
	q := squirrel.Select("*").From("posts").OrderBy("created_at DESC")

	if params.Limit > 0 {
		q = q.Limit(uint64(params.Limit))
	}

	if params.Offset > 0 {
		q = q.Offset(uint64(params.Offset))
	}

	q = q.RunWith(repo.DB)

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

	var posts []*blog.Post

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

func (repo *PostRepo) Count(ctx context.Context) (int, error) {
	q := squirrel.Select("COUNT(*)").From("posts")
	q = q.RunWith(repo.DB)

	var count int

	err := q.QueryRowContext(ctx).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error on count posts: %w", err)
	}

	return count, nil
}

func (repo *PostRepo) GetBySlug(ctx context.Context, slug string) (*blog.Post, error) {
	q := squirrel.Select("*").From("posts").Where(squirrel.Eq{"slug": slug})

	q = q.RunWith(repo.DB)

	post, err := scanPost(q.QueryRowContext(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, blog.PostBySlugNotFoundError{Slug: slug}
		}

		return nil, fmt.Errorf("error on scan post: %w", err)
	}

	return post, nil
}

func (repo *PostRepo) GetByID(ctx context.Context, id string) (*blog.Post, error) {
	q := squirrel.Select("*").From("posts").Where(squirrel.Eq{"id": id})

	q = q.RunWith(repo.DB)

	post, err := scanPost(q.QueryRowContext(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, blog.PostByIDNotFoundError{ID: id}
		}

		return nil, fmt.Errorf("error on scan post: %w", err)
	}

	return post, nil
}

func (repo *PostRepo) SlugExists(ctx context.Context, slug string) (bool, error) {
	q := squirrel.Select("COUNT(*)").From("posts").Where(squirrel.Eq{"slug": slug})

	q = q.RunWith(repo.DB)

	var count int

	err := q.QueryRowContext(ctx).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("error on query slug existence: %w", err)
	}

	return count > 0, nil
}

func scanPost(rs squirrel.RowScanner) (*blog.Post, error) {
	var post blog.Post

	err := rs.Scan(
		&post.ID,
		&post.Title,
		&post.Slug,
		&post.Excerpt,
		&post.Content,
		&post.AuthorID,
		&post.CreatedAt,
		&post.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("error on scan row: %w", err)
	}

	return &post, nil
}

func (repo *PostRepo) Create(ctx context.Context, post *blog.Post) error {
	q := squirrel.Insert("posts").
		Columns("id", "title", "slug", "excerpt", "content", "author_id", "created_at", "updated_at").
		Values(post.ID, post.Title, post.Slug, post.Excerpt, post.Content, post.AuthorID, post.CreatedAt, post.UpdatedAt)

	q = q.RunWith(repo.DB)

	_, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec insert: %w", err)
	}

	return nil
}

func (repo *PostRepo) Update(ctx context.Context, post *blog.Post) error {
	q := squirrel.Update("posts").
		Set("title", post.Title).
		Set("slug", post.Slug).
		Set("excerpt", post.Excerpt).
		Set("content", post.Content).
		Set("author_id", post.AuthorID).
		Set("updated_at", post.UpdatedAt).
		Where(squirrel.Eq{"id": post.ID})

	q = q.RunWith(repo.DB)

	result, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec replace: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error on get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return blog.PostByIDNotFoundError{ID: post.ID}
	}

	return nil
}

func (repo *PostRepo) Delete(ctx context.Context, id string) error {
	q := squirrel.Delete("posts").Where(squirrel.Eq{"id": id})

	q = q.RunWith(repo.DB)

	result, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec delete: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error on get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return blog.PostByIDNotFoundError{ID: id}
	}

	return nil
}
