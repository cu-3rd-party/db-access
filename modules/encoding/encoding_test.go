package encoding

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNegotiate(t *testing.T) {
	for accept, want := range map[string]string{
		"":                  "",
		"identity":          "",
		"gzip":              "gzip",
		"gzip, deflate, br": "br",
		"br;q=0, gzip":      "gzip",
		"br; q=0":           "",
		"br;q=0.5":          "br",
	} {
		assert.Equal(t, want, Negotiate(accept), accept)
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	body := []byte(strings.Repeat("<div>доступ</div>", 100))

	br, err := Encode(body, "br", QualityRequest)
	require.NoError(t, err)
	assert.Less(t, len(br), len(body))
	out, err := io.ReadAll(brotli.NewReader(bytes.NewReader(br)))
	require.NoError(t, err)
	assert.Equal(t, body, out)

	gz, err := Encode(body, "gzip", QualityRequest)
	require.NoError(t, err)
	r, err := gzip.NewReader(bytes.NewReader(gz))
	require.NoError(t, err)
	out, err = io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, body, out)

	plain, err := Encode(body, "", QualityRequest)
	require.NoError(t, err)
	assert.Equal(t, body, plain)
}
