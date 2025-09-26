package web

import (
	"context"
	"fmt"
	"time"
)

type PasswordResetToken struct {
	ID        string
	UserID    string
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type PasswordResetTokenRepository interface {
	GetByToken(ctx context.Context, tokenStr string) (passwordResetToken *PasswordResetToken, err error)
	Create(ctx context.Context, token *PasswordResetToken) (err error)
	Delete(ctx context.Context, tokenID string) (err error)
}

type PasswordResetTokenError struct {
	Token string
}

func (err PasswordResetTokenError) Error() string {
	return fmt.Sprintf("password reset token '%s' not found", err.Token)
}
