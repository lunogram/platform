package management

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate ensures all migrations defined in the embedded file system are
// applied to the database defined in uri. An error is returned if any of the migrations fail.
func Migrate(config Config) error {
	fs, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("failed to load embedded migration: %w", err)
	}

	conn, err := sql.Open("pgx", config.URI)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer conn.Close()

	db, err := pgx.WithInstance(conn, &pgx.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration database instance: %w", err)
	}
	defer db.Close() //nolint:errcheck

	migrator, err := migrate.NewWithInstance("iofs", fs, "pgx", db)
	if err != nil {
		return fmt.Errorf("failed to construct migrator: %w", err)
	}

	err = migrator.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migration: %w", err)
	}

	return nil
}
