package account

import (
	"testing"
	"time"

	"git.cu3rd.ru/cu3rd/db-access/models/db"
	"git.cu3rd.ru/cu3rd/db-access/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetByLogin(t *testing.T) {
	unittest.PrepareTestDatabase(t)
	a, err := GetByLogin(t.Context(), "fixtureA")
	require.NoError(t, err)
	assert.EqualValues(t, 1, a.ID)

	_, err = GetByLogin(t.Context(), "nobody00")
	assert.ErrorIs(t, err, db.ErrNotExist)
}

func TestSetLoginKeepsGrants(t *testing.T) {
	unittest.PrepareTestDatabase(t)
	a, err := GetByLogin(t.Context(), "fixtureA")
	require.NoError(t, err)
	require.NoError(t, SetLogin(t.Context(), a, "renamed0"))

	b, err := GetByLogin(t.Context(), "renamed0")
	require.NoError(t, err)
	assert.Equal(t, a.ID, b.ID)
	assert.NotZero(t, b.RotatedUnix)

	grants, err := Grants(t.Context(), b.ID)
	require.NoError(t, err)
	assert.Contains(t, grants, "cu-sql")
}

func TestSetGrantMovesExpiry(t *testing.T) {
	unittest.PrepareTestDatabase(t)
	at := time.Unix(2000000000, 0)
	_, err := SetGrant(t.Context(), 1, "cu-sql", at)
	require.NoError(t, err)
	grants, err := Grants(t.Context(), 1)
	require.NoError(t, err)
	assert.Equal(t, at.Unix(), grants["cu-sql"].ExpiresUnix)
	unittest.AssertCount(t, &WriteGrant{AccountID: 1}, 1)
}

func TestExpiredGrants(t *testing.T) {
	unittest.PrepareTestDatabase(t)
	rows, err := ExpiredGrants(t.Context(), time.Unix(1000, 0))
	require.NoError(t, err)
	assert.Empty(t, rows)

	// the fixture grant expires at unix time 1700000000
	rows, err = ExpiredGrants(t.Context(), time.Unix(1700000000, 0))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, ExpiredGrant{AccountID: 1, Login: "fixtureA", DBName: "cu-sql"}, rows[0])
}
