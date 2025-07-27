package blog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
)

type PasswordResetToken struct {
	ID        string
	UserID    string
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type PasswordResetTokenRepository struct {
	DB *sql.DB
}

func scanPasswordResetToken(rs squirrel.RowScanner) (*PasswordResetToken, error) {
	var token PasswordResetToken

	err := rs.Scan(&token.ID, &token.UserID, &token.Token, &token.CreatedAt, &token.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("error on scan row: %w", err)
	}

	return &token, nil
}

func (repo *PasswordResetTokenRepository) GetByToken(ctx context.Context, tokenStr string) (*PasswordResetToken, error) {
	q := squirrel.Select("*").From("password_reset_tokens").Where(squirrel.Eq{"token": tokenStr})

	q = q.RunWith(repo.DB)

	token, err := scanPasswordResetToken(q.QueryRowContext(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("password reset token '%s' not found", tokenStr)
		}

		return nil, fmt.Errorf("error on scan password reset token: %w", err)
	}

	return token, nil
}

func (repo *PasswordResetTokenRepository) Create(ctx context.Context, token *PasswordResetToken) error {
	q := squirrel.Insert("password_reset_tokens").
		Columns("id", "user_id", "token", "created_at", "expires_at").
		Values(token.ID, token.UserID, token.Token, token.CreatedAt, token.ExpiresAt).
		RunWith(repo.DB)

	_, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on create password reset token: %w", err)
	}

	return nil
}

func (repo *PasswordResetTokenRepository) Delete(ctx context.Context, tokenID string) error {
	q := squirrel.Delete("password_reset_tokens").Where(squirrel.Eq{"id": tokenID})
	q = q.RunWith(repo.DB)

	_, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on delete password reset token: %w", err)
	}

	return nil
}
