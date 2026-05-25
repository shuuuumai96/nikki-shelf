package db

import (
	"database/sql"
	"embed"
	"strings"
)

//go:embed schema.sql
var schemaFS embed.FS

func Migrate(database *sql.DB) error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}

	for _, statement := range strings.Split(string(schema), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}

		if _, err := database.Exec(statement); err != nil {
			return err
		}
	}

	return nil
}
