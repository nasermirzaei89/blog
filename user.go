package blog

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

type UserRepo struct {
	DB *sql.DB
}

func scanUser(rs squirrel.RowScanner) (*User, error) {
	var user User

	err := rs.Scan(
		&user.ID,
		&user.Username,
		&user.EmailAddress,
		&user.PasswordHash,
		&user.Name,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("error on scan row: %w", err)
	}

	return &user, nil
}

func (repo *UserRepo) GetByUsername(ctx context.Context, username string) (*User, error) {
	q := squirrel.Select("*").From("users").Where(squirrel.Eq{"username": username})

	q = q.RunWith(repo.DB)

	user, err := scanUser(q.QueryRowContext(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, UserByUsernameNotFoundError{Username: username}
		}

		return nil, fmt.Errorf("error on scan user: %w", err)
	}

	return user, nil
}

func (repo *UserRepo) GetByEmailAddress(
	ctx context.Context,
	emailAddress string,
) (*User, error) {
	q := squirrel.Select("*").From("users").Where(squirrel.Eq{"email_address": emailAddress})
	q = q.RunWith(repo.DB)

	user, err := scanUser(q.QueryRowContext(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, UserByEmailNotFoundError{EmailAddress: emailAddress}
		}

		return nil, fmt.Errorf("error on scan user: %w", err)
	}

	return user, nil
}

func (repo *UserRepo) GetByID(ctx context.Context, id string) (*User, error) {
	q := squirrel.Select("*").From("users").Where(squirrel.Eq{"id": id})
	q = q.RunWith(repo.DB)

	user, err := scanUser(q.QueryRowContext(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, UserByIDNotFoundError{ID: id}
		}

		return nil, fmt.Errorf("error on scan user: %w", err)
	}

	return user, nil
}

func (repo *UserRepo) List(ctx context.Context, params ListUsersParams) ([]*User, error) {
	q := squirrel.Select("*").From("users")

	if params.Username != "" {
		q = q.Where(squirrel.Eq{"username": params.Username})
	}

	if params.EmailAddress != "" {
		q = q.Where(squirrel.Eq{"email_address": params.EmailAddress})
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

func (repo *UserRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	q := squirrel.Select("1").From("users").Where(squirrel.Eq{"username": username})

	q = q.RunWith(repo.DB)

	var dummy int

	err := q.QueryRowContext(ctx).Scan(&dummy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, fmt.Errorf("error on scan user: %w", err)
	}

	return true, nil
}

func (repo *UserRepo) ExistsByEmailAddress(
	ctx context.Context,
	emailAddress string,
) (bool, error) {
	q := squirrel.Select("1").From("users").Where(squirrel.Eq{"email_address": emailAddress})

	q = q.RunWith(repo.DB)

	var dummy int

	err := q.QueryRowContext(ctx).Scan(&dummy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, fmt.Errorf("error on scan user: %w", err)
	}

	return true, nil
}

func (repo *UserRepo) Create(ctx context.Context, user *User) error {
	q := squirrel.Insert("users").
		Columns("id", "username", "email_address", "password_hash", "name", "avatar_url", "created_at", "updated_at").
		Values(user.ID, user.Username, user.EmailAddress, user.PasswordHash, user.Name, user.AvatarURL, user.CreatedAt, user.UpdatedAt)

	q = q.RunWith(repo.DB)

	_, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec query: %w", err)
	}

	return nil
}

func (repo *UserRepo) Update(ctx context.Context, user *User) error {
	q := squirrel.Update("users").
		Set("username", user.Username).
		Set("email_address", user.EmailAddress).
		Set("password_hash", user.PasswordHash).
		Set("name", user.Name).
		Set("avatar_url", user.AvatarURL).
		Set("updated_at", user.UpdatedAt).
		Where(squirrel.Eq{"id": user.ID})

	q = q.RunWith(repo.DB)

	_, err := q.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error on exec query: %w", err)
	}

	return nil
}
