package db

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(databaseURL string) (*sql.DB, error) {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	database.SetMaxOpenConns(12)
	database.SetMaxIdleConns(4)
	database.SetConnMaxLifetime(30 * time.Minute)

	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, err
	}

	return database, nil
}
