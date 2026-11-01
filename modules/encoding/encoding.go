// Package encoding compresses HTML responses. The page has to fit the first flight of
// TCP packets with htmx inside it, and only brotli gets it that small.
package encoding

import (
	"bytes"
	"compress/gzip"
	"strings"

	"github.com/andybalholm/brotli"
)

const (
	// QualityMax is for bodies compressed once and cached: slow, but the smallest.
	QualityMax = 11
	// QualityRequest is for bodies compressed on every request: within the CPU limit
	// of the pod it stays fast, and costs a few hundred bytes against QualityMax.
	QualityRequest = 10

	brotliWindow = 22
)

// Negotiate picks the best encoding the client accepts: "br", "gzip" or "".
func Negotiate(acceptEncoding string) string {
	var gz bool
	for part := range strings.SplitSeq(acceptEncoding, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.ReplaceAll(params, " ", "") == "q=0" {
			continue
		}
		switch strings.TrimSpace(name) {
		case "br":
			return "br"
		case "gzip":
			gz = true
		}
	}
	if gz {
		return "gzip"
	}
	return ""
}

// Encode compresses body. An unknown or empty encoding returns it unchanged.
func Encode(body []byte, enc string, quality int) ([]byte, error) {
	var buf bytes.Buffer
	switch enc {
	case "br":
		w := brotli.NewWriterOptions(&buf, brotli.WriterOptions{Quality: quality, LGWin: brotliWindow})
		if _, err := w.Write(body); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
	case "gzip":
		w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(body); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
	default:
		return body, nil
	}
	return buf.Bytes(), nil
}
