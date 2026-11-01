// Package access implements what a visitor can do: get credentials, rotate them and
// hold temporary write access to a database.
package access

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	account_model "git.cu3rd.ru/cu3rd/db-access/models/account"
	"git.cu3rd.ru/cu3rd/db-access/models/db"
	"git.cu3rd.ru/cu3rd/db-access/modules/identity"
	"git.cu3rd.ru/cu3rd/db-access/modules/secret"
	"git.cu3rd.ru/cu3rd/db-access/modules/setting"
	"git.cu3rd.ru/cu3rd/db-access/services/provision"
)

var (
	// ErrRateLimited means this address asked for too many credentials recently.
	ErrRateLimited = errors.New("слишком много запросов с вашего адреса, попробуйте позже")
	// ErrCapacity means the cap on database roles has been reached.
	ErrCapacity = errors.New("достигнут лимит пользователей, обратитесь к администратору")
	// ErrUnknownDatabase means the form named a database this service does not manage.
	ErrUnknownDatabase = fmt.Errorf("неизвестная база данных: %w", db.ErrInvalidArgument)
)

var (
	provisioner provision.Provisioner
	// a single replica runs; this keeps a role and its account row changing together
	mu      sync.Mutex
	limiter = newIPLimiter()
)

// SetProvisioner installs the Provisioner.
func SetProvisioner(p provision.Provisioner) { provisioner = p }

// ResetLimits forgets the per-address counters.
func ResetLimits() { limiter = newIPLimiter() }

// Credentials is the result of creating or rotating: the only moment a password exists.
type Credentials struct {
	Account  *account_model.Account
	Password string
}

// Ensure returns the account of login, creating the role on first use. Password is
// set only when the role has just been created. provision.ErrRoleExists means the
// caller has to retry with another login.
func Ensure(ctx context.Context, login, ip string) (*Credentials, error) {
	mu.Lock()
	defer mu.Unlock()

	acc, err := account_model.GetByLogin(ctx, login)
	if err == nil {
		return &Credentials{Account: acc}, nil
	}
	if !errors.Is(err, db.ErrNotExist) {
		return nil, err
	}
	count, err := account_model.Count(ctx)
	if err != nil {
		return nil, err
	}
	if count >= setting.MaxAccounts {
		return nil, ErrCapacity
	}
	if !limiter.allow(ip) {
		return nil, ErrRateLimited
	}

	password := secret.NewPassword()
	verifier, err := secret.Verifier(password)
	if err != nil {
		return nil, err
	}
	if err := provisioner.CreateRole(ctx, login, verifier); err != nil {
		return nil, err
	}
	acc, err = account_model.Create(ctx, login)
	if err != nil {
		if dropErr := provisioner.DropRole(ctx, login); dropErr != nil {
			slog.Error("drop role after failed account insert", "login", login, "err", dropErr)
		}
		return nil, err
	}
	return &Credentials{Account: acc, Password: password}, nil
}

// Rotate replaces both the login and the password. Write grants move with the role.
func Rotate(ctx context.Context, acc *account_model.Account, newLogin, ip string) (*Credentials, error) {
	mu.Lock()
	defer mu.Unlock()
	if !limiter.allow(ip) {
		return nil, ErrRateLimited
	}
	password := secret.NewPassword()
	verifier, err := secret.Verifier(password)
	if err != nil {
		return nil, err
	}
	oldLogin := acc.Login
	if err := provisioner.RenameRole(ctx, oldLogin, newLogin, verifier); err != nil {
		return nil, err
	}
	if err := account_model.SetLogin(ctx, acc, newLogin); err != nil {
		// the password is gone either way, but the account must keep matching its role
		acc.Login = oldLogin
		if backErr := provisioner.RenameRole(ctx, newLogin, oldLogin, verifier); backErr != nil {
			slog.Error("rename role back after failed account update", "login", oldLogin, "err", backErr)
		}
		return nil, err
	}
	return &Credentials{Account: acc, Password: password}, nil
}

// SetWrite turns write access to one database on for setting.WriteTTL, or off at once.
func SetWrite(ctx context.Context, acc *account_model.Account, database string, enabled bool) (*account_model.WriteGrant, error) {
	if !slices.Contains(setting.Databases, database) {
		return nil, ErrUnknownDatabase
	}
	mu.Lock()
	defer mu.Unlock()

	if !enabled {
		if err := provisioner.RevokeWrite(ctx, acc.Login, database); err != nil {
			return nil, err
		}
		return nil, account_model.DeleteGrant(ctx, acc.ID, database)
	}
	if err := provisioner.GrantWrite(ctx, acc.Login, database); err != nil {
		return nil, err
	}
	grant, err := account_model.SetGrant(ctx, acc.ID, database, time.Now().Add(setting.WriteTTL))
	if err != nil {
		// without a row nothing would ever take this access away
		if revokeErr := provisioner.RevokeWrite(ctx, acc.Login, database); revokeErr != nil {
			slog.Error("revoke write after failed grant insert", "login", acc.Login, "database", database, "err", revokeErr)
		}
		return nil, err
	}
	return grant, nil
}

// ExpireWrites takes away write access that is due. A grant stays until the revoke
// succeeds, so a failure is retried on the next run.
func ExpireWrites(ctx context.Context, now time.Time) error {
	mu.Lock()
	defer mu.Unlock()
	due, err := account_model.ExpiredGrants(ctx, now)
	if err != nil {
		return err
	}
	var errs []error
	for _, g := range due {
		if err := provisioner.RevokeWrite(ctx, g.Login, g.DBName); err != nil {
			errs = append(errs, fmt.Errorf("revoke %s on %s: %w", g.Login, g.DBName, err))
			continue
		}
		if err := account_model.DeleteGrant(ctx, g.AccountID, g.DBName); err != nil {
			errs = append(errs, err)
			continue
		}
		slog.Info("write access expired", "login", g.Login, "database", g.DBName)
	}
	return errors.Join(errs...)
}

// Bootstrap applies group roles and privileges.
func Bootstrap(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()
	return provisioner.Bootstrap(ctx)
}

// NewLogin returns an identifier not used by any account.
func NewLogin(ctx context.Context) (string, error) {
	for {
		login := identity.New()
		_, err := account_model.GetByLogin(ctx, login)
		if errors.Is(err, db.ErrNotExist) {
			return login, nil
		}
		if err != nil {
			return "", err
		}
	}
}

// ipLimiter counts events per address in a fixed window of one hour.
type ipLimiter struct {
	mu     sync.Mutex
	window time.Time
	counts map[string]int
}

func newIPLimiter() *ipLimiter { return &ipLimiter{counts: map[string]int{}} }

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now := time.Now(); now.Sub(l.window) >= time.Hour {
		l.window, l.counts = now, map[string]int{}
	}
	if l.counts[ip] >= setting.CredentialsPerIPHour {
		return false
	}
	l.counts[ip]++
	return true
}
