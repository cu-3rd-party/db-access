// Package templates renders html/template files from templates/. A template
// is named by its path without the .tmpl suffix ("access/page").
package templates

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"strings"
	"sync"

	"git.cu3rd.ru/cu3rd/db-access/modules/assets"
)

// HTMLRenderer renders named templates.
type HTMLRenderer struct {
	mu   sync.Mutex
	tmpl *template.Template
}

var renderer = &HTMLRenderer{}

// Renderer returns the shared renderer.
func Renderer() *HTMLRenderer { return renderer }

func (r *HTMLRenderer) load() (*template.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// development builds re-parse on every render so template edits are live
	if r.tmpl != nil && assets.IsEmbedded() {
		return r.tmpl, nil
	}
	root := template.New("").Funcs(NewFuncMap())
	fsys := assets.Templates()
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return err
		}
		content, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		_, err = root.New(strings.TrimSuffix(path, ".tmpl")).Parse(string(content))
		return err
	})
	if err != nil {
		return nil, err
	}
	r.tmpl = root
	return root, nil
}

// HTML renders template name into w.
func (r *HTMLRenderer) HTML(w io.Writer, name string, data any) error {
	tmpl, err := r.load()
	if err != nil {
		return fmt.Errorf("load templates: %w", err)
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}
	return nil
}
