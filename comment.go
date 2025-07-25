package blog

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/Masterminds/squirrel"
)

type Comment struct {
	ID            string
	PostID        string
	UserID        string
	UserUsername  string
	UserName      string
	UserAvatarURL string
	Content       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CommentRepository struct {
	DB *sql.DB
}

func (repo *CommentRepository) Create(ctx context.Context, comment *Comment) error {
	q := squirrel.Insert("comments").
		Columns("id", "post_id", "user_id", "content", "created_at", "updated_at").
		Values(comment.ID, comment.PostID, comment.UserID, comment.Content, comment.CreatedAt, comment.UpdatedAt)

	q = q.RunWith(repo.DB)

	_, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec query: %w", err)
	}

	return nil
}

func scanComment(rs squirrel.RowScanner) (*Comment, error) {
	var comment Comment

	err := rs.Scan(&comment.ID, &comment.PostID, &comment.UserID, &comment.UserUsername, &comment.UserName, &comment.UserAvatarURL, &comment.Content, &comment.CreatedAt, &comment.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("error on scan row: %w", err)
	}

	return &comment, nil
}

type ListCommentsParams struct {
	PostID string
}

func (repo *CommentRepository) List(ctx context.Context, params ListCommentsParams) ([]*Comment, error) {
	q := squirrel.Select(
		"c.id",
		"c.post_id",
		"c.user_id",
		"u.username",
		"u.name",
		"u.avatar_url",
		"c.content",
		"c.created_at",
		"c.updated_at",
	).From("comments c").Join("users u ON c.user_id = u.id").OrderBy("c.created_at ASC")

	if params.PostID != "" {
		q = q.Where(squirrel.Eq{"c.post_id": params.PostID})
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

	var comments []*Comment

	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return nil, fmt.Errorf("error on scan comment: %w", err)
		}

		comments = append(comments, comment)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error on rows: %w", err)
	}

	return comments, nil
}

func (repo *CommentRepository) GetByID(ctx context.Context, id string) (*Comment, error) {
	q := squirrel.Select(
		"c.id",
		"c.post_id",
		"c.user_id",
		"u.username",
		"u.name",
		"u.avatar_url",
		"c.content",
		"c.created_at",
		"c.updated_at",
	).From("comments c").Join("users u ON c.user_id = u.id").Where(squirrel.Eq{"c.id": id})

	q = q.RunWith(repo.DB)

	comment, err := scanComment(q.QueryRowContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("error on scan comment: %w", err)
	}

	return comment, nil
}

func (repo *CommentRepository) Update(ctx context.Context, comment *Comment) error {
	q := squirrel.Update("comments").SetMap(map[string]any{
		"post_id":    comment.PostID,
		"user_id":    comment.UserID,
		"content":    comment.Content,
		"updated_at": comment.UpdatedAt,
	}).Where(squirrel.Eq{"id": comment.ID})

	q = q.RunWith(repo.DB)

	result, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec query: %w", err)
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

func (repo *CommentRepository) Delete(ctx context.Context, id string) error {
	q := squirrel.Delete("comments").Where(squirrel.Eq{"id": id})

	q = q.RunWith(repo.DB)

	_, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec query: %w", err)
	}

	return nil
}
