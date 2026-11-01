package secret

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPassword(t *testing.T) {
	a, b := NewPassword(), NewPassword()
	assert.Regexp(t, `^[a-zA-Z0-9]{32}$`, a)
	assert.NotEqual(t, a, b)
}

func TestVerifierShapeAndSalt(t *testing.T) {
	a, err := Verifier("pw")
	require.NoError(t, err)
	b, err := Verifier("pw")
	require.NoError(t, err)
	assert.Regexp(t, `^SCRAM-SHA-256\$4096:[A-Za-z0-9+/=]+\$[A-Za-z0-9+/=]+:[A-Za-z0-9+/=]+$`, a)
	assert.NotEqual(t, a, b)
}
