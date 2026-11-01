// Package secret generates database passwords and their SCRAM-SHA-256 verifiers, so
// that the plaintext password never reaches the database server or its logs.
package secret

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"git.cu3rd.ru/cu3rd/db-access/modules/identity"
)

const (
	passwordLength  = 32
	scramIterations = 4096
)

// NewPassword returns a random password that needs no escaping in a connection URL.
func NewPassword() string { return identity.Random(passwordLength) }

// Verifier returns the rolpassword value for password in the format of RFC 5803.
func Verifier(password string) (string, error) {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt) // crypto/rand.Read never fails
	salted, err := pbkdf2.Key(sha256.New, password, salt, scramIterations, sha256.Size)
	if err != nil {
		return "", err
	}
	clientKey := mac(salted, "Client Key")
	storedKey := sha256.Sum256(clientKey)
	serverKey := mac(salted, "Server Key")
	enc := base64.StdEncoding.EncodeToString
	return fmt.Sprintf("SCRAM-SHA-256$%d:%s$%s:%s", scramIterations, enc(salt), enc(storedKey[:]), enc(serverKey)), nil
}

func mac(key []byte, msg string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(msg))
	return h.Sum(nil)
}
