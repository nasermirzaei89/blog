package main

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
	db *sql.DB
}

func (repo *CommentRepository) Insert(ctx context.Context, comment *Comment) error {
	q := squirrel.Insert("comments").
		Columns("id", "post_id", "user_id", "content", "created_at", "updated_at").
		Values(comment.ID, comment.PostID, comment.UserID, comment.Content, comment.CreatedAt, comment.UpdatedAt)

	q = q.RunWith(repo.db)

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
