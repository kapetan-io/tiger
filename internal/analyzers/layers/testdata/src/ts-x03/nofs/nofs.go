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
	_, _ = filepath.Glob("*")                // want `the "\./nofs" entry in tiger\.layers\.yaml doesn't list os, so this package can't call path/filepath\.Glob — add "os" to it, or take an fs\.FS or the data as an input`
	_ = filepath.Walk(".", nil)              // want `can't call path/filepath\.Walk `
	_ = filepath.WalkDir(".", nil)           // want `can't call path/filepath\.WalkDir `
	_, _ = filepath.Abs(".")                 // want `can't call path/filepath\.Abs `
	_, _ = filepath.EvalSymlinks(".")        // want `can't call path/filepath\.EvalSymlinks `
	_, _ = template.ParseFiles("a")          // want `can't call html/template\.ParseFiles `
	_, _ = template.ParseGlob("*")           // want `can't call html/template\.ParseGlob `
	_, _ = text.ParseFiles("a")              // want `can't call text/template\.ParseFiles `
	_, _ = text.ParseGlob("*")               // want `can't call text/template\.ParseGlob `
	_, _ = template.New("a").ParseFiles("a") // want `can't call html/template\.Template\.ParseFiles `
	_, _ = text.New("a").ParseGlob("*")      // want `can't call text/template\.Template\.ParseGlob `
	walk := filepath.WalkDir                 // want `can't call path/filepath\.WalkDir `
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
