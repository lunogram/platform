package verifiers

import (
	"os"
	"testing"

	"github.com/lunogram/platform/internal/password"
)

// TestMain lowers the argon2id cost for every test in this package, for the
// reasons set out on the same hook in the management controller tests.
func TestMain(m *testing.M) {
	password.DefaultParams.Memory = 8 * 1024
	password.DefaultParams.Time = 2

	os.Exit(m.Run())
}
