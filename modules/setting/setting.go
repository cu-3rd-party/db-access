// Package setting holds the global configuration, read from the environment.
package setting

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	// HTTPAddr is the listen address.
	HTTPAddr = ":3000"
	// DBType is "sqlite" or "postgres".
	DBType = "sqlite"
	// DBDSN is a file path for sqlite or a connection string for postgres.
	DBDSN = "data/dbaccess.db"
	// StaticRoot is where templates/ and public/ are read from in non-bindata builds.
	StaticRoot = "."

	// PGHost is the cluster holding the course databases.
	PGHost = "pg-grm-rw.postgres.svc.cluster.local"
	// PGPort is its port.
	PGPort = "5432"
	// PGSSLMode is the sslmode of provisioner connections.
	PGSSLMode = "require"
	// PGUser is the provisioner role: it needs CREATEROLE and membership in PGOwnerRole.
	PGUser = "cu_provisioner"
	// PGPassword is the password of PGUser.
	PGPassword = ""
	// PGOwnerRole owns the course databases.
	PGOwnerRole = "cu_owner"
	// Databases are the course databases handed out to visitors.
	Databases = []string{"cu-sql", "cu-db"}

	// PublicDBHost is shown to visitors in connection strings.
	PublicDBHost = "db.cu3rd.ru"
	// PublicDBPort is shown to visitors in connection strings.
	PublicDBPort = "5432"

	// CookieSecret signs the identity cookie: logins are visible in pg_roles to every
	// database user, so the login alone must not be enough to act as its owner.
	CookieSecret = ""
	// CookieSecure sets the Secure attribute; only local plain-HTTP runs turn it off.
	CookieSecure = true

	// MaxAccounts caps how many database roles visitors can create in total.
	MaxAccounts int64 = 5000
	// CredentialsPerIPHour limits role creations and rotations per client IP.
	// A whole classroom can share one address behind NAT.
	CredentialsPerIPHour = 300
	// WriteTTL is how long a write grant lives.
	WriteTTL = 24 * time.Hour
	// AccessLogRetention is how long access log rows are kept.
	AccessLogRetention = 90 * 24 * time.Hour
)

// Load reads DBACCESS_* environment variables.
func Load() {
	HTTPAddr = env("DBACCESS_HTTP_ADDR", HTTPAddr)
	DBType = env("DBACCESS_DB_TYPE", DBType)
	DBDSN = env("DBACCESS_DB_DSN", DBDSN)
	StaticRoot = env("DBACCESS_STATIC_ROOT", StaticRoot)
	PGHost = env("DBACCESS_PG_HOST", PGHost)
	PGPort = env("DBACCESS_PG_PORT", PGPort)
	PGSSLMode = env("DBACCESS_PG_SSLMODE", PGSSLMode)
	PGUser = env("DBACCESS_PG_USER", PGUser)
	PGPassword = env("DBACCESS_PG_PASSWORD", PGPassword)
	PGOwnerRole = env("DBACCESS_PG_OWNER_ROLE", PGOwnerRole)
	if v := env("DBACCESS_DATABASES", ""); v != "" {
		Databases = strings.Split(v, ",")
	}
	PublicDBHost = env("DBACCESS_PUBLIC_DB_HOST", PublicDBHost)
	PublicDBPort = env("DBACCESS_PUBLIC_DB_PORT", PublicDBPort)
	CookieSecret = env("DBACCESS_COOKIE_SECRET", CookieSecret)
	CookieSecure = env("DBACCESS_COOKIE_SECURE", "true") != "false"
	if n, err := strconv.ParseInt(env("DBACCESS_MAX_ACCOUNTS", ""), 10, 64); err == nil {
		MaxAccounts = n
	}
	if n, err := strconv.Atoi(env("DBACCESS_CREDENTIALS_PER_IP_HOUR", "")); err == nil {
		CredentialsPerIPHour = n
	}
}

// Validate reports settings the web server cannot run without.
func Validate() error {
	if PGPassword == "" {
		return errors.New("DBACCESS_PG_PASSWORD is required")
	}
	if len(CookieSecret) < 32 {
		return errors.New("DBACCESS_COOKIE_SECRET must be at least 32 characters")
	}
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
