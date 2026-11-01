// Package account stores who received database credentials and their write grants.
// Passwords are never stored: they exist only in the response that shows them.
package account

import (
	"context"
	"fmt"
	"time"

	"git.cu3rd.ru/cu3rd/db-access/models/db"
)

func init() {
	db.RegisterModel(new(Account))
	db.RegisterModel(new(WriteGrant))
}

// Account is a visitor; Login is the current cookie value and database role name.
type Account struct {
	ID          int64  `xorm:"pk autoincr"`
	Login       string `xorm:"VARCHAR(8) NOT NULL UNIQUE"`
	CreatedUnix int64  `xorm:"created"`
	RotatedUnix int64
}

// WriteGrant is a temporary write permission on one database.
type WriteGrant struct {
	ID          int64  `xorm:"pk autoincr"`
	AccountID   int64  `xorm:"NOT NULL UNIQUE(account_db)"`
	DBName      string `xorm:"VARCHAR(63) NOT NULL UNIQUE(account_db)"`
	ExpiresUnix int64  `xorm:"NOT NULL INDEX"`
}

// ExpiredGrant is a due WriteGrant with the login it belongs to.
type ExpiredGrant struct {
	AccountID int64
	Login     string
	DBName    string
}

// ErrNotExist means the login has no account yet.
type ErrNotExist struct{ Login string }

func (e ErrNotExist) Error() string { return fmt.Sprintf("account %q does not exist", e.Login) }

// Unwrap maps the error to db.ErrNotExist.
func (e ErrNotExist) Unwrap() error { return db.ErrNotExist }

// GetByLogin returns the account with this login.
func GetByLogin(ctx context.Context, login string) (*Account, error) {
	a := &Account{}
	has, err := db.GetEngine(ctx).Where("login = ?", login).Get(a)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrNotExist{Login: login}
	}
	return a, nil
}

// Create inserts an account.
func Create(ctx context.Context, login string) (*Account, error) {
	a := &Account{Login: login}
	if _, err := db.GetEngine(ctx).Insert(a); err != nil {
		return nil, err
	}
	return a, nil
}

// Count returns the number of accounts.
func Count(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).Count(new(Account))
}

// SetLogin replaces the login of an account after its role was renamed.
func SetLogin(ctx context.Context, a *Account, login string) error {
	a.Login, a.RotatedUnix = login, time.Now().Unix()
	_, err := db.GetEngine(ctx).ID(a.ID).Cols("login", "rotated_unix").Update(a)
	return err
}

// Grants returns the account's write grants by database name.
func Grants(ctx context.Context, accountID int64) (map[string]*WriteGrant, error) {
	var rows []*WriteGrant
	if err := db.GetEngine(ctx).Where("account_id = ?", accountID).Find(&rows); err != nil {
		return nil, err
	}
	out := make(map[string]*WriteGrant, len(rows))
	for _, g := range rows {
		out[g.DBName] = g
	}
	return out, nil
}

// SetGrant creates the grant or moves its expiry.
func SetGrant(ctx context.Context, accountID int64, dbName string, expires time.Time) (*WriteGrant, error) {
	var g *WriteGrant
	err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := DeleteGrant(ctx, accountID, dbName); err != nil {
			return err
		}
		g = &WriteGrant{AccountID: accountID, DBName: dbName, ExpiresUnix: expires.Unix()}
		_, err := db.GetEngine(ctx).Insert(g)
		return err
	})
	return g, err
}

// DeleteGrant removes a grant; a missing one is not an error.
func DeleteGrant(ctx context.Context, accountID int64, dbName string) error {
	_, err := db.GetEngine(ctx).Where("account_id = ? AND db_name = ?", accountID, dbName).Delete(new(WriteGrant))
	return err
}

// ExpiredGrants returns the grants due at now.
func ExpiredGrants(ctx context.Context, now time.Time) ([]ExpiredGrant, error) {
	var rows []ExpiredGrant
	err := db.GetEngine(ctx).Table("write_grant").
		Join("INNER", "account", "account.id = write_grant.account_id").
		Where("write_grant.expires_unix <= ?", now.Unix()).
		Cols("write_grant.account_id", "account.login", "write_grant.db_name").
		Find(&rows)
	return rows, err
}
