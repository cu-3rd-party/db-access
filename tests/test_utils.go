// Package tests holds helpers shared by tests/integration and tests/e2e.
package tests

import (
	"testing"

	"git.cu3rd.ru/cu3rd/db-access/models/unittest"
)

// PrepareTestEnv reloads the fixtures; call it at the top of every test.
func PrepareTestEnv(t testing.TB) {
	t.Helper()
	unittest.PrepareTestDatabase(t)
}
