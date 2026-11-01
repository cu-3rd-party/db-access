package access

import (
	"context"
	"errors"
	"testing"
	"time"

	account_model "git.cu3rd.ru/cu3rd/db-access/models/account"
	"git.cu3rd.ru/cu3rd/db-access/models/unittest"
	"git.cu3rd.ru/cu3rd/db-access/modules/setting"
	"git.cu3rd.ru/cu3rd/db-access/services/provision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { unittest.MainTest(m) }

func prepare(t *testing.T) *provision.Fake {
	t.Helper()
	unittest.PrepareTestDatabase(t)
	ResetLimits()
	fake := provision.NewFake()
	SetProvisioner(fake)
	return fake
}

func TestEnsureShowsPasswordOnce(t *testing.T) {
	fake := prepare(t)
	first, err := Ensure(t.Context(), "newLogin", "1.1.1.1")
	require.NoError(t, err)
	assert.Len(t, first.Password, 32)
	assert.NotContains(t, fake.Roles["newLogin"], first.Password)

	second, err := Ensure(t.Context(), "newLogin", "1.1.1.1")
	require.NoError(t, err)
	assert.Empty(t, second.Password)
	assert.Equal(t, first.Account.ID, second.Account.ID)
}

func TestEnsureReportsRoleConflict(t *testing.T) {
	fake := prepare(t)
	fake.Roles["postgres"] = "taken"
	_, err := Ensure(t.Context(), "postgres", "1.1.1.1")
	require.ErrorIs(t, err, provision.ErrRoleExists)
	unittest.AssertNotExistsBean(t, &account_model.Account{Login: "postgres"})
}

func TestEnsureLimits(t *testing.T) {
	prepare(t)
	t.Cleanup(func() { setting.CredentialsPerIPHour, setting.MaxAccounts = 300, 5000 })

	setting.CredentialsPerIPHour = 1
	_, err := Ensure(t.Context(), "aaaaaaa1", "1.1.1.1")
	require.NoError(t, err)
	_, err = Ensure(t.Context(), "aaaaaaa2", "1.1.1.1")
	require.ErrorIs(t, err, ErrRateLimited)
	_, err = Ensure(t.Context(), "aaaaaaa2", "2.2.2.2")
	require.NoError(t, err)

	setting.MaxAccounts = 3
	_, err = Ensure(t.Context(), "aaaaaaa3", "3.3.3.3")
	require.ErrorIs(t, err, ErrCapacity)
}

func TestRotateMovesRoleAndWrite(t *testing.T) {
	fake := prepare(t)
	first, err := Ensure(t.Context(), "oldLogin", "1.1.1.1")
	require.NoError(t, err)
	_, err = SetWrite(t.Context(), first.Account, "cu-db", true)
	require.NoError(t, err)
	oldVerifier := fake.Roles["oldLogin"]

	rotated, err := Rotate(t.Context(), first.Account, "newLogin", "1.1.1.1")
	require.NoError(t, err)
	assert.Equal(t, "newLogin", rotated.Account.Login)
	assert.NotEqual(t, first.Password, rotated.Password)
	assert.NotContains(t, fake.Roles, "oldLogin")
	assert.NotEqual(t, oldVerifier, fake.Roles["newLogin"])
	assert.True(t, fake.Writes["newLogin/cu-db"])
}

func TestSetWriteActsOnOneDatabase(t *testing.T) {
	fake := prepare(t)
	c, err := Ensure(t.Context(), "someUser", "1.1.1.1")
	require.NoError(t, err)

	start := time.Now()
	grant, err := SetWrite(t.Context(), c.Account, "cu-sql", true)
	require.NoError(t, err)
	assert.True(t, fake.Writes["someUser/cu-sql"])
	assert.False(t, fake.Writes["someUser/cu-db"])
	assert.InDelta(t, start.Add(24*time.Hour).Unix(), grant.ExpiresUnix, 2)

	_, err = SetWrite(t.Context(), c.Account, "cu-sql", false)
	require.NoError(t, err)
	assert.False(t, fake.Writes["someUser/cu-sql"])
	unittest.AssertNotExistsBean(t, &account_model.WriteGrant{AccountID: c.Account.ID})

	_, err = SetWrite(t.Context(), c.Account, "postgres", true)
	require.ErrorIs(t, err, ErrUnknownDatabase)
}

func TestExpireWrites(t *testing.T) {
	fake := prepare(t)
	c, err := Ensure(t.Context(), "someUser", "1.1.1.1")
	require.NoError(t, err)
	_, err = SetWrite(t.Context(), c.Account, "cu-db", true)
	require.NoError(t, err)

	require.NoError(t, ExpireWrites(t.Context(), time.Now().Add(23*time.Hour)))
	assert.True(t, fake.Writes["someUser/cu-db"])

	require.NoError(t, ExpireWrites(t.Context(), time.Now().Add(25*time.Hour)))
	assert.False(t, fake.Writes["someUser/cu-db"])
	unittest.AssertNotExistsBean(t, &account_model.WriteGrant{AccountID: c.Account.ID})
}

type failingRevoke struct{ *provision.Fake }

func (failingRevoke) RevokeWrite(context.Context, string, string) error { return errors.New("down") }

func TestExpireWritesRetriesAfterFailure(t *testing.T) {
	fake := prepare(t)
	c, err := Ensure(t.Context(), "someUser", "1.1.1.1")
	require.NoError(t, err)
	_, err = SetWrite(t.Context(), c.Account, "cu-db", true)
	require.NoError(t, err)

	SetProvisioner(failingRevoke{fake})
	require.Error(t, ExpireWrites(t.Context(), time.Now().Add(25*time.Hour)))
	unittest.AssertExistsAndLoadBean(t, &account_model.WriteGrant{AccountID: c.Account.ID})

	SetProvisioner(fake)
	require.NoError(t, ExpireWrites(t.Context(), time.Now().Add(25*time.Hour)))
	assert.False(t, fake.Writes["someUser/cu-db"])
}
