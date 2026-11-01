// Package provision manages the database roles handed out to visitors.
package provision

import (
	"context"
	"errors"
)

// ErrRoleExists means the requested role name is already taken in the cluster.
var ErrRoleExists = errors.New("role already exists")

// Provisioner changes roles and privileges in the course databases.
type Provisioner interface {
	// Bootstrap creates the group roles and (re)applies privileges in every database.
	// It is idempotent and picks up objects created since the previous run.
	Bootstrap(ctx context.Context) error
	// CreateRole creates a read-only login on every database.
	CreateRole(ctx context.Context, login, verifier string) error
	// DropRole removes a role that has just been created and owns nothing.
	DropRole(ctx context.Context, login string) error
	// RenameRole renames the role, replaces its password and ends its sessions.
	RenameRole(ctx context.Context, oldLogin, newLogin, verifier string) error
	// GrantWrite gives the role write access to one database.
	GrantWrite(ctx context.Context, login, database string) error
	// RevokeWrite takes write access to one database away and ends the role's sessions.
	// Objects the role created there stay, owned by a role without privileges.
	RevokeWrite(ctx context.Context, login, database string) error
}
