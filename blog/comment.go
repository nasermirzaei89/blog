package blog

import (
	"context"
	"fmt"
	"time"
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

type ListCommentsParams struct {
	PostID string
}

type CommentRepository interface {
	Create(ctx context.Context, comment *Comment) (err error)
	List(ctx context.Context, params ListCommentsParams) (comments []*Comment, err error)
	GetByID(ctx context.Context, id string) (comment *Comment, err error)
	Update(ctx context.Context, comment *Comment) (err error)
	Delete(ctx context.Context, id string) (err error)
}

type CommentByIDNotFoundError struct {
	ID string
}

func (err CommentByIDNotFoundError) Error() string {
	return fmt.Sprintf("comment with ID %q not found", err.ID)
}
