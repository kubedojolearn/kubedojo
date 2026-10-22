// Package testenv gives integration tests their connection settings. Integration tests are
// built only with `-tags=integration` and fail (never skip) when their dependencies are missing,
// so a green run always means they actually ran.
package testenv

import (
	"os"
	"testing"
)

// Require returns the value of an environment variable or fails the test.
func Require(t testing.TB, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Fatalf("%s is not set. Run integration tests with `make test-integration`, which starts Postgres and Redis.", key)
	}
	return v
}
