package provision

import (
	"context"
	"sync"
)

// Fake is an in-memory Provisioner for tests that do not need a database cluster.
type Fake struct {
	mu sync.Mutex
	// Roles maps a login to its password verifier.
	Roles map[string]string
	// Writes holds "login/database" pairs with write access.
	Writes map[string]bool
	// Bootstraps counts Bootstrap calls.
	Bootstraps int
}

// NewFake returns an empty Fake.
func NewFake() *Fake { return &Fake{Roles: map[string]string{}, Writes: map[string]bool{}} }

// Bootstrap implements Provisioner.
func (f *Fake) Bootstrap(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Bootstraps++
	return nil
}

// CreateRole implements Provisioner.
func (f *Fake) CreateRole(_ context.Context, login, verifier string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.Roles[login]; ok {
		return ErrRoleExists
	}
	f.Roles[login] = verifier
	return nil
}

// DropRole implements Provisioner.
func (f *Fake) DropRole(_ context.Context, login string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Roles, login)
	return nil
}

// RenameRole implements Provisioner.
func (f *Fake) RenameRole(_ context.Context, oldLogin, newLogin, verifier string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.Roles[newLogin]; ok {
		return ErrRoleExists
	}
	delete(f.Roles, oldLogin)
	f.Roles[newLogin] = verifier
	for key := range f.Writes {
		if len(key) > len(oldLogin) && key[:len(oldLogin)] == oldLogin {
			delete(f.Writes, key)
			f.Writes[newLogin+key[len(oldLogin):]] = true
		}
	}
	return nil
}

// GrantWrite implements Provisioner.
func (f *Fake) GrantWrite(_ context.Context, login, database string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Writes[login+"/"+database] = true
	return nil
}

// RevokeWrite implements Provisioner.
func (f *Fake) RevokeWrite(_ context.Context, login, database string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Writes, login+"/"+database)
	return nil
}
