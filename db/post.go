package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/nasermirzaei89/blog/service"
	"log"
)

type PostRepository struct {
	db    *sql.DB
	table string
	cols  []string
}

func (repo *PostRepository) Insert(ctx context.Context, post *service.Post) error {
	q := squirrel.Insert(repo.table).Columns(repo.cols...).Values(
		post.UUID,
		post.CreatedAt,
		post.UpdatedAt,
		post.Title,
		post.Slug,
		post.Status,
		post.PublishedAt,
		post.Excerpt,
		post.Content,
	)

	_, err := q.RunWith(repo.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec db query: %w", err)
	}

	return nil
}

func (repo *PostRepository) Get(ctx context.Context, postUUID uuid.UUID) (*service.Post, error) {
	q := squirrel.Select(repo.cols...).From(repo.table).Where(squirrel.Eq{"uuid": postUUID})

	var post service.Post

	err := q.RunWith(repo.db).QueryRowContext(ctx).Scan(
		&post.UUID,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.Title,
		&post.Slug,
		&post.Status,
		&post.PublishedAt,
		&post.Excerpt,
		&post.Content,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.PostByUUIDNotFoundError{UUID: postUUID}
		}

		return nil, fmt.Errorf("error on query db row: %w", err)
	}

	return &post, nil
}

func (repo *PostRepository) GetBySlug(ctx context.Context, slug string) (*service.Post, error) {
	q := squirrel.Select(repo.cols...).From(repo.table).Where(squirrel.Eq{"slug": slug})

	var post service.Post

	err := q.RunWith(repo.db).QueryRowContext(ctx).Scan(
		&post.UUID,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.Title,
		&post.Slug,
		&post.Status,
		&post.PublishedAt,
		&post.Excerpt,
		&post.Content,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.PostBySlugNotFoundError{Slug: slug}
		}

		return nil, fmt.Errorf("error on query db row: %w", err)
	}

	return &post, nil
}

func (repo *PostRepository) ListAll(ctx context.Context) ([]service.Post, error) {
	q := squirrel.Select(repo.cols...).From(repo.table).OrderBy("updated_at DESC")

	rows, err := q.RunWith(repo.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error on query db: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("error closing rows: %v", err)
		}
	}()

	var postList []service.Post

	for rows.Next() {
		var post service.Post

		err := rows.Scan(
			&post.UUID,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.Title,
			&post.Slug,
			&post.Status,
			&post.PublishedAt,
			&post.Excerpt,
			&post.Content,
		)
		if err != nil {
			return nil, fmt.Errorf("error on row scan: %w", err)
		}

		postList = append(postList, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error on rows scan: %w", err)
	}

	return postList, nil
}

func (repo *PostRepository) ListPublished(ctx context.Context) ([]service.Post, error) {
	q := squirrel.Select(repo.cols...).From(repo.table).
		Where(squirrel.Eq{"status": service.PostStatusPublished}).
		OrderBy("published_at DESC")

	rows, err := q.RunWith(repo.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error on query db: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("error closing rows: %v", err)
		}
	}()

	var postList []service.Post

	for rows.Next() {
		var post service.Post

		err := rows.Scan(
			&post.UUID,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.Title,
			&post.Slug,
			&post.Status,
			&post.PublishedAt,
			&post.Excerpt,
			&post.Content,
		)

		if err != nil {
			return nil, fmt.Errorf("error on row scan: %w", err)
		}

		postList = append(postList, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error on rows scan: %w", err)
	}

	return postList, nil
}

func (repo *PostRepository) Replace(ctx context.Context, itemUUID uuid.UUID, post *service.Post) error {
	q := squirrel.Update(repo.table).
		SetMap(map[string]interface{}{
			"uuid":         post.UUID,
			"created_at":   post.CreatedAt,
			"updated_at":   post.UpdatedAt,
			"title":        post.Title,
			"slug":         post.Slug,
			"status":       post.Status,
			"published_at": post.PublishedAt,
			"excerpt":      post.Excerpt,
			"content":      post.Content,
		},
		).Where(squirrel.Eq{"uuid": itemUUID})

	_, err := q.RunWith(repo.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec db query: %w", err)
	}

	return nil
}

func (repo *PostRepository) Delete(ctx context.Context, itemUUID uuid.UUID) error {
	q := squirrel.Delete(repo.table).Where(squirrel.Eq{"uuid": itemUUID})

	_, err := q.RunWith(repo.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec db query: %w", err)
	}

	return nil
}

var _ service.PostRepository = (*PostRepository)(nil)

func NewPostRepository(db *sql.DB) *PostRepository {
	return &PostRepository{
		db:    db,
		table: "posts",
		cols: []string{
			"uuid",
			"created_at",
			"updated_at",
			"title",
			"slug",
			"status",
			"published_at",
			"excerpt",
			"content",
		},
	}
}
