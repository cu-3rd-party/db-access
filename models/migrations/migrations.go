// Package migrations holds versioned schema migrations.
// Migrations must not import application models
// (enforced by depguard): each one declares the struct as it was when written.
package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"xorm.io/xorm"
)

type migration struct {
	description string
	migrate     func(*xorm.Engine) error
}

// Append-only: never edit or reorder existing entries.
var migrations = []migration{
	{"create account, write grant and access log tables", v1CreateAccessTables},
}

// Version stores the current schema version.
type Version struct {
	ID      int64 `xorm:"pk autoincr"`
	Version int64
}

// ExpectedVersion is the schema version of this binary.
func ExpectedVersion() int64 { return int64(len(migrations)) }

// Migrate applies all pending migrations.
func Migrate(ctx context.Context, x *xorm.Engine) error {
	if err := x.Sync(new(Version)); err != nil {
		return err
	}
	current := &Version{}
	has, err := x.Context(ctx).ID(1).Get(current)
	if err != nil {
		return err
	}
	if !has {
		current = &Version{ID: 1}
		if _, err := x.Context(ctx).Insert(current); err != nil {
			return err
		}
	}
	if current.Version > ExpectedVersion() {
		return fmt.Errorf("database version %d is newer than this binary (%d)", current.Version, ExpectedVersion())
	}
	for i, m := range migrations[current.Version:] {
		v := current.Version + int64(i) + 1
		slog.Info("migration", "version", v, "description", m.description)
		if err := m.migrate(x); err != nil {
			return fmt.Errorf("migration %d (%s): %w", v, m.description, err)
		}
		if _, err := x.Context(ctx).ID(1).Cols("version").Update(&Version{Version: v}); err != nil {
			return err
		}
	}
	return nil
}
