package management

import (
	"embed"

	"github.com/lunogram/platform/internal/store"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate ensures all migrations defined in the embedded file system are
// applied to the database defined in uri. An error is returned if any of the migrations fail.
func Migrate(uri string) error {
	return store.Migrate(uri, migrations)
}
