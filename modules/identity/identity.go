// Package identity issues the visitor identifier: 8 characters that are both the
// value of the long-lived cookie and the database login, plus an HMAC proving that
// this service issued the identifier.
package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
)

// Length is the number of characters in an identifier.
const Length = 8

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var valid = regexp.MustCompile(`^[a-zA-Z0-9]{8}$`)

// Random returns n uniformly random characters from a-z A-Z 0-9.
func Random(n int) string {
	out := make([]byte, 0, n)
	buf := make([]byte, n*2)
	for len(out) < n {
		_, _ = rand.Read(buf) // crypto/rand.Read never fails
		for _, b := range buf {
			// reject the top of the byte range to keep the distribution uniform
			if int(b) < 256-256%len(alphabet) && len(out) < n {
				out = append(out, alphabet[int(b)%len(alphabet)])
			}
		}
	}
	return string(out)
}

// New returns a fresh identifier.
func New() string { return Random(Length) }

// Valid reports whether s has the shape of an identifier.
func Valid(s string) bool { return valid.MatchString(s) }

// Sign returns the signature of login.
func Sign(secret, login string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(login))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify reports whether login is well-formed and sig is its signature.
func Verify(secret, login, sig string) bool {
	return Valid(login) && hmac.Equal([]byte(Sign(secret, login)), []byte(sig))
}
