package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewIsValidAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		id := New()
		assert.True(t, Valid(id), id)
		assert.False(t, seen[id])
		seen[id] = true
	}
}

func TestValidRejectsAnythingElse(t *testing.T) {
	for _, s := range []string{"", "abc", "abcdefghi", "abc-efgh", `abc"efgh`, "абвгдежз", "abcdefg\n"} {
		assert.False(t, Valid(s), s)
	}
}

func TestVerify(t *testing.T) {
	sig := Sign("secret", "aB3dE9xZ")
	assert.True(t, Verify("secret", "aB3dE9xZ", sig))
	assert.False(t, Verify("other", "aB3dE9xZ", sig))
	assert.False(t, Verify("secret", "aB3dE9xY", sig))
	assert.False(t, Verify("secret", "aB3dE9xZ", ""))
}
