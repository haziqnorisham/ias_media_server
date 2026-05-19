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

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	stmt := `
	CREATE TABLE IF NOT EXISTS devices (
		id                   INTEGER PRIMARY KEY AUTOINCREMENT,
		name                 TEXT NOT NULL,
		ip                   TEXT NOT NULL,
		port                 INTEGER NOT NULL DEFAULT 80,
		username             TEXT NOT NULL,
		password             TEXT NOT NULL,
		stream_profile_token TEXT NOT NULL DEFAULT '',
		notes                TEXT NOT NULL DEFAULT '',
		created_at           TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at           TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_devices_ip ON devices(ip);
	`
	_, err := db.Exec(stmt)
	return err
}
