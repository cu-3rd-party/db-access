package provision

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"git.cu3rd.ru/cu3rd/db-access/modules/identity"
	"git.cu3rd.ru/cu3rd/db-access/modules/setting"

	"github.com/lib/pq"
)

// archiveRole owns what a user created once their write access is over. It has no
// privileges of its own, so a SECURITY DEFINER function or a view left behind by
// a user cannot run with more rights than that user was entitled to. Reassigning
// such objects to the database owner would hand out its rights.
const archiveRole = "cu_archive"

const (
	connectionLimit          = 5
	statementTimeout         = "60s"
	idleInTransactionTimeout = "60s"
	duplicateObject          = "42710"
)

// Postgres is the Provisioner for a real cluster.
type Postgres struct {
	owner     string
	databases []string
	conns     map[string]*sql.DB
}

// NewPostgres opens a small pool per course database using the provisioner role from setting.
func NewPostgres() (*Postgres, error) {
	p := &Postgres{owner: setting.PGOwnerRole, databases: setting.Databases, conns: map[string]*sql.DB{}}
	for _, name := range p.databases {
		query := url.Values{"sslmode": {setting.PGSSLMode}, "search_path": {"pg_catalog"}}
		dsn := url.URL{
			Scheme:   "postgres",
			User:     url.UserPassword(setting.PGUser, setting.PGPassword),
			Host:     net.JoinHostPort(setting.PGHost, setting.PGPort),
			Path:     "/" + name,
			RawQuery: query.Encode(),
		}
		conn, err := sql.Open("postgres", dsn.String())
		if err != nil {
			return nil, err
		}
		conn.SetMaxOpenConns(2)
		p.conns[name] = conn
	}
	return p, nil
}

// Close closes the pools.
func (p *Postgres) Close() {
	for _, conn := range p.conns {
		_ = conn.Close()
	}
}

func groupRole(database, suffix string) string {
	return strings.ReplaceAll(database, "-", "_") + "_" + suffix
}

func readRole(database string) string  { return groupRole(database, "ro") }
func writeRole(database string) string { return groupRole(database, "rw") }

func quote(name string) string { return pq.QuoteIdentifier(name) }

// any database reaches cluster-wide objects such as roles
func (p *Postgres) anyConn() *sql.DB { return p.conns[p.databases[0]] }

func inTx(ctx context.Context, conn *sql.DB, statements ...string) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			// the statement may carry a password verifier: report only the server error
			return err
		}
	}
	return tx.Commit()
}

// selfGrant makes roles created in this transaction members of the provisioner: it needs
// their privileges to reassign their objects and to end their sessions.
const selfGrant = `SET LOCAL createrole_self_grant = 'set, inherit'`

// Bootstrap implements Provisioner.
func (p *Postgres) Bootstrap(ctx context.Context) error {
	groups := []string{archiveRole}
	for _, name := range p.databases {
		groups = append(groups, readRole(name), writeRole(name))
	}
	for _, group := range groups {
		var exists bool
		if err := p.anyConn().QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, group).Scan(&exists); err != nil {
			return fmt.Errorf("check role %s: %w", group, err)
		}
		if exists {
			continue
		}
		if err := inTx(ctx, p.anyConn(), selfGrant, `CREATE ROLE `+quote(group)+` NOLOGIN`); err != nil {
			return fmt.Errorf("create role %s: %w", group, err)
		}
	}
	for _, name := range p.databases {
		if err := p.applyPrivileges(ctx, name); err != nil {
			return fmt.Errorf("privileges on %s: %w", name, err)
		}
	}
	return nil
}

// listSchemas returns the quoted names of the non-system schemas.
func listSchemas(ctx context.Context, conn *sql.DB) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT nspname FROM pg_namespace WHERE nspname !~ '^pg_' AND nspname <> 'information_schema'`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var schemas []string
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, err
		}
		schemas = append(schemas, quote(schema))
	}
	return schemas, rows.Err()
}

func (p *Postgres) applyPrivileges(ctx context.Context, database string) error {
	conn := p.conns[database]
	ro, rw, owner, db := quote(readRole(database)), quote(writeRole(database)), quote(p.owner), quote(database)

	// other roles of the cluster have no business here; the hourly backup still does
	statements := []string{
		`REVOKE ALL ON DATABASE ` + db + ` FROM PUBLIC`,
		fmt.Sprintf(`GRANT CONNECT ON DATABASE %s TO %s, %s`, db, ro, rw),
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES FOR ROLE %s GRANT USAGE ON SCHEMAS TO %s, %s`, owner, ro, rw),
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES FOR ROLE %s GRANT SELECT ON TABLES TO %s`, owner, ro),
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES FOR ROLE %s GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s`, owner, rw),
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES FOR ROLE %s GRANT SELECT ON SEQUENCES TO %s`, owner, ro),
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES FOR ROLE %s GRANT ALL ON SEQUENCES TO %s`, owner, rw),
	}
	var hasBackup bool
	if err := conn.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'backup')`).Scan(&hasBackup); err != nil {
		return err
	}
	if hasBackup {
		statements = append(statements, `GRANT CONNECT ON DATABASE `+db+` TO backup`)
	}

	schemas, err := listSchemas(ctx, conn)
	if err != nil {
		return err
	}
	for _, s := range schemas {
		statements = append(statements,
			fmt.Sprintf(`GRANT USAGE ON SCHEMA %s TO %s, %s`, s, ro, rw),
			fmt.Sprintf(`GRANT CREATE ON SCHEMA %s TO %s`, s, rw),
			fmt.Sprintf(`GRANT SELECT ON ALL TABLES IN SCHEMA %s TO %s`, s, ro),
			fmt.Sprintf(`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %s TO %s`, s, rw),
			fmt.Sprintf(`GRANT SELECT ON ALL SEQUENCES IN SCHEMA %s TO %s`, s, ro),
			fmt.Sprintf(`GRANT ALL ON ALL SEQUENCES IN SCHEMA %s TO %s`, s, rw),
		)
	}
	return inTx(ctx, conn, statements...)
}

// CreateRole implements Provisioner.
func (p *Postgres) CreateRole(ctx context.Context, login, verifier string) error {
	if !identity.Valid(login) {
		return fmt.Errorf("invalid login %q", login)
	}
	role := quote(login)
	readGroups := make([]string, 0, len(p.databases))
	for _, name := range p.databases {
		readGroups = append(readGroups, quote(readRole(name)))
	}
	err := inTx(ctx, p.anyConn(),
		selfGrant,
		fmt.Sprintf(`CREATE ROLE %s LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS CONNECTION LIMIT %d PASSWORD %s`,
			role, connectionLimit, pq.QuoteLiteral(verifier)),
		fmt.Sprintf(`ALTER ROLE %s SET statement_timeout = %s`, role, pq.QuoteLiteral(statementTimeout)),
		fmt.Sprintf(`ALTER ROLE %s SET idle_in_transaction_session_timeout = %s`, role, pq.QuoteLiteral(idleInTransactionTimeout)),
		fmt.Sprintf(`GRANT %s TO %s`, strings.Join(readGroups, ", "), role),
	)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == duplicateObject {
		return ErrRoleExists
	}
	return err
}

// DropRole implements Provisioner.
func (p *Postgres) DropRole(ctx context.Context, login string) error {
	if !identity.Valid(login) {
		return fmt.Errorf("invalid login %q", login)
	}
	return inTx(ctx, p.anyConn(), `DROP ROLE IF EXISTS `+quote(login))
}

func (p *Postgres) terminateSessions(ctx context.Context, login string) error {
	rows, err := p.anyConn().QueryContext(ctx,
		`SELECT pg_terminate_backend(a.pid) FROM pg_stat_activity a JOIN pg_roles r ON r.oid = a.usesysid WHERE r.rolname = $1`, login)
	if err != nil {
		return err
	}
	return rows.Close()
}

// RenameRole implements Provisioner.
func (p *Postgres) RenameRole(ctx context.Context, oldLogin, newLogin, verifier string) error {
	if !identity.Valid(oldLogin) || !identity.Valid(newLogin) {
		return errors.New("invalid login")
	}
	err := inTx(ctx, p.anyConn(),
		fmt.Sprintf(`ALTER ROLE %s RENAME TO %s`, quote(oldLogin), quote(newLogin)),
		fmt.Sprintf(`ALTER ROLE %s PASSWORD %s`, quote(newLogin), pq.QuoteLiteral(verifier)),
	)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == duplicateObject {
		return ErrRoleExists
	}
	if err != nil {
		return err
	}
	return p.terminateSessions(ctx, newLogin)
}

func (p *Postgres) conn(database string) (*sql.DB, error) {
	conn, ok := p.conns[database]
	if !ok {
		return nil, fmt.Errorf("unknown database %q", database)
	}
	return conn, nil
}

// GrantWrite implements Provisioner.
func (p *Postgres) GrantWrite(ctx context.Context, login, database string) error {
	if !identity.Valid(login) {
		return fmt.Errorf("invalid login %q", login)
	}
	conn, err := p.conn(database)
	if err != nil {
		return err
	}
	return inTx(ctx, conn, fmt.Sprintf(`GRANT %s TO %s`, quote(writeRole(database)), quote(login)))
}

// RevokeWrite implements Provisioner.
func (p *Postgres) RevokeWrite(ctx context.Context, login, database string) error {
	if !identity.Valid(login) {
		return fmt.Errorf("invalid login %q", login)
	}
	conn, err := p.conn(database)
	if err != nil {
		return err
	}
	if err := inTx(ctx, conn, fmt.Sprintf(`REVOKE %s FROM %s`, quote(writeRole(database)), quote(login))); err != nil {
		return err
	}
	// a session may have run SET ROLE to the group: it keeps the rights until it ends
	if err := p.terminateSessions(ctx, login); err != nil {
		return err
	}
	// changing the owner of a function needs CREATE for the new owner. It lasts only for
	// this transaction: a leftover SECURITY DEFINER function must not be able to create objects.
	schemas, err := listSchemas(ctx, conn)
	if err != nil {
		return err
	}
	archive := quote(archiveRole)
	statements := make([]string, 0, 2*len(schemas)+1)
	for _, s := range schemas {
		statements = append(statements, fmt.Sprintf(`GRANT CREATE ON SCHEMA %s TO %s`, s, archive))
	}
	statements = append(statements, fmt.Sprintf(`REASSIGN OWNED BY %s TO %s`, quote(login), archive))
	for _, s := range schemas {
		statements = append(statements, fmt.Sprintf(`REVOKE CREATE ON SCHEMA %s FROM %s`, s, archive))
	}
	if err := inTx(ctx, conn, statements...); err != nil {
		return err
	}
	// rows in tables the user owned become readable like everything else
	return p.applyPrivileges(ctx, database)
}
