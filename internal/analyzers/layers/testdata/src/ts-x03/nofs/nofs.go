// Package nofs has an empty entry, so none of the standard library functions
// that read the file system or the working directory may be called.
package nofs

import (
	"html/template"
	"io/fs"
	"path/filepath"
	text "text/template"
)

func Reads() {
	_, _ = filepath.Glob("*")                // want `the "\./nofs" entry in tiger\.layers\.yaml doesn't list os, so this package can't use path/filepath\.Glob — add "os" to it, or take an fs\.FS or the data as an input`
	_ = filepath.Walk(".", nil)              // want `can't use path/filepath\.Walk `
	_ = filepath.WalkDir(".", nil)           // want `can't use path/filepath\.WalkDir `
	_, _ = filepath.Abs(".")                 // want `can't use path/filepath\.Abs `
	_, _ = filepath.EvalSymlinks(".")        // want `can't use path/filepath\.EvalSymlinks `
	_, _ = template.ParseFiles("a")          // want `can't use html/template\.ParseFiles `
	_, _ = template.ParseGlob("*")           // want `can't use html/template\.ParseGlob `
	_, _ = text.ParseFiles("a")              // want `can't use text/template\.ParseFiles `
	_, _ = text.ParseGlob("*")               // want `can't use text/template\.ParseGlob `
	_, _ = template.New("a").ParseFiles("a") // want `can't use html/template\.Template\.ParseFiles `
	_, _ = text.New("a").ParseGlob("*")      // want `can't use text/template\.Template\.ParseGlob `
	walk := filepath.WalkDir                 // want `can't use path/filepath\.WalkDir `
	_ = walk
}

// Pure path functions and the fs.FS forms stay allowed.
func Pure(fsys fs.FS) {
	_ = filepath.Join("a", "b")
	_ = filepath.Clean("a//b")
	_ = filepath.Base("a/b")
	_ = filepath.Ext("a.go")
	_, _ = fs.Glob(fsys, "*")
	_ = fs.WalkDir(fsys, ".", nil)
	_, _ = template.ParseFS(fsys, "*")
	_, _ = text.ParseFS(fsys, "*")
}
