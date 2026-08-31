package v1

import (
	"os"
	"testing"

	"github.com/lunogram/platform/internal/password"
)

// TestMain lowers the argon2id cost for every test in this package.
//
// The production parameters hold 64 MiB for the duration of each in-flight
// hash, and the race detector shadows roughly three times that again. These
// tests run in parallel, each driving registration and login handlers that hash
// on the request goroutine, and the aggregate is enough for the kernel to kill
// the test binary. Nothing here asserts on what a hash costs, only on the flows
// around it.
//
// Time stays above a single pass: the rehash tests derive an outdated hash by
// weakening these parameters, and one of them weakens Time on its own.
func TestMain(m *testing.M) {
	password.DefaultParams.Memory = 8 * 1024
	password.DefaultParams.Time = 2

	os.Exit(m.Run())
}
