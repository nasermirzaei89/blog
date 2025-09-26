package sqlite3

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Masterminds/squirrel"
	"github.com/nasermirzaei89/fullstackgo/web"
)

type UserRepo struct {
	DB *sql.DB
}

func scanUser(rs squirrel.RowScanner) (*web.User, error) {
	var user web.User

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

func (repo *UserRepo) GetByUsername(ctx context.Context, username string) (*web.User, error) {
	q := squirrel.Select("*").From("users").Where(squirrel.Eq{"username": username})

	q = q.RunWith(repo.DB)

	user, err := scanUser(q.QueryRowContext(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, web.UserByUsernameNotFoundError{Username: username}
		}

		return nil, fmt.Errorf("error on scan user: %w", err)
	}

	return user, nil
}

func (repo *UserRepo) GetByEmailAddress(
	ctx context.Context,
	emailAddress string,
) (*web.User, error) {
	q := squirrel.Select("*").From("users").Where(squirrel.Eq{"email_address": emailAddress})
	q = q.RunWith(repo.DB)

	user, err := scanUser(q.QueryRowContext(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, web.UserByEmailNotFoundError{EmailAddress: emailAddress}
		}

		return nil, fmt.Errorf("error on scan user: %w", err)
	}

	return user, nil
}

func (repo *UserRepo) GetByID(ctx context.Context, id string) (*web.User, error) {
	q := squirrel.Select("*").From("users").Where(squirrel.Eq{"id": id})
	q = q.RunWith(repo.DB)

	user, err := scanUser(q.QueryRowContext(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, web.UserByIDNotFoundError{ID: id}
		}

		return nil, fmt.Errorf("error on scan user: %w", err)
	}

	return user, nil
}

func (repo *UserRepo) List(ctx context.Context, params web.ListUsersParams) ([]*web.User, error) {
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

	var users []*web.User

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

func (repo *UserRepo) Create(ctx context.Context, user *web.User) error {
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

func (repo *UserRepo) Update(ctx context.Context, user *web.User) error {
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
