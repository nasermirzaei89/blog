package db

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/nasermirzaei89/blog/service"
	"log"
	"time"
)

type SettingRepository struct {
	db    *sql.DB
	table string
}

func (repo *SettingRepository) Load(ctx context.Context) (*service.Settings, error) {
	q := squirrel.Select("name", "value").
		From(repo.table).
		Where(squirrel.Eq{"name": []string{"title", "tagline", "timeZone"}})

	rows, err := q.RunWith(repo.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error on query db: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("error closing rows: %v", err)
		}
	}()

	var settings service.Settings

	for rows.Next() {
		var (
			name  string
			value string
		)

		err := rows.Scan(&name, &value)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		switch name {
		case "title":
			settings.Title = value
		case "tagline":
			settings.Tagline = value
		case "timeZone":
			tz, err := time.LoadLocation(value)
			if err != nil {
				return nil, fmt.Errorf("error loading timezone: %w", err)
			}

			settings.TimeZone = tz
		}
	}

	return &settings, nil
}

func (repo *SettingRepository) Save(ctx context.Context, settings *service.Settings) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error beginning transaction: %w", err)
	}

	// Title
	q1 := squirrel.Update(repo.table).Set("value", settings.Title).Where(squirrel.Eq{"name": "title"})

	_, err = q1.RunWith(tx).ExecContext(ctx)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			return fmt.Errorf("error rolling back transaction: %w", err2)
		}

		return fmt.Errorf("error exec db query: %w", err)
	}

	// Tagline
	q2 := squirrel.Update(repo.table).Set("value", settings.Tagline).Where(squirrel.Eq{"name": "tagline"})

	_, err = q2.RunWith(tx).ExecContext(ctx)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			return fmt.Errorf("error rolling back transaction: %w", err2)
		}

		return fmt.Errorf("error exec db query: %w", err)
	}

	// Time Zone
	q3 := squirrel.Update(repo.table).Set("value", settings.TimeZone.String()).Where(squirrel.Eq{"name": "timeZone"})

	_, err = q3.RunWith(tx).ExecContext(ctx)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			return fmt.Errorf("error rolling back transaction: %w", err2)
		}

		return fmt.Errorf("error exec db query: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

var _ service.SettingRepository = (*SettingRepository)(nil)

func NewSettingRepository(db *sql.DB) *SettingRepository {
	return &SettingRepository{
		db:    db,
		table: "settings",
	}
}
