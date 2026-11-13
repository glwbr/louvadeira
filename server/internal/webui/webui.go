package webui

import (
	"embed"
	"io/fs"
)

// NOTE: dist/ is filled by `mise run web:build`; only .gitkeep is committed.

//go:embed all:dist
var embedded embed.FS

func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
