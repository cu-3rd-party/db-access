// Package db wraps the xorm engine and carries it through context.Context.
package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"git.cu3rd.ru/cu3rd/db-access/models/migrations"
	"git.cu3rd.ru/cu3rd/db-access/modules/setting"

	_ "github.com/lib/pq"  // postgres driver
	_ "modernc.org/sqlite" // pure-Go sqlite driver
	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

var x *xorm.Engine

var tables []any

// RegisterModel registers a bean so tests can sync and truncate it.
func RegisterModel(bean any) {
	tables = append(tables, bean)
}

// Tables returns all registered models.
func Tables() []any { return tables }

// NewEngine opens the database configured in setting without migrating it.
func NewEngine() (*xorm.Engine, error) {
	driver, dsn := setting.DBType, setting.DBDSN
	switch driver {
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
			return nil, err
		}
		dsn = "file:" + dsn + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	case "postgres":
	default:
		return nil, fmt.Errorf("unsupported database type %q", driver)
	}
	engine, err := xorm.NewEngine(driver, dsn)
	if err != nil {
		return nil, err
	}
	if driver == "sqlite" {
		engine.SetMaxOpenConns(1)
	}
	engine.SetMapper(names.GonicMapper{})
	return engine, nil
}

// InitEngine opens the database, runs migrations and installs the global engine.
func InitEngine(ctx context.Context) error {
	engine, err := NewEngine()
	if err != nil {
		return err
	}
	if err := migrations.Migrate(ctx, engine); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	SetEngine(engine)
	return nil
}

// SetEngine replaces the global engine.
func SetEngine(engine *xorm.Engine) { x = engine }

type txKey struct{}

// GetEngine returns a session bound to ctx, or the transaction stored in ctx.
func GetEngine(ctx context.Context) xorm.Interface {
	if sess, ok := ctx.Value(txKey{}).(*xorm.Session); ok {
		return sess
	}
	return x.Context(ctx)
}

// WithTx runs fn inside a transaction; nested calls reuse the outer one.
func WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*xorm.Session); ok {
		return fn(ctx)
	}
	sess := x.NewSession().Context(ctx)
	defer func() { _ = sess.Close() }()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := fn(context.WithValue(ctx, txKey{}, sess)); err != nil {
		return err
	}
	return sess.Commit()
}
