package provision

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"git.cu3rd.ru/cu3rd/db-access/modules/secret"
	"git.cu3rd.ru/cu3rd/db-access/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Runs only against a disposable cluster, because it creates databases and roles:
//
//	TEST_PROVISION_SUPERUSER_DSN=postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable
func superuserDSN(t *testing.T) *url.URL {
	t.Helper()
	dsn := os.Getenv("TEST_PROVISION_SUPERUSER_DSN")
	if dsn == "" {
		t.Skip("TEST_PROVISION_SUPERUSER_DSN is not set")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	return u
}

func open(t *testing.T, base *url.URL, user, password, database string) *sql.DB {
	t.Helper()
	u := *base
	u.User = url.UserPassword(user, password)
	u.Path = "/" + database
	conn, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	conn.SetMaxOpenConns(1)
	return conn
}

func mustExec(t *testing.T, conn *sql.DB, statements ...string) {
	t.Helper()
	for _, stmt := range statements {
		_, err := conn.ExecContext(t.Context(), stmt)
		require.NoError(t, err, stmt)
	}
}

func setupCluster(t *testing.T) (*Postgres, *url.URL) {
	t.Helper()
	base := superuserDSN(t)
	superPassword, _ := base.User.Password()
	super := open(t, base, base.User.Username(), superPassword, strings.TrimPrefix(base.Path, "/"))

	// the same layout as pg-grm: an owner, and a provisioner that is a member of it
	var exists bool
	require.NoError(t, super.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'cu-sql')`).Scan(&exists))
	if exists {
		t.Fatal("the cluster is not empty: use a disposable one")
	}
	mustExec(t, super,
		`CREATE ROLE cu_owner LOGIN PASSWORD 'owner'`,
		`CREATE ROLE cu_provisioner LOGIN CREATEROLE PASSWORD 'prov' IN ROLE cu_owner`,
		`CREATE ROLE backup LOGIN PASSWORD 'backup'`,
		`CREATE ROLE stranger LOGIN PASSWORD 'stranger'`,
		`CREATE DATABASE "cu-sql" OWNER cu_owner`,
		`CREATE DATABASE "cu-db" OWNER cu_owner`,
	)
	for _, name := range []string{"cu-sql", "cu-db"} {
		owner := open(t, base, "cu_owner", "owner", name)
		mustExec(t, owner,
			`CREATE SCHEMA course`,
			`CREATE TABLE course.t (id serial PRIMARY KEY, v text)`,
			`INSERT INTO course.t (v) VALUES ('seed')`,
		)
	}

	setting.PGHost, setting.PGPort = base.Hostname(), base.Port()
	setting.PGSSLMode, setting.PGUser, setting.PGPassword = "disable", "cu_provisioner", "prov"
	setting.PGOwnerRole, setting.Databases = "cu_owner", []string{"cu-sql", "cu-db"}
	p, err := NewPostgres()
	require.NoError(t, err)
	t.Cleanup(p.Close)
	require.NoError(t, p.Bootstrap(t.Context()))
	return p, base
}

func newRole(t *testing.T, p *Postgres, login string) string {
	t.Helper()
	password := secret.NewPassword()
	verifier, err := secret.Verifier(password)
	require.NoError(t, err)
	require.NoError(t, p.CreateRole(t.Context(), login, verifier))
	return password
}

func canWrite(ctx context.Context, conn *sql.DB) bool {
	_, err := conn.ExecContext(ctx, `INSERT INTO course.t (v) VALUES ('w')`)
	return err == nil
}

func TestPostgresProvisioner(t *testing.T) {
	p, base := setupCluster(t)
	ctx := t.Context()
	password := newRole(t, p, "aB3dE9xZ")
	sqlDB := open(t, base, "aB3dE9xZ", password, "cu-sql")
	coreDB := open(t, base, "aB3dE9xZ", password, "cu-db")

	t.Run("new role reads both databases and writes neither", func(t *testing.T) {
		for _, conn := range []*sql.DB{sqlDB, coreDB} {
			var n int
			require.NoError(t, conn.QueryRow(`SELECT count(*) FROM course.t`).Scan(&n))
			assert.Equal(t, 1, n)
			assert.False(t, canWrite(ctx, conn))
			_, err := conn.Exec(`CREATE TABLE course.mine (id int)`)
			require.Error(t, err)
			_, err = conn.Exec(`CREATE TEMP TABLE tmp (id int)`)
			require.Error(t, err)
		}
	})

	t.Run("generated scram verifier is what the server stores", func(t *testing.T) {
		var timeout string
		require.NoError(t, sqlDB.QueryRow(`SHOW statement_timeout`).Scan(&timeout))
		assert.Equal(t, "1min", timeout)
		require.Error(t, open(t, base, "aB3dE9xZ", "wrong", "cu-sql").Ping())
	})

	t.Run("role of another application cannot connect", func(t *testing.T) {
		require.Error(t, open(t, base, "stranger", "stranger", "cu-sql").Ping())
		require.NoError(t, open(t, base, "backup", "backup", "cu-sql").Ping())
	})

	t.Run("name collisions are reported", func(t *testing.T) {
		verifier, err := secret.Verifier("x")
		require.NoError(t, err)
		require.ErrorIs(t, p.CreateRole(ctx, "aB3dE9xZ", verifier), ErrRoleExists)
		require.Error(t, p.CreateRole(ctx, `bad"name`, verifier))
	})

	t.Run("write applies to one database only", func(t *testing.T) {
		require.NoError(t, p.GrantWrite(ctx, "aB3dE9xZ", "cu-sql"))
		assert.True(t, canWrite(ctx, sqlDB))
		assert.False(t, canWrite(ctx, coreDB))
		_, err := sqlDB.Exec(`CREATE TABLE course.mine (id int)`)
		require.NoError(t, err)
		_, err = sqlDB.Exec(`DROP TABLE course.t`)
		require.Error(t, err, "others' tables can be changed, but not dropped")
	})

	t.Run("expiry leaves no way to keep writing", func(t *testing.T) {
		// everything a user could leave behind to outlive their write access
		mustExec(t, sqlDB,
			`CREATE FUNCTION course.backdoor() RETURNS void LANGUAGE sql SECURITY DEFINER AS $$ INSERT INTO course.t (v) VALUES ('definer') $$`,
			`CREATE VIEW course.backdoor_view AS SELECT * FROM course.t`,
			`GRANT ALL ON course.backdoor_view TO PUBLIC`,
			`SET ROLE cu_sql_rw`,
		)
		require.NoError(t, p.RevokeWrite(ctx, "aB3dE9xZ", "cu-sql"))

		var conn *sql.DB
		require.Eventually(t, func() bool {
			conn = open(t, base, "aB3dE9xZ", password, "cu-sql")
			return conn.Ping() == nil
		}, 5*time.Second, 100*time.Millisecond)
		assert.False(t, canWrite(ctx, conn), "SET ROLE session must be gone")
		_, err := conn.Exec(`SELECT course.backdoor()`)
		require.Error(t, err)
		_, err = conn.Exec(`INSERT INTO course.backdoor_view (v) VALUES ('via view')`)
		require.Error(t, err)
		_, err = conn.Exec(`INSERT INTO course.mine VALUES (1)`)
		require.Error(t, err)
		_, err = conn.Exec(`CREATE TABLE course.after (id int)`)
		require.Error(t, err)

		var owner string
		require.NoError(t, conn.QueryRow(`SELECT tableowner FROM pg_tables WHERE tablename = 'mine'`).Scan(&owner))
		assert.Equal(t, archiveRole, owner)
		var archiveCanCreate bool
		require.NoError(t, conn.QueryRow(`SELECT has_schema_privilege('cu_archive', 'course', 'CREATE')`).Scan(&archiveCanCreate))
		assert.False(t, archiveCanCreate)
		var n int
		require.NoError(t, conn.QueryRow(`SELECT count(*) FROM course.mine`).Scan(&n), "stays readable")
	})

	t.Run("privileges reach tables created later", func(t *testing.T) {
		owner := open(t, base, "cu_owner", "owner", "cu-sql")
		mustExec(t, owner, `CREATE SCHEMA later`, `CREATE TABLE later.t (id int)`)
		require.NoError(t, p.Bootstrap(ctx))
		conn := open(t, base, "aB3dE9xZ", password, "cu-sql")
		var n int
		require.NoError(t, conn.QueryRow(`SELECT count(*) FROM later.t`).Scan(&n))
	})

	t.Run("rotation keeps grants and kills the old credentials", func(t *testing.T) {
		require.NoError(t, p.GrantWrite(ctx, "aB3dE9xZ", "cu-db"))
		newPassword := secret.NewPassword()
		verifier, err := secret.Verifier(newPassword)
		require.NoError(t, err)
		require.NoError(t, p.RenameRole(ctx, "aB3dE9xZ", "Zx9Ed3Ba", verifier))

		require.Error(t, open(t, base, "aB3dE9xZ", password, "cu-db").Ping())
		require.Error(t, open(t, base, "Zx9Ed3Ba", password, "cu-db").Ping())
		conn := open(t, base, "Zx9Ed3Ba", newPassword, "cu-db")
		require.NoError(t, conn.Ping())
		assert.True(t, canWrite(ctx, conn))
	})

	t.Run("the new role cannot reach the owner or create roles", func(t *testing.T) {
		password := newRole(t, p, "qQ1wW2eE")
		conn := open(t, base, "qQ1wW2eE", password, "cu-db")
		for _, stmt := range []string{`SET ROLE cu_owner`, `SET ROLE cu_provisioner`, `SET ROLE cu_db_rw`, `CREATE ROLE x`, `CREATE DATABASE x`} {
			_, err := conn.Exec(stmt)
			require.Error(t, err, stmt)
		}
	})
}
