package consumer

import (
	"testing"

	"github.com/jmoiron/sqlx"
	teststore "github.com/lunogram/platform/internal/store/test"
)

func runPostgreSQL(t *testing.T) (mgmt, usrs, jrny *sqlx.DB) {
	t.Helper()
	return teststore.RunPostgreSQL(t)
}
