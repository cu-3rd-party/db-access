// Package assets gives access to templates/ and public/. Development builds
// read them from disk (setting.StaticRoot) so edits are picked up without a
// rebuild; `TAGS=bindata make build` embeds them into the binary (see
// /bindata.go).
package assets

import (
	"io/fs"
	"os"
	"path/filepath"

	"git.cu3rd.ru/cu3rd/db-access/modules/setting"
)

var embedded fs.FS

// SetEmbedded installs the embedded file system (called from bindata.go).
func SetEmbedded(f fs.FS) { embedded = f }

// IsEmbedded reports whether assets are compiled into the binary.
func IsEmbedded() bool { return embedded != nil }

// Templates returns the templates/ directory.
func Templates() fs.FS { return sub("templates") }

// Public returns the public/ directory (webpack output).
func Public() fs.FS { return sub("public") }

func sub(dir string) fs.FS {
	if embedded != nil {
		f, err := fs.Sub(embedded, dir)
		if err != nil {
			panic(err)
		}
		return f
	}
	return os.DirFS(filepath.Join(setting.StaticRoot, dir))
}
