package auth

import (
	"context"
	"fmt"
	"time"
)

type User struct {
	ID           string
	Username     string
	EmailAddress string
	PasswordHash string
	Name         string
	AvatarURL    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ListUsersParams struct {
	Username     string
	EmailAddress string
}

type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (user *User, err error)
	GetByEmailAddress(ctx context.Context, emailAddress string) (user *User, err error)
	GetByID(ctx context.Context, id string) (user *User, err error)
	List(ctx context.Context, params ListUsersParams) (users []*User, err error)
	ExistsByUsername(ctx context.Context, username string) (exists bool, err error)
	ExistsByEmailAddress(ctx context.Context, emailAddress string) (exists bool, err error)
	Create(ctx context.Context, user *User) (err error)
	Update(ctx context.Context, user *User) (err error)
}

type UserByUsernameNotFoundError struct {
	Username string
}

func (err UserByUsernameNotFoundError) Error() string {
	return fmt.Sprintf("user by username '%s' not found", err.Username)
}

type UserByEmailNotFoundError struct {
	EmailAddress string
}

func (err UserByEmailNotFoundError) Error() string {
	return fmt.Sprintf("user by email address '%s' not found", err.EmailAddress)
}

type UserByIDNotFoundError struct {
	ID string
}

func (err UserByIDNotFoundError) Error() string {
	return fmt.Sprintf("user by id '%s' not found", err.ID)
}
