// Package unittest is the test harness for packages touching the database:
// every test package calls MainTest
// from its TestMain, every test calls PrepareTestDatabase to reload the YAML
// fixtures from models/fixtures.
package unittest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"git.cu3rd.ru/cu3rd/db-access/models/db"
	"git.cu3rd.ru/cu3rd/db-access/models/migrations"
	"git.cu3rd.ru/cu3rd/db-access/modules/setting"

	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

var fixtures *testfixtures.Loader

// ProjectRoot returns the repository root.
func ProjectRoot() string {
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		return root
	}
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// MainTest sets up a migrated database with fixtures, runs setup and then
// the tests. By default it uses a fresh sqlite file; set DBACCESS_DB_TYPE=postgres
// and DBACCESS_DB_DSN to run against PostgreSQL (its public schema is wiped).
func MainTest(m *testing.M, setup ...func()) {
	os.Exit(mainTest(m, setup...))
}

func mainTest(m *testing.M, setup ...func()) int {
	setting.Load()
	setting.StaticRoot = ProjectRoot()
	setting.CookieSecret = "test-only-cookie-secret-0123456789abcdef"
	setting.CookieSecure = false
	if setting.DBType == "sqlite" {
		dir, err := os.MkdirTemp("", "dbaccess-test-")
		if err != nil {
			return fatal("tempdir: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()
		setting.DBDSN = filepath.Join(dir, "test.db")
	}
	if err := InitFixtures(context.Background()); err != nil {
		return fatal("init fixtures: %v", err)
	}
	for _, f := range setup {
		f()
	}
	return m.Run()
}

func fatal(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	return 1
}

// InitFixtures migrates the database and prepares the fixture loader.
func InitFixtures(ctx context.Context) error {
	engine, err := db.NewEngine()
	if err != nil {
		return err
	}
	if setting.DBType == "postgres" {
		if _, err := engine.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
			return err
		}
	}
	if err := migrations.Migrate(ctx, engine); err != nil {
		return err
	}
	db.SetEngine(engine)

	dialect := setting.DBType
	if dialect == "sqlite" {
		dialect = "sqlite3"
	}
	opts := []func(*testfixtures.Loader) error{
		testfixtures.Database(engine.DB().DB),
		testfixtures.Dialect(dialect),
		testfixtures.Directory(filepath.Join(ProjectRoot(), "models", "fixtures")),
		testfixtures.DangerousSkipTestDatabaseCheck(),
	}
	if setting.DBType == "postgres" {
		// new rows get ids above the fixtures
		opts = append(opts, testfixtures.ResetSequencesTo(1000))
	}
	fixtures, err = testfixtures.New(opts...)
	return err
}

// PrepareTestDatabase reloads all fixtures.
func PrepareTestDatabase(t testing.TB) {
	t.Helper()
	require.NoError(t, LoadFixtures())
}

// LoadFixtures reloads all fixtures (for harnesses without a testing.TB).
func LoadFixtures() error {
	return fixtures.Load()
}

// AssertExistsAndLoadBean asserts exactly one row matches the conditions and returns it.
func AssertExistsAndLoadBean[T any](t testing.TB, bean T, conds ...any) T {
	t.Helper()
	sess := db.GetEngine(context.Background()).Where(builder.Expr("1=1"))
	for _, c := range conds {
		sess = sess.And(c)
	}
	has, err := sess.Get(bean)
	require.NoError(t, err)
	require.True(t, has, "expected bean %#v to exist", bean)
	return bean
}

// AssertNotExistsBean asserts no row matches bean's non-zero fields
// (note: xorm ignores bool fields in bean conditions).
func AssertNotExistsBean(t testing.TB, bean any) {
	t.Helper()
	has, err := db.GetEngine(context.Background()).Get(bean)
	require.NoError(t, err)
	assert.False(t, has, "expected bean %#v not to exist", bean)
}

// AssertCount asserts the number of rows matching bean's non-zero fields.
func AssertCount(t testing.TB, bean any, expected int64) {
	t.Helper()
	n, err := db.GetEngine(context.Background()).Count(bean)
	require.NoError(t, err)
	assert.Equal(t, expected, n)
}
