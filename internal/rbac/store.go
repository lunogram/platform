package rbac

import (
	"github.com/openfga/openfga/pkg/logger"
	"github.com/openfga/openfga/pkg/storage"
	"github.com/openfga/openfga/pkg/storage/migrate"
	"github.com/openfga/openfga/pkg/storage/postgres"
	"github.com/openfga/openfga/pkg/storage/sqlcommon"
)

// NewStore creates a new OpenFGA datastore using the provided PostgreSQL connection string.
// This allows the RBAC engine to store its relationship tuples and authorization models
// in the management database.
func NewStore(config Config) (storage.OpenFGADatastore, error) {
	cfg := sqlcommon.NewConfig(
		sqlcommon.WithLogger(logger.NewNoopLogger()),
	)
	return postgres.New(config.PostgresURI, cfg)
}

func Migrate(config Config) error {
	return migrate.RunMigrations(migrate.MigrationConfig{
		Engine: "postgres",
		URI:    config.PostgresURI,
	})
}
