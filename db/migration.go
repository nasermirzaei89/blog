package db

import (
	"database/sql"
	"embed"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/pkg/errors"
	"log"
)

//go:embed migrations/*.sql
var fs embed.FS

func Migrate(sqlDB *sql.DB) error {
	sourceDriver, err := iofs.New(fs, "migrations") // Get migrations from sql folder
	if err != nil {
		log.Fatal(err)
	}

	dbDriver, err := sqlite3.WithInstance(sqlDB, &sqlite3.Config{})
	if err != nil {
		return errors.Wrap(err, "could not create database driver")
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		"sqlite3",
		dbDriver,
	)
	if err != nil {
		return errors.Wrap(err, "could not create migration instance")
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return errors.Wrap(err, "could not run migrations")
	}

	log.Println("Migration applied successfully.")

	return nil
}
