//go:build bindata

package main

import (
	"embed"

	"git.cu3rd.ru/cu3rd/db-access/modules/assets"
)

//go:embed templates public
var bindata embed.FS

func init() {
	assets.SetEmbedded(bindata)
}
