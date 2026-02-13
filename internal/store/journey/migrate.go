package journey

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
func Migrate(uri string) error {
	fsDriver, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("failed to load embedded migration: %w", err)
	}

	conn, err := sql.Open("pgx", uri)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer conn.Close()

	// Use a separate migrations table to avoid conflicts when running multiple
	// migrations on the same database (e.g., in tests)
	db, err := pgx.WithInstance(conn, &pgx.Config{
		MigrationsTable: "schema_migrations_journey",
	})
	if err != nil {
		return fmt.Errorf("failed to create migration database instance: %w", err)
	}
	defer db.Close() //nolint:errcheck

	migrator, err := migrate.NewWithInstance("iofs", fsDriver, "pgx", db)
	if err != nil {
		return fmt.Errorf("failed to construct migrator: %w", err)
	}

	err = migrator.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migration: %w", err)
	}

	return nil
}
