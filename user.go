package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Masterminds/squirrel"
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

type UserRepository struct {
	db *sql.DB
}

func scanUser(rs squirrel.RowScanner) (*User, error) {
	var user User

	err := rs.Scan(&user.ID, &user.Username, &user.EmailAddress, &user.PasswordHash, &user.Name, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("error on scan row: %w", err)
	}

	return &user, nil
}

type UserByUsernameNotFoundError struct {
	Username string
}

func (err UserByUsernameNotFoundError) Error() string {
	return fmt.Sprintf("user by username '%s' not found", err.Username)
}

func (repo *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	q := squirrel.Select("*").From("users").Where(squirrel.Eq{"username": username})

	q = q.RunWith(repo.db)

	user, err := scanUser(q.QueryRowContext(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, UserByUsernameNotFoundError{Username: username}
		}

		return nil, fmt.Errorf("error on scan user: %w", err)
	}

	return user, nil
}

type ListUsersParams struct {
	Username     string
	EmailAddress string
}

func (repo *UserRepository) List(ctx context.Context, params ListUsersParams) ([]*User, error) {
	q := squirrel.Select("*").From("users")

	if params.Username != "" {
		q = q.Where(squirrel.Eq{"username": params.Username})
	}

	if params.EmailAddress != "" {
		q = q.Where(squirrel.Eq{"email_address": params.EmailAddress})
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

	var users []*User

	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("error on scan user: %w", err)
		}

		users = append(users, user)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error on rows: %w", err)
	}

	return users, nil
}

func (repo *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	users, err := repo.List(ctx, ListUsersParams{Username: username})
	if err != nil {
		return false, fmt.Errorf("error on list users by username: %w", err)
	}

	return len(users) > 0, nil
}

func (repo *UserRepository) ExistsByEmailAddress(ctx context.Context, emailAddress string) (bool, error) {
	users, err := repo.List(ctx, ListUsersParams{EmailAddress: emailAddress})
	if err != nil {
		return false, fmt.Errorf("error on list users by emailAddress: %w", err)
	}

	return len(users) > 0, nil
}

func (repo *UserRepository) Insert(ctx context.Context, user *User) error {
	q := squirrel.Insert("users").
		Columns("id", "username", "email_address", "password_hash", "name", "avatar_url", "created_at", "updated_at").
		Values(user.ID, user.Username, user.EmailAddress, user.PasswordHash, user.Name, user.AvatarURL, user.CreatedAt, user.UpdatedAt)

	q = q.RunWith(repo.db)

	_, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec query: %w", err)
	}

	return nil
}
