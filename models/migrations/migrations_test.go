package migrations

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
	"xorm.io/xorm"
)

func TestMigrateIsIdempotent(t *testing.T) {
	x, err := xorm.NewEngine("sqlite", "file:"+filepath.Join(t.TempDir(), "m.db"))
	require.NoError(t, err)
	defer x.Close()

	require.NoError(t, Migrate(t.Context(), x))
	require.NoError(t, Migrate(t.Context(), x))

	v := &Version{}
	has, err := x.ID(1).Get(v)
	require.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, ExpectedVersion(), v.Version)
}
