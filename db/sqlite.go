package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const driverName = "sqlite"

func Init(path string) (*sql.DB, error) {
	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}

	return db, nil
}
