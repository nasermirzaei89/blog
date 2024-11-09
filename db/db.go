package db

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

const defaultDSN = "file::memory:"

func NewDB(dsn string) (*sql.DB, error) {
	if dsn == "" {
		dsn = defaultDSN
	}

	sqliteDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return sqliteDB, nil
}
